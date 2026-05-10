package knowledge

import (
	"context"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// KnowledgePromptContributor injects relevant knowledge base chunks
// into the agent's context layer before each LLM call.
type KnowledgePromptContributor struct {
	store      *Store
	maxResults int
}

// NewKnowledgePromptContributor creates a contributor that queries the store
// for relevant chunks and injects them into PromptLayerContext/PromptSlotMemory.
func NewKnowledgePromptContributor(store *Store, maxResults int) *KnowledgePromptContributor {
	if maxResults <= 0 {
		maxResults = 5
	}
	return &KnowledgePromptContributor{
		store:      store,
		maxResults: maxResults,
	}
}

// PromptSource returns the descriptor for this contributor.
func (c *KnowledgePromptContributor) PromptSource() agent.PromptSourceDescriptor {
	return agent.PromptSourceDescriptor{
		ID:          "knowledge_base:context",
		Owner:       "knowledge_base",
		Description: "Relevant knowledge base excerpts retrieved for the current query",
		Allowed: []agent.PromptPlacement{
			{Layer: agent.PromptLayerContext, Slot: agent.PromptSlotMemory},
		},
		StableByDefault: false,
	}
}

// ContributePrompt searches the knowledge base for chunks relevant to the
// current user message and returns them as a formatted context block.
func (c *KnowledgePromptContributor) ContributePrompt(
	ctx context.Context,
	req agent.PromptBuildRequest,
) ([]agent.PromptPart, error) {
	if c.store == nil {
		return nil, nil
	}

	query := strings.TrimSpace(req.CurrentMessage)
	if query == "" {
		return nil, nil
	}

	results, err := c.store.Search(ctx, query, c.maxResults)
	if err != nil {
		logger.WarnCF("knowledge", "search failed", map[string]any{"error": err.Error()})
		return nil, nil
	}
	if len(results) == 0 {
		return nil, nil
	}

	content := formatKnowledgeContext(results)
	if content == "" {
		return nil, nil
	}

	part := agent.PromptPart{
		ID:    "knowledge_base:retrieved",
		Layer: agent.PromptLayerContext,
		Slot:  agent.PromptSlotMemory,
		Source: agent.PromptSource{
			ID:   "knowledge_base:context",
			Name: "Knowledge Base",
		},
		Title:   "Relevant Knowledge",
		Content: content,
		Stable:  false,
	}

	return []agent.PromptPart{part}, nil
}

// formatKnowledgeContext formats search results into a readable context block.
func formatKnowledgeContext(results []SearchResult) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Knowledge Base Context\n\n")
	sb.WriteString("The following excerpts from the knowledge base may be relevant:\n\n")

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("### [%d] %s\n\n", i+1, r.DocName))
		sb.WriteString(r.Chunk.Content)
		sb.WriteString("\n\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("*Use the above context to inform your response when relevant.*\n")

	return sb.String()
}
