import {
  IconBook2,
  IconFileUpload,
  IconSearch,
  IconTrash,
  IconToggleLeft,
  IconToggleRight,
  IconFileText,
  IconAlertCircle,
} from "@tabler/icons-react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  deleteDocument,
  listDocuments,
  searchKnowledge,
  setDocumentEnabled,
  uploadDocument,
  type KnowledgeDocument,
  type KnowledgeSearchResult,
} from "@/api/knowledge"

export function KnowledgePage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [searchQuery, setSearchQuery] = useState("")
  const [searchResults, setSearchResults] = useState<KnowledgeSearchResult[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [uploadSuccess, setUploadSuccess] = useState<string | null>(null)

  const { data: documents = [], isLoading } = useQuery({
    queryKey: ["knowledge-documents"],
    queryFn: listDocuments,
  })

  const uploadMutation = useMutation({
    mutationFn: (file: File) => uploadDocument(file),
    onSuccess: (result) => {
      setUploadError(null)
      setUploadSuccess(`"${result.name}" indexed with ${result.chunk_count} chunks`)
      queryClient.invalidateQueries({ queryKey: ["knowledge-documents"] })
      setTimeout(() => setUploadSuccess(null), 4000)
    },
    onError: (err: Error) => {
      setUploadError(err.message)
      setUploadSuccess(null)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteDocument,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["knowledge-documents"] })
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      setDocumentEnabled(id, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["knowledge-documents"] })
    },
  })

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setUploadError(null)
    uploadMutation.mutate(file)
    e.target.value = ""
  }

  const handleSearch = async () => {
    if (!searchQuery.trim()) return
    setIsSearching(true)
    try {
      const res = await searchKnowledge(searchQuery, 5)
      setSearchResults(res.results)
    } catch (err) {
      console.error(err)
    } finally {
      setIsSearching(false)
    }
  }

  const formatBytes = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader
        title="Knowledge Base"
        description="Upload documents to give the AI agent persistent knowledge. Supported: PDF, Word, Excel, CSV, TXT, Markdown."
      />

      {/* Upload Section */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <IconFileUpload className="h-4 w-4" />
            Upload Document
          </CardTitle>
          <CardDescription>
            Documents are automatically parsed, chunked, and indexed for retrieval.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-3">
            <input
              ref={fileInputRef}
              type="file"
              className="hidden"
              accept=".pdf,.docx,.xlsx,.xls,.csv,.txt,.md,.markdown"
              onChange={handleFileChange}
            />
            <Button
              onClick={() => fileInputRef.current?.click()}
              disabled={uploadMutation.isPending}
              className="gap-2"
            >
              <IconFileUpload className="h-4 w-4" />
              {uploadMutation.isPending ? "Uploading..." : "Choose File"}
            </Button>
            <span className="text-muted-foreground text-sm">
              Max 20 MB · PDF, DOCX, XLSX, CSV, TXT, MD
            </span>
          </div>

          {uploadError && (
            <div className="mt-3 flex items-center gap-2 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
              <IconAlertCircle className="h-4 w-4 shrink-0" />
              {uploadError}
            </div>
          )}
          {uploadSuccess && (
            <div className="mt-3 rounded-md bg-green-500/10 px-3 py-2 text-sm text-green-700 dark:text-green-400">
              ✓ {uploadSuccess}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Documents List */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <IconBook2 className="h-4 w-4" />
            Documents
            <Badge variant="secondary" className="ml-1">
              {documents.length}
            </Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-muted-foreground text-sm">Loading...</p>
          ) : documents.length === 0 ? (
            <div className="flex flex-col items-center gap-2 py-8 text-center">
              <IconFileText className="text-muted-foreground h-10 w-10" />
              <p className="text-muted-foreground text-sm">
                No documents yet. Upload a file to get started.
              </p>
            </div>
          ) : (
            <div className="divide-y">
              {documents.map((doc) => (
                <DocumentRow
                  key={doc.id}
                  doc={doc}
                  formatBytes={formatBytes}
                  onDelete={() => deleteMutation.mutate(doc.id)}
                  onToggle={() =>
                    toggleMutation.mutate({ id: doc.id, enabled: !doc.enabled })
                  }
                  isDeleting={deleteMutation.isPending}
                  isToggling={toggleMutation.isPending}
                />
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Search Section */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <IconSearch className="h-4 w-4" />
            Test Search
          </CardTitle>
          <CardDescription>
            Preview what the agent will retrieve for a given query.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex gap-2">
            <Input
              placeholder="Enter a search query..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSearch()}
            />
            <Button onClick={handleSearch} disabled={isSearching} className="gap-2">
              <IconSearch className="h-4 w-4" />
              {isSearching ? "Searching..." : "Search"}
            </Button>
          </div>

          {searchResults.length > 0 && (
            <ScrollArea className="h-80">
              <div className="flex flex-col gap-3">
                {searchResults.map((result, i) => (
                  <div
                    key={result.chunk.id}
                    className="rounded-md border bg-muted/30 p-3 text-sm"
                  >
                    <div className="mb-1 flex items-center justify-between">
                      <span className="font-medium">
                        [{i + 1}] {result.doc_name}
                      </span>
                      <Badge variant="outline" className="text-xs">
                        score: {result.score.toFixed(2)}
                      </Badge>
                    </div>
                    <p className="text-muted-foreground line-clamp-4 whitespace-pre-wrap text-xs">
                      {result.chunk.content}
                    </p>
                  </div>
                ))}
              </div>
            </ScrollArea>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

interface DocumentRowProps {
  doc: KnowledgeDocument
  formatBytes: (n: number) => string
  onDelete: () => void
  onToggle: () => void
  isDeleting: boolean
  isToggling: boolean
}

function DocumentRow({
  doc,
  formatBytes,
  onDelete,
  onToggle,
  isDeleting,
  isToggling,
}: DocumentRowProps) {
  return (
    <div className="flex items-center justify-between py-3">
      <div className="flex min-w-0 flex-col gap-0.5">
        <div className="flex items-center gap-2">
          <span className="truncate font-medium text-sm">{doc.name}</span>
          {!doc.enabled && (
            <Badge variant="secondary" className="text-xs">
              disabled
            </Badge>
          )}
        </div>
        <span className="text-muted-foreground text-xs">
          {doc.filename} · {formatBytes(doc.size_bytes)} · {doc.chunk_count} chunks
        </span>
      </div>
      <div className="flex shrink-0 items-center gap-1">
        <Button
          variant="ghost"
          size="icon"
          onClick={onToggle}
          disabled={isToggling}
          title={doc.enabled ? "Disable" : "Enable"}
        >
          {doc.enabled ? (
            <IconToggleRight className="h-4 w-4 text-green-500" />
          ) : (
            <IconToggleLeft className="text-muted-foreground h-4 w-4" />
          )}
        </Button>
        <Button
          variant="ghost"
          size="icon"
          onClick={onDelete}
          disabled={isDeleting}
          title="Delete"
        >
          <IconTrash className="h-4 w-4 text-destructive" />
        </Button>
      </div>
    </div>
  )
}
