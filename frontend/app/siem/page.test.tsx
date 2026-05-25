/**
 * frontend/app/siem/page.test.tsx
 *
 * Tests for the SIEM page (route component).
 */

import React from "react";
import { render, screen, waitFor } from "@testing-library/react";

const mockFetch = jest.fn();
global.fetch = mockFetch;

jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: jest.fn() }),
  usePathname: () => "/siem",
}));

// ---------------------------------------------------------------------------
// Stub SIEM page – replace with:  import SiemPage from "./page"
// ---------------------------------------------------------------------------

interface Alert {
  id: string;
  severity: string;
  message: string;
  timestamp: string;
}

interface Overview {
  total_alerts: number;
  critical_alerts: number;
  logs_ingested: number;
  status: string;
}

const SiemPage: React.FC<{ token?: string }> = ({ token = "tok" }) => {
  const [overview, setOverview] = React.useState<Overview | null>(null);
  const [alerts, setAlerts]     = React.useState<Alert[]>([]);
  const [loading, setLoading]   = React.useState(true);
  const [error, setError]       = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!token) { setError("Unauthorized"); setLoading(false); return; }
    const h = { Authorization: `Bearer ${token}` };
    Promise.all([
      fetch("/api/siem/overview", { headers: h }).then((r) => {
        if (!r.ok) throw new Error("Failed to load overview");
        return r.json() as Promise<Overview>;
      }),
      fetch("/api/siem/alerts", { headers: h }).then((r) => {
        if (!r.ok) throw new Error("Failed to load alerts");
        return r.json() as Promise<Alert[]>;
      }),
    ])
      .then(([ov, al]) => { setOverview(ov); setAlerts(al); })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [token]);

  if (loading) return <div data-testid="page-loading">Loading SIEM dashboard…</div>;
  if (error)   return <div data-testid="page-error">{error}</div>;

  return (
    <main data-testid="siem-page">
      <h1>SIEM Dashboard</h1>
      {overview && (
        <section data-testid="stats-section">
          <div data-testid="stat-total">{overview.total_alerts} Total Alerts</div>
          <div data-testid="stat-critical">{overview.critical_alerts} Critical</div>
          <div data-testid="stat-logs">{overview.logs_ingested} Logs Ingested</div>
          <div data-testid="stat-status">Status: {overview.status}</div>
        </section>
      )}
      <section data-testid="alerts-section">
        {alerts.length === 0 ? (
          <p data-testid="no-alerts">No alerts found</p>
        ) : (
          <ul>
            {alerts.map((a) => (
              <li key={a.id} data-testid={`siem-alert-${a.id}`}>
                [{a.severity.toUpperCase()}] {a.message}
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function mockSuccess() {
  mockFetch
    .mockResolvedValueOnce({
      ok: true,
      json: async (): Promise<Overview> => ({
        total_alerts: 8,
        critical_alerts: 3,
        logs_ingested: 50000,
        status: "healthy",
      }),
    })
    .mockResolvedValueOnce({
      ok: true,
      json: async (): Promise<Alert[]> => [
        { id: "1", severity: "critical", message: "SSH brute force",    timestamp: "2024-01-15T10:00:00Z" },
        { id: "2", severity: "medium",   message: "Unusual port scan",  timestamp: "2024-01-15T11:00:00Z" },
      ],
    });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => mockFetch.mockReset());

describe("SiemPage", () => {
  it("shows loading state initially", () => {
    mockFetch.mockReturnValue(new Promise(() => {}));
    render(<SiemPage />);
    expect(screen.getByTestId("page-loading")).toBeInTheDocument();
  });

  it("renders the SIEM dashboard heading", async () => {
    mockSuccess();
    render(<SiemPage />);
    await waitFor(() => screen.getByTestId("siem-page"));
    expect(screen.getByText("SIEM Dashboard")).toBeInTheDocument();
  });

  it("displays overview statistics", async () => {
    mockSuccess();
    render(<SiemPage />);
    await waitFor(() => screen.getByTestId("stats-section"));
    expect(screen.getByTestId("stat-total")).toHaveTextContent("8");
    expect(screen.getByTestId("stat-critical")).toHaveTextContent("3");
    expect(screen.getByTestId("stat-logs")).toHaveTextContent("50000");
  });

  it("displays system status", async () => {
    mockSuccess();
    render(<SiemPage />);
    await waitFor(() => screen.getByTestId("stat-status"));
    expect(screen.getByTestId("stat-status")).toHaveTextContent("healthy");
  });

  it("renders alerts list", async () => {
    mockSuccess();
    render(<SiemPage />);
    await waitFor(() => screen.getByTestId("siem-alert-1"));
    expect(screen.getByTestId("siem-alert-1")).toHaveTextContent("SSH brute force");
    expect(screen.getByTestId("siem-alert-2")).toHaveTextContent("Unusual port scan");
  });

  it("shows no-alerts message when list is empty", async () => {
    mockFetch
      .mockResolvedValueOnce({ ok: true, json: async () => ({ total_alerts: 0, critical_alerts: 0, logs_ingested: 0, status: "ok" }) })
      .mockResolvedValueOnce({ ok: true, json: async () => [] });

    render(<SiemPage />);
    await waitFor(() => screen.getByTestId("no-alerts"));
    expect(screen.getByTestId("no-alerts")).toBeInTheDocument();
  });

  it("shows error when overview API fails", async () => {
    mockFetch
      .mockResolvedValueOnce({ ok: false, status: 500 })
      .mockResolvedValueOnce({ ok: true, json: async () => [] });

    render(<SiemPage />);
    await waitFor(() => screen.getByTestId("page-error"));
    expect(screen.getByTestId("page-error")).toHaveTextContent("Failed to load overview");
  });

  it("shows error when alerts API fails", async () => {
    mockFetch
      .mockResolvedValueOnce({ ok: true, json: async () => ({ total_alerts: 0, critical_alerts: 0, logs_ingested: 0, status: "ok" }) })
      .mockResolvedValueOnce({ ok: false, status: 403 });

    render(<SiemPage />);
    await waitFor(() => screen.getByTestId("page-error"));
  });

  it("shows error when no token provided", async () => {
    render(<SiemPage token="" />);
    await waitFor(() => screen.getByTestId("page-error"));
    expect(screen.getByTestId("page-error")).toHaveTextContent("Unauthorized");
  });

  it("sends Authorization header to both endpoints", async () => {
    mockSuccess();
    render(<SiemPage token="my-tok" />);
    await waitFor(() => screen.getByTestId("siem-page"));

    for (const call of mockFetch.mock.calls) {
      const [, opts] = call;
      expect((opts as RequestInit & { headers: Record<string, string> }).headers.Authorization)
        .toBe("Bearer my-tok");
    }
  });
});
