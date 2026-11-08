import type { User, Session, Server, ServerMetric } from "../types";

export interface LoginCredentials {
  username: string;
  password: string;
}

const API_BASE = "/api";

function headers() {
  return { "Content-Type": "application/json" };
}

export async function apiRequest(
  path: string,
  { method = "GET", body, token }: { method?: string; body?: unknown; token?: string | null } = {}
): Promise<Response> {
  const init: RequestInit = { method, headers: headers() };
  if (token) init.headers = { ...init.headers, Cookie: `lokro_session=${token}` };
  if (body) init.body = JSON.stringify(body);
  return fetch(`${API_BASE}${path}`, init);
}

export function getCookie(name: string): string | undefined {
  if (typeof document === "undefined") return undefined;
  const match = document.cookie.match(new RegExp("(?:^|; )" + name + "=([^;]*)"));
  return match ? match[1] : undefined;
}

export function setCookie(name: string, value: string, seconds: number) {
  if (typeof document === "undefined") return;
  document.cookie = `${name}=${value}; Path=/; Max-Age=${seconds}; SameSite=Lax`;
}

export function deleteCookie(name: string) {
  if (typeof document === "undefined") return;
  document.cookie = `${name}=; Path=/; Max-Age=0; SameSite=Lax`;
}

/* ---------- Auth ---------- */

export async function login(creds: LoginCredentials): Promise<{ user: any }> {
  const res = await apiRequest("/auth/login", { method: "POST", body: creds });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function logout(): Promise<void> {
  const token = getCookie("lokro_session");
  await apiRequest("/auth/logout", { method: "POST", token });
  deleteCookie("lokro_session");
}

export async function me(): Promise<{ user: any }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/auth/me", { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function changePassword(current: string, nw: string): Promise<{ message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/auth/change-password", {
    method: "POST",
    body: { current_password: current, new_password: nw },
    token,
  });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function listSessions(): Promise<{ sessions: Session[] }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/auth/sessions", { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function revokeSession(id: number): Promise<{ message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/auth/sessions/${id}`, { method: "DELETE", token });
  if (!res.ok) throw await res.json();
  return res.json();
}

/* ---------- Users ---------- */

export async function listUsers(): Promise<{ users: User[] }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/users", { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function createUser(body: {
  username: string;
  email: string;
  password: string;
  role: string;
  is_active: boolean;
}): Promise<{ id: number; message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/users", { method: "POST", body, token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function getUser(id: number): Promise<{ user: User }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/users/${id}`, { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function updateUser(
  id: number,
  body: { email?: string; role?: string; is_active?: boolean }
): Promise<{ message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/users/${id}`, { method: "PUT", body, token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function removeUser(id: number): Promise<{ message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/users/${id}`, { method: "DELETE", token });
  if (!res.ok) throw await res.json();
  return res.json();
}

/* ---------- Permissions ---------- */

export async function listUserPermissions(userId: number): Promise<{ permissions: any[] }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/users/${userId}/permissions`, { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function listPermissions(): Promise<{ permissions: any[] }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/permissions", { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function grantPermission(body: {
  user_id: number;
  permission_id: number;
  server_id?: number | null;
  granted: boolean;
}): Promise<{ message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/permissions/grant", { method: "POST", body, token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function revokePermission(body: {
  user_id: number;
  permission_id: number;
}): Promise<{ message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/permissions/revoke", { method: "POST", body, token });
  if (!res.ok) throw await res.json();
  return res.json();
}

/* ---------- Servers ---------- */

export async function listServers(): Promise<{ servers: Server[] }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/servers", { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function createServer(body: {
  uuid: string;
  name: string;
  hostname?: string;
  ip_address?: string;
  os?: string;
  agent_version?: string;
}): Promise<{ id: number; message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/servers", { method: "POST", body, token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function getServer(id: number): Promise<{ server: Server }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/servers/${id}`, { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function updateServer(id: number, body: Partial<Server>): Promise<{ message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/servers/${id}`, { method: "PUT", body, token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function removeServer(id: number): Promise<{ message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/servers/${id}`, { method: "DELETE", token });
  if (!res.ok) throw await res.json();
  return res.json();
}

/* ---------- Metrics ---------- */

export async function serverMetrics(serverId: number, limit = 60): Promise<{ metrics: ServerMetric[] }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/metrics/servers/${serverId}?limit=${limit}`, { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

/* ---------- Uptime ---------- */

export async function listUptimeChecks(): Promise<{ checks: any[] }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/uptime/checks", { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function createUptimeCheck(body: {
  name: string;
  url: string;
  type: string;
  interval?: number;
  timeout?: number;
  expected_status?: number;
  is_active?: boolean;
}): Promise<{ id: number; message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest("/uptime/checks", { method: "POST", body, token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function getUptimeCheck(id: number): Promise<{ check: any }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/uptime/checks/${id}`, { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function updateUptimeCheck(id: number, body: Partial<{
  name: string;
  url: string;
  type: string;
  interval: number;
  timeout: number;
  expected_status: number;
  is_active: boolean;
}>): Promise<{ message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/uptime/checks/${id}`, { method: "PUT", body, token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function removeUptimeCheck(id: number): Promise<{ message: string }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/uptime/checks/${id}`, { method: "DELETE", token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function listUptimeResults(checkId: number, limit = 100): Promise<{ results: any[] }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/uptime/results/${checkId}?limit=${limit}`, { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

export async function getUptimeSummary(checkId: number): Promise<{ summary: any }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/uptime/summary/${checkId}`, { token });
  if (!res.ok) throw await res.json();
  return res.json();
}

/* ---------- Audit ---------- */

export async function listAudit(limit = 100, offset = 0): Promise<{ audit: any[] }> {
  const token = getCookie("lokro_session");
  const res = await apiRequest(`/audit?limit=${limit}&offset=${offset}`, { token });
  if (!res.ok) throw await res.json();
  return res.json();
}
