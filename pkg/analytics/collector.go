package analytics

import (
	"context"
	"strings"
	"time"

	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// Collector subscribes to the runtime event bus and writes analytics
// records to the Store whenever a turn completes.
type Collector struct {
	store *Store
	sub   runtimeevents.Subscription
}

// NewCollector creates a Collector and subscribes to the event bus.
// Call Close() to unsubscribe when done.
func NewCollector(ctx context.Context, bus runtimeevents.Bus, store *Store) (*Collector, error) {
	sub, err := bus.Channel().
		OfKind(runtimeevents.KindAgentTurnEnd).
		Subscribe(ctx, runtimeevents.SubscribeOptions{
			Name:         "analytics-collector",
			Buffer:       64,
			Concurrency:  runtimeevents.Locked,
			Backpressure: runtimeevents.DropOldest,
		}, func(ctx context.Context, evt runtimeevents.Event) error {
			return handleTurnEnd(ctx, store, evt)
		})
	if err != nil {
		return nil, err
	}
	return &Collector{store: store, sub: sub}, nil
}

// Close unsubscribes from the event bus.
func (c *Collector) Close() {
	if c.sub != nil {
		_ = c.sub.Close()
	}
}

// handleTurnEnd extracts turn data from an agent.turn.end event and persists it.
func handleTurnEnd(ctx context.Context, store *Store, evt runtimeevents.Event) error {
	// Extract scope fields
	channel := strings.TrimSpace(evt.Scope.Channel)
	if channel == "" {
		channel = "unknown"
	}
	agentID := strings.TrimSpace(evt.Scope.AgentID)

	// Extract payload fields via type assertion or map
	var status string
	var durationMS int64
	var iterations int
	var model string

	switch p := evt.Payload.(type) {
	case map[string]any:
		if v, ok := p["status"].(string); ok {
			status = v
		}
		if v, ok := p["duration_ms"].(float64); ok {
			durationMS = int64(v)
		}
		if v, ok := p["iterations_total"].(float64); ok {
			iterations = int(v)
		}
	default:
		// Try attrs fallback
		if evt.Attrs != nil {
			if v, ok := evt.Attrs["status"].(string); ok {
				status = v
			}
		}
	}

	if status == "" {
		status = "completed"
	}

	// Try to get model from attrs (set by agent loop)
	if evt.Attrs != nil {
		if v, ok := evt.Attrs["model"].(string); ok {
			model = v
		}
	}

	rec := TurnRecord{
		Timestamp:  evt.Time,
		AgentID:    agentID,
		Channel:    channel,
		Model:      model,
		Status:     status,
		DurationMS: durationMS,
		Iterations: iterations,
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now()
	}

	if err := store.RecordTurn(ctx, rec); err != nil {
		logger.WarnCF("analytics", "failed to record turn", map[string]any{
			"error": err.Error(),
		})
	}
	return nil
}
