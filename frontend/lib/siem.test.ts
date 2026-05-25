/**
 * frontend/lib/siem.test.ts
 *
 * Tests for SIEM data-fetching helpers.
 * Adjust to match your real siem.ts exports.
 */

const mockFetch = jest.fn();
global.fetch = mockFetch;

// ---------------------------------------------------------------------------
// Mirror of siem.ts – replace with real imports once wired.
// ---------------------------------------------------------------------------

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export interface SiemAlert {
  id: string;
  severity: "critical" | "high" | "medium" | "low" | "info";
  message: string;
  source_ip?: string;
  timestamp: string;
}

export interface SiemOverview {
  total_alerts: number;
  critical_alerts: number;
  high_alerts: number;
  logs_ingested: number;
  status: string;
}

async function fetchAlerts(token: string): Promise<SiemAlert[]> {
  const res = await fetch(`${BASE_URL}/api/siem/alerts`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) throw new Error(`Failed to fetch alerts: ${res.status}`);
  return res.json();
}

async function fetchOverview(token: string): Promise<SiemOverview> {
  const res = await fetch(`${BASE_URL}/api/siem/overview`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) throw new Error(`Failed to fetch overview: ${res.status}`);
  return res.json();
}

async function dismissAlert(token: string, alertId: string): Promise<void> {
  const res = await fetch(`${BASE_URL}/api/siem/alerts/${alertId}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) throw new Error(`Failed to dismiss alert: ${res.status}`);
}

function filterBySeverity(alerts: SiemAlert[], severity: SiemAlert["severity"]): SiemAlert[] {
  return alerts.filter((a) => a.severity === severity);
}

function sortByTimestamp(alerts: SiemAlert[], order: "asc" | "desc" = "desc"): SiemAlert[] {
  return [...alerts].sort((a, b) => {
    const diff = new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime();
    return order === "asc" ? diff : -diff;
  });
}

function severityToColor(severity: SiemAlert["severity"]): string {
  const map: Record<SiemAlert["severity"], string> = {
    critical: "#ef4444",
    high: "#f97316",
    medium: "#eab308",
    low: "#3b82f6",
    info: "#6b7280",
  };
  return map[severity] ?? "#6b7280";
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

const TOKEN = "test-token";

beforeEach(() => mockFetch.mockReset());

describe("fetchAlerts", () => {
  it("calls /api/siem/alerts with auth header", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => [] });
    await fetchAlerts(TOKEN);
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/siem/alerts"),
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: `Bearer ${TOKEN}` }),
      })
    );
  });

  it("returns array of alerts", async () => {
    const alerts: SiemAlert[] = [
      { id: "1", severity: "critical", message: "Brute force", timestamp: "2024-01-15T10:00:00Z" },
    ];
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => alerts });
    const result = await fetchAlerts(TOKEN);
    expect(result).toHaveLength(1);
    expect(result[0].severity).toBe("critical");
  });

  it("throws on non-ok response", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 403 });
    await expect(fetchAlerts(TOKEN)).rejects.toThrow("Failed to fetch alerts: 403");
  });

  it("returns empty array when no alerts", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => [] });
    const result = await fetchAlerts(TOKEN);
    expect(result).toEqual([]);
  });
});

describe("fetchOverview", () => {
  it("calls /api/siem/overview", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ total_alerts: 10, critical_alerts: 2, high_alerts: 3, logs_ingested: 5000, status: "ok" }),
    });
    await fetchOverview(TOKEN);
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/siem/overview"),
      expect.any(Object)
    );
  });

  it("returns overview data", async () => {
    const overview: SiemOverview = { total_alerts: 10, critical_alerts: 2, high_alerts: 3, logs_ingested: 5000, status: "healthy" };
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => overview });
    const result = await fetchOverview(TOKEN);
    expect(result.status).toBe("healthy");
    expect(result.total_alerts).toBe(10);
  });

  it("throws on 401", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });
    await expect(fetchOverview(TOKEN)).rejects.toThrow();
  });
});

describe("dismissAlert", () => {
  it("sends DELETE request", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true });
    await dismissAlert(TOKEN, "alert-123");
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/alert-123"),
      expect.objectContaining({ method: "DELETE" })
    );
  });

  it("throws on failure", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 404 });
    await expect(dismissAlert(TOKEN, "bad-id")).rejects.toThrow();
  });
});

describe("filterBySeverity", () => {
  const alerts: SiemAlert[] = [
    { id: "1", severity: "critical", message: "A", timestamp: "2024-01-01T00:00:00Z" },
    { id: "2", severity: "low",      message: "B", timestamp: "2024-01-02T00:00:00Z" },
    { id: "3", severity: "critical", message: "C", timestamp: "2024-01-03T00:00:00Z" },
  ];

  it("returns only matching severity", () => {
    expect(filterBySeverity(alerts, "critical")).toHaveLength(2);
  });

  it("returns empty array when no matches", () => {
    expect(filterBySeverity(alerts, "high")).toHaveLength(0);
  });
});

describe("sortByTimestamp", () => {
  const alerts: SiemAlert[] = [
    { id: "1", severity: "info", message: "A", timestamp: "2024-01-01T00:00:00Z" },
    { id: "2", severity: "info", message: "B", timestamp: "2024-01-03T00:00:00Z" },
    { id: "3", severity: "info", message: "C", timestamp: "2024-01-02T00:00:00Z" },
  ];

  it("sorts descending by default", () => {
    const sorted = sortByTimestamp(alerts);
    expect(sorted[0].id).toBe("2");
    expect(sorted[2].id).toBe("1");
  });

  it("sorts ascending when specified", () => {
    const sorted = sortByTimestamp(alerts, "asc");
    expect(sorted[0].id).toBe("1");
    expect(sorted[2].id).toBe("2");
  });

  it("does not mutate original array", () => {
    const copy = [...alerts];
    sortByTimestamp(alerts, "asc");
    expect(alerts).toEqual(copy);
  });
});

describe("severityToColor", () => {
  it("returns red for critical", () => {
    expect(severityToColor("critical")).toBe("#ef4444");
  });
  it("returns orange for high", () => {
    expect(severityToColor("high")).toBe("#f97316");
  });
  it("returns yellow for medium", () => {
    expect(severityToColor("medium")).toBe("#eab308");
  });
  it("returns blue for low", () => {
    expect(severityToColor("low")).toBe("#3b82f6");
  });
  it("returns grey for info", () => {
    expect(severityToColor("info")).toBe("#6b7280");
  });
});
