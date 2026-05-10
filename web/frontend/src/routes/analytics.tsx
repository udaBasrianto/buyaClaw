import { createFileRoute } from "@tanstack/react-router"
import { AnalyticsDashboard } from "@/components/analytics/analytics-dashboard"

export const Route = createFileRoute("/analytics")({
  component: AnalyticsDashboard,
})
