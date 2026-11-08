import { useState, useEffect } from "react";
import { Plus, Trash2, Edit } from "lucide-react";
import { listServers, createServer, removeServer } from "../lib/api";
import type { Server } from "../types";

export default function Servers() {
  const [servers, setServers] = useState<Server[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({
    uuid: "",
    name: "",
    hostname: "",
    ip_address: "",
    os: "",
    agent_version: "",
  });

  useEffect(() => { load(); }, []);

  const load = async () => {
    try {
      setLoading(true);
      const data = await listServers();
      setServers(data.servers);
    } catch (err: any) {
      setError("Fehler beim Laden: " + (err?.error || err));
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await createServer(form);
      setOpen(false);
      setForm({ uuid: "", name: "", hostname: "", ip_address: "", os: "", agent_version: "" });
      await load();
    } catch (err: any) {
      setError(err?.error || "Server konnte nicht erstellt werden");
    }
  };

  const handleDelete = async (id: number) => {
    if (!window.confirm("Server wirklich entfernen?")) return;
    try {
      await removeServer(id);
      await load();
    } catch (err: any) {
      setError(err?.error || "Löschen fehlgeschlagen");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-100">Server</h1>
          <p className="text-sm text-slate-500">Überwachte Server und Agenten verwalten.</p>
        </div>
        <button onClick={() => setOpen(true)} className="btn-primary">
          <Plus size={16} /> Server hinzufügen
        </button>
      </div>

      {error && <div className="rounded bg-[#7f1d1d]/10 border border-[#9f2525]/30 p-3 text-sm text-red-400">{error}</div>}

      <div className="panel p-5 overflow-x-auto">
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Hostname</th>
              <th>IP</th>
              <th>OS</th>
              <th>Agent</th>
              <th>Status</th>
              <th className="text-right">Aktion</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={7} className="text-slate-500 text-center py-8">Lade…</td></tr>
            ) : servers.length ? (
              servers.map((s) => (
                <tr key={s.id}>
                  <td className="font-medium">{s.name}</td>
                  <td className="text-slate-400">{s.hostname || "—"}</td>
                  <td className="text-slate-400 font-mono text-xs">{s.ip_address || "—"}</td>
                  <td className="text-slate-400 text-xs">{s.os || "—"}</td>
                  <td className="text-slate-400 text-xs">{s.agent_version || "—"}</td>
                  <td>
                    <span className={`badge ${s.status === "online" ? "badge-online" : "badge-offline"}`}>
                      <span className={`w-2 h-2 rounded-full ${s.status === "online" ? "bg-green-500" : "bg-slate-500"}`} /> {s.status}
                    </span>
                  </td>
                  <td className="text-right">
                    <button onClick={() => handleDelete(s.id)} className="btn btn-danger text-xs p-1.5" title="Löschen">
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              ))
            ) : (
              <tr><td colSpan={7} className="text-slate-500 text-center py-8">Keine Server erfasst.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Create dialog */}
      <div className={`fixed inset-0 z-50 ${open ? "flex" : "hidden"} items-center justify-center`}>
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setOpen(false)} />
        <div className="bg-[#121921] border border-[#1f2a33] rounded-lg p-6 w-full max-w-md shadow-2xl relative z-10">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-bold text-slate-100">Server hinzufügen</h2>
            <button onClick={() => setOpen(false)} className="p-2 text-slate-400 hover:text-white"><Edit size={20} /></button>
          </div>
          <form onSubmit={handleCreate} className="space-y-4">
            <div className="space-y-2">
              <label className="block text-xs font-medium text-slate-400">Name</label>
              <input className="input" placeholder="z. B. Webserver-01" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
            </div>
            <div className="space-y-2">
              <label className="block text-xs font-medium text-slate-400">UUID (eindeutig)</label>
              <input className="input input" placeholder="z. B. srv-001" value={form.uuid} onChange={(e) => setForm({ ...form, uuid: e.target.value })} required />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="block text-xs font-medium text-slate-400">Hostname</label>
                <input className="input" value={form.hostname} onChange={(e) => setForm({ ...form, hostname: e.target.value })} />
              </div>
              <div className="space-y-2">
                <label className="block text-xs font-medium text-slate-400">IP-Adresse</label>
                <input className="input" value={form.ip_address} onChange={(e) => setForm({ ...form, ip_address: e.target.value })} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="block text-xs font-medium text-slate-400">Betriebssystem</label>
                <input className="input" value={form.os} onChange={(e) => setForm({ ...form, os: e.target.value })} />
              </div>
              <div className="space-y-2">
                <label className="block text-xs font-medium text-slate-400">Agent-Version</label>
                <input className="input" value={form.agent_version} onChange={(e) => setForm({ ...form, agent_version: e.target.value })} />
              </div>
            </div>
            <div className="flex justify-end gap-3 pt-4">
              <button type="button" onClick={() => setOpen(false)} className="btn-ghost">Abbrechen</button>
              <button type="submit" className="btn-primary" disabled={loading}>
                {loading ? "Erstellt…" : "Server erstellen"}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
