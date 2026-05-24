import { apiFetch } from "./api";

// ── Types ────────────────────────────────────────────────────────────────────

export type SiemSummary = {
  total_events: number;
  critical_alerts: number;
  auth_failures: number;
  auth_successes: number;
};

export type AlertLevel = {
  level: number;
  count: number;
};

export type TimeSeriesPoint = {
  timestamp: string; // ISO
  counts: Record<string, number>; // level → count  OR  agent_name → count
};

export type MitreTechnique = {
  technique: string;
  tactic: string;
  count: number;
  percentage: number;
};

export type AgentStat = {
  agent_id: string;
  agent_name: string;
  total: number;
  percentage: number;
};

export type SecurityAlert = {
  id: number;
  timestamp: string;
  agent_id: string;
  agent_name: string;
  technique: string | null;
  tactic: string | null;
  description: string;
  level: number;
  rule_id: string;
};

export type AlertsPage = {
  items: SecurityAlert[];
  total: number;
  page: number;
  page_size: number;
};

export type SiemOverview = {
  summary: SiemSummary;
  alert_levels_series: TimeSeriesPoint[];
  top_mitre: MitreTechnique[];
  top_agents: AgentStat[];
  agent_series: TimeSeriesPoint[];
};

// ── Fetchers ─────────────────────────────────────────────────────────────────

export async function fetchSiemOverview(token: string): Promise<SiemOverview> {
  return apiFetch<SiemOverview>("/api/siem/overview", { method: "GET" }, token);
}

export async function fetchSiemAlerts(
  token: string,
  page = 1,
  pageSize = 10
): Promise<AlertsPage> {
  return apiFetch<AlertsPage>(
    `/api/siem/alerts?page=${page}&page_size=${pageSize}`,
    { method: "GET" },
    token
  );
}
