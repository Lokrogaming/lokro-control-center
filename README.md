# LOKRO Server Control Center

Produktionsfähige Server-Monitoring- und Management-Plattform für Rootserver
(Ubuntu 24.04 LTS+, Debian 12+, x86_64, **ohne Docker**).

Erreichbar über: `https://server.lokro.dev` (Nginx Reverse Proxy, Let's Encrypt).

---

## Architektur

| Bereich | Technologie |
|---------|-------------|
| Backend | Go (stdlib-only), REST API + WebSocket-Grundlage |
| Speicher | Atomische JSON-Datei (Standard); `Store`-Interface für späteren SQL-Treiber (MySQL/MariaDB via Pterodactyl, PostgreSQL, SQLite) |
| Frontend | React, TypeScript, Vite, Tailwind CSS, Recharts, Lucide Icons |
| Authentifizierung | Session-Cookies (HttpOnly, Secure, SameSite=Lax), PBKDF2-HMAC-SHA256 |
| Autorisierung | Rollen (Super Admin / Admin / User) + granulare, serverseitig geprüfte Permissions |
| Prozessüberwachung | Linux `/proc` und `/sys` (keine Shell-Ausführung über HTTP) |
| Web-Analytics | Nginx Access-Log-Parsing, GeoIP (optional, lokal), DSGVO-konform |
| Uptime-Monitor | HTTP/HTTPS/TCP-Checks mit Verfügbarkeit, Antwortzeit, Historie (wie Uptime Kuma) |
| Betrieb | systemd-Service, Nginx Reverse Proxy, Certbot/Let's Encrypt |

### Sicherheit (keine Abkürzungen)
- Passwörter werden mit PBKDF2-HMAC-SHA256 (200.000 Iterationen) gehasht, nie im Klartext gespeichert.
- Session-Cookies: `HttpOnly`, `Secure` (bei HTTPS), `SameSite=Lax`.
- Account-Sperre nach konfigurierbaren Fehlversuchen.
- CSRF-Shutz über SameSite-Cookies + Origin-Prüfung.
- Rate Limiting auf Login.
- Audit-Logs für Logins, User-, Permission-, Server- und Einstellungsänderungen.
- Berechtigungen werden **seitenserver bei jedem API-Aufruf geprüft** (nicht nur im Frontend ausgeblendet).
- Keine Shell-Injection, keine SQL-Injection, keine Default-Admin-Zugangsdaten im Code.
- Der Applikationsprozess läuft als dedizierter, unprivilegierter System-Benutzer (`lokro`), nicht als Root.

---

## Verzeichnisse

```
/opt/lokro-control-center/        Applikation
/opt/lokro-control-center/data/   JSON-Datenspeicher (Standard)
/opt/lokro-control-center/.env    Umgebungskonfiguration
/etc/systemd/system/lokro-control-center.service
/etc/nginx/sites-available/lokro-control-center
/var/backups/lokro-control-center/ Backups
```

---

## Schnellstart (Installation auf dem Zielserver)

Als `root` auf dem Zielserver:

```bash
# Quellcode auf den Server bringen (z. B. per git oder rsync), dann:
sudo bash install.sh
```

Das Installationsskript:
1. Erkennt das Betriebssystem (Ubuntu/Debian).
2. Installiert Abhängigkeiten (Go, Nginx, Certbot, …).
3. Legt den System-Benutzer `lokro` an.
4. Kopiert die Applikation nach `/opt/lokro-control-center` und baut Backend **und** Frontend
   (`npm ci` + `vite build` → `frontend/dist`).
5. Legt Verzeichnisse und Berechtigungen an (750/700, unprivilegiert).
6. Erstellt `.env` aus `.env.example` (Session-Secret wird automatisch generiert).
7. Erzeugt die systemd-Unit und startet den Dienst.
8. Legt die Nginx-Reverse-Proxy-Konfiguration an (existierende Konfigs werden **nicht** überschrieben).
9. Beantragt ein Let's-Encrypt-Zertifikat via Certbot (falls die Domain erreichbar ist).
10. Richtet den initialen Super-Admin über Umgebungsvariablen `INITIAL_ADMIN_USERNAME` / `INITIAL_ADMIN_PASSWORD` ein.

### Frontend-Auslieferung

Das Go-Backend liefert den kompilierten Vite-Build selbst aus; Nginx bleibt ein
reiner Reverse-Proxy. Das Build-Verzeichnis ist über `FRONTEND_DIR` konfigurierbar
(Standard: `frontend/dist`, relativ zum WorkingDirectory des Service):

- `/` und alle React-Client-Routen (z. B. `/servers/42`) liefern `index.html`.
- Vorhandene Dateien unter `/assets/` werden direkt ausgeliefert (mit langem
  Cache für die gehashten Dateinamen).
- Fehlende Dateien mit Endung liefern `404` statt fälschlich `index.html`.
- `/api/*` und `/health` werden ausschließlich vom Go-Router verarbeitet und
  niemals vom SPA-Fallback abgefangen.
- Directory Traversal (auch über Symlinks) wird abgewiesen.

### Initialer Admin-Account

Beim ersten Start wird ein Super-Administrator angelegt, wenn Umgebungsvariablen gesetzt sind:

```bash
export INITIAL_ADMIN_USERNAME=admin
export INITIAL_ADMIN_PASSWORD="sicheresPasswort123"
export INITIAL_ADMIN_EMAIL=admin@lokro.dev
sudo bash install.sh
```

Werden diese nicht gesetzt, wird **kein** Standard-Account erstellt (keine Default-Zugangsdaten im Code). Der erste Benutzer muss dann manuell über die API oder eine angepasste `.env` erstellt werden.

---

## Skripte

| Skript | Zweck |
|--------|-------|
| `install.sh` | Vollständige Installation (idempotent) |
| `uninstall.sh` | Deinstallation (Service, Unit, Nginx-Site, Applikationsverzeichnis) |
| `update.sh` | Update (Backup → Quellcode kopieren → Frontend + Backend neu bauen → Neustart) |
| `backup.sh` | Konsistentes Backup von `data/` und `.env` |
| `restore.sh` | Wiederherstellung aus einem Backup-Archiv |

Alle Skripte erfordern `root` und sind wiederholbar ausführbar.

---

## Entwicklung

Das Repository enthält Backend (`backend/`) und Frontend (`frontend/`).

### Backend (Go, stdlib)
```bash
cd backend
go build ./...
go test ./...
go vet ./...
```

### Frontend (React + Vite + Tailwind)
```bash
cd frontend
npm install
npm run build     # erzeugt frontend/dist (install.sh/update.sh bauen ebenfalls automatisch)
npm run lint      # tsc --noEmit
```

### Phasen
- **Phase 1 (abgeschlossen):** Projektstruktur, Backend, JSON-Speicher, Authentifizierung, Rollen- & Permission-System, API, Frontend (Login, Übersicht, Server, Nutzer, Berechtigungen, Profil), Tests, Installer, Nginx/systemd.
- **Phase 2+:** Linux-Monitoring-Agent, Live-Metriken (CPU/RAM/Disk/Network), Prozesse, Nginx-Log-Analytics, Multi-Server, Docker, Logs, Benachrichtigungen, Security-Hardening, weitere Tests.

---

## API (Auszug)

Alle Endpunkte erfordern (außer `/api/auth/login`) ein gültiges Session-Cookie `lokro_session`.

```
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me
POST   /api/auth/change-password
GET    /api/auth/sessions
DELETE /api/auth/sessions/:id

GET    /api/users
POST   /api/users
GET    /api/users/:id
GET    /api/users/:id/permissions
PUT    /api/users/:id
DELETE /api/users/:id

GET    /api/permissions
POST   /api/permissions/grant
POST   /api/permissions/revoke

GET    /api/servers
POST   /api/servers
GET    /api/servers/:id
PUT    /api/servers/:id
DELETE /api/servers/:id

GET    /api/metrics/servers/:id
GET    /api/audit

# Uptime-Monitor
GET    /api/uptime/checks
POST   /api/uptime/checks
GET    /api/uptime/checks/:id
PUT    /api/uptime/checks/:id
DELETE /api/uptime/checks/:id
GET    /api/uptime/results/:id
GET    /api/uptime/summary/:id

GET    /health
```

---

## Sicherheit & Betrieb

- Der Backend-Prozess lauscht nur auf `127.0.0.1:8080` (kein direkter Internetzugriff).
- Nginx ist der einzige öffentlich erreichbare Einstieg (443 TLS).
- Der Applikationsprozess läuft als `lokro` (kein Root).
- Keine unnötigen offenen Ports; nur 80/443 via Nginx.
- Uptime Kuma und andere bestehende Dienste bleiben unberührt (keine bestehenden Nginx-Konfigurationen werden überschrieben).

---

## Lizenz & Haftung

Interne Entwicklung. Keine Gewährleistung für den Produktivbetrieb ohne eigene Sicherheitsprüfung.
