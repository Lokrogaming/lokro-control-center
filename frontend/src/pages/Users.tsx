import { useState, useEffect } from "react";
import { Plus, Trash2, Save, RefreshCw } from "lucide-react";
import { listUsers, createUser, updateUser, removeUser } from "../lib/api";
import type { User } from "../types";

export default function Users() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState({ username: "", email: "", password: "", role: "user", is_active: true });
  const [saving, setSaving] = useState(false);

  useEffect(() => { load(); }, []);

  const load = async () => {
    try {
      setLoading(true);
      const data = await listUsers();
      setUsers(data.users);
    } catch (err: any) {
      setError("Fehler beim Laden: " + (err?.error || err));
    } finally {
      setLoading(false);
    }
  };

  const openCreate = () => {
    setEditId(null);
    setForm({ username: "", email: "", password: "", role: "user", is_active: true });
    setOpen(true);
  };

  const openEdit = (u: User) => {
    setEditId(u.id);
    setForm({ username: u.username, email: u.email, password: "", role: u.role, is_active: u.is_active });
    setOpen(true);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      if (editId) {
        await updateUser(editId, { email: form.email, role: form.role, is_active: form.is_active });
      } else {
        await createUser(form);
      }
      setOpen(false);
      await load();
    } catch (err: any) {
      setError(err?.error || "Speichern fehlgeschlagen");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!window.confirm("Nutzer wirklich löschen?")) return;
    try {
      await removeUser(id);
      await load();
    } catch (err: any) {
      setError(err?.error || "Löschen fehlgeschlagen");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-100">Nutzerverwaltung</h1>
          <p className="text-sm text-slate-500">Benutzer anlegen, bearbeiten und deaktivieren.</p>
        </div>
        <button onClick={openCreate} className="btn-primary">
          <Plus size={16} /> Nutzer anlegen
        </button>
      </div>

      {error && <div className="rounded bg-[#7f1d1d]/10 border border-[#9f2525]/30 p-3 text-sm text-red-400">{error}</div>}

      <div className="panel p-5 overflow-x-auto">
        <table className="table">
          <thead>
            <tr>
              <th>Benutzername</th>
              <th>E-Mail</th>
              <th>Rolle</th>
              <th>Status</th>
              <th className="text-right">Aktion</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={5} className="text-slate-500 text-center py-8">Lade…</td></tr>
            ) : users.length ? (
              users.map((u) => (
                <tr key={u.id}>
                  <td className="font-medium">{u.username}</td>
                  <td className="text-slate-400 text-xs">{u.email}</td>
                  <td>
                    <span className={`badge ${u.role === "super_admin" ? "badge-online" : u.role === "admin" ? "badge-unknown" : "badge-offline"}`}>
                      {u.role === "super_admin" ? "Super Admin" : u.role === "admin" ? "Admin" : "Nutzer"}
                    </span>
                  </td>
                  <td>
                    <span className={`badge ${u.is_active ? "badge-online" : "badge-offline"}`}>
                      {u.is_active ? "Aktiv" : "Inaktiv"}
                    </span>
                  </td>
                  <td className="text-right">
                    <button onClick={() => openEdit(u)} className="btn btn-ghost text-xs p-1.5 mr-2" title="Bearbeiten">
                      <Save size={14} />
                    </button>
                    <button onClick={() => handleDelete(u.id)} className="btn btn-danger text-xs p-1.5" title="Löschen">
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              ))
            ) : (
              <tr><td colSpan={5} className="text-slate-500 text-center py-8">Keine Nutzer.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Create/Edit dialog */}
      <div className={`fixed inset-0 z-50 ${open ? "flex" : "hidden"} items-center justify-center`}>
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setOpen(false)} />
        <div className="bg-[#121921] border border-[#1f2a33] rounded-lg p-6 w-full max-w-md shadow-2xl relative z-10">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-bold text-slate-100">{editId ? "Nutzer bearbeiten" : "Neuer Nutzer"}</h2>
            <button onClick={() => setOpen(false)} className="p-2 text-slate-400 hover:text-white"><RefreshCw size={20} /></button>
          </div>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <label className="block text-xs font-medium text-slate-400">Benutzername</label>
              <input className="input" disabled={!!editId} value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} required />
            </div>
            <div className="space-y-2">
              <label className="block text-xs font-medium text-slate-400">E-Mail</label>
              <input className="input" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required />
            </div>
            {!editId && (
              <div className="space-y-2">
                <label className="block text-xs font-medium text-slate-400">Passwort</label>
                <input className="input" type="password" placeholder="••••••••" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required />
              </div>
            )}
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="block text-xs font-medium text-slate-400">Rolle</label>
                <select className="input" value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}>
                  <option value="super_admin">Super Admin</option>
                  <option value="admin">Admin</option>
                  <option value="user">Nutzer</option>
                </select>
              </div>
              <div className="space-y-2 flex items-end">
                <label className="block text-xs font-medium text-slate-400">Aktiv</label>
                <input type="checkbox" className="rounded bg-[#0f141a] border-[#1f2a33] text-cyan-600 focus:ring-cyan-500" checked={form.is_active} onChange={(e) => setForm({ ...form, is_active: e.target.checked })} />
              </div>
            </div>
            <div className="flex justify-end gap-3 pt-4">
              <button type="button" onClick={() => setOpen(false)} className="btn-ghost">Abbrechen</button>
              <button type="submit" className="btn-primary" disabled={saving}>
                {saving ? "Speichert…" : "Speichern"}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
