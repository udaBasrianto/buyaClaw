import { createFileRoute } from "@tanstack/react-router"

import { KnowledgePage } from "@/components/agent/knowledge/knowledge-page"

export const Route = createFileRoute("/agent/knowledge")({
  component: KnowledgeRoute,
})

function KnowledgeRoute() {
  return <KnowledgePage />
}
