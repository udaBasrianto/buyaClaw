// Package analytics provides lightweight usage statistics for the dashboard.
// It subscribes to the runtime event bus and persists aggregated metrics
// to a SQLite database so the dashboard can query them without scanning
// all session files on every request.
package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS daily_stats (
    date        TEXT NOT NULL,          -- YYYY-MM-DD
    metric      TEXT NOT NULL,          -- e.g. "turns", "messages", "errors"
    dimension   TEXT NOT NULL DEFAULT '',-- e.g. channel name, model name
    value       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (date, metric, dimension)
);

CREATE TABLE IF NOT EXISTS turn_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    agent_id    TEXT,
    channel     TEXT,
    model       TEXT,
    status      TEXT,
    duration_ms INTEGER,
    iterations  INTEGER,
    tokens_in   INTEGER DEFAULT 0,
    tokens_out  INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS turn_log_ts   ON turn_log(ts);
CREATE INDEX IF NOT EXISTS turn_log_chan ON turn_log(channel);
CREATE INDEX IF NOT EXISTS turn_log_mod  ON turn_log(model);
`

// Store persists analytics data to SQLite.
type Store struct {
	db  *sql.DB
	mu  sync.Mutex
}

// NewStore opens (or creates) the analytics SQLite database at dir/analytics.db.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("analytics: create dir: %w", err)
	}
	dbPath := filepath.Join(dir, "analytics.db")
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("analytics: open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("analytics: init schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// RecordTurn inserts a completed turn into the log and increments daily counters.
func (s *Store) RecordTurn(ctx context.Context, t TurnRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO turn_log (ts, agent_id, channel, model, status, duration_ms, iterations, tokens_in, tokens_out)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Timestamp, t.AgentID, t.Channel, t.Model, t.Status,
		t.DurationMS, t.Iterations, t.TokensIn, t.TokensOut,
	)
	if err != nil {
		return fmt.Errorf("analytics: insert turn: %w", err)
	}

	date := t.Timestamp.Format("2006-01-02")
	increments := []struct{ metric, dim string }{
		{"turns", ""},
		{"turns_by_channel", t.Channel},
		{"turns_by_model", t.Model},
		{"turns_by_status", t.Status},
	}
	if t.Status == "error" {
		increments = append(increments, struct{ metric, dim string }{"errors", ""})
	}
	for _, inc := range increments {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO daily_stats (date, metric, dimension, value) VALUES (?, ?, ?, 1)
			ON CONFLICT(date, metric, dimension) DO UPDATE SET value = value + 1`,
			date, inc.metric, inc.dim,
		)
		if err != nil {
			return fmt.Errorf("analytics: increment %s: %w", inc.metric, err)
		}
	}

	// Token counters
	if t.TokensIn > 0 || t.TokensOut > 0 {
		for _, row := range []struct {
			metric string
			val    int
		}{
			{"tokens_in", t.TokensIn},
			{"tokens_out", t.TokensOut},
			{"tokens_in_by_model_" + t.Model, t.TokensIn},
			{"tokens_out_by_model_" + t.Model, t.TokensOut},
		} {
			if row.val == 0 {
				continue
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO daily_stats (date, metric, dimension, value) VALUES (?, ?, '', ?)
				ON CONFLICT(date, metric, dimension) DO UPDATE SET value = value + ?`,
				date, row.metric, row.val, row.val,
			)
			if err != nil {
				return fmt.Errorf("analytics: increment tokens: %w", err)
			}
		}
	}

	return tx.Commit()
}

// Summary holds aggregated stats for the dashboard.
type Summary struct {
	TotalTurns     int            `json:"total_turns"`
	TurnsToday     int            `json:"turns_today"`
	TurnsThisWeek  int            `json:"turns_this_week"`
	ErrorsToday    int            `json:"errors_today"`
	TokensInTotal  int            `json:"tokens_in_total"`
	TokensOutTotal int            `json:"tokens_out_total"`
	ByChannel      map[string]int `json:"by_channel"`
	ByModel        map[string]int `json:"by_model"`
	ByStatus       map[string]int `json:"by_status"`
	DailyTurns     []DailyCount   `json:"daily_turns"`      // last 30 days
}

// DailyCount is a date + count pair for charts.
type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// TurnRecord holds data for one completed agent turn.
type TurnRecord struct {
	Timestamp  time.Time
	AgentID    string
	Channel    string
	Model      string
	Status     string
	DurationMS int64
	Iterations int
	TokensIn   int
	TokensOut  int
}

// GetSummary returns aggregated stats for the dashboard.
func (s *Store) GetSummary(ctx context.Context) (*Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sum := &Summary{
		ByChannel: make(map[string]int),
		ByModel:   make(map[string]int),
		ByStatus:  make(map[string]int),
	}

	today := time.Now().Format("2006-01-02")
	weekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")

	// Total turns
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(value),0) FROM daily_stats WHERE metric='turns'`).
		Scan(&sum.TotalTurns)

	// Turns today
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(value),0) FROM daily_stats WHERE metric='turns' AND date=?`, today).
		Scan(&sum.TurnsToday)

	// Turns this week
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(value),0) FROM daily_stats WHERE metric='turns' AND date>=?`, weekAgo).
		Scan(&sum.TurnsThisWeek)

	// Errors today
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(value),0) FROM daily_stats WHERE metric='errors' AND date=?`, today).
		Scan(&sum.ErrorsToday)

	// Total tokens
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(value),0) FROM daily_stats WHERE metric='tokens_in'`).
		Scan(&sum.TokensInTotal)
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(value),0) FROM daily_stats WHERE metric='tokens_out'`).
		Scan(&sum.TokensOutTotal)

	// By channel
	rows, err := s.db.QueryContext(ctx, `
		SELECT dimension, SUM(value) FROM daily_stats
		WHERE metric='turns_by_channel' AND dimension!=''
		GROUP BY dimension ORDER BY SUM(value) DESC LIMIT 10`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var dim string
			var val int
			if rows.Scan(&dim, &val) == nil && dim != "" {
				sum.ByChannel[dim] = val
			}
		}
	}

	// By model
	rows2, err := s.db.QueryContext(ctx, `
		SELECT dimension, SUM(value) FROM daily_stats
		WHERE metric='turns_by_model' AND dimension!=''
		GROUP BY dimension ORDER BY SUM(value) DESC LIMIT 10`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var dim string
			var val int
			if rows2.Scan(&dim, &val) == nil && dim != "" {
				sum.ByModel[dim] = val
			}
		}
	}

	// By status
	rows3, err := s.db.QueryContext(ctx, `
		SELECT dimension, SUM(value) FROM daily_stats
		WHERE metric='turns_by_status' AND dimension!=''
		GROUP BY dimension`)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var dim string
			var val int
			if rows3.Scan(&dim, &val) == nil && dim != "" {
				sum.ByStatus[dim] = val
			}
		}
	}

	// Daily turns last 30 days
	rows4, err := s.db.QueryContext(ctx, `
		SELECT date, SUM(value) FROM daily_stats
		WHERE metric='turns' AND date >= date('now','-30 days')
		GROUP BY date ORDER BY date ASC`)
	if err == nil {
		defer rows4.Close()
		for rows4.Next() {
			var dc DailyCount
			if rows4.Scan(&dc.Date, &dc.Count) == nil {
				sum.DailyTurns = append(sum.DailyTurns, dc)
			}
		}
	}

	return sum, nil
}

// RecentTurns returns the most recent N turns.
func (s *Store) RecentTurns(ctx context.Context, limit int) ([]TurnRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts, agent_id, channel, model, status, duration_ms, iterations, tokens_in, tokens_out
		FROM turn_log ORDER BY ts DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var turns []TurnRecord
	for rows.Next() {
		var t TurnRecord
		if err := rows.Scan(&t.Timestamp, &t.AgentID, &t.Channel, &t.Model,
			&t.Status, &t.DurationMS, &t.Iterations, &t.TokensIn, &t.TokensOut); err == nil {
			turns = append(turns, t)
		}
	}
	return turns, rows.Err()
}
