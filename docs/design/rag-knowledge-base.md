# RAG Knowledge Base — Design Spec

## Overview

Fitur knowledge base memungkinkan admin mengupload dokumen (PDF, Word, Excel, TXT, dll)
yang dijadikan sumber pengetahuan permanen untuk semua agent. Setiap kali user bertanya,
sistem secara otomatis mencari chunk dokumen yang relevan dan menyuntikkannya ke konteks
LLM sebelum menjawab.

## Arsitektur

```
Upload Dokumen (Admin UI)
        │
        ▼
  Document Parser          ← pkg/knowledge/parser.go
  (PDF/Word/Excel/TXT)
        │
        ▼
  Text Chunker             ← pkg/knowledge/chunker.go
  (token-based, overlap)
        │
        ▼
  BM25 Indexer             ← pkg/knowledge/store.go
  (SQLite FTS5)
        │
        ▼
  KnowledgeStore           ← pkg/knowledge/store.go
  (SQLite: docs + chunks)
        │
   ┌────┴────┐
   ▼         ▼
RetrieveTool  PromptContributor
(agent tool)  (auto-inject context)
```

## Storage Schema (SQLite)

```sql
CREATE TABLE documents (
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

CREATE TABLE chunks (
    id          TEXT PRIMARY KEY,
    doc_id      TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content     TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE VIRTUAL TABLE chunks_fts USING fts5(
    content,
    content='chunks',
    content_rowid='rowid'
);
```

## Komponen yang Dibuat

### Backend (Go)

| File | Deskripsi |
|------|-----------|
| `pkg/knowledge/store.go` | KnowledgeStore — CRUD dokumen + chunks di SQLite |
| `pkg/knowledge/parser.go` | Parse PDF/Word/Excel/TXT → plain text |
| `pkg/knowledge/chunker.go` | Split text → chunks dengan overlap |
| `pkg/knowledge/retriever.go` | BM25 search via SQLite FTS5 |
| `pkg/knowledge/contributor.go` | PromptContributor — auto-inject ke context layer |
| `pkg/tools/knowledge_tool.go` | Tool `retrieve_knowledge` untuk agent |
| `web/backend/api/knowledge.go` | REST API endpoints |

### Frontend (React/TypeScript)

| File | Deskripsi |
|------|-----------|
| `web/frontend/src/api/knowledge.ts` | API client |
| `web/frontend/src/features/knowledge/` | UI components |
| `web/frontend/src/routes/knowledge.tsx` | Halaman admin knowledge base |

## API Endpoints

```
GET    /api/knowledge                    List semua dokumen
POST   /api/knowledge/upload             Upload dokumen baru (multipart)
DELETE /api/knowledge/:id                Hapus dokumen
PUT    /api/knowledge/:id/state          Enable/disable dokumen
GET    /api/knowledge/search?q=...       Search chunks
```

## Config

```json
{
  "tools": {
    "knowledge_base": {
      "enabled": true,
      "max_document_size_mb": 20,
      "chunk_size": 512,
      "chunk_overlap": 64,
      "max_search_results": 5,
      "auto_inject": true
    }
  }
}
```

## Format Dokumen yang Didukung

| Format | Library |
|--------|---------|
| TXT, MD | stdlib |
| PDF | `github.com/ledongthuc/pdf` |
| DOCX | `github.com/nguyenthenguyen/docx` |
| XLSX | `github.com/xuri/excelize/v2` |
| CSV | stdlib `encoding/csv` |

## Alur Kerja

1. Admin upload dokumen via UI
2. Backend parse → chunk → index ke SQLite FTS5
3. Saat user kirim pesan, `KnowledgePromptContributor` query FTS5
4. Top-K chunks relevan di-inject ke `PromptLayerContext`
5. LLM menjawab dengan konteks dari knowledge base

## Fase Implementasi

- **Fase 1**: Storage + Parser + Chunker (pkg/knowledge/)
- **Fase 2**: Tool + API endpoints
- **Fase 3**: Prompt contributor (auto-inject)
- **Fase 4**: Frontend UI
