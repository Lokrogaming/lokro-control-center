#!/usr/bin/env bash
###############################################################################
# LOKRO Server Control Center - Deinstallationsskript
#
# Usage: sudo bash uninstall.sh
#        CONFIRM=1 sudo bash uninstall.sh   (ohne Nachfrage)
###############################################################################
set -euo pipefail

APP_NAME="lokro-control-center"
APP_USER="lokro"
APP_DIR="/opt/${APP_NAME}"
SERVICE_NAME="${APP_NAME}"
NGINX_SITE_AVAILABLE="/etc/nginx/sites-available/${APP_NAME}"
NGINX_SITE_ENABLED="/etc/nginx/sites-enabled/${APP_NAME}"

CONFIRM="${CONFIRM:-0}"

log()  { echo "[lokro-uninstall] $*"; }
err()  { echo "[lokro-uninstall] FEHLER: $*" >&2; exit 1; }

if [ "$(id -u)" -ne 0 ]; then err "Als root ausfuehren (sudo bash uninstall.sh)."; fi

if [ "${CONFIRM}" != "1" ]; then
  echo "ACHTUNG: Dieser Vorgang entfernt die Applikation unter ${APP_DIR}"
  echo "und die zugehörige systemd-Unit sowie die Nginx-Site."
  echo "Daten unter ${APP_DIR}/data gehen VERLOREN (sofern nicht gebackupt)."
  read -rp "Fortfahren? (ja/NEIN): " ans
  [ "${ans}" = "ja" ] || err "Abbruch durch Nutzer."
fi

# 1. Service stoppen & deaktivieren
if systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1; then
  log "Stoppe Service '${SERVICE_NAME}' ..."
  systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
  systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
  rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
  systemctl daemon-reload
  log "Service entfernt."
else
  log "Keine systemd-Unit gefunden."
fi

# 2. Nginx-Site entfernen (nur das, was wir verwalten)
if [ -e "${NGINX_SITE_ENABLED}" ]; then
  rm -f "${NGINX_SITE_ENABLED}"
  log "Nginx-Symlink entfernt."
fi
if [ -f "${NGINX_SITE_AVAILABLE}" ]; then
  rm -f "${NGINX_SITE_AVAILABLE}"
  log "Nginx-Site-Konfiguration entfernt."
fi
nginx -t 2>/dev/null && systemctl reload nginx 2>/dev/null || true

# 3. Applikationsverzeichnis entfernen
if [ -d "${APP_DIR}" ]; then
  log "Entferne ${APP_DIR} ..."
  rm -rf "${APP_DIR}"
fi

# 4. System-Benutzer optional entfernen (Kommentar: Datenverlust vermeiden)
if id -u "${APP_USER}" >/dev/null 2>&1; then
  log "System-Benutzer '${APP_USER}' verbleibt (Datenvermeidung). Entfernen optional via: deluser ${APP_USER}"
fi

log "------------------------------------------------------------"
log "Deinstallation abgeschlossen."
log "Eventuell noch erledigen: Zertifikat (certbot) entfernen, falls nicht mehr benötigt."
log "------------------------------------------------------------"
