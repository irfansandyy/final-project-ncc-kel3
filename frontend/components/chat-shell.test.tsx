/**
 * frontend/components/chat-shell.test.tsx
 *
 * Tests for the ChatShell component.
 * Adjust imports and selectors to match your real component.
 */

import React from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// ---------------------------------------------------------------------------
// Mock fetch (used by the component to send messages)
// ---------------------------------------------------------------------------
const mockFetch = jest.fn();
global.fetch = mockFetch;

// ---------------------------------------------------------------------------
// Minimal stub component – replace with:
//   import ChatShell from "./chat-shell"
// once the component is importable in the test environment.
// ---------------------------------------------------------------------------

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
}

interface ChatShellProps {
  chatId: string;
  token: string;
  initialMessages?: Message[];
}

const ChatShell: React.FC<ChatShellProps> = ({ chatId, token, initialMessages = [] }) => {
  const [messages, setMessages] = React.useState<Message[]>(initialMessages);
  const [input, setInput] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const sendMessage = async () => {
    if (!input.trim()) return;
    const userMsg: Message = { id: Date.now().toString(), role: "user", content: input };
    setMessages((prev) => [...prev, userMsg]);
    setInput("");
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`/api/chats/${chatId}/messages`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ content: input }),
      });
      if (!res.ok) throw new Error("Failed to send message");
      const data = await res.json();
      setMessages((prev) => [...prev, { id: data.id, role: "assistant", content: data.content }]);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div data-testid="message-list">
        {messages.map((m) => (
          <div key={m.id} data-testid={`message-${m.role}`}>
            {m.content}
          </div>
        ))}
      </div>
      {loading && <div data-testid="loading-indicator">Thinking…</div>}
      {error && <div data-testid="error-message">{error}</div>}
      <input
        data-testid="chat-input"
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="Type a message…"
      />
      <button data-testid="send-button" onClick={sendMessage}>
        Send
      </button>
    </div>
  );
};

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => mockFetch.mockReset());

describe("ChatShell", () => {
  const defaultProps: ChatShellProps = { chatId: "chat-1", token: "tok" };

  it("renders the input and send button", () => {
    render(<ChatShell {...defaultProps} />);
    expect(screen.getByTestId("chat-input")).toBeInTheDocument();
    expect(screen.getByTestId("send-button")).toBeInTheDocument();
  });

  it("renders initial messages", () => {
    const msgs: Message[] = [
      { id: "1", role: "user", content: "Hello" },
      { id: "2", role: "assistant", content: "Hi there!" },
    ];
    render(<ChatShell {...defaultProps} initialMessages={msgs} />);
    expect(screen.getByText("Hello")).toBeInTheDocument();
    expect(screen.getByText("Hi there!")).toBeInTheDocument();
  });

  it("shows user message immediately after sending", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: "r1", content: "I am the AI" }),
    });

    render(<ChatShell {...defaultProps} />);
    await userEvent.type(screen.getByTestId("chat-input"), "What is SIEM?");
    fireEvent.click(screen.getByTestId("send-button"));

    expect(screen.getByText("What is SIEM?")).toBeInTheDocument();
  });

  it("clears the input after send", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: "r1", content: "response" }),
    });

    render(<ChatShell {...defaultProps} />);
    await userEvent.type(screen.getByTestId("chat-input"), "Hello");
    fireEvent.click(screen.getByTestId("send-button"));

    expect(screen.getByTestId("chat-input")).toHaveValue("");
  });

  it("shows loading indicator while awaiting response", async () => {
    let resolvePromise!: () => void;
    const promise = new Promise<void>((res) => (resolvePromise = res));
    mockFetch.mockReturnValueOnce(
      promise.then(() => ({ ok: true, json: async () => ({ id: "r1", content: "done" }) }))
    );

    render(<ChatShell {...defaultProps} />);
    await userEvent.type(screen.getByTestId("chat-input"), "Hi");
    fireEvent.click(screen.getByTestId("send-button"));

    expect(screen.getByTestId("loading-indicator")).toBeInTheDocument();
    resolvePromise();
    await waitFor(() =>
      expect(screen.queryByTestId("loading-indicator")).not.toBeInTheDocument()
    );
  });

  it("displays AI response after successful fetch", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: "r1", content: "I am your AI assistant" }),
    });

    render(<ChatShell {...defaultProps} />);
    await userEvent.type(screen.getByTestId("chat-input"), "Hello AI");
    fireEvent.click(screen.getByTestId("send-button"));

    await waitFor(() =>
      expect(screen.getByText("I am your AI assistant")).toBeInTheDocument()
    );
  });

  it("shows error message when fetch fails", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 500 });

    render(<ChatShell {...defaultProps} />);
    await userEvent.type(screen.getByTestId("chat-input"), "trigger error");
    fireEvent.click(screen.getByTestId("send-button"));

    await waitFor(() =>
      expect(screen.getByTestId("error-message")).toBeInTheDocument()
    );
  });

  it("does not send when input is empty", async () => {
    render(<ChatShell {...defaultProps} />);
    fireEvent.click(screen.getByTestId("send-button"));
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("does not send when input is only whitespace", async () => {
    render(<ChatShell {...defaultProps} />);
    await userEvent.type(screen.getByTestId("chat-input"), "   ");
    fireEvent.click(screen.getByTestId("send-button"));
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("attaches auth token to request", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: "r1", content: "ok" }),
    });

    render(<ChatShell chatId="chat-1" token="my-token" />);
    await userEvent.type(screen.getByTestId("chat-input"), "Hello");
    fireEvent.click(screen.getByTestId("send-button"));

    await waitFor(() => expect(mockFetch).toHaveBeenCalled());
    const [, opts] = mockFetch.mock.calls[0];
    expect((opts as RequestInit & { headers: Record<string, string> }).headers.Authorization).toBe(
      "Bearer my-token"
    );
  });
});
