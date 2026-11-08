import { useState, useEffect } from "react";
import { useParams } from "react-router-dom";
import { listServers, serverMetrics } from "../lib/api";
import { Server, Monitor, Cpu, HardDrive, Activity, RefreshCw } from "lucide-react";

type ServerRow = { id: number; name: string; status: string; ip_address?: string };

export default function Overview() {
  const { id: paramId } = useParams<{ id: string }>();
  const [servers, setServers] = useState<ServerRow[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(paramId ? Number(paramId) : null);
  const [metrics, setMetrics] = useState<any[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    loadServers();
  }, []);

  const loadServers = async () => {
    try {
      const data = await listServers();
      setServers(data.servers);
      if (!selectedId && data.servers.length) setSelectedId(data.servers[0].id);
    } catch (err: any) {
      setError("Server konnten nicht geladen werden: " + (err?.error || err));
    }
  };

  const loadMetrics = async () => {
    if (!selectedId) return;
    try {
      const data = await serverMetrics(selectedId, 30);
      setMetrics(data.metrics);
    } catch (err: any) {
      setError("Metriken konnten nicht geladen werden: " + (err?.error || err));
    }
  };

  useEffect(() => {
    loadMetrics();
    const t = setInterval(loadMetrics, 5000);
    return () => clearInterval(t);
  }, [selectedId]);

  const latest = metrics[0];
  const onlineCount = servers.filter((s) => s.status === "online").length;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-slate-100">Übersicht</h1>
          <p className="text-sm text-slate-500">Live-Zustand des überwachten Servers.</p>
        </div>
        <button
          onClick={loadServers}
          className="btn-ghost"
          title="Aktualisieren"
        >
          <RefreshCw size={16} className="mr-2" /> Aktualisieren
        </button>
      </div>

      {error && (
        <div className="rounded bg-[#7f1d1d]/10 border border-[#9f2525]/30 p-4 text-sm text-red-400">{error}</div>
      )}

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <StatCard title="Server" value={servers.length} icon={Server} />
        <StatCard title="Online" value={onlineCount} icon={Monitor} color="text-green-400" border="border-green-500/30" bg="bg-green-900/20" />
        <StatCard title="Offline / Unbekannt" value={servers.length - onlineCount} icon={Activity} color="text-slate-400" />
        <StatCard title="Aktualisierung" value="5s" icon={RefreshCw} color="text-cyan-400" />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Server list */}
        <div className="lg:col-span-1 panel p-5">
          <h2 className="text-sm font-semibold text-slate-300 mb-4">Server</h2>
          <div className="space-y-2">
            {servers.length ? (
              servers.map((s) => {
                const active = selectedId === s.id;
                return (
                  <button
                    key={s.id}
                    onClick={() => setSelectedId(s.id)}
                    className={`w-full flex items-center justify-between px-3 py-2.5 rounded-lg text-left transition ${
                      active
                        ? "bg-cyan-500/10 text-cyan-400 border border-cyan-500/50"
                        : "bg-[#0f141a] border border-transparent hover:border-[#1f2a33] hover:bg-[#151d26] text-slate-300"
                    }`}
                  >
                    <div>
                      <div className="text-sm font-medium">{s.name}</div>
                      <div className="text-xs text-slate-500">{s.ip_address || "—"}</div>
                    </div>
                    <span className={`flex h-2 w-2 items-center ${s.status === "online" ? "bg-green-500" : "bg-slate-500"} rounded-full`} />
                  </button>
                );
              })
            ) : (
              <div className="text-xs text-slate-500 text-center py-4">Keine Server erfasst.</div>
            )}
          </div>
        </div>

        {/* Live metrics */}
        <div className="lg:col-span-2 panel p-5">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-semibold text-slate-300">Live-Metriken</h2>
            <span className="text-xs text-slate-500">Alle 5 Sekunden</span>
          </div>

          {!selectedId ? (
            <div className="rounded bg-[#0f141a] p-8 text-center text-slate-500 text-sm">Wählen Sie einen Server links aus.</div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <MetricCard title="CPU-Auslastung" value={latest?.cpu_percent != null ? latest.cpu_percent.toFixed(1) + "%" : "—"} icon={Cpu} />
              <MetricCard title="Arbeitsspeicher" value={latest?.mem_used != null && latest?.mem_total ? ((latest.mem_used / latest.mem_total) * 100).toFixed(1) + "%" : "—"} icon={HardDrive} />
              <MetricCard title="Load Average (1m)" value={latest?.load_1?.toFixed(2) ?? "—"} icon={Activity} />
              <MetricCard title="Swap-Auslastung" value={latest?.swap_used != null && latest?.swap_total ? ((latest.swap_used / latest.swap_total) * 100).toFixed(1) + "%" : "—"} icon={HardDrive} />
            </div>
          )}
        </div>
      </div>

      {metrics.length > 0 && selectedId && (
        <div className="panel p-5">
          <h2 className="text-sm font-semibold text-slate-300 mb-3">Letzte Messwerte</h2>
          <div className="overflow-x-auto">
            <table className="table text-sm">
              <thead>
                <tr>
                  <th>Zeitpunkt</th>
                  <th>CPU %</th>
                  <th>Load 1m</th>
                  <th>RAM %</th>
                  <th>Swap %</th>
                  <th>Prozesse</th>
                </tr>
              </thead>
              <tbody>
                {metrics.slice(0, 10).reverse().map((m) => {
                  const memPct = m.mem_total ? ((m.mem_used / m.mem_total) * 100).toFixed(1) : "—";
                  const swapPct = m.swap_total ? ((m.swap_used / m.swap_total) * 100).toFixed(1) : "—";
                  return (
                    <tr key={m.id}>
                      <td className="text-slate-400 font-mono text-xs">{new Date(m.recorded_at).toLocaleTimeString("de-DE")}</td>
                      <td>{m.cpu_percent?.toFixed(1) ?? "—"}</td>
                      <td>{m.load_1?.toFixed(2) ?? "—"}</td>
                      <td>{memPct}</td>
                      <td>{swapPct}</td>
                      <td>{m.process_count ?? "—"}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

function StatCard({ title, value, icon: Icon, color = "text-cyan-400", border = "border-cyan-500/20", bg = "bg-cyan-900/10" }: { title: string; value: string | number; icon: React.ElementType; color?: string; border?: string; bg?: string }) {
  return (
    <div className={`panel p-4 ${bg} ${border}`}>
      <div className="flex items-center justify-between mb-2">
        <div className={`w-8 h-8 rounded-lg ${bg} flex items-center justify-center`}>
          <Icon size={14} className={color} />
        </div>
      </div>
      <div className="text-2xl font-bold text-slate-100">{value}</div>
      <div className="text-xs text-slate-500">{title}</div>
    </div>
  );
}

function MetricCard({ title, value, icon: Icon }: { title: string; value: string; icon: React.ElementType }) {
  return (
    <div className="panel p-4">
      <div className="flex items-center justify-between mb-2">
        <Icon size={16} className="text-slate-500" />
      </div>
      <div className="text-xl font-bold text-slate-100">{value}</div>
      <div className="text-xs text-slate-500">{title}</div>
    </div>
  );
}
