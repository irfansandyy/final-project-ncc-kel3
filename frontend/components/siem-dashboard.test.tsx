/**
 * frontend/components/siem-dashboard.test.tsx
 *
 * Tests for the SiemDashboard component.
 */

import React from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const mockFetch = jest.fn();
global.fetch = mockFetch;

// ---------------------------------------------------------------------------
// Stub component – replace with:  import SiemDashboard from "./siem-dashboard"
// ---------------------------------------------------------------------------

interface Alert {
  id: string;
  severity: "critical" | "high" | "medium" | "low" | "info";
  message: string;
  source_ip?: string;
  timestamp: string;
}

interface Overview {
  total_alerts: number;
  critical_alerts: number;
  logs_ingested: number;
  status: string;
}

interface SiemDashboardProps {
  token: string;
}

const SiemDashboard: React.FC<SiemDashboardProps> = ({ token }) => {
  const [overview, setOverview] = React.useState<Overview | null>(null);
  const [alerts, setAlerts] = React.useState<Alert[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [filter, setFilter] = React.useState<string>("all");

  React.useEffect(() => {
    const headers = { Authorization: `Bearer ${token}` };
    Promise.all([
      fetch("/api/siem/overview", { headers }).then((r) => {
        if (!r.ok) throw new Error("overview failed");
        return r.json();
      }),
      fetch("/api/siem/alerts", { headers }).then((r) => {
        if (!r.ok) throw new Error("alerts failed");
        return r.json();
      }),
    ])
      .then(([ov, al]) => {
        setOverview(ov);
        setAlerts(al);
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [token]);

  const dismiss = async (id: string) => {
    await fetch(`/api/siem/alerts/${id}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${token}` },
    });
    setAlerts((prev) => prev.filter((a) => a.id !== id));
  };

  const visible = filter === "all" ? alerts : alerts.filter((a) => a.severity === filter);

  if (loading) return <div data-testid="loading">Loading…</div>;
  if (error)   return <div data-testid="error">{error}</div>;

  return (
    <div>
      {overview && (
        <div data-testid="overview">
          <span data-testid="total-alerts">{overview.total_alerts}</span>
          <span data-testid="critical-count">{overview.critical_alerts}</span>
          <span data-testid="logs-ingested">{overview.logs_ingested}</span>
          <span data-testid="status">{overview.status}</span>
        </div>
      )}
      <select
        data-testid="severity-filter"
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
      >
        <option value="all">All</option>
        <option value="critical">Critical</option>
        <option value="high">High</option>
        <option value="medium">Medium</option>
        <option value="low">Low</option>
      </select>
      <ul data-testid="alert-list">
        {visible.map((a) => (
          <li key={a.id} data-testid={`alert-${a.id}`}>
            <span data-testid={`severity-${a.id}`}>{a.severity}</span>
            <span>{a.message}</span>
            <button onClick={() => dismiss(a.id)} data-testid={`dismiss-${a.id}`}>
              Dismiss
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const makeOverview = (): Overview => ({
  total_alerts: 5,
  critical_alerts: 2,
  logs_ingested: 10000,
  status: "healthy",
});

const makeAlerts = (): Alert[] => [
  { id: "a1", severity: "critical", message: "Brute force detected", timestamp: "2024-01-15T10:00:00Z" },
  { id: "a2", severity: "low",      message: "Normal scan",           timestamp: "2024-01-15T11:00:00Z" },
  { id: "a3", severity: "high",     message: "Privilege escalation",  timestamp: "2024-01-15T12:00:00Z" },
];

function setupSuccessMocks(overview = makeOverview(), alerts = makeAlerts()) {
  mockFetch
    .mockResolvedValueOnce({ ok: true, json: async () => overview })
    .mockResolvedValueOnce({ ok: true, json: async () => alerts });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => mockFetch.mockReset());

describe("SiemDashboard", () => {
  it("shows loading initially", () => {
    mockFetch.mockReturnValue(new Promise(() => {})); // never resolves
    render(<SiemDashboard token="tok" />);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders overview stats after load", async () => {
    setupSuccessMocks();
    render(<SiemDashboard token="tok" />);
    await waitFor(() => expect(screen.getByTestId("overview")).toBeInTheDocument());
    expect(screen.getByTestId("total-alerts").textContent).toBe("5");
    expect(screen.getByTestId("critical-count").textContent).toBe("2");
  });

  it("renders alert list", async () => {
    setupSuccessMocks();
    render(<SiemDashboard token="tok" />);
    await waitFor(() => expect(screen.getByTestId("alert-a1")).toBeInTheDocument());
    expect(screen.getByTestId("alert-a2")).toBeInTheDocument();
    expect(screen.getByTestId("alert-a3")).toBeInTheDocument();
  });

  it("shows error message when overview fails", async () => {
    mockFetch
      .mockResolvedValueOnce({ ok: false, status: 500 })
      .mockResolvedValueOnce({ ok: true, json: async () => [] });
    render(<SiemDashboard token="tok" />);
    await waitFor(() => expect(screen.getByTestId("error")).toBeInTheDocument());
  });

  it("shows error message when alerts fetch fails", async () => {
    mockFetch
      .mockResolvedValueOnce({ ok: true, json: async () => makeOverview() })
      .mockResolvedValueOnce({ ok: false, status: 403 });
    render(<SiemDashboard token="tok" />);
    await waitFor(() => expect(screen.getByTestId("error")).toBeInTheDocument());
  });

  it("filters alerts by severity", async () => {
    setupSuccessMocks();
    render(<SiemDashboard token="tok" />);
    await waitFor(() => screen.getByTestId("severity-filter"));

    await userEvent.selectOptions(screen.getByTestId("severity-filter"), "critical");

    expect(screen.getByTestId("alert-a1")).toBeInTheDocument();
    expect(screen.queryByTestId("alert-a2")).not.toBeInTheDocument();
    expect(screen.queryByTestId("alert-a3")).not.toBeInTheDocument();
  });

  it("dismisses an alert and removes it from the list", async () => {
    setupSuccessMocks();
    mockFetch.mockResolvedValueOnce({ ok: true }); // DELETE

    render(<SiemDashboard token="tok" />);
    await waitFor(() => screen.getByTestId("dismiss-a1"));

    await userEvent.click(screen.getByTestId("dismiss-a1"));

    await waitFor(() =>
      expect(screen.queryByTestId("alert-a1")).not.toBeInTheDocument()
    );
    // Other alerts remain
    expect(screen.getByTestId("alert-a2")).toBeInTheDocument();
  });

  it("displays severity label on each alert", async () => {
    setupSuccessMocks();
    render(<SiemDashboard token="tok" />);
    await waitFor(() => screen.getByTestId("severity-a1"));
    expect(screen.getByTestId("severity-a1").textContent).toBe("critical");
    expect(screen.getByTestId("severity-a3").textContent).toBe("high");
  });

  it("shows logs_ingested count", async () => {
    setupSuccessMocks();
    render(<SiemDashboard token="tok" />);
    await waitFor(() => screen.getByTestId("logs-ingested"));
    expect(screen.getByTestId("logs-ingested").textContent).toBe("10000");
  });
});
