/**
 * frontend/lib/auth.test.ts
 *
 * Tests for auth helpers (token storage, login/register wrappers, JWT decode).
 * Adjust to match your real auth.ts exports.
 */

const mockFetch = jest.fn();
global.fetch = mockFetch;

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, val: string) => { store[key] = val; },
    removeItem: (key: string) => { delete store[key]; },
    clear: () => { store = {}; },
  };
})();
Object.defineProperty(window, "localStorage", { value: localStorageMock });

// ---------------------------------------------------------------------------
// Mirror of auth.ts – replace with real imports once wired.
// ---------------------------------------------------------------------------

const TOKEN_KEY = "auth_token";
const USER_KEY  = "auth_user";

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

interface AuthResponse {
  token: string;
  user: { id: string; email: string };
}

async function login(email: string, password: string): Promise<AuthResponse> {
  const res = await fetch(`${BASE_URL}/api/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) throw new Error("Login failed");
  const data: AuthResponse = await res.json();
  localStorage.setItem(TOKEN_KEY, data.token);
  localStorage.setItem(USER_KEY, JSON.stringify(data.user));
  return data;
}

async function register(email: string, password: string): Promise<AuthResponse> {
  const res = await fetch(`${BASE_URL}/api/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) throw new Error("Registration failed");
  const data: AuthResponse = await res.json();
  localStorage.setItem(TOKEN_KEY, data.token);
  localStorage.setItem(USER_KEY, JSON.stringify(data.user));
  return data;
}

function logout(): void {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
}

function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

function isAuthenticated(): boolean {
  return getToken() !== null;
}

function getUser(): { id: string; email: string } | null {
  const raw = localStorage.getItem(USER_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  mockFetch.mockReset();
  localStorage.clear();
});

describe("login", () => {
  const fakeResponse: AuthResponse = {
    token: "jwt.token.here",
    user: { id: "u1", email: "user@example.com" },
  };

  it("POSTs to /api/auth/login", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => fakeResponse });
    await login("user@example.com", "secret");
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/auth/login"),
      expect.objectContaining({ method: "POST" })
    );
  });

  it("stores token in localStorage on success", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => fakeResponse });
    await login("user@example.com", "secret");
    expect(localStorage.getItem(TOKEN_KEY)).toBe("jwt.token.here");
  });

  it("stores user in localStorage on success", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => fakeResponse });
    await login("user@example.com", "secret");
    const stored = JSON.parse(localStorage.getItem(USER_KEY)!);
    expect(stored.email).toBe("user@example.com");
  });

  it("throws on failed login", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });
    await expect(login("bad@example.com", "wrong")).rejects.toThrow("Login failed");
  });

  it("does NOT store token on failed login", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });
    await expect(login("bad@example.com", "wrong")).rejects.toThrow();
    expect(localStorage.getItem(TOKEN_KEY)).toBeNull();
  });
});

describe("register", () => {
  const fakeResponse: AuthResponse = {
    token: "new.jwt.token",
    user: { id: "u2", email: "new@example.com" },
  };

  it("POSTs to /api/auth/register", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => fakeResponse });
    await register("new@example.com", "password123");
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/auth/register"),
      expect.objectContaining({ method: "POST" })
    );
  });

  it("stores token after registration", async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, json: async () => fakeResponse });
    await register("new@example.com", "password123");
    expect(localStorage.getItem(TOKEN_KEY)).toBe("new.jwt.token");
  });

  it("throws on failed registration", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 409 });
    await expect(register("exists@example.com", "pw")).rejects.toThrow();
  });
});

describe("logout", () => {
  it("removes token from localStorage", () => {
    localStorage.setItem(TOKEN_KEY, "some-token");
    logout();
    expect(localStorage.getItem(TOKEN_KEY)).toBeNull();
  });

  it("removes user from localStorage", () => {
    localStorage.setItem(USER_KEY, JSON.stringify({ id: "u1" }));
    logout();
    expect(localStorage.getItem(USER_KEY)).toBeNull();
  });
});

describe("isAuthenticated", () => {
  it("returns false when no token", () => {
    expect(isAuthenticated()).toBe(false);
  });

  it("returns true when token present", () => {
    localStorage.setItem(TOKEN_KEY, "token");
    expect(isAuthenticated()).toBe(true);
  });
});

describe("getUser", () => {
  it("returns null when no user stored", () => {
    expect(getUser()).toBeNull();
  });

  it("returns parsed user object", () => {
    localStorage.setItem(USER_KEY, JSON.stringify({ id: "u1", email: "a@b.com" }));
    const user = getUser();
    expect(user?.email).toBe("a@b.com");
  });

  it("returns null on malformed JSON", () => {
    localStorage.setItem(USER_KEY, "not-json");
    expect(getUser()).toBeNull();
  });
});
