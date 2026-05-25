"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  type AgentStat,
  type AlertsPage,
  type MitreTechnique,
  type SecurityAlert,
  type SiemOverview,
  type TimeSeriesPoint,
  fetchSiemAlerts,
  fetchSiemOverview,
  getMockAlerts,
  getMockOverview,
} from "@/lib/siem";
import { clearSession, getToken } from "@/lib/auth";
import { APIError } from "@/lib/api";

// ── Feature flag ─────────────────────────────────────────────────────────────
// Set to false once /api/siem/* Go handlers are wired up.
const USE_MOCK = false;

// ── Colour maps ───────────────────────────────────────────────────────────────
const LEVEL_COLORS: Record<string, string> = {
  "14": "#f85149",
  "12": "#d29922",
  "10": "#bc8cff",
  "8": "#58a6ff",
  "6": "#3fb950",
  "3": "#39d353",
};

const AGENT_COLORS = ["#f85149", "#58a6ff", "#3fb950", "#d29922", "#bc8cff"];

const MITRE_COLORS = [
  "#f85149",
  "#58a6ff",
  "#d29922",
  "#bc8cff",
  "#3fb950",
  "#8b949e",
];

const FALLBACK_COLOR = "#555";

// ── Helpers ───────────────────────────────────────────────────────────────────
function fmtTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function levelClass(level: number): string {
  if (level >= 12) return "siem-lv siem-lv-crit";
  if (level >= 8) return "siem-lv siem-lv-high";
  if (level >= 5) return "siem-lv siem-lv-med";
  return "siem-lv siem-lv-info";
}

// ── Area chart path builder (extracted from component to reduce nesting) ──────
function buildAreaPath(
  values: number[],
  maxVal: number,
  W: number,
  H: number,
  fill: boolean
): string {
  const pts = values.map((v, i) => [
    (i / Math.max(values.length - 1, 1)) * W,
    H - (v / maxVal) * H,
  ]);
  const line = pts
    .map(([x, y], i) => `${i === 0 ? "M" : "L"}${(x ?? 0).toFixed(1)},${(y ?? 0).toFixed(1)}`)
    .join(" ");
  if (!fill) return line;
  return `${line} L${W},${H} L0,${H} Z`;
}

// ── Mini bar chart (SVG — no external dep) ────────────────────────────────────
function StackedBarChart({
  series,
  keys,
  colors,
}: {
  readonly series: TimeSeriesPoint[];
  readonly keys: string[];
  readonly colors: string[];
}) {
  const maxTotal = useMemo(() => {
    return Math.max(
      ...series.map((p) => keys.reduce((s, k) => s + (p.counts[k] ?? 0), 0))
    );
  }, [series, keys]);

  const W = 540;
  const H = 160;
  const barW = Math.floor((W - (series.length - 1) * 4) / Math.max(series.length, 1));

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="siem-chart-svg" aria-hidden="true">
      {series.map((point, xi) => {
        const x = xi * (barW + 4);
        let yOffset = H;
        return keys.map((k, ki) => {
          const val = point.counts[k] ?? 0;
          const h = maxTotal > 0 ? Math.round((val / maxTotal) * H) : 0;
          yOffset -= h;
          return (
            <rect
              key={`${xi}-${ki}`}
              x={x}
              y={yOffset}
              width={barW}
              height={h}
              fill={colors[ki] ?? FALLBACK_COLOR}
              opacity={0.82}
            />
          );
        });
      })}
    </svg>
  );
}

