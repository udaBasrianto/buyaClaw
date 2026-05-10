package agent

import (
	"context"

	"github.com/sipeed/picoclaw/pkg/knowledge"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// knowledgeContributorAdapter wraps knowledge.KnowledgePromptContributor
// and implements the agent.PromptContributor interface.
// This adapter lives in pkg/agent (not pkg/knowledge) to avoid the
// import cycle: agent → knowledge → agent.
type knowledgeContributorAdapter struct {
	inner *knowledge.KnowledgePromptContributor
}

// NewKnowledgeContributor wraps a KnowledgePromptContributor as a PromptContributor.
func NewKnowledgeContributor(c *knowledge.KnowledgePromptContributor) PromptContributor {
	return &knowledgeContributorAdapter{inner: c}
}

func (a *knowledgeContributorAdapter) PromptSource() PromptSourceDescriptor {
	return PromptSourceDescriptor{
		ID:          "knowledge_base:context",
		Owner:       "knowledge_base",
		Description: "Relevant knowledge base excerpts retrieved for the current query",
		Allowed: []PromptPlacement{
			{Layer: PromptLayerContext, Slot: PromptSlotMemory},
		},
		StableByDefault: false,
	}
}

func (a *knowledgeContributorAdapter) ContributePrompt(
	ctx context.Context,
	req PromptBuildRequest,
) ([]PromptPart, error) {
	query := req.CurrentMessage
	if query == "" {
		return nil, nil
	}

	content, err := a.inner.RetrieveContext(ctx, query)
	if err != nil {
		logger.WarnCF("knowledge", "retrieval failed", map[string]any{"error": err.Error()})
		return nil, nil
	}
	if content == "" {
		return nil, nil
	}

	return []PromptPart{{
		ID:    "knowledge_base:retrieved",
		Layer: PromptLayerContext,
		Slot:  PromptSlotMemory,
		Source: PromptSource{
			ID:   "knowledge_base:context",
			Name: "Knowledge Base",
		},
		Title:   "Relevant Knowledge",
		Content: content,
		Stable:  false,
	}}, nil
}
