import { launcherFetch } from "@/api/http"

export interface TurnSummary {
  total_turns: number
  turns_today: number
  turns_this_week: number
  errors_today: number
  tokens_in_total: number
  tokens_out_total: number
  by_channel: Record<string, number>
  by_model: Record<string, number>
  by_status: Record<string, number>
  daily_turns: { date: string; count: number }[]
}

export interface SessionSummary {
  total: number
  total_messages: number
  active_today: number
  active_this_week: number
}

export interface TurnRecord {
  Timestamp: string
  AgentID: string
  Channel: string
  Model: string
  Status: string
  DurationMS: number
  Iterations: number
  TokensIn: number
  TokensOut: number
}

export interface AnalyticsSummary {
  turns: TurnSummary
  sessions: SessionSummary
  recent_turns: TurnRecord[]
  generated_at: string
}

export async function getAnalyticsSummary(): Promise<AnalyticsSummary> {
  const res = await launcherFetch("/api/analytics/summary")
  if (!res.ok) throw new Error(`Analytics API error: ${res.status}`)
  return res.json()
}
