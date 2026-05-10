import { launcherFetch } from "@/api/http"

export interface KnowledgeDocument {
  id: string
  name: string
  filename: string
  mime_type: string
  size_bytes: number
  chunk_count: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface KnowledgeSearchResult {
  chunk: {
    id: string
    doc_id: string
    chunk_index: number
    content: string
    token_count: number
  }
  doc_name: string
  score: number
}

interface DocumentsResponse {
  documents: KnowledgeDocument[]
}

interface SearchResponse {
  results: KnowledgeSearchResult[]
  query: string
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await launcherFetch(path, options)
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body?.error ?? res.statusText)
  }
  return res.json() as Promise<T>
}

export async function listDocuments(): Promise<KnowledgeDocument[]> {
  const data = await request<DocumentsResponse>("/api/knowledge")
  return data.documents ?? []
}

export async function uploadDocument(
  file: File,
  name?: string,
): Promise<{ status: string; name: string; chunk_count: number }> {
  const form = new FormData()
  form.append("file", file)
  if (name) form.append("name", name)
  return request("/api/knowledge/upload", { method: "POST", body: form })
}

export async function deleteDocument(id: string): Promise<void> {
  await request(`/api/knowledge/${encodeURIComponent(id)}`, { method: "DELETE" })
}

export async function setDocumentEnabled(id: string, enabled: boolean): Promise<void> {
  await request(`/api/knowledge/${encodeURIComponent(id)}/state`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled }),
  })
}

export async function searchKnowledge(
  query: string,
  limit = 5,
): Promise<SearchResponse> {
  const params = new URLSearchParams({ q: query, limit: String(limit) })
  return request<SearchResponse>(`/api/knowledge/search?${params}`)
}
