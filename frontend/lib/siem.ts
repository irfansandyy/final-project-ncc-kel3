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

// ── Mock data (used when backend /api/siem/* endpoints are not yet wired) ────
// Remove this section and the `useMock` flag in siem-dashboard.tsx once the
// Go handlers are implemented.

export function getMockOverview(): SiemOverview {
  const days = ["Jan 18", "Jan 19", "Jan 20", "Jan 21", "Jan 22", "Jan 23", "Jan 24"];

  const alert_levels_series: TimeSeriesPoint[] = days.map((d, i) => ({
    timestamp: d,
    counts: {
      "14": [40, 55, 48, 60, 70, 65, 130][i],
      "12": [80, 100, 95, 110, 120, 105, 180][i],
      "10": [150, 170, 160, 190, 200, 185, 280][i],
      "8": [300, 320, 310, 340, 360, 345, 500][i],
      "6": [500, 530, 510, 560, 590, 575, 750][i],
      "3": [800, 850, 820, 900, 940, 920, 1100][i],
    },
  }));

  const agent_series: TimeSeriesPoint[] = days.map((d, i) => ({
    timestamp: d,
    counts: {
      "chatbot-api": [310, 330, 315, 350, 380, 360, 480][i],
      "ncc-web-srv": [220, 240, 225, 255, 275, 260, 340][i],
      "db-postgres": [130, 145, 138, 155, 168, 158, 200][i],
      "redis-cache": [60, 70, 65, 75, 80, 74, 95][i],
      "nginx-proxy": [25, 30, 28, 33, 36, 32, 45][i],
    },
  }));

  return {
    summary: {
      total_events: 54249,
      critical_alerts: 4132,
      auth_failures: 3214,
      auth_successes: 349,
    },
    alert_levels_series,
    top_mitre: [
      { technique: "Brute Force", tactic: "Credential Access", count: 1572, percentage: 38 },
      { technique: "Valid Accounts", tactic: "Initial Access", count: 869, percentage: 21 },
      { technique: "Endpoint DoS", tactic: "Impact", count: 579, percentage: 14 },
      { technique: "Data Collection", tactic: "Collection", count: 496, percentage: 12 },
      { technique: "Credential Access", tactic: "Credential Access", count: 372, percentage: 9 },
      { technique: "Other", tactic: "", count: 248, percentage: 6 },
    ],
    top_agents: [
      { agent_id: "014", agent_name: "chatbot-api", total: 21847, percentage: 92 },
      { agent_id: "001", agent_name: "ncc-web-srv", total: 16203, percentage: 68 },
      { agent_id: "008", agent_name: "db-postgres", total: 9814, percentage: 41 },
      { agent_id: "002", agent_name: "redis-cache", total: 4521, percentage: 24 },
      { agent_id: "005", agent_name: "nginx-proxy", total: 1864, percentage: 10 },
    ],
    agent_series,
  };
}

export function getMockAlerts(): AlertsPage {
  const items: SecurityAlert[] = [
    {
      id: 1,
      timestamp: "2022-01-24T09:39:10.986Z",
      agent_id: "014",
      agent_name: "chatbot-api",
      technique: null,
      tactic: null,
      description: "Prompt injection attempt detected in user input.",
      level: 7,
      rule_id: "91233",
    },
    {
      id: 2,
      timestamp: "2022-01-24T09:38:37.176Z",
      agent_id: "001",
      agent_name: "ncc-web-srv",
      technique: null,
      tactic: null,
      description: "Host-based anomaly detection event (rootcheck).",
      level: 7,
      rule_id: "510",
    },
    {
      id: 3,
      timestamp: "2022-01-24T09:38:31.268Z",
      agent_id: "008",
      agent_name: "db-postgres",
      technique: null,
      tactic: null,
      description: "Apache: attempt to access forbidden directory index.",
      level: 5,
      rule_id: "30306",
    },
    {
      id: 4,
      timestamp: "2022-01-24T09:38:24.899Z",
      agent_id: "002",
      agent_name: "ncc-web-srv",
      technique: "T1190",
      tactic: "Initial Access",
      description: "sshd: possible attack on the ssh server (version gathering).",
      level: 8,
      rule_id: "5701",
    },
    {
      id: 5,
      timestamp: "2022-01-24T09:38:22.345Z",
      agent_id: "005",
      agent_name: "db-postgres",
      technique: null,
      tactic: null,
      description: "CVE-2019-1010204 affects binutils package.",
      level: 7,
      rule_id: "23504",
    },
    {
      id: 6,
      timestamp: "2022-01-24T09:38:17.693Z",
      agent_id: "005",
      agent_name: "db-postgres",
      technique: "T1114",
      tactic: "Collection",
      description: "Unusual DB query volume — possible data exfiltration.",
      level: 5,
      rule_id: "3105",
    },
    {
      id: 7,
      timestamp: "2022-01-24T09:38:12.377Z",
      agent_id: "001",
      agent_name: "ncc-web-srv",
      technique: null,
      tactic: null,
      description: "AWS GuardDuty: unusual outbound EC2 communication on port 5060.",
      level: 6,
      rule_id: "80302",
    },
    {
      id: 8,
      timestamp: "2022-01-24T09:38:03.373Z",
      agent_id: "005",
      agent_name: "nginx-proxy",
      technique: null,
      tactic: null,
      description: "GitHub organisation update: member repository creation permission.",
      level: 3,
      rule_id: "91188",
    },
    {
      id: 9,
      timestamp: "2022-01-24T09:38:02.044Z",
      agent_id: "014",
      agent_name: "chatbot-api",
      technique: null,
      tactic: null,
      description: "Chatbot session token reuse detected — possible replay attack.",
      level: 7,
      rule_id: "553",
    },
    {
      id: 10,
      timestamp: "2022-01-24T09:37:45.630Z",
      agent_id: "005",
      agent_name: "nginx-proxy",
      technique: null,
      tactic: null,
      description: "OpenSCAP: record events that modify the system's network environment (not passed).",
      level: 5,
      rule_id: "81529",
    },
  ];

  return { items, total: 1000, page: 1, page_size: 10 };
}