// ── Area sparkline ────────────────────────────────────────────────────────────
function AreaChart({
  series,
  keys,
  colors,
}: {
  readonly series: TimeSeriesPoint[];
  readonly keys: string[];
  readonly colors: string[];
}) {
  const W = 540;
  const H = 160;

  const totals = series.map((p) =>
    keys.reduce((s, k) => s + (p.counts[k] ?? 0), 0)
  );
  const maxVal = Math.max(...totals, 1);

  // Accumulate stacked values per key (bottom key is drawn first / largest)
  const stackedKeys = [...keys].reverse();
  const stackedColors = [...colors].reverse();

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="siem-chart-svg" aria-hidden="true">
      {stackedKeys.map((k, ki) => {
        const cumValues = series.map((p) =>
          stackedKeys.slice(ki).reduce((s, sk) => s + (p.counts[sk] ?? 0), 0)
        );
        return (
          <path
            key={k}
            d={buildAreaPath(cumValues, maxVal, W, H, true)}
            fill={stackedColors[ki] ?? FALLBACK_COLOR}
            opacity={0.18 + ki * 0.04}
          />
        );
      })}
      {stackedKeys.map((k, ki) => {
        const cumValues = series.map((p) =>
          stackedKeys.slice(ki).reduce((s, sk) => s + (p.counts[sk] ?? 0), 0)
        );
        return (
          <path
            key={`line-${k}`}
            d={buildAreaPath(cumValues, maxVal, W, H, false)}
            fill="none"
            stroke={stackedColors[ki] ?? FALLBACK_COLOR}
            strokeWidth={1.5}
            opacity={0.9}
          />
        );
      })}
    </svg>
  );
}

// ── Donut chart ───────────────────────────────────────────────────────────────
function DonutChart({ data }: { readonly data: MitreTechnique[] }) {
  const R = 54;
  const CX = 70;
  const CY = 70;
  const circumference = 2 * Math.PI * R;
  let offset = 0;

  return (
    <svg viewBox="0 0 140 140" width={140} height={140} aria-hidden="true">
      {data.map((item, i) => {
        const dash = (item.percentage / 100) * circumference;
        const gap = circumference - dash;
        const rotation = (offset / 100) * 360 - 90;
        offset += item.percentage;
        return (
          <circle
            key={item.technique}
            cx={CX}
            cy={CY}
            r={R}
            fill="none"
            stroke={MITRE_COLORS[i] ?? FALLBACK_COLOR}
            strokeWidth={18}
            strokeDasharray={`${dash} ${gap}`}
            strokeDashoffset={0}
            transform={`rotate(${rotation} ${CX} ${CY})`}
          />
        );
      })}
      <circle cx={CX} cy={CY} r={42} fill="#161b22" />
    </svg>
  );
}

// ── Horizontal bar ────────────────────────────────────────────────────────────
function AgentBar({ agent, color }: { readonly agent: AgentStat; readonly color: string }) {
  return (
    <div className="siem-agent-row">
      <span className="siem-agent-name">{agent.agent_name}</span>
      <div className="siem-agent-track">
        <div
          className="siem-agent-fill"
          style={{ width: `${agent.percentage}%`, background: color }}
        />
      </div>
      <span className="siem-agent-count">{agent.total.toLocaleString()}</span>
    </div>
  );
}

// ── Alert level pill ──────────────────────────────────────────────────────────
function LevelPill({ level }: { readonly level: number }) {
  return <span className={levelClass(level)}>{level}</span>;
}

// ── Badge ─────────────────────────────────────────────────────────────────────
function Badge({ label, variant }: { readonly label: string; readonly variant: "blue" | "orange" | "gray" }) {
  return <span className={`siem-badge siem-badge-${variant}`}>{label}</span>;
}

