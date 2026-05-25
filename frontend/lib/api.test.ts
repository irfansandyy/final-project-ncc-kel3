/**
 * frontend/lib/api.test.ts
 *
 * Tests for the base API utility layer.
 * Adjust BASE_URL and function names to match your actual api.ts exports.
 */

// Mock the global fetch
const mockFetch = jest.fn();
global.fetch = mockFetch;

// ---------------------------------------------------------------------------
// Mirror the shape of your api.ts – replace with real imports once wired:
//   import { apiFetch, buildHeaders, handleApiError } from './api'
// ---------------------------------------------------------------------------

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
  token?: string
): Promise<T> {
  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(options.headers ?? {}),
  };
  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API error ${res.status}: ${text}`);
  }
  return res.json() as Promise<T>;
}

function buildHeaders(token?: string): Record<string, string> {
  const h: Record<string, string> = { "Content-Type": "application/json" };
  if (token) h["Authorization"] = `Bearer ${token}`;
  return h;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  mockFetch.mockReset();
});

describe("apiFetch", () => {
  it("calls the correct URL", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: "ok" }),
    });

    await apiFetch("/health");

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/health"),
      expect.any(Object)
    );
  });

  it("sets Content-Type to application/json", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({}),
    });

    await apiFetch("/health");

    const [, opts] = mockFetch.mock.calls[0];
    expect((opts as RequestInit).headers).toMatchObject({
      "Content-Type": "application/json",
    });
  });

  it("attaches Authorization header when token is provided", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({}),
    });

    await apiFetch("/api/chats", {}, "my-token");

    const [, opts] = mockFetch.mock.calls[0];
    expect((opts as RequestInit).headers).toMatchObject({
      Authorization: "Bearer my-token",
    });
  });

  it("does NOT attach Authorization header when no token", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({}),
    });

    await apiFetch("/health");

    const [, opts] = mockFetch.mock.calls[0];
    expect(
      (opts as RequestInit & { headers: Record<string, string> }).headers
        .Authorization
    ).toBeUndefined();
  });

  it("throws on non-ok response", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      text: async () => "Unauthorized",
    });

    await expect(apiFetch("/api/chats")).rejects.toThrow("API error 401");
  });

  it("throws on 500 with server message", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => "Internal Server Error",
    });

    await expect(apiFetch("/api/siem/alerts")).rejects.toThrow("API error 500");
  });

  it("returns parsed JSON on success", async () => {
    const payload = { id: "1", name: "test" };
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => payload,
    });

    const result = await apiFetch<typeof payload>("/api/something");
    expect(result).toEqual(payload);
  });

  it("passes method and body through to fetch", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({}),
    });

    const body = JSON.stringify({ message: "hello" });
    await apiFetch("/api/chats/1/messages", { method: "POST", body });

    const [, opts] = mockFetch.mock.calls[0];
    expect((opts as RequestInit).method).toBe("POST");
    expect((opts as RequestInit).body).toBe(body);
  });
});

describe("buildHeaders", () => {
  it("always includes Content-Type", () => {
    const h = buildHeaders();
    expect(h["Content-Type"]).toBe("application/json");
  });

  it("includes Authorization when token given", () => {
    const h = buildHeaders("tok123");
    expect(h["Authorization"]).toBe("Bearer tok123");
  });

  it("omits Authorization when no token", () => {
    const h = buildHeaders();
    expect(h["Authorization"]).toBeUndefined();
  });
});
