import { AlertCircle } from "lucide-react";

export default function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] text-center">
      <div className="w-16 h-16 rounded-full bg-[#1f2a33] flex items-center justify-center mb-4">
        <AlertCircle size={24} className="text-slate-500" />
      </div>
      <h1 className="text-3xl font-bold text-slate-100 mb-2">Seite nicht gefunden</h1>
      <p className="text-slate-500 mb-6">Die angeforderte Seite existiert nicht.</p>
      <button className="btn-primary" onClick={() => { window.location.href = "/"; }}>
        Zurück zur Übersicht
      </button>
    </div>
  );
}
