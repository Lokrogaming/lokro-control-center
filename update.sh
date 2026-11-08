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

if [ "$(id -u)" -ne 0 ]; then err "Als root ausfuehren (sudo bash update.sh)."; fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Vor dem Update ein Backup anlegen
mkdir -p "${BACKUP_DIR}"
TS="$(date +%Y%m%d%H%M%S)"
BACKUP_FILE="${BACKUP_DIR}/lokro-pre-update-${TS}.tar.gz"

log "Service '${SERVICE_NAME}' stoppen ..."
systemctl stop "${SERVICE_NAME}" 2>/dev/null || true

log "Backup erstellen: ${BACKUP_FILE} ..."
tar -czf "${BACKUP_FILE}" -C "$(dirname "${APP_DIR}")" "$(basename "${APP_DIR}")" 2>/dev/null || \
  tar -czf "${BACKUP_FILE}" -C "${APP_DIR}" . 2>/dev/null || true

# Quellcode aktualisieren (nur Dateien, die im Installer-Paket sind; data/ bleibt erhalten)
log "Kopiere neue Dateien ..."
cp -a "${SCRIPT_DIR}/." "${APP_DIR}/"
chown -R lokro:lokro "${APP_DIR}"

# Backend neu bauen
log "Baue Go-Backend neu ..."
( cd "${APP_DIR}/backend" && go build -o lokro-server ./cmd/server )
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
  log "Update erfolgreich. Service läuft."
else
  err "Service startet nicht. 'systemctl status ${SERVICE_NAME}' prüfen."
fi
