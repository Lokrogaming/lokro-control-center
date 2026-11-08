import { useState } from "react";
import { useLocation } from "react-router-dom";
import { Bell, User as UserIcon } from "lucide-react";
import { useAuth } from "../lib/auth";

export default function Navbar() {
  const { user, logout } = useAuth();
  const location = useLocation();
  const [userMenuOpen, setUserMenuOpen] = useState(false);

  const titleByRoute: Record<string, string> = {
    "/": "Übersicht",
    "/servers": "Server",
    "/users": "Nutzerverwaltung",
    "/permissions": "Berechtigungen",
    "/profile": "Profil & Sessions",
  };

  return (
    <header className="h-14 border-b border-[#1f2a33] bg-[#0b0f13]/80 backdrop-blur sticky top-0 z-30 flex items-center justify-between px-4">
      <div className="flex items-center gap-3">
        <h1 className="font-bold text-sm tracking-tight text-cyan-400">LOKRO<span className="text-slate-300">Control Center</span></h1>
        <span className="hidden md:block text-xs text-slate-500 border border-[#1f2a33] px-2 py-0.5 rounded ml-2">
          {titleByRoute[location.pathname] || "Dashboard"}
        </span>
      </div>

      <div className="flex items-center gap-3">
        <button className="p-2 text-slate-400 hover:text-white hover:bg-[#1f2a33] rounded-lg transition">
          <Bell size={18} />
        </button>
        <div className="relative" onClick={() => setUserMenuOpen(!userMenuOpen)}>
          <button className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-[#121921] border border-[#1f2a33] text-slate-300 hover:text-white hover:bg-[#1f2a33]/80 transition">
            <UserIcon size={16} />
            <span className="text-xs hidden sm:block">{user?.username}</span>
            <div className="w-2 h-2 bg-green-500 rounded-full border border-[#0b0f13]"></div>
          </button>
          {userMenuOpen && (
            <div className="absolute right-0 mt-2 w-48 bg-[#121921] border border-[#1f2a33] rounded-lg shadow-lg p-1 z-50 animate-in fade-in slide-in-from-right-2 duration-200">
              <div className="px-3 py-2 text-xs text-slate-400 border-b border-[#1f2a33]">{user?.role}</div>
              <button
                onClick={() => { setUserMenuOpen(false); logout(); }}
                className="w-full text-left px-3 py-2 text-sm text-red-400 hover:bg-[#1f2a33] rounded"
              >
                Abmelden
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
