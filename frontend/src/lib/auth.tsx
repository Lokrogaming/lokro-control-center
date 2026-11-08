import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import type { Role } from "../types";
import { login, logout, me, getCookie } from "./api";

type AuthCtx = {
  user: { id: number; username: string; email: string; role: Role } | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  isAuthenticated: () => boolean;
  hasRole: (roles: Role[]) => boolean;
};

const ctx = createContext<AuthCtx | null>(null);

export function useAuth() {
  const c = useContext(ctx);
  if (!c) throw new Error("useAuth must be used within AuthProvider");
  return c;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<{ id: number; username: string; email: string; role: Role } | null>(null);
  const [loading, setLoading] = useState(true);

  const loadUser = async () => {
    const token = getCookie("lokro_session");
    if (!token) { setLoading(false); return; }
    try {
      const data = await me();
      setUser(data.user);
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadUser();
  }, []);

  const doLogin = async (username: string, password: string) => {
    const data = await login({ username, password });
    setUser(data.user);
  };

  const doLogout = async () => {
    await logout();
    setUser(null);
  };

  const isAuthenticated = () => !!user;
  const hasRole = (roles: Role[]) => !!user && roles.includes(user.role);

  const value = { user, loading, login: doLogin, logout: doLogout, isAuthenticated, hasRole };
  return (
    <ctx.Provider value={value}>
      {children}
    </ctx.Provider>
  );
}
