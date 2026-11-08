#!/usr/bin/env bash
###############################################################################
# LOKRO Server Control Center - Backup-Skript
#
# Usage: sudo bash backup.sh [ZIELVERZEICHNIS]
#
# Erstellt ein konsistentes Archiv von Daten + Umgebung + Konfiguration.
# Der Dienst wird kurz angehalten, damit der JSON-Store nicht mitten im
# Schreibvorgang kopiert wird.
###############################################################################
set -euo pipefail

APP_NAME="lokro-control-center"
APP_DIR="/opt/${APP_NAME}"
DATA_DIR="${APP_DIR}/data"
ENV_FILE="${APP_DIR}/.env"
SERVICE_NAME="${APP_NAME}"
TARGET_DIR="${1:-/var/backups/${APP_NAME}}"

log()  { echo "[lokro-backup] $*"; }
err()  { echo "[lokro-backup] FEHLER: $*" >&2; exit 1; }

if [ "$(id -u)" -ne 0 ]; then err "Als root ausfuehren (sudo bash backup.sh)."; fi

mkdir -p "${TARGET_DIR}"
TS="$(date +%Y%m%d%H%M%S)"
ARCHIVE="${TARGET_DIR}/${APP_NAME}-backup-${TS}.tar.gz"

log "Service '${SERVICE_NAME}' kurz anhalten ..."
systemctl stop "${SERVICE_NAME}" 2>/dev/null || true

log "Erstelle Archiv ${ARCHIVE} ..."
tar -czf "${ARCHIVE}" \
  -C "${APP_DIR}" data .env 2>/dev/null || \
  tar -czf "${ARCHIVE}" -C "${APP_DIR}" . 2>/dev/null || \
  err "Archiv konnte nicht erstellt werden."

systemctl start "${SERVICE_NAME}" 2>/dev/null || true
systemctl enable "${SERVICE_NAME}" 2>/dev/null || true

log "Backup fertiggestellt: ${ARCHIVE}"
log "Größe: $(du -h "${ARCHIVE}" | cut -f1)"
