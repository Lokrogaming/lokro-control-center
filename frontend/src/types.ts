export type Role = "super_admin" | "admin" | "user";

export interface User {
  id: number;
  username: string;
  email: string;
  role: Role;
  is_active: boolean;
  failed_logins?: number;
  last_login_at?: string;
  created_at: string;
  updated_at: string;
}

export interface Session {
  id: number;
  expires_at: string;
  created_at: string;
  ip_address?: string;
  user_agent?: string;
  is_revoked: boolean;
  current?: boolean;
}

export interface Permission {
  id: number;
  name: string;
  description: string;
  resource: string;
  action: string;
}

export interface UserPermission {
  id: number;
  user_id: number;
  permission_id: number;
  server_id?: number | null;
  granted: boolean;
  permission?: Permission;
}

export interface Server {
  id: number;
  uuid: string;
  name: string;
  hostname?: string;
  ip_address?: string;
  os?: string;
  agent_version?: string;
  status: string;
  last_heartbeat?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ServerMetric {
  id: number;
  server_id: number;
  recorded_at: string;
  cpu_percent?: number;
  load_1?: number;
  load_5?: number;
  load_15?: number;
  mem_total?: number;
  mem_used?: number;
  mem_free?: number;
  mem_cached?: number;
  swap_total?: number;
  swap_used?: number;
  uptime?: number;
  process_count?: number;
}

export interface AuditLog {
  id: number;
  user_id: number;
  action: string;
  resource: string;
  resource_id?: string;
  details?: string;
  ip_address?: string;
  user_agent?: string;
  created_at: string;
}

export interface AuthUser {
  id: number;
  username: string;
  email: string;
  role: Role;
}

export interface ApiError {
  error: string;
}
