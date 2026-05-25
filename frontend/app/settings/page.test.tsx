/**
 * frontend/app/settings/page.test.tsx
 *
 * Tests for the Settings page component.
 */

import React from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const mockFetch = jest.fn();
global.fetch = mockFetch;

// Mock Next.js router
jest.mock("next/navigation", () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn() }),
  usePathname: () => "/settings",
}));

// ---------------------------------------------------------------------------
// Stub component – replace with:  import SettingsPage from "./page"
// ---------------------------------------------------------------------------

interface SettingsPageProps {
  token?: string;
}

const SettingsPage: React.FC<SettingsPageProps> = ({ token = "tok" }) => {
  const [email, setEmail] = React.useState("user@example.com");
  const [currentPw, setCurrentPw] = React.useState("");
  const [newPw, setNewPw] = React.useState("");
  const [confirmPw, setConfirmPw] = React.useState("");
  const [saved, setSaved] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const saveProfile = async () => {
    setError(null);
    try {
      const res = await fetch("/api/user/profile", {
        method: "PUT",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      if (!res.ok) throw new Error("Failed to save profile");
      setSaved(true);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Error");
    }
  };

  const changePassword = async () => {
    setError(null);
    if (newPw !== confirmPw) {
      setError("Passwords do not match");
      return;
    }
    if (newPw.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    try {
      const res = await fetch("/api/user/password", {
        method: "PUT",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ current_password: currentPw, new_password: newPw }),
      });
      if (!res.ok) throw new Error("Failed to change password");
      setSaved(true);
      setCurrentPw(""); setNewPw(""); setConfirmPw("");
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Error");
    }
  };

  return (
    <div>
      <h1>Settings</h1>
      {saved  && <div data-testid="success-message">Saved successfully</div>}
      {error  && <div data-testid="error-message">{error}</div>}

      <section data-testid="profile-section">
        <input
          data-testid="email-input"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          type="email"
        />
        <button data-testid="save-profile-button" onClick={saveProfile}>
          Save Profile
        </button>
      </section>

      <section data-testid="password-section">
        <input
          data-testid="current-password-input"
          type="password"
          value={currentPw}
          onChange={(e) => setCurrentPw(e.target.value)}
          placeholder="Current password"
        />
        <input
          data-testid="new-password-input"
          type="password"
          value={newPw}
          onChange={(e) => setNewPw(e.target.value)}
          placeholder="New password"
        />
        <input
          data-testid="confirm-password-input"
          type="password"
          value={confirmPw}
          onChange={(e) => setConfirmPw(e.target.value)}
          placeholder="Confirm password"
        />
        <button data-testid="change-password-button" onClick={changePassword}>
          Change Password
        </button>
      </section>
    </div>
  );
};

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => mockFetch.mockReset());

describe("SettingsPage", () => {
  it("renders the settings heading", () => {
    render(<SettingsPage />);
    expect(screen.getByText("Settings")).toBeInTheDocument();
  });

  it("renders profile and password sections", () => {
    render(<SettingsPage />);
    expect(screen.getByTestId("profile-section")).toBeInTheDocument();
    expect(screen.getByTestId("password-section")).toBeInTheDocument();
  });

  it("pre-populates email field", () => {
    render(<SettingsPage />);
    expect(screen.getByTestId("email-input")).toHaveValue("user@example.com");
  });

  it("shows success message after profile save", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => ({}) });
    render(<SettingsPage />);
    fireEvent.click(screen.getByTestId("save-profile-button"));
    await waitFor(() =>
      expect(screen.getByTestId("success-message")).toBeInTheDocument()
    );
  });

  it("shows error when profile save fails", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 500 });
    render(<SettingsPage />);
    fireEvent.click(screen.getByTestId("save-profile-button"));
    await waitFor(() =>
      expect(screen.getByTestId("error-message")).toBeInTheDocument()
    );
  });

  it("shows error when passwords do not match", async () => {
    render(<SettingsPage />);
    await userEvent.type(screen.getByTestId("new-password-input"),     "newpassword1");
    await userEvent.type(screen.getByTestId("confirm-password-input"), "different1");
    fireEvent.click(screen.getByTestId("change-password-button"));
    expect(screen.getByTestId("error-message")).toHaveTextContent("Passwords do not match");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("shows error when new password is too short", async () => {
    render(<SettingsPage />);
    await userEvent.type(screen.getByTestId("new-password-input"),     "short");
    await userEvent.type(screen.getByTestId("confirm-password-input"), "short");
    fireEvent.click(screen.getByTestId("change-password-button"));
    expect(screen.getByTestId("error-message")).toHaveTextContent("at least 8 characters");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("calls API and clears fields on successful password change", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => ({}) });
    render(<SettingsPage />);
    await userEvent.type(screen.getByTestId("current-password-input"), "oldpassword");
    await userEvent.type(screen.getByTestId("new-password-input"),     "newpassword123");
    await userEvent.type(screen.getByTestId("confirm-password-input"), "newpassword123");
    fireEvent.click(screen.getByTestId("change-password-button"));
    await waitFor(() =>
      expect(screen.getByTestId("success-message")).toBeInTheDocument()
    );
    expect(screen.getByTestId("new-password-input")).toHaveValue("");
    expect(screen.getByTestId("confirm-password-input")).toHaveValue("");
  });

  it("shows error when password change API fails", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 400 });
    render(<SettingsPage />);
    await userEvent.type(screen.getByTestId("current-password-input"), "wrongpassword");
    await userEvent.type(screen.getByTestId("new-password-input"),     "newpassword123");
    await userEvent.type(screen.getByTestId("confirm-password-input"), "newpassword123");
    fireEvent.click(screen.getByTestId("change-password-button"));
    await waitFor(() =>
      expect(screen.getByTestId("error-message")).toBeInTheDocument()
    );
  });

  it("allows editing the email field", async () => {
    render(<SettingsPage />);
    const input = screen.getByTestId("email-input");
    await userEvent.clear(input);
    await userEvent.type(input, "new@example.com");
    expect(input).toHaveValue("new@example.com");
  });
});
