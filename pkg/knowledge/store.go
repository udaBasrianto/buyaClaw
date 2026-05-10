// Package knowledge implements a RAG (Retrieval Augmented Generation) knowledge base
// backed by SQLite with FTS5 full-text search.
package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS documents (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    filename    TEXT NOT NULL,
    mime_type   TEXT NOT NULL,
    size_bytes  INTEGER NOT NULL,
    chunk_count INTEGER DEFAULT 0,
    enabled     INTEGER DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chunks (
    id          TEXT PRIMARY KEY,
    doc_id      TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content     TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
    content,
    content='chunks',
    content_rowid='rowid',
    tokenize='porter unicode61'
);

CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
    INSERT INTO chunks_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
END;

CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, content) VALUES ('delete', old.rowid, old.content);
    INSERT INTO chunks_fts(rowid, content) VALUES (new.rowid, new.content);
END;
`

// Document represents an uploaded knowledge base document.
type Document struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Filename   string    `json:"filename"`
	MimeType   string    `json:"mime_type"`
	SizeBytes  int64     `json:"size_bytes"`
	ChunkCount int       `json:"chunk_count"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Chunk represents a text segment from a document.
type Chunk struct {
	ID         string `json:"id"`
	DocID      string `json:"doc_id"`
	ChunkIndex int    `json:"chunk_index"`
	Content    string `json:"content"`
	TokenCount int    `json:"token_count"`
}

// SearchResult is a chunk with its relevance score.
type SearchResult struct {
	Chunk    Chunk   `json:"chunk"`
	DocName  string  `json:"doc_name"`
	Score    float64 `json:"score"`
}

// Store manages the knowledge base SQLite database.
type Store struct {
	db  *sql.DB
	mu  sync.RWMutex
	dir string
}

// NewStore opens (or creates) the knowledge base SQLite database at dir/knowledge.db.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("knowledge: create dir: %w", err)
	}

	dbPath := filepath.Join(dir, "knowledge.db")
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("knowledge: open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("knowledge: init schema: %w", err)
	}

	return &Store{db: db, dir: dir}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// AddDocument inserts a new document record and its chunks atomically.
func (s *Store) AddDocument(ctx context.Context, doc Document, chunks []Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("knowledge: begin tx: %w", err)
	}
	defer tx.Rollback()

	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}
	now := time.Now()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO documents (id, name, filename, mime_type, size_bytes, chunk_count, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		doc.ID, doc.Name, doc.Filename, doc.MimeType, doc.SizeBytes, len(chunks), now, now,
	)
	if err != nil {
		return fmt.Errorf("knowledge: insert document: %w", err)
	}

	for _, chunk := range chunks {
		if chunk.ID == "" {
			chunk.ID = uuid.New().String()
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO chunks (id, doc_id, chunk_index, content, token_count, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			chunk.ID, doc.ID, chunk.ChunkIndex, chunk.Content, chunk.TokenCount, now,
		)
		if err != nil {
			return fmt.Errorf("knowledge: insert chunk %d: %w", chunk.ChunkIndex, err)
		}
	}

	return tx.Commit()
}

// DeleteDocument removes a document and all its chunks.
func (s *Store) DeleteDocument(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("knowledge: delete document: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("knowledge: document not found: %s", id)
	}
	return nil
}

// SetEnabled enables or disables a document.
func (s *Store) SetEnabled(ctx context.Context, id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := 0
	if enabled {
		v = 1
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE documents SET enabled = ?, updated_at = ? WHERE id = ?`,
		v, time.Now(), id,
	)
	return err
}

// ListDocuments returns all documents ordered by creation time descending.
func (s *Store) ListDocuments(ctx context.Context) ([]Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, filename, mime_type, size_bytes, chunk_count, enabled, created_at, updated_at
		 FROM documents ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var d Document
		var enabledInt int
		if err := rows.Scan(&d.ID, &d.Name, &d.Filename, &d.MimeType, &d.SizeBytes,
			&d.ChunkCount, &enabledInt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.Enabled = enabledInt == 1
		docs = append(docs, d)
	}
	return docs, rows.Err()
}

// Search performs BM25 full-text search over enabled document chunks.
// Returns up to maxResults results ranked by relevance.
func (s *Store) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if maxResults <= 0 {
		maxResults = 5
	}

	// SQLite FTS5 BM25 — lower (more negative) rank = more relevant
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.doc_id, c.chunk_index, c.content, c.token_count,
		       d.name,
		       -bm25(chunks_fts) AS score
		FROM chunks_fts
		JOIN chunks c ON c.rowid = chunks_fts.rowid
		JOIN documents d ON d.id = c.doc_id
		WHERE chunks_fts MATCH ?
		  AND d.enabled = 1
		ORDER BY score DESC
		LIMIT ?
	`, sanitizeFTSQuery(query), maxResults)
	if err != nil {
		return nil, fmt.Errorf("knowledge: search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(
			&r.Chunk.ID, &r.Chunk.DocID, &r.Chunk.ChunkIndex,
			&r.Chunk.Content, &r.Chunk.TokenCount,
			&r.DocName, &r.Score,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// sanitizeFTSQuery escapes special FTS5 characters to prevent query syntax errors.
func sanitizeFTSQuery(q string) string {
	// Wrap each word in double quotes for phrase-safe matching
	words := strings.Fields(q)
	if len(words) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.ReplaceAll(w, `"`, `""`)
		quoted = append(quoted, `"`+w+`"`)
	}
	return strings.Join(quoted, " OR ")
}
