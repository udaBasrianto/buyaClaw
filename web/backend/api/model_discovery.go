package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const modelDiscoveryTimeout = 5 * time.Second

// ModelDiscoveryItem is a rich model entry returned by the discovery endpoint.
type ModelDiscoveryItem struct {
	ID             string `json:"id"`
	Name           string `json:"name,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Description    string `json:"description,omitempty"`
	ContextLen     int    `json:"context_length,omitempty"`
	PricePrompt    string `json:"price_prompt,omitempty"`    // price per 1M tokens input (USD)
	PriceOutput    string `json:"price_output,omitempty"`    // price per 1M tokens output (USD)
	IsFree         bool   `json:"is_free,omitempty"`
	ExpirationDate string `json:"expiration_date,omitempty"` // promo end date YYYY-MM-DD
}

// handleFetchAvailableModels fetches the list of models available from a
// provider's API endpoint. Used by the Add Model form to populate the
// Model Identifier table automatically.
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

	items, err := discoverModelItems(r.Context(), provider, apiBase, apiKey)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"models": []ModelDiscoveryItem{},
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"models": items,
	})
}

// discoverModelItems fetches rich model info from a provider's API.
func discoverModelItems(ctx context.Context, provider, apiBase, apiKey string) ([]ModelDiscoveryItem, error) {
	ctx, cancel := context.WithTimeout(ctx, modelDiscoveryTimeout)
	defer cancel()

	switch provider {
	case "ollama":
		return discoverOllamaModelItems(ctx, apiBase)
	case "anthropic", "anthropic-messages", "anthropic_messages":
		return discoverAnthropicModelItems(ctx, apiBase, apiKey)
	case "openai", "openai-compat", "openai_compat",
		"groq", "deepseek", "mistral", "together", "fireworks",
		"perplexity", "anyscale", "openrouter", "novita",
		"cerebras", "nvidia", "avian", "vivgrid", "moonshot",
		"qwen", "volcengine", "zhipu", "minimax", "longcat",
		"modelscope", "shengsuanyun", "lmstudio", "vllm",
		"azure", "bedrock":
		return discoverOpenAICompatibleModelItems(ctx, apiBase, apiKey)
	default:
		if apiBase != "" {
			return discoverOpenAICompatibleModelItems(ctx, apiBase, apiKey)
		}
		return nil, fmt.Errorf("provider %q does not support automatic model discovery", provider)
	}
}

// discoverOpenAICompatibleModelItems fetches rich model info from GET /models.
func discoverOpenAICompatibleModelItems(ctx context.Context, apiBase, apiKey string) ([]ModelDiscoveryItem, error) {
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// OpenRouter / OpenAI / Sumopod format:
	// { "data": [{ "id": "openai/gpt-4", "name": "GPT-4", "pricing": {...}, "expiration_date": "2026-06-01", ... }] }
	var raw struct {
		Data []struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			Description    string `json:"description"`
			ContextLen     int    `json:"context_length"`
			ExpirationDate string `json:"expiration_date"`
			Pricing        *struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	seen := make(map[string]struct{})
	var items []ModelDiscoveryItem

	for _, m := range raw.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		item := ModelDiscoveryItem{
			ID:             id,
			Name:           m.Name,
			Description:    truncate(m.Description, 120),
			ContextLen:     m.ContextLen,
			Provider:       extractProvider(id),
			ExpirationDate: m.ExpirationDate,
		}
		if m.Pricing != nil {
			item.PricePrompt = formatPricePer1M(m.Pricing.Prompt)
			item.PriceOutput = formatPricePer1M(m.Pricing.Completion)
			item.IsFree = m.Pricing.Prompt == "0" && m.Pricing.Completion == "0"
		}
		items = append(items, item)
	}

	// Fallback: plain models list
	for _, m := range raw.Models {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			id = strings.TrimSpace(m.Name)
		}
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		items = append(items, ModelDiscoveryItem{
			ID:       id,
			Name:     m.Name,
			Provider: extractProvider(id),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

// discoverOllamaModelItems fetches models from Ollama's /api/tags endpoint.
func discoverOllamaModelItems(ctx context.Context, apiBase string) ([]ModelDiscoveryItem, error) {
	if apiBase == "" {
		apiBase = "http://localhost:11434"
	}
	base := strings.TrimSuffix(strings.TrimSuffix(apiBase, "/v1/"), "/v1")

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

	var result struct {
		Models []struct {
			Name    string `json:"name"`
			Details *struct {
				Family string `json:"family"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	items := make([]ModelDiscoveryItem, 0, len(result.Models))
	for _, m := range result.Models {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			continue
		}
		item := ModelDiscoveryItem{
			ID:       name,
			Name:     name,
			Provider: "ollama",
			IsFree:   true,
		}
		if m.Details != nil && m.Details.Family != "" {
			item.Provider = m.Details.Family
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

// discoverAnthropicModelItems fetches models from Anthropic's /v1/models endpoint.
func discoverAnthropicModelItems(ctx context.Context, apiBase, apiKey string) ([]ModelDiscoveryItem, error) {
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

	var result struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	items := make([]ModelDiscoveryItem, 0, len(result.Data))
	for _, m := range result.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			items = append(items, ModelDiscoveryItem{
				ID:       id,
				Name:     m.DisplayName,
				Provider: "anthropic",
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

// extractProvider extracts the provider name from a model ID like "openai/gpt-4".
func extractProvider(id string) string {
	if idx := strings.Index(id, "/"); idx > 0 {
		return id[:idx]
	}
	return ""
}

// formatPricePer1M converts a per-token price string to a per-1M-token price string.
// Input: "0.000003" (per token) → Output: "$3.00"
func formatPricePer1M(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return "Free"
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || f == 0 {
		return "Free"
	}
	per1M := f * 1_000_000
	if per1M < 0.01 {
		return fmt.Sprintf("$%.4f", per1M)
	}
	return fmt.Sprintf("$%.2f", per1M)
}

// truncate shortens a string to maxLen characters, adding "…" if truncated.
func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
