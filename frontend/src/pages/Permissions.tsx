import { useState, useEffect } from "react";
import { ShieldAlert, Plus, ChevronRight, RefreshCw } from "lucide-react";
import { listUsers, listPermissions, listUserPermissions, grantPermission, revokePermission } from "../lib/api";
import type { User, Permission } from "../types";

export default function Permissions() {
  const [users, setUsers] = useState<User[]>([]);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  // grants[userId][permissionId] = {granted, server_id}
  const [grants, setGrants] = useState<Map<number, Map<number, { granted: boolean; server_id: number | null }>>>(new Map());
  const [loading, setLoading] = useState({ users: false, perms: false, user: new Set<number>() });
  const [error, setError] = useState("");
  const [expanded, setExpanded] = useState<Set<number>>(new Set());

  useEffect(() => {
    Promise.all([loadUsers(), loadPermissions()]).catch((e) => setError(String(e)));
  }, []);

  const loadUsers = async () => {
    setLoading((s) => ({ ...s, users: true }));
    try {
      const data = await listUsers();
      setUsers(data.users);
      data.users.forEach((u) => loadUserPerms(u.id));
    } catch (err: any) {
      setError("Nutzer konnten nicht geladen werden: " + (err?.error || err));
    } finally {
      setLoading((s) => ({ ...s, users: false }));
    }
  };

  const loadPermissions = async () => {
    setLoading((s) => ({ ...s, perms: true }));
    try {
      const data = await listPermissions();
      setPermissions(data.permissions);
    } catch (err: any) {
      setError("Berechtigungen konnten nicht geladen werden: " + (err?.error || err));
    } finally {
      setLoading((s) => ({ ...s, perms: false }));
    }
  };

  const loadUserPerms = async (userId: number) => {
    setLoading((s) => ({ ...s, user: new Set([...s.user, userId]) }));
    try {
      const data = await listUserPermissions(userId);
      const m = new Map<number, { granted: boolean; server_id: number | null }>();
      data.permissions.forEach((p: any) => {
        m.set(p.permission_id, { granted: p.granted, server_id: p.server_id ?? null });
      });
      setGrants((g) => {
        const ng = new Map(g);
        ng.set(userId, m);
        return ng;
      });
    } catch (err: any) {
      setError("Berechtigungen für Nutzer " + userId + " konnten nicht geladen werden: " + (err?.error || err));
    } finally {
      setLoading((s) => { const ns = new Set(s.user); ns.delete(userId); return { ...s, user: ns }; });
    }
  };

  const toggle = async (userId: number, permId: number) => {
    const userMap = grants.get(userId);
    const cur = userMap?.get(permId);
    const nextGranted = !(cur?.granted ?? false);
    // optimistic
    setGrants((g) => {
      const ng = new Map(g);
      const um = new Map(ng.get(userId) ?? []);
      um.set(permId, { granted: nextGranted, server_id: cur?.server_id ?? null });
      ng.set(userId, um);
      return ng;
    });
    try {
      if (nextGranted) {
        await grantPermission({ user_id: userId, permission_id: permId, server_id: null, granted: true });
      } else {
        await revokePermission({ user_id: userId, permission_id: permId });
      }
      // refresh to reflect server truth
      loadUserPerms(userId);
    } catch (err: any) {
      // revert optimistic
      loadUserPerms(userId);
      setError(err?.error || "Fehler beim Ändern der Berechtigung");
    }
  };

  const [serverInput, setServerInput] = useState<{ user: number; perm: number; value: string }>({ user: 0, perm: 0, value: "" });

  const grantScoped = async (userId: number, permId: number) => {
    const sid = serverInput.value === "" ? null : Number(serverInput.value);
    try {
      await grantPermission({ user_id: userId, permission_id: permId, server_id: sid, granted: true });
      loadUserPerms(userId);
      setServerInput({ user: 0, perm: 0, value: "" });
    } catch (err: any) {
      setError(err?.error || "Zuweisung fehlgeschlagen");
    }
  };

  const userGrantMap = (userId: number) => grants.get(userId) ?? new Map();

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-100">Berechtigungs-Matrix</h1>
          <p className="text-sm text-slate-500">Granulare Rechte pro Nutzer und (optional) Server. Zyklus: keines → erteilt → verweigert.</p>
        </div>
        <button className="btn-ghost" title="Aktualisieren">
          <RefreshCw size={16} className="mr-2" /> Aktualisieren
        </button>
      </div>

      {error && <div className="rounded bg-[#7f1d1d]/10 border border-[#9f2525]/30 p-3 text-sm text-red-400">{error}</div>}

      {loading.users || loading.perms ? (
        <div className="panel p-8 text-center text-slate-500 text-sm">Lade…</div>
      ) : (
        <div className="grid gap-4">
          {users.map((u) => {
            const gm = userGrantMap(u.id);
            const isExpanded = expanded.has(u.id);
            const userLoading = loading.user.has(u.id);
            return (
              <div key={u.id} className="panel p-5 border-[#1f2a33]">
                <div className="flex items-center justify-between mb-4 cursor-pointer" onClick={() => {
                  if (isExpanded) setExpanded((s) => { const n = new Set(s); n.delete(u.id); return n; });
                  else setExpanded((s) => { const n = new Set(s); n.add(u.id); return n; });
                }}>
                  <div className="flex items-center gap-2">
                    <div className={`w-2 h-2 rounded-full ${u.role === "super_admin" ? "bg-red-500" : u.role === "admin" ? "bg-cyan-500" : "bg-green-500"}`} />
                    <span className="font-medium">{u.username}</span>
                    <span className="text-xs text-slate-500 ml-auto">{u.role === "super_admin" ? "Super Admin" : u.role === "admin" ? "Admin" : "Nutzer"}</span>
                  </div>
                  <ChevronRight size={16} className={`transition-transform ${isExpanded ? "rotate-90" : ""}`} />
                </div>

                {isExpanded && (
                  <div className="space-y-2">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-2 mb-3">
                      <div className="text-xs font-semibold text-slate-500 uppercase">Berechtigung</div>
                      <div className="text-xs font-semibold text-slate-500 uppercase text-right">Status</div>
                      <div className="text-xs font-semibold text-slate-500 uppercase text-right">Server-ID</div>
                    </div>
                    {permissions.map((p) => {
                      const cur = gm.get(p.id);
                      const granted = cur?.granted ?? false;
                      return (
                        <div key={p.id} className="flex items-center justify-between px-3 py-2 rounded bg-[#0f141a]/50 border border-transparent hover:border-[#1f2a33]">
                          <div className="flex items-center gap-3">
                            <ShieldAlert size={14} className={granted ? "text-cyan-400" : "text-slate-600"} />
                            <span className="text-sm">{p.resource}:{p.action}</span>
                            <span className="ml-auto text-[10px] font-medium text-slate-500">{p.description}</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <button
                              onClick={() => toggle(u.id, p.id)}
                              disabled={userLoading}
                              className={`rounded px-2.5 py-1.5 text-xs transition ${
                                granted
                                  ? "bg-green-900/30 text-green-400 border border-green-500/30 hover:bg-green-900/50"
                                  : "bg-[#1f2a33] text-slate-400 border border-[#1f2a33] hover:bg-[#273542] hover:text-slate-200"
                              }`}
                            >
                              {granted ? "Erteilt" : "Erteilen"}
                            </button>
                            {!granted && (
                              <div className="flex items-center gap-2">
                                <input
                                  className="input text-xs w-16"
                                  placeholder="Server"
                                  value={serverInput.user === u.id && serverInput.perm === p.id ? serverInput.value : ""}
                                  onChange={(e) => setServerInput({ user: u.id, perm: p.id, value: e.target.value })}
                                />
                                <button
                                  onClick={() => grantScoped(u.id, p.id)}
                                  disabled={userLoading}
                                  className="btn-success text-xs px-2 py-1.5"
                                >
                                  <Plus size={12} />
                                </button>
                              </div>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
