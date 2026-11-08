import { useState, useEffect } from "react";
import { Plus, Trash2, Save, Monitor, RefreshCw } from "lucide-react";
import {
  listUptimeChecks,
  createUptimeCheck,
  updateUptimeCheck,
  removeUptimeCheck,
  getUptimeSummary,
} from "../lib/api";

export default function Uptime() {
  const [checks, setChecks] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState({
    name: "",
    url: "",
    type: "https",
    interval: 60,
    timeout: 10,
    expected_status: 200,
    is_active: true,
  });
  const [saving, setSaving] = useState(false);
  const [summaries, setSummaries] = useState<Map<number, any>>(new Map());

  useEffect(() => {
    loadChecks();
  }, []);

  const loadChecks = async () => {
    setLoading(true);
    try {
      const data = await listUptimeChecks();
      setChecks(data.checks);
      data.checks.forEach((c: any) => loadSummary(c.id));
    } catch (err: any) {
      setError("Checks konnten nicht geladen werden: " + (err?.error || err));
    } finally {
      setLoading(false);
    }
  };

  const loadSummary = async (checkId: number) => {
    try {
      const data = await getUptimeSummary(checkId);
      setSummaries((m) => {
        const nm = new Map(m);
        nm.set(checkId, data.summary);
        return nm;
      });
    } catch {
      // summary might fail if no results yet
    }
  };

  const openCreate = () => {
    setEditId(null);
    setForm({ name: "", url: "", type: "https", interval: 60, timeout: 10, expected_status: 200, is_active: true });
    setOpen(true);
  };

  const openEdit = (c: any) => {
    setEditId(c.id);
    setForm({ ...c });
    setOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      if (editId) {
        await updateUptimeCheck(editId, form);
      } else {
        await createUptimeCheck(form);
      }
      setOpen(false);
      await loadChecks();
    } catch (err: any) {
      setError(err?.error || "Speichern fehlgeschlagen");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!window.confirm("Check wirklich löschen?")) return;
    try {
      await removeUptimeCheck(id);
      await loadChecks();
    } catch (err: any) {
      setError(err?.error || "Löschen fehlgeschlagen");
    }
  };

  const toggleActive = async (c: any) => {
    try {
      await updateUptimeCheck(c.id, { ...c, is_active: !c.is_active });
      await loadChecks();
    } catch (err: any) {
      setError(err?.error || "Status ändern fehlgeschlagen");
    }
  };

  const summary = (id: number) => summaries.get(id);

  const statusColor = (status: string) => {
    if (status === "up") return "text-green-400 bg-green-900/20 border border-green-500/30";
    return "text-red-400 bg-[#7f1d1d]/20 border border-[#9f2525]/30";
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-100">Uptime-Monitor</h1>
          <p className="text-sm text-slate-500">
            Überwachen Sie Webseiten und Dienste auf Erreichbarkeit (wie Uptime Kuma).
          </p>
        </div>
        <button onClick={openCreate} className="btn-primary">
          <Plus size={16} /> Check hinzufügen
        </button>
      </div>

      {error && <div className="rounded bg-[#7f1d1d]/10 border border-[#9f2525]/30 p-3 text-sm text-red-400">{error}</div>}

      {loading ? (
        <div className="panel p-8 text-center text-slate-500 text-sm">Lade…</div>
      ) : (
        <div className="grid gap-4">
          {checks.length === 0 ? (
            <div className="panel p-8 text-center text-slate-500 text-sm">Keine Checks erfasst. Fügen Sie einen hinzu.</div>
          ) : (
            checks.map((c) => {
              const s = summary(c.id);
              return (
                <div key={c.id} className="panel p-5 border-[#1f2a33]">
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-3">
                      <div className={`w-2 h-2 rounded-full ${s?.last_status === "up" ? "bg-green-500" : "bg-slate-500"}`} />
                      <span className="font-medium">{c.name}</span>
                      <span className="text-xs text-slate-500 ml-auto">{c.url}</span>
                    </div>
                    <button
                      onClick={() => toggleActive(c)}
                      className={`p-1.5 rounded transition ${c.is_active ? "text-green-400 hover:bg-green-900/20" : "text-slate-500 hover:bg-[#1f2a33]"}`}
                    >
                      {c.is_active ? <Monitor size={16} /> : <Monitor size={16} className="opacity-50" />}
                    </button>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-4 gap-3 mb-3">
                    <div className="bg-[#0f141a]/50 rounded p-3">
                      <div className="text-xs text-slate-500">Verfügbarkeit</div>
                      <div className={`text-lg font-bold ${statusColor(s?.last_status ?? "down")}`}>
                        {s?.up_percent?.toFixed(1) ?? "—"}%
                      </div>
                    </div>
                    <div className="bg-[#0f141a]/50 rounded p-3">
                      <div className="text-xs text-slate-500">Letzter Status</div>
                      <div className="text-sm font-mono text-slate-200">{s?.last_status ?? "—"}</div>
                    </div>
                    <div className="bg-[#0f141a]/50 rounded p-3">
                      <div className="text-xs text-slate-500">Ø Antwortzeit</div>
                      <div className="text-sm font-mono text-slate-200">{s?.avg_response?.toFixed(0) ?? "—"} ms</div>
                    </div>
                    <div className="bg-[#0f141a]/50 rounded p-3">
                      <div className="text-xs text-slate-500">Intervall</div>
                      <div className="text-sm font-mono text-slate-200">{c.interval}s</div>
                    </div>
                  </div>

                  {s?.last_checked && (
                    <div className="text-xs text-slate-500 mb-3">
                      Letzte Prüfung: {s.last_checked}
                    </div>
                  )}

                  <div className="flex justify-end gap-2">
                    <button onClick={() => openEdit(c)} className="btn-ghost text-xs p-1.5 mr-2">
                      <Save size={14} /> Bearbeiten
                    </button>
                    <button onClick={() => handleDelete(c.id)} className="btn btn-danger text-xs p-1.5">
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>
              );
            })
          )}
        </div>
      )}

      {/* Create/Edit dialog */}
      <div className={`fixed inset-0 z-50 ${open ? "flex" : "hidden"} items-center justify-center`}>
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setOpen(false)} />
        <div className="bg-[#121921] border border-[#1f2a33] rounded-lg p-6 w-full max-w-md shadow-2xl relative z-10">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-bold text-slate-100">{editId ? "Check bearbeiten" : "Neuer Uptime-Check"}</h2>
            <button onClick={() => setOpen(false)} className="p-2 text-slate-400 hover:text-white"><RefreshCw size={20} /></button>
          </div>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <label className="block text-xs font-medium text-slate-400">Name</label>
              <input className="input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
            </div>
            <div className="space-y-2">
              <label className="block text-xs font-medium text-slate-400">URL / Ziel</label>
              <input className="input" placeholder="z. B. https://server.lokro.dev" value={form.url} onChange={(e) => setForm({ ...form, url: e.target.value })} required />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="block text-xs font-medium text-slate-400">Typ</label>
                <select className="input" value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
                  <option value="https">HTTPS</option>
                  <option value="http">HTTP</option>
                  <option value="tcp">TCP</option>
                </select>
              </div>
              <div className="space-y-2">
                <label className="block text-xs font-medium text-slate-400">Erwarteter Status</label>
                <input className="input" type="number" value={form.expected_status} onChange={(e) => setForm({ ...form, expected_status: parseInt(e.target.value) || 200 })} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="block text-xs font-medium text-slate-400">Intervall (Sekunden)</label>
                <input className="input" type="number" value={form.interval} onChange={(e) => setForm({ ...form, interval: parseInt(e.target.value) || 60 })} />
              </div>
              <div className="space-y-2">
                <label className="block text-xs font-medium text-slate-400">Timeout (Sekunden)</label>
                <input className="input" type="number" value={form.timeout} onChange={(e) => setForm({ ...form, timeout: parseInt(e.target.value) || 10 })} />
              </div>
            </div>
            <div className="space-y-2 flex items-end">
              <label className="block text-xs font-medium text-slate-400">Aktiv</label>
              <input type="checkbox" className="rounded bg-[#0f141a] border-[#1f2a33] text-cyan-600 focus:ring-cyan-500" checked={form.is_active} onChange={(e) => setForm({ ...form, is_active: e.target.checked })} />
            </div>
            <div className="flex justify-end gap-3 pt-4">
              <button type="button" onClick={() => setOpen(false)} className="btn-ghost">Abbrechen</button>
              <button type="submit" className="btn-primary" disabled={saving}>
                {saving ? "Speichert…" : "Check erstellen"}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
