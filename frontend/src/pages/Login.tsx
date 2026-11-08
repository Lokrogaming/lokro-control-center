import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth";
import { AlertTriangle, Server } from "lucide-react";

export default function Login() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login(username, password);
      navigate("/");
    } catch (err: any) {
      setError(err?.error || "Anmeldung fehlgeschlagen");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-[#0b0f13] px-4">
      <div className="w-full max-w-md">
        <div className="panel p-8 mx-auto shadow-[0 0 30px -12px rgb(34 211 238 / 0.25)]">
          <div className="flex items-center gap-2 mb-6">
            <div className="w-3 h-3 bg-cyan-500 rounded-full"></div>
            <span className="font-bold text-lg tracking-tight text-cyan-400">LOKRO<span className="text-slate-300">Control Center</span></span>
          </div>

          <h2 className="mb-6 text-lg font-semibold text-slate-100">Anmeldung</h2>

          {error && (
            <div className="mb-5 p-3 rounded bg-[#7f1d1d]/10 border border-[#9f2525]/30 text-sm text-red-400">
              <AlertTriangle size={14} className="mr-2 shrink-0" /> {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <label className="block text-xs font-medium text-slate-400">Benutzername</label>
              <input
                className="input"
                placeholder="z. B. admin"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
                required
              />
            </div>
            <div className="space-y-2">
              <label className="block text-xs font-medium text-slate-400">Passwort</label>
              <input
                className="input"
                type="password"
                placeholder="••••••••"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
            </div>
            <button
              className="w-full btn-primary disabled:opacity-70"
              disabled={loading}
            >
              {loading ? "Anmelden..." : "Anmelden"}
            </button>
          </form>

          <div className="mt-8 pt-6 border-t border-[#1f2a33] flex items-center gap-2">
            <Server size={14} className="text-slate-500" />
            <span className="text-xs text-slate-500">Keine Standard-Zugangsdaten. Die Anmeldeinformationen werden bei der Installation festgelegt.</span>
          </div>
        </div>
      </div>
    </div>
  );
}
