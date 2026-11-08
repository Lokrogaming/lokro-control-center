#!/usr/bin/env bash
###############################################################################
# LOKRO Server Control Center - Restore-Skript
#
# Usage: sudo bash restore.sh /pfad/zum/backup.tar.gz
#
# Stellt Daten + Umgebung aus einem Backup wieder her.
# Der Dienst wird währenddessen angehalten.
###############################################################################
set -euo pipefail

APP_NAME="lokro-control-center"
APP_DIR="/opt/${APP_NAME}"
SERVICE_NAME="${APP_NAME}"

BACKUP_FILE="${1:-}"

log()  { echo "[lokro-restore] $*"; }
err()  { echo "[lokro-restore] FEHLER: $*" >&2; exit 1; }

if [ "$(id -u)" -ne 0 ]; then err "Als root ausfuehren (sudo bash restore.sh)."; fi
if [ -z "${BACKUP_FILE}" ]; then err "Usage: sudo bash restore.sh /pfad/zum/backup.tar.gz"; fi
if [ ! -f "${BACKUP_FILE}" ]; then err "Backup-Datei nicht gefunden: ${BACKUP_FILE}"; fi

log "Service '${SERVICE_NAME}' stoppen ..."
systemctl stop "${SERVICE_NAME}" 2>/dev/null || true

# Aktuelles Verzeichnis sichern (falls vorhanden)
if [ -d "${APP_DIR}" ]; then
  TS="$(date +%Y%m%d%H%M%S)"
  mv "${APP_DIR}" "${APP_DIR}.pre-restore-${TS}" 2>/dev/null || true
fi

mkdir -p "${APP_DIR}"
log "Extrahiere Backup nach ${APP_DIR} ..."
tar -xzf "${BACKUP_FILE}" -C "${APP_DIR}"

chown -R lokro:lokro "${APP_DIR}"
chmod 750 "${APP_DIR}"
chmod 700 "${APP_DIR}/data" 2>/dev/null || true
chmod 600 "${APP_DIR}/.env" 2>/dev/null || true

log "Service '${SERVICE_NAME}' starten ..."
systemctl start "${SERVICE_NAME}"
systemctl enable "${SERVICE_NAME}" 2>/dev/null || true

sleep 2
if systemctl is-active --quiet "${SERVICE_NAME}"; then
  log "Restore erfolgreich. Service läuft."
  log "Altes Verzeichnis (falls vorhanden): ${APP_DIR}.pre-restore-*"
else
  err "Service startet nicht. 'systemctl status ${SERVICE_NAME}' prüfen."
fi