// ── Main component ────────────────────────────────────────────────────────────
export default function SiemDashboard() {
  const router = useRouter();
  const token = useMemo(() => getToken(), []);

  const [overview, setOverview] = useState<SiemOverview | null>(null);
  const [alertsPage, setAlertsPage] = useState<AlertsPage | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeFilter, setActiveFilter] = useState<{ key: string; value: string } | null>(null);
  const [showFilterModal, setShowFilterModal] = useState(false);
  const [filterKey, setFilterKey] = useState("level");
  const [filterValue, setFilterValue] = useState("");
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const handleUnauthorized = useCallback(() => {
    clearSession();
    router.replace("/login");
  }, [router]);

  const load = useCallback(
    async (showLoader = true) => {
      if (!token) {
        handleUnauthorized();
        return;
      }
      if (showLoader) setLoading(true);
      setError(null);
      try {
        if (USE_MOCK) {
          await new Promise((r) => setTimeout(r, 400));
          setOverview(getMockOverview());
          setAlertsPage(getMockAlerts());
        } else {
          const filterParam = activeFilter
            ? `&${activeFilter.key}=${encodeURIComponent(activeFilter.value)}`
            : "";
          const [ov, al] = await Promise.all([
            fetchSiemOverview(token),
            fetchSiemAlerts(token, page, 10, filterParam),
          ]);
          setOverview(ov);
          setAlertsPage(al);
        }
      } catch (err) {
        const apiError = err as APIError;
        if (apiError.status === 401) {
          handleUnauthorized();
          return;
        }
        setError(apiError.message ?? "Failed to load SIEM data.");
      } finally {
        setLoading(false);
      }
    },
    [handleUnauthorized, page, token, activeFilter]
  );

  useEffect(() => {
    load(true).catch(() => undefined);
    intervalRef.current = setInterval(() => { load(false).catch(() => undefined); }, 30_000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [load]);

  const levelKeys = useMemo(
    () => Object.keys(LEVEL_COLORS).sort((a, b) => Number(b) - Number(a)),
    []
  );
  const agentKeys = useMemo(
    () => overview?.top_agents.map((a) => a.agent_name) ?? [],
    [overview]
  );

  const totalPages = alertsPage ? Math.ceil(alertsPage.total / alertsPage.page_size) : 1;
  const shownPages = useMemo(() => {
    const pages: number[] = [];
    for (let i = Math.max(1, page - 2); i <= Math.min(totalPages, page + 2); i++) {
      pages.push(i);
    }
    return pages;
  }, [page, totalPages]);

  if (loading && !overview) {
    return (
      <div className="siem-shell siem-loading">
        <div className="siem-spinner" />
        <p>Loading SIEM data…</p>
      </div>
    );
  }

  return (
    <div className="siem-shell">
      {/* ── Top bar ── */}
      <header className="siem-topbar">
        <div className="siem-topbar-left">
          <div className="siem-logo">
            <span className="siem-logo-dot" />
            {" SENTINEL "}
            <span className="siem-logo-sep">{"//"}</span>
            {" chatbot-siem"}
          </div>
          <nav className="siem-nav">
            <span className="siem-nav-tab siem-nav-active">Dashboard</span>
          </nav>
        </div>
        <div className="siem-topbar-right">
          <span className="siem-live-pill">
            <span className="siem-live-dot" />{" LIVE"}
          </span>
          <span className="siem-topbar-meta">prod-cluster</span>
          <button
            className="siem-refresh-btn"
            type="button"
            onClick={() => { load(false).catch(() => undefined); }}
            disabled={loading}
          >
            ↺ Refresh
          </button>
        </div>
      </header>

      {/* ── Filter bar ── */}
      <div className="siem-filterbar">
        <span className="siem-filter-chip">
          <span className="siem-chip-key">cluster:</span>{" chatbot-ncc-kel3"}
        </span>
        <span className="siem-filter-chip">
          <span className="siem-chip-key">env:</span>{" production"}
        </span>
        {activeFilter ? (
          <span className="siem-filter-chip siem-filter-chip-active">
            <span className="siem-chip-key">{activeFilter.key}:</span>
            {" "}{activeFilter.value}
            <button
              type="button"
              className="siem-chip-remove"
              onClick={() => { setActiveFilter(null); setPage(1); }}
              aria-label="Remove filter"
            >×</button>
          </span>
        ) : null}
        <button
          type="button"
          className="siem-add-filter"
          onClick={() => setShowFilterModal(true)}
        >+ Add filter</button>
        <span className="siem-time-badge">⏱ Last 7 days</span>
      </div>

      {/* ── Filter modal ── */}
      {showFilterModal ? (
        <div className="siem-modal-overlay" onClick={() => setShowFilterModal(false)}>
          <div className="siem-modal" onClick={(e) => e.stopPropagation()}>
            <div className="siem-modal-title">Add Filter</div>
            <div className="siem-modal-row">
              <label className="siem-modal-label">Field</label>
              <select
                className="siem-modal-select"
                value={filterKey}
                onChange={(e) => setFilterKey(e.target.value)}
              >
                <option value="severity">Severity</option>
                <option value="status">Status</option>
                <option value="level">Level</option>
              </select>
            </div>
            <div className="siem-modal-row">
              <label className="siem-modal-label">Value</label>
              {filterKey === "severity" ? (
                <select className="siem-modal-select" value={filterValue} onChange={(e) => setFilterValue(e.target.value)}>
                  <option value="">-- select --</option>
                  <option value="CRITICAL">CRITICAL</option>
                  <option value="HIGH">HIGH</option>
                  <option value="WARN">WARN</option>
                  <option value="INFO">INFO</option>
                </select>
              ) : filterKey === "status" ? (
                <select className="siem-modal-select" value={filterValue} onChange={(e) => setFilterValue(e.target.value)}>
                  <option value="">-- select --</option>
                  <option value="open">open</option>
                  <option value="acknowledged">acknowledged</option>
                  <option value="resolved">resolved</option>
                </select>
              ) : (
                <input
                  className="siem-modal-input"
                  type="text"
                  placeholder="e.g. ERROR"
                  value={filterValue}
                  onChange={(e) => setFilterValue(e.target.value)}
                />
              )}
            </div>
            <div className="siem-modal-actions">
              <button type="button" className="siem-modal-cancel" onClick={() => setShowFilterModal(false)}>Cancel</button>
              <button
                type="button"
                className="siem-modal-apply"
                disabled={!filterValue}
                onClick={() => {
                  setActiveFilter({ key: filterKey, value: filterValue });
                  setPage(1);
                  setShowFilterModal(false);
                }}
              >Apply</button>
            </div>
          </div>
        </div>
      ) : null}

      {error ? <p className="siem-error">{error}</p> : null}

      {/* ── Metrics ── */}
      {overview ? (
        <>
          <div className="siem-metrics">
            <div className="siem-metric">
              <p className="siem-metric-label">Total Events</p>
              <p className="siem-metric-value siem-blue">
                {overview.summary.total_events.toLocaleString()}
              </p>
              <p className="siem-metric-sub">↑ 12% vs last week</p>
            </div>
            <div className="siem-metric">
              <p className="siem-metric-label">Critical Alerts (≥12)</p>
              <p className="siem-metric-value siem-red">
                {overview.summary.critical_alerts.toLocaleString()}
              </p>
              <p className="siem-metric-sub">↑ 3.1% — review needed</p>
            </div>
            <div className="siem-metric">
              <p className="siem-metric-label">Auth Failures</p>
              <p className="siem-metric-value siem-orange">
                {overview.summary.auth_failures.toLocaleString()}
              </p>
              <p className="siem-metric-sub">Possible brute force</p>
            </div>
            <div className="siem-metric">
              <p className="siem-metric-label">Auth Successes</p>
              <p className="siem-metric-value siem-green">
                {overview.summary.auth_successes.toLocaleString()}
              </p>
              <p className="siem-metric-sub">
                {Math.round(
                  (overview.summary.auth_successes /
                    (overview.summary.auth_failures +
                      overview.summary.auth_successes)) *
                    100
                )}
                % success rate
              </p>
            </div>
          </div>

          {/* ── Charts row ── */}
          <div className="siem-charts-row">
            {/* Alert level evolution */}
            <div className="siem-panel">
              <div className="siem-panel-header">
                <span className="siem-panel-title">Alert level evolution</span>
              </div>
              <div className="siem-chart-wrap">
                <AreaChart
                  series={overview.alert_levels_series}
                  keys={levelKeys}
                  colors={levelKeys.map((k) => LEVEL_COLORS[k] ?? FALLBACK_COLOR)}
                />
              </div>
              <div className="siem-legend">
                {levelKeys.map((k) => (
                  <span key={k} className="siem-legend-item">
                    <span
                      className="siem-legend-dot"
                      style={{ background: LEVEL_COLORS[k] ?? FALLBACK_COLOR }}
                    />
                    Level {k}
                  </span>
                ))}
              </div>
            </div>

            {/* MITRE */}
            <div className="siem-panel">
              <div className="siem-panel-header">
                <span className="siem-panel-title">Top MITRE ATT&amp;CK</span>
              </div>
              <div className="siem-donut-wrap">
                <DonutChart data={overview.top_mitre} />
              </div>
              <div className="siem-mitre-list">
                {overview.top_mitre.map((item, i) => (
                  <div key={item.technique} className="siem-mitre-row">
                    <span className="siem-mitre-name">
                      <span
                        className="siem-legend-dot"
                        style={{ background: MITRE_COLORS[i] ?? FALLBACK_COLOR }}
                      />
                      {item.technique}
                    </span>
                    <span className="siem-mitre-pct">{item.percentage}%</span>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* ── Agent row ── */}
          <div className="siem-charts-row">
            {/* Top agents bars */}
            <div className="siem-panel siem-panel-narrow">
              <div className="siem-panel-header">
                <span className="siem-panel-title">Top 5 agents</span>
              </div>
              <div className="siem-agents-wrap">
                {overview.top_agents.map((agent, i) => (
                  <AgentBar
                    key={agent.agent_id}
                    agent={agent}
                    color={AGENT_COLORS[i] ?? FALLBACK_COLOR}
                  />
                ))}
              </div>
            </div>

            {/* Agent stacked bar */}
            <div className="siem-panel">
              <div className="siem-panel-header">
                <span className="siem-panel-title">Alert evolution — top 5 agents</span>
              </div>
              <div className="siem-chart-wrap">
                <StackedBarChart
                  series={overview.agent_series}
                  keys={agentKeys}
                  colors={AGENT_COLORS}
                />
              </div>
              <div className="siem-legend">
                {overview.top_agents.map((agent, i) => (
                  <span key={agent.agent_id} className="siem-legend-item">
                    <span
                      className="siem-legend-dot"
                      style={{ background: AGENT_COLORS[i] ?? FALLBACK_COLOR }}
                    />
                    {agent.agent_name}
                  </span>
                ))}
              </div>
            </div>
          </div>
        </>
      ) : null}

      {/* ── Security Alerts table ── */}
      <div className="siem-panel siem-table-panel">
        <div className="siem-panel-header">
          <span className="siem-panel-title">⚠ Security Alerts</span>
        </div>
        {alertsPage ? (
          <>
            <div className="siem-table-wrap">
              <table className="siem-table">
                <thead>
                  <tr>
                    <th />
                    <th>Time</th>
                    <th>Agent</th>
                    <th>Agent name</th>
                    <th>Technique(s)</th>
                    <th>Tactic(s)</th>
                    <th>Description</th>
                    <th>Level</th>
                    <th>Rule ID</th>
                  </tr>
                </thead>
                <tbody>
                  {alertsPage.items.map((alert: SecurityAlert) => (
                    <tr key={alert.id}>
                      <td className="siem-td-chevron">›</td>
                      <td className="siem-td-time">{fmtTime(alert.timestamp)}</td>
                      <td className="siem-td-agent">{alert.agent_id}</td>
                      <td>{alert.agent_name}</td>
                      <td>
                        {alert.technique ? (
                          <Badge label={alert.technique} variant="blue" />
                        ) : null}
                      </td>
                      <td>
                        {alert.tactic ? (
                          <Badge label={alert.tactic} variant="orange" />
                        ) : null}
                      </td>
                      <td className="siem-td-desc">{alert.description}</td>
                      <td>
                        <LevelPill level={alert.level} />
                      </td>
                      <td className="siem-td-ruleid">{alert.rule_id}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="siem-pagination">
              <span className="siem-page-info">
                Rows per page: 10 &nbsp;|&nbsp; Total: {alertsPage.total.toLocaleString()}
              </span>
              <div className="siem-page-btns">
                <button
                  className="siem-pg-btn"
                  type="button"
                  disabled={page === 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  ‹
                </button>
                {shownPages.map((p) => (
                  <button
                    key={p}
                    type="button"
                    className={`siem-pg-btn ${page === p ? "siem-pg-active" : ""}`}
                    onClick={() => setPage(p)}
                  >
                    {p}
                  </button>
                ))}
                <button
                  className="siem-pg-btn"
                  type="button"
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                >
                  ›
                </button>
              </div>
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
}
