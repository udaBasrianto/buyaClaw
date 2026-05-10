package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const modelDiscoveryTimeout = 5 * time.Second

// handleFetchAvailableModels fetches the list of models available from a
// provider's API endpoint. Used by the Add Model form to populate the
// Model Identifier dropdown automatically.
//
//	POST /api/models/available
//	Body: { "provider": "openai", "api_base": "https://...", "api_key": "sk-..." }
func (h *Handler) handleFetchAvailableModels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		APIBase  string `json:"api_base"`
		APIKey   string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return
	}

	provider := strings.TrimSpace(strings.ToLower(req.Provider))
	apiBase := strings.TrimRight(strings.TrimSpace(req.APIBase), "/")
	apiKey := strings.TrimSpace(req.APIKey)

	models, err := discoverModels(r.Context(), provider, apiBase, apiKey)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"models": []string{},
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"models": models,
	})
}

// discoverModels fetches available model IDs from a provider's API.
func discoverModels(ctx context.Context, provider, apiBase, apiKey string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, modelDiscoveryTimeout)
	defer cancel()

	switch provider {
	case "ollama":
		return discoverOllamaModels(ctx, apiBase)
	case "openai", "openai-compat", "openai_compat",
		"groq", "deepseek", "mistral", "together", "fireworks",
		"perplexity", "anyscale", "openrouter", "novita",
		"cerebras", "nvidia", "avian", "vivgrid", "moonshot",
		"qwen", "volcengine", "zhipu", "minimax", "longcat",
		"modelscope", "shengsuanyun", "lmstudio", "vllm",
		"azure", "bedrock":
		return discoverOpenAICompatibleModels(ctx, apiBase, apiKey)
	case "anthropic", "anthropic-messages", "anthropic_messages":
		return discoverAnthropicModels(ctx, apiBase, apiKey)
	default:
		// Try OpenAI-compatible as fallback for unknown providers
		if apiBase != "" {
			return discoverOpenAICompatibleModels(ctx, apiBase, apiKey)
		}
		return nil, fmt.Errorf("provider %q does not support automatic model discovery", provider)
	}
}

// discoverOpenAICompatibleModels fetches models from a GET /models endpoint.
func discoverOpenAICompatibleModels(ctx context.Context, apiBase, apiKey string) ([]string, error) {
	if apiBase == "" {
		return nil, fmt.Errorf("api_base is required")
	}

	url := apiBase + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("invalid API key or insufficient permissions")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// OpenAI format: { "data": [{ "id": "gpt-4" }, ...] }
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		// Some providers use "models" instead of "data"
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	seen := make(map[string]struct{})
	var models []string

	for _, m := range result.Data {
		id := strings.TrimSpace(m.ID)
		if id != "" {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				models = append(models, id)
			}
		}
	}
	for _, m := range result.Models {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			id = strings.TrimSpace(m.Name)
		}
		if id != "" {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				models = append(models, id)
			}
		}
	}

	sort.Strings(models)
	return models, nil
}

// discoverOllamaModels fetches models from Ollama's /api/tags endpoint.
func discoverOllamaModels(ctx context.Context, apiBase string) ([]string, error) {
	if apiBase == "" {
		apiBase = "http://localhost:11434"
	}
	// Ollama uses /api/tags, not /v1/models
	base := strings.TrimSuffix(apiBase, "/v1")
	base = strings.TrimSuffix(base, "/v1/")

	url := base + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Ollama format: { "models": [{ "name": "llama3:latest" }, ...] }
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	models := make([]string, 0, len(result.Models))
	for _, m := range result.Models {
		if name := strings.TrimSpace(m.Name); name != "" {
			models = append(models, name)
		}
	}
	sort.Strings(models)
	return models, nil
}

// discoverAnthropicModels fetches models from Anthropic's /v1/models endpoint.
func discoverAnthropicModels(ctx context.Context, apiBase, apiKey string) ([]string, error) {
	if apiBase == "" {
		apiBase = "https://api.anthropic.com"
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required for Anthropic")
	}

	url := strings.TrimRight(apiBase, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid API key")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Anthropic format: { "data": [{ "id": "claude-3-5-sonnet-20241022" }, ...] }
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	return models, nil
}
