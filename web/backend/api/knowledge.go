package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sipeed/picoclaw/pkg/knowledge"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/web/backend/utils"
)

const (
	maxUploadSize    = 20 * 1024 * 1024 // 20 MB
	knowledgeDBDir   = "knowledge"
)

// knowledgeStore is a process-level singleton opened lazily via sync.Once.
var (
	knowledgeStore     *knowledge.Store
	knowledgeStoreOnce sync.Once
	knowledgeStoreErr  error
)

func getKnowledgeStore() (*knowledge.Store, error) {
	knowledgeStoreOnce.Do(func() {
		dir := filepath.Join(utils.GetPicoclawHome(), knowledgeDBDir)
		knowledgeStore, knowledgeStoreErr = knowledge.NewStore(dir)
	})
	return knowledgeStore, knowledgeStoreErr
}

// registerKnowledgeRoutes binds knowledge base endpoints to the mux.
func (h *Handler) registerKnowledgeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/knowledge", h.handleListDocuments)
	mux.HandleFunc("POST /api/knowledge/upload", h.handleUploadDocument)
	mux.HandleFunc("DELETE /api/knowledge/{id}", h.handleDeleteDocument)
	mux.HandleFunc("PUT /api/knowledge/{id}/state", h.handleSetDocumentState)
	mux.HandleFunc("GET /api/knowledge/search", h.handleSearchKnowledge)
}

// handleListDocuments returns all knowledge base documents.
//
//	GET /api/knowledge
func (h *Handler) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	store, err := getKnowledgeStore()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	docs, err := store.ListDocuments(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if docs == nil {
		docs = []knowledge.Document{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

// handleUploadDocument parses a multipart upload, extracts text, chunks it,
// and indexes it into the knowledge base.
//
//	POST /api/knowledge/upload
func (h *Handler) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeJSONError(w, http.StatusBadRequest, "file too large or invalid form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	if header.Size > maxUploadSize {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("file exceeds %d MB limit", maxUploadSize/1024/1024))
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, maxUploadSize))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	filename := filepath.Base(header.Filename)
	// Use custom name if provided
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	// Parse document to plain text
	text, err := knowledge.ParseFile(filename, data)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("cannot parse file: %v", err))
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		writeJSONError(w, http.StatusBadRequest, "document appears to be empty or unreadable")
		return
	}

	// Chunk the text
	cfg := knowledge.DefaultChunkConfig()
	rawChunks := knowledge.ChunkText(text, cfg)
	if len(rawChunks) == 0 {
		writeJSONError(w, http.StatusBadRequest, "no content could be extracted from document")
		return
	}

	// Build chunk records
	chunks := make([]knowledge.Chunk, 0, len(rawChunks))
	for i, c := range rawChunks {
		chunks = append(chunks, knowledge.Chunk{
			ChunkIndex: i,
			Content:    c,
			TokenCount: len(strings.Fields(c)),
		})
	}

	doc := knowledge.Document{
		Name:      name,
		Filename:  filename,
		MimeType:  knowledge.DetectMimeType(filename),
		SizeBytes: int64(len(data)),
	}

	store, err := getKnowledgeStore()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := store.AddDocument(r.Context(), doc, chunks); err != nil {
		logger.ErrorCF("knowledge", "failed to add document", map[string]any{
			"name":  name,
			"error": err.Error(),
		})
		writeJSONError(w, http.StatusInternalServerError, "failed to index document")
		return
	}

	logger.InfoCF("knowledge", "document indexed", map[string]any{
		"name":   name,
		"chunks": len(chunks),
		"bytes":  len(data),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"name":        name,
		"chunk_count": len(chunks),
	})
}

// handleDeleteDocument removes a document from the knowledge base.
//
//	DELETE /api/knowledge/{id}
func (h *Handler) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing document id")
		return
	}

	store, err := getKnowledgeStore()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := store.DeleteDocument(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSONError(w, http.StatusNotFound, err.Error())
		} else {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// handleSetDocumentState enables or disables a document.
//
//	PUT /api/knowledge/{id}/state
func (h *Handler) handleSetDocumentState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing document id")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	store, err := getKnowledgeStore()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := store.SetEnabled(r.Context(), id, req.Enabled); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "enabled": req.Enabled})
}

// handleSearchKnowledge searches the knowledge base.
//
//	GET /api/knowledge/search?q=...&limit=5
func (h *Handler) handleSearchKnowledge(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSONError(w, http.StatusBadRequest, "missing query parameter 'q'")
		return
	}

	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	store, err := getKnowledgeStore()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	results, err := store.Search(r.Context(), q, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []knowledge.SearchResult{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results, "query": q})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

// GetKnowledgeStore returns the process-level knowledge store, opening it if needed.
// Used by the agent to register the prompt contributor.
func GetKnowledgeStore() (*knowledge.Store, error) {
	return getKnowledgeStore()
}