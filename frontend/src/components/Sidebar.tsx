import { useState } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  Server,
  User,
  Shield,
  ShieldAlert,
  FileText,
  Activity,
  BarChart3,
  Monitor,
  Smartphone,
  Settings,
  LogOut,
  X,
  ChevronLeft,
} from "lucide-react";
import { useAuth } from "../lib/auth";

type NavItem = {
  label: string;
  href: string;
  icon: React.ElementType;
  permission?: string; // "resource:action"
  roles?: ("super_admin" | "admin" | "user")[];
  disabled?: boolean;
};

const NAV: NavItem[] = [
  { label: "Übersicht", href: "/", icon: LayoutDashboard, roles: ["super_admin", "admin", "user"] },
  { label: "Uptime", href: "/uptime", icon: Monitor, roles: ["super_admin", "admin", "user"] },
  { label: "Server", href: "/servers", icon: Server, roles: ["super_admin", "admin", "user"] },
  { label: "Analytics", href: "/analytics", icon: BarChart3, roles: ["super_admin", "admin"], disabled: true },
  { label: "Prozesse", href: "/processes", icon: Activity, roles: ["super_admin", "admin"], disabled: true },
  { label: "Netzwerk", href: "/network", icon: Monitor, roles: ["super_admin", "admin"], disabled: true },
  { label: "Speicher", href: "/storage", icon: Smartphone, roles: ["super_admin", "admin"], disabled: true },
  { label: "Dienste", href: "/services", icon: Shield, roles: ["super_admin", "admin"], disabled: true },
  { label: "Docker", href: "/docker", icon: Activity, roles: ["super_admin"], disabled: true },
  { label: "Logs", href: "/logs", icon: FileText, roles: ["super_admin", "admin"], disabled: true },
  { label: "Nutzer", href: "/users", icon: User, roles: ["super_admin", "admin"] },
  { label: "Berechtigungen", href: "/permissions", icon: ShieldAlert, roles: ["super_admin", "admin"] },
  { label: "Einstellungen", href: "/settings", icon: Settings, roles: ["super_admin", "admin"], disabled: true },
  { label: "Audit-Logs", href: "/audit", icon: FileText, roles: ["super_admin"], disabled: true },
];

export default function Sidebar() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(typeof window === "undefined" || window.innerWidth < 768);

  const items = NAV.filter((item) => {
    if (!item.roles) return true;
    if (!user) return false;
    if (user.role === "super_admin") return true;
    return item.roles.includes(user.role);
  });

  const toggle = () => setCollapsed(!collapsed);

  const isActive = (href: string) => location.pathname === href || (href !== "/" && location.pathname.startsWith(href));

  return (
    <aside
      className={`fixed left-0 top-0 h-screen z-40 border-r border-[#1f2a33] transition-transform duration-300 ${
        collapsed
          ? "-translate-x-full md:translate-x-0 w-16"
          : "translate-x-0 md:w-64"
      } bg-[#0b0f13]`}
    >
      <div className="flex items-center justify-between h-14 px-3 bg-[#0b0f13] border-b border-[#1f2a33]">
        {!collapsed && (
          <div className="flex items-center gap-2">
            <div className="w-2.5 h-2.5 bg-cyan-500 rounded-full"></div>
            <span className="font-bold text-xs tracking-wider text-cyan-400">LOKRO</span>
          </div>
        )}
        <button onClick={toggle} className="p-2 hover:bg-[#1f2a33] rounded text-slate-400 md:hidden">
          {collapsed ? <ChevronLeft size={18} /> : <X size={18} />}
        </button>
      </div>

      <nav className="mt-4 px-2 space-y-1">
        {items.map((item) => {
          const active = isActive(item.href);
          const allowed = !item.disabled;
          const Icon = item.icon as React.ElementType;
          return (
            <a
              key={item.href}
              href={item.href}
              onClick={(e) => {
                if (item.disabled) e.preventDefault();
                else navigate(item.href);
              }}
              className={`flex items-center gap-3 px-3 py-2.5 text-sm rounded-lg transition ${
                active
                  ? "bg-cyan-500/10 text-cyan-400 border-r-2 border-cyan-500"
                  : "text-slate-400 hover:bg-[#1f2a33] hover:text-slate-200"
              } ${!allowed ? "opacity-50 cursor-not-allowed" : ""}`}
              title={item.disabled ? "In Vorbereitung (Phase 2+)" : undefined}
            >
              <Icon size={16} className="shrink-0" />
              {!collapsed && <span>{item.label}</span>}
              {!allowed && <span className="ml-auto text-[10px] text-slate-500">bald</span>}
            </a>
          );
        })}
      </nav>

      <div className="absolute bottom-0 left-0 w-full p-3 border-t border-[#1f2a33]">
        <button
          onClick={() => { logout(); navigate("/login"); }}
          className="w-full flex items-center gap-3 px-3 py-2.5 text-sm text-slate-400 hover:text-red-400 hover:bg-[#1f2a33] rounded-lg transition"
        >
          <LogOut size={16} /> <span>Abmelden</span>
        </button>
      </div>
    </aside>
  );
}
