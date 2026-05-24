const TOKEN_KEY = "chat_app_token";
const EMAIL_KEY = "chat_app_email";
const USERNAME_KEY = "chat_app_username";

function isBrowser(): boolean {
  return typeof window !== "undefined";
}

export function getToken(): string | null {
  if (!isBrowser()) {
    return null;
  }
  return localStorage.getItem(TOKEN_KEY);
}

export function setSession(token: string, email: string, username?: string): void {
  if (!isBrowser()) {
    return;
  }
  localStorage.setItem(TOKEN_KEY, token);
  localStorage.setItem(EMAIL_KEY, email);
  if (typeof username === "string") {
    localStorage.setItem(USERNAME_KEY, username);
  }
}

export function clearSession(): void {
  if (!isBrowser()) {
    return;
  }
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(EMAIL_KEY);
  localStorage.removeItem(USERNAME_KEY);
}

export function getEmail(): string | null {
  if (!isBrowser()) {
    return null;
  }
  return localStorage.getItem(EMAIL_KEY);
}

export function getUsername(): string | null {
  if (!isBrowser()) {
    return null;
  }
  return localStorage.getItem(USERNAME_KEY);
}

export function setUsername(username: string): void {
  if (!isBrowser()) {
    return;
  }
  localStorage.setItem(USERNAME_KEY, username);
}
