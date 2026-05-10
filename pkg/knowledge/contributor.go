package knowledge

import (
	"context"
	"fmt"
	"strings"
)

const (
	// maxKnowledgeResults is the maximum number of chunks injected per turn.
	// Keep low to avoid 413 errors from providers with small request limits.
	maxKnowledgeResults = 3

	// maxChunkInjectLen is the maximum characters per chunk injected into context.
	maxChunkInjectLen = 400

	// maxTotalKnowledgeLen is the hard cap on total injected knowledge text.
	maxTotalKnowledgeLen = 1500

	// promptLayerContext and promptSlotMemory mirror pkg/agent constants
	// to avoid an import cycle (agent → knowledge → agent).
	promptLayerContext = "context"
	promptSlotMemory   = "memory"
)

// PromptSourceDescriptor mirrors agent.PromptSourceDescriptor.
type PromptSourceDescriptor struct {
	ID              string
	Owner           string
	Description     string
	Allowed         []PromptPlacement
	StableByDefault bool
}

// PromptPlacement mirrors agent.PromptPlacement.
type PromptPlacement struct {
	Layer string
	Slot  string
}

// PromptSource mirrors agent.PromptSource.
type PromptSource struct {
	ID   string
	Name string
}

// PromptPart mirrors agent.PromptPart.
type PromptPart struct {
	ID     string
	Layer  string
	Slot   string
	Source PromptSource
	Title  string
	Content string
	Stable bool
}

// PromptBuildRequest mirrors the fields of agent.PromptBuildRequest
// that the knowledge contributor needs.
type PromptBuildRequest struct {
	CurrentMessage string
}

// KnowledgePromptContributor injects relevant knowledge base chunks
// into the agent's context layer before each LLM call.
//
// It implements agent.PromptContributor via duck-typing — the agent
// package calls PromptSource() and ContributePrompt() by interface,
// so no direct import of pkg/agent is needed here.
type KnowledgePromptContributor struct {
	store      *Store
	maxResults int
}

// NewKnowledgePromptContributor creates a contributor that queries the store
// for relevant chunks and injects them into PromptLayerContext/PromptSlotMemory.
func NewKnowledgePromptContributor(store *Store, maxResults int) *KnowledgePromptContributor {
	if maxResults <= 0 || maxResults > maxKnowledgeResults {
		maxResults = maxKnowledgeResults
	}
	return &KnowledgePromptContributor{
		store:      store,
		maxResults: maxResults,
	}
}

// KnowledgePromptSource returns the descriptor for this contributor.
// Named differently to avoid collision with the agent interface method.
func (c *KnowledgePromptContributor) KnowledgePromptSource() PromptSourceDescriptor {
	return PromptSourceDescriptor{
		ID:          "knowledge_base:context",
		Owner:       "knowledge_base",
		Description: "Relevant knowledge base excerpts retrieved for the current query",
		Allowed: []PromptPlacement{
			{Layer: promptLayerContext, Slot: promptSlotMemory},
		},
		StableByDefault: false,
	}
}

// RetrieveContext searches the knowledge base for chunks relevant to the
// given query and returns a formatted context string.
// Returns empty string if no relevant chunks found.
func (c *KnowledgePromptContributor) RetrieveContext(ctx context.Context, query string) (string, error) {
	if c.store == nil {
		return "", nil
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return "", nil
	}

	results, err := c.store.Search(ctx, query, c.maxResults)
	if err != nil {
		return "", fmt.Errorf("knowledge search: %w", err)
	}
	if len(results) == 0 {
		return "", nil
	}

	return formatKnowledgeContext(results), nil
}

// formatKnowledgeContext formats search results into a compact context block.
// Each chunk is truncated to maxChunkInjectLen chars and total output is
// capped at maxTotalKnowledgeLen to avoid 413 errors from providers.
func formatKnowledgeContext(results []SearchResult) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Knowledge Base\n\n")

	totalLen := 0
	for i, r := range results {
		chunk := strings.TrimSpace(r.Chunk.Content)

		// Truncate individual chunk
		if len(chunk) > maxChunkInjectLen {
			chunk = chunk[:maxChunkInjectLen] + "…"
		}

		entry := fmt.Sprintf("[%d] %s\n%s\n\n", i+1, r.DocName, chunk)

		// Stop if adding this entry would exceed total cap
		if totalLen+len(entry) > maxTotalKnowledgeLen {
			break
		}

		sb.WriteString(entry)
		totalLen += len(entry)
	}

	return strings.TrimRight(sb.String(), "\n")
}
