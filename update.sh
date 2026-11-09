#!/usr/bin/env bash
###############################################################################
# LOKRO Server Control Center - Update-Skript
#
# Usage: sudo bash update.sh
#
# Haltet den laufenden Dienst an, baut das Backend neu, startet wieder.
# Daten unter /opt/lokro-control-center/data bleiben erhalten.
###############################################################################
set -euo pipefail

APP_NAME="lokro-control-center"
APP_DIR="/opt/${APP_NAME}"
SERVICE_NAME="${APP_NAME}"
BACKUP_DIR="/var/backups/${APP_NAME}"

log()  { echo "[lokro-update] $*"; }
err()  { echo "[lokro-update] FEHLER: $*" >&2; exit 1; }

# Copy the application payload without touching runtime state (.env, data/) or
# development artefacts (node_modules, local builds, git metadata).
copy_payload() {
  local src="$1" dst="$2"
  mkdir -p "${dst}"
  tar -C "${src}" \
    --exclude='./.git' \
    --exclude='./.env' \
    --exclude='./data' \
    --exclude='./node_modules' \
    --exclude='./frontend/node_modules' \
    --exclude='./frontend/dist' \
    --exclude='./backend/lokro-server' \
    -cf - . | tar -C "${dst}" -xf -
}

# Build the Vite frontend into frontend/dist. Node.js is only needed at build
# time; the running service serves the static files via Go.
build_frontend() {
  local dir="$1"
  [ -f "${dir}/package.json" ] || err "frontend/package.json fehlt unter ${dir}."
  command -v node >/dev/null 2>&1 || err "Node.js fehlt. Installation: apt-get install -y nodejs npm"
  command -v npm >/dev/null 2>&1 || err "npm fehlt. Installation: apt-get install -y nodejs npm"

  log "Installiere Frontend-Abhaengigkeiten ..."
  if [ -f "${dir}/package-lock.json" ]; then
    ( cd "${dir}" && npm ci --no-audit --no-fund )
  else
    ( cd "${dir}" && npm install --no-audit --no-fund )
  fi

  log "Baue Frontend (Vite) ..."
  ( cd "${dir}" && npm run build )
  [ -f "${dir}/dist/index.html" ] || err "Frontend-Build fehlgeschlagen: ${dir}/dist/index.html fehlt."
}

if [ "$(id -u)" -ne 0 ]; then err "Als root ausfuehren (sudo bash update.sh)."; fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# If anything below fails, try to bring the previously running service back up.
on_error() {
  local code=$?
  log "Update fehlgeschlagen (Exit ${code}); versuche Service wieder zu starten ..."
  systemctl start "${SERVICE_NAME}" 2>/dev/null || true
}
trap on_error ERR

# Vor dem Update ein Backup anlegen
mkdir -p "${BACKUP_DIR}"
TS="$(date +%Y%m%d%H%M%S)"
BACKUP_FILE="${BACKUP_DIR}/lokro-pre-update-${TS}.tar.gz"

log "Service '${SERVICE_NAME}' stoppen ..."
systemctl stop "${SERVICE_NAME}" 2>/dev/null || true

log "Backup erstellen: ${BACKUP_FILE} ..."
tar -czf "${BACKUP_FILE}" -C "$(dirname "${APP_DIR}")" "$(basename "${APP_DIR}")" 2>/dev/null || \
  tar -czf "${BACKUP_FILE}" -C "${APP_DIR}" . 2>/dev/null || true

# Quellcode aktualisieren (data/ und .env bleiben erhalten)
log "Kopiere neue Dateien ..."
copy_payload "${SCRIPT_DIR}" "${APP_DIR}"

# Frontend neu bauen (wird vom Go-Backend aus frontend/dist ausgeliefert)
build_frontend "${APP_DIR}/frontend"

# Backend neu bauen
log "Baue Go-Backend neu ..."
( cd "${APP_DIR}/backend" && go build -buildvcs=false -o lokro-server ./cmd/server )
[ -x "${APP_DIR}/backend/lokro-server" ] || err "Backend-Build fehlgeschlagen."

# Berechtigungen
chown -R lokro:lokro "${APP_DIR}"
chmod 750 "${APP_DIR}"
chmod 700 "${APP_DIR}/data"

# Nginx-Config aktualisieren (falls vorhanden, nicht überschreiben bei manuellen Änderungen)
if [ -f "${APP_DIR}/deploy/nginx/lokro.conf.template" ] && [ -f /etc/nginx/sites-available/${APP_NAME} ]; then
  # Nur kopieren, wenn die Datei im Paket neuer ist; ansonsten bestehen lassen.
  :
fi

log "Service '${SERVICE_NAME}' starten ..."
systemctl start "${SERVICE_NAME}"
systemctl enable "${SERVICE_NAME}" 2>/dev/null || true
nginx -t 2>/dev/null && systemctl reload nginx 2>/dev/null || true

sleep 2
if systemctl is-active --quiet "${SERVICE_NAME}"; then
  trap - ERR
  log "Update erfolgreich. Service läuft."
else
  err "Service startet nicht. 'systemctl status ${SERVICE_NAME}' prüfen."
fi
