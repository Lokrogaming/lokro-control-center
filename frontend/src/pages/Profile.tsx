import { useState, useEffect } from "react";
import { RefreshCw, Trash2, Lock, Terminal } from "lucide-react";
import { me, changePassword, listSessions, revokeSession } from "../lib/api";

export default function Profile() {
  const [profile, setProfile] = useState<any>(null);
  const [sessions, setSessions] = useState<any[]>([]);
  const [error, setError] = useState("");
  const [cp, setCp] = useState({ current: "", new: "" });
  const [cpError, setCpError] = useState("");
  const [cpOk, setCpOk] = useState("");

  useEffect(() => {
    loadProfile();
    loadSessions();
  }, []);

  const loadProfile = async () => {
    try {
      const data = await me();
      setProfile(data.user);
    } catch (err: any) {
      setError("Profil konnte nicht geladen werden: " + (err?.error || err));
    }
  };

  const loadSessions = async () => {
    try {
      const data = await listSessions();
      setSessions(data.sessions);
    } catch (err: any) {
      setError("Sessions konnten nicht geladen werden: " + (err?.error || err));
    }
  };

  const handleCp = async (e: React.FormEvent) => {
    e.preventDefault();
    setCpError("");
    setCpOk("");
    try {
      await changePassword(cp.current, cp.new);
      setCpOk("Passwort wurde geändert. Alle anderen Sessions wurden beendet.");
      setCp({ current: "", new: "" });
    } catch (err: any) {
      setCpError(err?.error || "Passwort-Änderung fehlgeschlagen");
    }
  };

  const handleRevoke = async (id: number) => {
    if (!window.confirm("Diese Session wirklich beenden?")) return;
    try {
      await revokeSession(id);
      await loadSessions();
    } catch (err: any) {
      setError(err?.error || "Session konnte nicht beendet werden");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-100">Profil & Sessions</h1>
          <p className="text-sm text-slate-500">Eigene Kontoangaben, aktive Sitzungen und Passwort-Änderung.</p>
        </div>
        <button onClick={() => { loadProfile(); loadSessions(); }} className="btn-ghost">
          <RefreshCw size={16} className="mr-2" /> Aktualisieren
        </button>
      </div>

      {error && <div className="rounded bg-[#7f1d1d]/10 border border-[#9f2525]/30 p-3 text-sm text-red-400">{error}</div>}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 panel p-5">
          <h2 className="text-sm font-semibold text-slate-300 mb-4">Konto</h2>
          {profile ? (
            <div className="space-y-3">
              <div className="flex items-center gap-3">
                <div className="flex items-center gap-2">
                  <Terminal size={20} className="text-cyan-500" />
                  <span className="font-medium">{profile.username}</span>
                </div>
                <span className={`badge ${profile.role === "super_admin" ? "badge-online" : profile.role === "admin" ? "badge-unknown" : "badge-offline"}`}>
                  {profile.role === "super_admin" ? "Super Admin" : profile.role === "admin" ? "Admin" : "Nutzer"}
                </span>
              </div>
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div>
                  <div className="text-slate-500 text-xs">E-Mail</div>
                  <div className="text-slate-200 font-mono">{profile.email}</div>
                </div>
                <div>
                  <div className="text-slate-500 text-xs">ID</div>
                  <div className="text-slate-200 font-mono">{profile.id}</div>
                </div>
              </div>
              {profile.last_login_at && (
                <div className="pt-3 border-t border-[#1f2a33] text-sm">
                  <div className="text-slate-500 text-xs">Letzter Login</div>
                  <div className="text-slate-200 font-mono">{new Date(profile.last_login_at).toLocaleString("de-DE")}</div>
                </div>
              )}
            </div>
          ) : (
            <div className="text-slate-500 text-center py-8">Lade…</div>
          )}
        </div>

        <div className="lg:col-span-1 panel p-5">
          <h2 className="text-sm font-semibold text-slate-300 mb-4">Passwort ändern</h2>
          {cpOk && <div className="mb-3 rounded bg-[#166534]/10 border border-[#22c55e]/30 p-3 text-sm text-green-400">{cpOk}</div>}
          {cpError && <div className="mb-3 rounded bg-[#7f1d1d]/10 border border-[#9f2525]/30 p-3 text-sm text-red-400">{cpError}</div>}
          <form onSubmit={handleCp} className="space-y-3">
            <div className="space-y-2">
              <label className="block text-xs font-medium text-slate-400">Aktuelles Passwort</label>
              <input className="input" type="password" placeholder="••••••••" value={cp.current} onChange={(e) => setCp({ ...cp, current: e.target.value })} required />
            </div>
            <div className="space-y-2">
              <label className="block text-xs font-medium text-slate-400">Neues Passwort (mind. 8 Zeichen)</label>
              <input className="input" type="password" placeholder="••••••••" value={cp.new} onChange={(e) => setCp({ ...cp, new: e.target.value })} required />
            </div>
            <button className="w-full btn-primary">
              <Lock size={14} className="mr-2" /> Passwort ändern
            </button>
          </form>
        </div>
      </div>

      <div className="panel p-5">
        <h2 className="text-sm font-semibold text-slate-300 mb-4">Aktive Sitzungen</h2>
        <div className="overflow-x-auto">
          <table className="table text-sm">
            <thead>
              <tr>
                <th>Erstellt</th>
                <th>Läuft ab</th>
                <th>IP-Adresse</th>
                <th>User-Agent</th>
                <th>Aktuell</th>
                <th className="text-right">Aktion</th>
              </tr>
            </thead>
            <tbody>
              {sessions.length === 0 ? (
                <tr><td colSpan={6} className="text-slate-500 text-center py-8">Keine aktiven Sitzungen.</td></tr>
              ) : (
                sessions.map((s) => (
                  <tr key={s.id}>
                    <td className="text-slate-400 font-mono text-xs">{new Date(s.created_at).toLocaleString("de-DE")}</td>
                    <td className="text-slate-400 font-mono text-xs">{new Date(s.expires_at).toLocaleString("de-DE")}</td>
                    <td className="text-slate-400 font-mono text-xs">{s.ip_address || "—"}</td>
                    <td className="text-slate-400 text-xs truncate max-w-[200px]">{s.user_agent || "—"}</td>
                    <td>
                      <span className={`badge ${s.current ? "badge-online" : "badge-offline"}`}>
                        {s.current ? "Ja" : "Nein"}
                      </span>
                    </td>
                    <td className="text-right">
                      {!s.current && (
                        <button onClick={() => handleRevoke(s.id)} className="btn btn-danger text-xs p-1.5" title="Beenden">
                          <Trash2 size={14} />
                        </button>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
