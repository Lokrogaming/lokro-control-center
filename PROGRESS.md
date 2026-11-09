# LOKRO Server Control Center - Fortschritt

## Status: Phase 1 ABGESCHLOSSEN (mit Uptime-Monitor)

### Phase 1 (fertig)
- [x] Projektstruktur & Repository initialisieren
- [x] Go Backend (stdlib-only: net/http, crypto/pbkdf2, JSON-Store mit Store-Interface für späteres SQL)
- [x] Authentifizierung (Session-Cookie HttpOnly, PBKDF2-HMAC-SHA256, Account-Sperre, Sessions)
- [x] Permission-System (RBAC + granular, serverseitig bei jedem API-Aufruf)
- [x] API (Auth, Users, Permissions, Servers, Metrics, Audit, Uptime)
- [x] Backend-Tests (Store, Auth, API) alle grün
- [x] React Frontend (Vite, TS, Tailwind, Login, Übersicht, Uptime, Server, Nutzer, Berechtigungen, Profil)
- [x] Frontend-Build (tsc + vite) erfolgreich
- [x] E2E-Rauchtest: Login / ME / Servers / Uptime / falsches Login funktionieren
- [x] Installationsskripte (install.sh, uninstall.sh, update.sh, backup.sh, restore.sh)
- [x] Nginx + systemd Templates
- [x] .env.example & README.md & .gitattributes

### Geplant (Phase 2-5)
- [ ] Linux Monitoring Agent (Go, /proc, /sys)
- [ ] Live-Dashboard (CPU, RAM, Disk, Network, System)
- [ ] Prozessübersicht
- [ ] Nginx Log Parsing & Analytics
- [ ] GeoIP, Charts
- [ ] Multi-Server, Services, Docker, Logs
- [ ] Notifications, Security Hardening, Tests
- [ ] Deployment-Dokumentation

### Abgeschlossen
- Phase 1 (Backend + Frontend + Uptime-Monitor + Installer + Docs)

## Zielsystem
https://server.lokro.dev (Nginx Reverse Proxy, Let's Encrypt, kein Docker)

## Wichtige Hinweise
- Backend lauscht nur auf 127.0.0.1:8080 (nicht direkt im Internet).
- Der Prozess läuft als unprivilegierter Benutzer `lokro` (nicht Root).
- Bestehende Nginx-Konfigurationen und Uptime Kuma werden nicht angetastet (keine bestehenden Nginx-Konfigurationen werden überschrieben).
- `install.sh`/`update.sh` bauen das Frontend automatisch (`npm ci` + `vite build` → `frontend/dist`); das Go-Backend liefert es inklusive SPA-Fallback aus.
- Pterodactyl-DB (MySQL/MariaDB) kann über Umgebungsvariablen (`LOKRO_DB_*`) vorbereitet werden; der Standard-Betrieb nutzt den sofort funktionsfähigen JSON-Store ohne externe Treiber.
