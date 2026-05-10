import {
  IconActivity,
  IconAlertCircle,
  IconChartBar,
  IconMessage,
  IconRefresh,
  IconRobot,
  IconUsers,
} from "@tabler/icons-react"
import { useQuery } from "@tanstack/react-query"

import { getAnalyticsSummary, type TurnRecord } from "@/api/analytics"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ScrollArea } from "@/components/ui/scroll-area"

export function AnalyticsDashboard() {
  const { data, isLoading, error, refetch, isFetching } = useQuery({
    queryKey: ["analytics-summary"],
    queryFn: getAnalyticsSummary,
    refetchInterval: 30_000, // auto-refresh every 30s
  })

  const turns = data?.turns
  const sessions = data?.sessions
  const recent = data?.recent_turns ?? []

  return (
    <div className="flex flex-col gap-6 p-6">
      <PageHeader title="Analytics">
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetch()}
          disabled={isFetching}
          className="gap-2"
        >
          <IconRefresh className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
          Refresh
        </Button>
      </PageHeader>

      {error && (
        <div className="flex items-center gap-2 rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <IconAlertCircle className="h-4 w-4 shrink-0" />
          Failed to load analytics. Start chatting to generate data.
        </div>
      )}

      {/* Stat cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <StatCard
          icon={<IconActivity className="h-5 w-5 text-blue-500" />}
          label="Turns Today"
          value={turns?.turns_today ?? 0}
          sub={`${turns?.turns_this_week ?? 0} this week`}
          loading={isLoading}
        />
        <StatCard
          icon={<IconChartBar className="h-5 w-5 text-indigo-500" />}
          label="Total Turns"
          value={turns?.total_turns ?? 0}
          loading={isLoading}
        />
        <StatCard
          icon={<IconUsers className="h-5 w-5 text-green-500" />}
          label="Sessions"
          value={sessions?.total ?? 0}
          sub={`${sessions?.active_today ?? 0} active today`}
          loading={isLoading}
        />
        <StatCard
          icon={<IconMessage className="h-5 w-5 text-purple-500" />}
          label="Messages"
          value={sessions?.total_messages ?? 0}
          sub={`${sessions?.active_this_week ?? 0} sessions this week`}
          loading={isLoading}
        />
      </div>

      {/* Token usage */}
      {(turns?.tokens_in_total ?? 0) > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium">Token Usage</CardTitle>
          </CardHeader>
          <CardContent className="flex gap-8">
            <div>
              <p className="text-2xl font-bold">{fmtNum(turns?.tokens_in_total ?? 0)}</p>
              <p className="text-muted-foreground text-xs">Input tokens</p>
            </div>
            <div>
              <p className="text-2xl font-bold">{fmtNum(turns?.tokens_out_total ?? 0)}</p>
              <p className="text-muted-foreground text-xs">Output tokens</p>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {/* By channel */}
        <BreakdownCard
          title="By Channel"
          data={turns?.by_channel ?? {}}
          loading={isLoading}
          emptyText="No channel data yet"
        />

        {/* By model */}
        <BreakdownCard
          title="By Model"
          data={turns?.by_model ?? {}}
          loading={isLoading}
          emptyText="No model data yet"
        />

        {/* By status */}
        <BreakdownCard
          title="By Status"
          data={turns?.by_status ?? {}}
          loading={isLoading}
          emptyText="No status data yet"
          colorFn={(key) =>
            key === "completed"
              ? "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400"
              : key === "error"
                ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                : "bg-muted text-muted-foreground"
          }
        />
      </div>

      {/* Daily chart */}
      {(turns?.daily_turns?.length ?? 0) > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium">Daily Turns (last 30 days)</CardTitle>
          </CardHeader>
          <CardContent>
            <MiniBarChart data={turns?.daily_turns ?? []} />
          </CardContent>
        </Card>
      )}

      {/* Recent turns */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <IconRobot className="h-4 w-4" />
            Recent Turns
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {recent.length === 0 ? (
            <p className="text-muted-foreground px-4 py-6 text-center text-sm">
              No turns recorded yet. Start chatting to see data here.
            </p>
          ) : (
            <ScrollArea className="h-72">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-xs text-muted-foreground">
                    <th className="px-4 py-2">Time</th>
                    <th className="px-4 py-2">Channel</th>
                    <th className="px-4 py-2">Model</th>
                    <th className="px-4 py-2">Status</th>
                    <th className="px-4 py-2 text-right">Duration</th>
                  </tr>
                </thead>
                <tbody>
                  {recent.map((t, i) => (
                    <TurnRow key={i} turn={t} />
                  ))}
                </tbody>
              </table>
            </ScrollArea>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function StatCard({
  icon,
  label,
  value,
  sub,
  loading,
}: {
  icon: React.ReactNode
  label: string
  value: number
  sub?: string
  loading: boolean
}) {
  return (
    <Card>
      <CardContent className="pt-4">
        <div className="flex items-center justify-between">
          {icon}
          <span className="text-muted-foreground text-xs">{label}</span>
        </div>
        <p className={`mt-2 text-3xl font-bold ${loading ? "animate-pulse text-muted" : ""}`}>
          {loading ? "—" : fmtNum(value)}
        </p>
        {sub && <p className="text-muted-foreground mt-0.5 text-xs">{sub}</p>}
      </CardContent>
    </Card>
  )
}

function BreakdownCard({
  title,
  data,
  loading,
  emptyText,
  colorFn,
}: {
  title: string
  data: Record<string, number>
  loading: boolean
  emptyText: string
  colorFn?: (key: string) => string
}) {
  const entries = Object.entries(data).sort((a, b) => b[1] - a[1])
  const total = entries.reduce((s, [, v]) => s + v, 0)

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-muted-foreground text-sm">Loading…</p>
        ) : entries.length === 0 ? (
          <p className="text-muted-foreground text-sm">{emptyText}</p>
        ) : (
          <div className="space-y-2">
            {entries.map(([key, val]) => (
              <div key={key} className="flex items-center justify-between gap-2">
                <span
                  className={`rounded-full px-2 py-0.5 text-xs ${
                    colorFn
                      ? colorFn(key)
                      : "bg-muted text-muted-foreground"
                  }`}
                >
                  {key || "unknown"}
                </span>
                <div className="flex items-center gap-2">
                  <div className="bg-muted h-1.5 w-20 overflow-hidden rounded-full">
                    <div
                      className="h-full rounded-full bg-primary"
                      style={{ width: `${total > 0 ? (val / total) * 100 : 0}%` }}
                    />
                  </div>
                  <span className="text-muted-foreground w-8 text-right text-xs">{val}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function MiniBarChart({ data }: { data: { date: string; count: number }[] }) {
  const max = Math.max(...data.map((d) => d.count), 1)
  return (
    <div className="flex h-20 items-end gap-0.5">
      {data.map((d) => (
        <div
          key={d.date}
          className="group relative flex-1"
          title={`${d.date}: ${d.count} turns`}
        >
          <div
            className="bg-primary/70 hover:bg-primary w-full rounded-sm transition-all"
            style={{ height: `${(d.count / max) * 100}%`, minHeight: 2 }}
          />
        </div>
      ))}
    </div>
  )
}

function TurnRow({ turn }: { turn: TurnRecord }) {
  const ts = new Date(turn.Timestamp)
  const timeStr = ts.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
  const dateStr = ts.toLocaleDateString([], { month: "short", day: "numeric" })

  return (
    <tr className="border-b border-border/40 last:border-0 hover:bg-muted/30">
      <td className="px-4 py-2 text-xs text-muted-foreground">
        {dateStr} {timeStr}
      </td>
      <td className="px-4 py-2">
        <Badge variant="outline" className="text-xs">
          {turn.Channel || "—"}
        </Badge>
      </td>
      <td className="px-4 py-2 font-mono text-xs text-muted-foreground">
        {turn.Model || "—"}
      </td>
      <td className="px-4 py-2">
        <span
          className={`rounded-full px-2 py-0.5 text-xs ${
            turn.Status === "completed"
              ? "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400"
              : turn.Status === "error"
                ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                : "bg-muted text-muted-foreground"
          }`}
        >
          {turn.Status || "—"}
        </span>
      </td>
      <td className="px-4 py-2 text-right text-xs text-muted-foreground">
        {turn.DurationMS > 0 ? `${(turn.DurationMS / 1000).toFixed(1)}s` : "—"}
      </td>
    </tr>
  )
}

function fmtNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}
