package api

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/sipeed/picoclaw/pkg/analytics"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/memory"
	"github.com/sipeed/picoclaw/web/backend/utils"
)

// registerAnalyticsRoutes binds analytics endpoints to the mux.
func (h *Handler) registerAnalyticsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/analytics/summary", h.handleAnalyticsSummary)
}

// handleAnalyticsSummary returns aggregated usage stats.
//
//	GET /api/analytics/summary
func (h *Handler) handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Turn stats from analytics SQLite
	analyticsDir := filepath.Join(utils.GetPicoclawHome(), "analytics")
	store, err := analytics.NewStore(analyticsDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	defer store.Close()

	turnSummary, err := store.GetSummary(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	recentTurns, _ := store.RecentTurns(ctx, 20)

	// Session stats from JSONL store
	sessSummary := buildSessionStats(ctx, h.configPath)

	writeJSON(w, http.StatusOK, map[string]any{
		"turns":        turnSummary,
		"recent_turns": recentTurns,
		"sessions":     sessSummary,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// sessionSummary holds aggregated session-level metrics.
type sessionSummary struct {
	Total          int `json:"total"`
	TotalMessages  int `json:"total_messages"`
	ActiveToday    int `json:"active_today"`
	ActiveThisWeek int `json:"active_this_week"`
}

// buildSessionStats scans the JSONL session store for aggregate counts.
func buildSessionStats(ctx context.Context, configPath string) sessionSummary {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return sessionSummary{}
	}

	workspace := cfg.Agents.Defaults.Workspace
	if workspace == "" {
		workspace = filepath.Join(config.GetHome(), "workspace")
	}
	if len(workspace) >= 2 && workspace[:2] == "~/" {
		workspace = filepath.Join(config.GetHome(), workspace[2:])
	}

	sessionsDir := filepath.Join(workspace, "sessions")
	store, err := memory.NewJSONLStore(sessionsDir)
	if err != nil {
		return sessionSummary{}
	}
	defer store.Close()

	sessions := store.ListSessions()
	stats := sessionSummary{Total: len(sessions)}

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	weekAgo := today.AddDate(0, 0, -7)

	for _, key := range sessions {
		meta, err := store.GetSessionMeta(ctx, key)
		if err != nil {
			continue
		}
		stats.TotalMessages += meta.Count
		if meta.UpdatedAt.After(today) {
			stats.ActiveToday++
		}
		if meta.UpdatedAt.After(weekAgo) {
			stats.ActiveThisWeek++
		}
	}

	return stats
}
