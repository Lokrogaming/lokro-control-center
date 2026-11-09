#!/usr/bin/env bash
###############################################################################
# LOKRO Server Control Center - Installationsskript
#
# Usage: sudo bash install.sh
#        (mit Initial-Admin: sudo -E bash install.sh, nachdem INITIAL_ADMIN_*
#         Umgebungsvariablen gesetzt wurden)
#
# Zielsysteme: Ubuntu 24.04 LTS+, Debian 12+ (x86_64), ohne Docker.
# Nicht-Docker, idempotent, ueberschreibt keine existierenden Nginx-Konfigs.
###############################################################################
set -euo pipefail

# -----------------------------------------------------------------------------
# Konfiguration (ueber Environment-Variablen ueberschreibbar)
# -----------------------------------------------------------------------------
APP_NAME="lokro-control-center"
APP_USER="lokro"
APP_DIR="/opt/${APP_NAME}"
DATA_DIR="${APP_DIR}/data"
ENV_FILE="${APP_DIR}/.env"
SERVICE_NAME="${APP_NAME}"
DOMAIN="${LOKRO_DOMAIN:-server.lokro.dev}"
ADMIN_EMAIL="${LOKRO_ADMIN_EMAIL:-admin@${DOMAIN}}"
APP_PORT="${LOKRO_PORT:-8080}"
APP_HOST="${LOKRO_HOST:-127.0.0.1}"

# Optional: Pterodactyl-Datenbank (MySQL/MariaDB) für späteren SQL-Betrieb.
# Der Standard-Betrieb nutzt den integrierten JSON-Store (keine externen Treiber nötig).
PD_DB_HOST="${LOKRO_DB_HOST:-}"
PD_DB_PORT="${LOKRO_DB_PORT:-3306}"
PD_DB_NAME="${LOKRO_DB_NAME:-lokro}"
PD_DB_USER="${LOKRO_DB_USER:-lokro}"
PD_DB_PASS="${LOKRO_DB_PASS:-}"

NGINX_SITE_AVAILABLE="/etc/nginx/sites-available/${APP_NAME}"
NGINX_SITE_ENABLED="/etc/nginx/sites-enabled/${APP_NAME}"

log()  { echo "[lokro-install] $*"; }
err()  { echo "[lokro-install] FEHLER: $*" >&2; exit 1; }

# -----------------------------------------------------------------------------
# 0.5. Automatischer Update-Check vor der Installation
#      Über GitHub API nach einer neuen Version suchen, herunterladen und
#      install.sh aus dem neuen Quellcode neu starten.
#      Steuern über LOKRO_AUTO_UPDATE (1 = an, 0 = aus; Default: an).
# -----------------------------------------------------------------------------
check_remote_update() {
  [ "${LOKRO_AUTO_UPDATE:-1}" = "1" ] || return 0
  command -v curl >/dev/null 2>&1 || return 0

  local remote_sha installed_sha=""
  remote_sha="$(curl -fsSL --max-time 15 \
    "https://api.github.com/repos/Lokrogaming/lokro-control-center/commits/main" 2>/dev/null \
    | grep -o '"sha": *"[a-z0-9]*"' | head -1 \
    | sed 's/.*"sha": *"\([a-z0-9]*\)".*/\1/')" || true

  if [ -z "${remote_sha}" ]; then
    log "Remote-Version konnte nicht ermittelt werden. Fahre mit lokaler Version fort."
    return 0
  fi

  if [ -f "${APP_DIR}/.installed_commit" ]; then
    installed_sha="$(cat "${APP_DIR}/.installed_commit" 2>/dev/null || true)"
  fi

  # Bereits aktuell – keine Aktion nötig.
  if [ -n "${installed_sha}" ] && [ "${remote_sha}" = "${installed_sha}" ]; then
    return 0
  fi

  if [ -n "${installed_sha}" ]; then
    log "Neuere Version auf GitHub gefunden (${remote_sha:0:7} vs ${installed_sha:0:7})."
    log "Lade aktuellen Quellcode herunter und starte install.sh neu ..."
  fi

  local tmp_dir
  tmp_dir="$(mktemp -d)"
  # Aufräumen bei normalem Skript-Ende (exec ersetzt den Prozess, dann greift
  # dieser Trap nicht mehr – was gewollt ist, da der neue Prozess läuft).
  trap 'rm -rf "${tmp_dir}"' EXIT

  if ! curl -fsSL --max-time 120 \
    -o "${tmp_dir}/lokro.tar.gz" \
    "https://github.com/Lokrogaming/lokro-control-center/archive/refs/heads/main.tar.gz"; then
    log "WARNUNG: Update-Download fehlgeschlagen. Fahre mit lokaler Version fort."
    trap - EXIT
    return 0
  fi

  mkdir -p "${tmp_dir}/lokro-src"
  if ! tar -xzf "${tmp_dir}/lokro.tar.gz" -C "${tmp_dir}/lokro-src" --strip-components=1; then
    log "WARNUNG: Update-Archiv konnte nicht entpackt werden. Fahre mit lokaler Version fort."
    trap - EXIT
    return 0
  fi

  # Commit-Stand bereits im Zielverzeichnis merken, damit das neu gestartete
  # install.sh nicht sofort erneut ein Update erkennt (Endlosschleife vermeiden).
  echo "${remote_sha}" > "${APP_DIR}/.installed_commit" 2>/dev/null || true
  chown "${APP_USER}:${APP_USER}" "${APP_DIR}/.installed_commit" 2>/dev/null || true

  log "Neuer Quellcode liegt unter ${tmp_dir}/lokro-src. Starte install.sh neu ..."
  exec bash "${tmp_dir}/lokro-src/install.sh" "$@"
}

# Copy the application payload without touching runtime state (.env, data/) or
# development artefacts (node_modules, local builds, git metadata).
copy_payload() {
  local src="$1" dst="$2"
  mkdir -p "${dst}"
  # 'file changed as we read it' Warnings von tar sind harmlos; mit set -e +
  # pipefail würden sie das Skript jedoch abbrechen, daher temporär ausstellen.
  set +e
  tar -C "${src}" \
    --exclude='./.git' \
    --exclude='./.env' \
    --exclude='./data' \
    --exclude='./node_modules' \
    --exclude='./frontend/node_modules' \
    --exclude='./frontend/dist' \
    --exclude='./backend/lokro-server' \
    -cf - . | tar -C "${dst}" -xf -
  set -e
}

# Build the Vite frontend into frontend/dist. Node.js is only needed here (at
# build/deploy time); the running service serves the static files via Go.
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

# -----------------------------------------------------------------------------
# 0. Voraussetzungen: Root, unterstütztes OS
# -----------------------------------------------------------------------------
if [ "$(id -u)" -ne 0 ]; then
  err "Dieses Skript muss als root ausgefuehrt werden (sudo bash install.sh)."
fi

if [ ! -f /etc/os-release ]; then
  err "/etc/os-release nicht gefunden."
fi
# shellcheck disable=SC1091
. /etc/os-release
case "${ID:-}" in
  ubuntu|debian) : ;;
  *) err "Unterstütztes Betriebssystem erforderlich (Ubuntu/Debian), erkannt: ${ID:-unbekannt}" ;;
esac
log "Betriebssystem: ${PRETTY_NAME:-${ID}}"

# Vor der eigentlichen Installation auf neuere Version prüfen (ggf. Selbst-Update).
check_remote_update

# -----------------------------------------------------------------------------
# 1. Abhängigkeiten installieren
# -----------------------------------------------------------------------------
log "Aktualisiere Paketquellen und installiere Abhängigkeiten ..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y --no-install-recommends \
  golang-go nginx certbot curl ca-certificates openssl nodejs npm

command -v go >/dev/null 2>&1 || err "go konnte nicht installiert werden."
command -v node >/dev/null 2>&1 || err "Node.js konnte nicht installiert werden."
command -v npm >/dev/null 2>&1 || err "npm konnte nicht installiert werden."

# -----------------------------------------------------------------------------
# 2. Dedizierter System-Benutzer
# -----------------------------------------------------------------------------
if ! id -u "${APP_USER}" >/dev/null 2>&1; then
  log "Erstelle System-Benutzer '${APP_USER}' ..."
  useradd --system --create-home --shell /usr/sbin/nologin "${APP_USER}"
else
  log "Benutzer '${APP_USER}' existiert bereits."
fi

# -----------------------------------------------------------------------------
# 3. Applikation unter /opt/lokro-control-center installieren
# -----------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
log "Installiere Applikation nach ${APP_DIR} ..."
mkdir -p "${APP_DIR}"
# Quellcode kopieren (aus dem Verzeichnis des Installers). data/ und .env bleiben
# bei einer Wiederholung unangetastet.
copy_payload "${SCRIPT_DIR}" "${APP_DIR}"

# Backend bauen
log "Baue Go-Backend ..."
( cd "${APP_DIR}/backend" && go build -buildvcs=false -o lokro-server ./cmd/server )
[ -x "${APP_DIR}/backend/lokro-server" ] || err "Backend-Build fehlgeschlagen."

# Frontend bauen (wird vom Go-Backend aus frontend/dist ausgeliefert)
build_frontend "${APP_DIR}/frontend"

# -----------------------------------------------------------------------------
# 4. Verzeichnisse & Berechtigungen
# -----------------------------------------------------------------------------
log "Konfiguriere Verzeichnisse und Berechtigungen ..."
mkdir -p "${DATA_DIR}"
chown -R "${APP_USER}:${APP_USER}" "${APP_DIR}"
chmod 750 "${APP_DIR}"
chmod 700 "${DATA_DIR}"

# -----------------------------------------------------------------------------
# 5. Umgebungsdatei (.env) anlegen (nur bei Erstinstallation)
# -----------------------------------------------------------------------------
if [ ! -f "${ENV_FILE}" ]; then
  log "Erstelle initiale .env ..."
  cp "${APP_DIR}/.env.example" "${ENV_FILE}"
  SESSION_SECRET="$(openssl rand -hex 32)"
  sed -i "s|^SESSION_SECRET=.*|SESSION_SECRET=${SESSION_SECRET}|" "${ENV_FILE}"
  sed -i "s|^DATABASE_URL=.*|DATABASE_URL=${DATA_DIR}/lokro.json|" "${ENV_FILE}"
  sed -i "s|^APP_PORT=.*|APP_PORT=${APP_PORT}|" "${ENV_FILE}"
  sed -i "s|^APP_HOST=.*|APP_HOST=${APP_HOST}|" "${ENV_FILE}"
  # Optional: Pterodactyl-DB Verbindungsdaten (für späteren SQL-Betrieb reserviert).
  if [ -n "${PD_DB_HOST}" ]; then
    sed -i "s|^#PTERODACTYL_DB_HOST=.*|PTERODACTYL_DB_HOST=${PD_DB_HOST}|" "${ENV_FILE}"
    sed -i "s|^#PTERODACTYL_DB_PORT=.*|PTERODACTYL_DB_PORT=${PD_DB_PORT}|" "${ENV_FILE}"
    sed -i "s|^#PTERODACTYL_DB_NAME=.*|PTERODACTYL_DB_NAME=${PD_DB_NAME}|" "${ENV_FILE}"
    sed -i "s|^#PTERODACTYL_DB_USER=.*|PTERODACTYL_DB_USER=${PD_DB_USER}|" "${ENV_FILE}"
    sed -i "s|^#PTERODACTYL_DB_PASS=.*|PTERODACTYL_DB_PASS=${PD_DB_PASS}|" "${ENV_FILE}"
    log "Pterodactyl-DB Verbindungsdaten in .env hinterlegt (SQL-Treiber muss nachinstalliert werden)."
  fi
  # Secure-Cookie erst nur aktivieren, wenn ein HTTPS-Zertifikat vorliegt (siehe Certbot-Schritt weiter unten).
  sed -i "s|^COOKIE_SECURE=.*|COOKIE_SECURE=false|" "${ENV_FILE}"

  # Initiale Admin-Credentials in .env sichern, falls als Umgebungsvariable
  # an install.sh übergeben (z. B. sudo -E bash install.sh).
  # Das systemd-Service lädt .env via EnvironmentFile; Go liest sie bei
  # seedDefaultAdmin beim ersten Start (nur wenn noch kein User existiert).
  if [ -n "${INITIAL_ADMIN_USERNAME:-}" ]; then
    if grep -q '^INITIAL_ADMIN_USERNAME=' "${ENV_FILE}"; then
      sed -i "s|^INITIAL_ADMIN_USERNAME=.*|INITIAL_ADMIN_USERNAME=${INITIAL_ADMIN_USERNAME}|" "${ENV_FILE}"
    else
      echo "INITIAL_ADMIN_USERNAME=${INITIAL_ADMIN_USERNAME}" >> "${ENV_FILE}"
    fi
  fi
  if [ -n "${INITIAL_ADMIN_PASSWORD:-}" ]; then
    if grep -q '^INITIAL_ADMIN_PASSWORD=' "${ENV_FILE}"; then
      sed -i "s|^INITIAL_ADMIN_PASSWORD=.*|INITIAL_ADMIN_PASSWORD=${INITIAL_ADMIN_PASSWORD}|" "${ENV_FILE}"
    else
      echo "INITIAL_ADMIN_PASSWORD=${INITIAL_ADMIN_PASSWORD}" >> "${ENV_FILE}"
    fi
  fi
  if [ -n "${INITIAL_ADMIN_EMAIL:-}" ]; then
    if grep -q '^INITIAL_ADMIN_EMAIL=' "${ENV_FILE}"; then
      sed -i "s|^INITIAL_ADMIN_EMAIL=.*|INITIAL_ADMIN_EMAIL=${INITIAL_ADMIN_EMAIL}|" "${ENV_FILE}"
    else
      echo "INITIAL_ADMIN_EMAIL=${INITIAL_ADMIN_EMAIL}" >> "${ENV_FILE}"
    fi
  fi

  chown "${APP_USER}:${APP_USER}" "${ENV_FILE}"
  chmod 600 "${ENV_FILE}"
  log ".env erstellt (Session-Secret automatisch generiert)."
else
  log ".env existiert bereits - nicht ueberschrieben."
fi

# -----------------------------------------------------------------------------
# 6. systemd-Service-Unit
# -----------------------------------------------------------------------------
log "Erstelle systemd-Unit ..."
cp "${APP_DIR}/deploy/systemd/lokro.service.template" "/etc/systemd/system/${SERVICE_NAME}.service"
sed -i "s|^EnvironmentFile=.*|EnvironmentFile=${ENV_FILE}|" "/etc/systemd/system/${SERVICE_NAME}.service"
systemctl daemon-reload
systemctl enable --now "${SERVICE_NAME}"
log "Service '${SERVICE_NAME}' aktiviert und gestartet."

# Kurze Wartezeit, damit der JSON-Store initialisiert wird
sleep 2

# -----------------------------------------------------------------------------
# 7. Nginx Reverse Proxy (existierende Konfigurationen nicht ueberschreiben)
# -----------------------------------------------------------------------------
log "Richte Nginx-Reverse-Proxy ein ..."
if [ ! -f "${NGINX_SITE_AVAILABLE}" ]; then
  cp "${APP_DIR}/deploy/nginx/lokro.conf.template" "${NGINX_SITE_AVAILABLE}"
  # Site aktivieren, falls noch nicht geschehen
  if [ ! -e "${NGINX_SITE_ENABLED}" ]; then
    ln -s "${NGINX_SITE_AVAILABLE}" "${NGINX_SITE_ENABLED}"
  fi
  # Self-Signed Fallback-Zertifikat, damit Nginx auch vor Certbot startet
  if [ ! -f /etc/ssl/certs/lokro-selfsigned.crt ]; then
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
      -keyout /etc/ssl/certs/lokro-selfsigned.key \
      -out /etc/ssl/certs/lokro-selfsigned.crt \
      -subj "/CN=${DOMAIN}" >/dev/null 2>&1 || true
  fi
  # Falls Zertifikat noch nicht existiert, verweise Nginx auf das Self-Signed-Zertifikat
  if [ ! -d "/etc/letsencrypt/live/${DOMAIN}" ] && [ -f /etc/ssl/certs/lokro-selfsigned.crt ]; then
    sed -i "s|/etc/letsencrypt/live/${DOMAIN}/fullchain.pem|/etc/ssl/certs/lokro-selfsigned.crt|" "${NGINX_SITE_AVAILABLE}"
    sed -i "s|/etc/letsencrypt/live/${DOMAIN}/privkey.pem|/etc/ssl/certs/lokro-selfsigned.key|" "${NGINX_SITE_AVAILABLE}"
  fi
  nginx -t && systemctl reload nginx
  log "Nginx konfiguriert und neu geladen."
else
  log "Nginx-Site existiert bereits unter ${NGINX_SITE_AVAILABLE} - nicht ueberschrieben."
fi

# -----------------------------------------------------------------------------
# 8. SSL via Certbot (falls Moeglichkeit besteht)
# -----------------------------------------------------------------------------
if [ ! -d "/etc/letsencrypt/live/${DOMAIN}" ]; then
  log "Versuche, SSL-Zertifikat via Certbot zu beantragen (Domain: ${DOMAIN}) ..."
  if certbot --nginx -d "${DOMAIN}" --non-interactive --agree-tos \
     --email "${ADMIN_EMAIL}" --redirect --hsts --staple-ocsp 2>/dev/null; then
    log "SSL-Zertifikat installiert. Nginx wurde von Certbot neu konfiguriert."
    systemctl reload nginx
  else
    log "WARNUNG: Certbot konnte kein Zertifikat beantragen (Domain evtl. nicht erreichbar)."
    log "         Die Seite lauft voruebergehend mit dem Self-Signed-Zertifikat bzw. auf HTTP."
    log "         Führen Sie später erneut aus: certbot --nginx -d ${DOMAIN}"
  fi
else
  log "SSL-Zertifikat existiert bereits."
fi

# Secure-Cookie aktivieren, wenn HTTPS nun verfügbar ist
if [ -d "/etc/letsencrypt/live/${DOMAIN}" ] && [ -f "${ENV_FILE}" ]; then
  if grep -q '^COOKIE_SECURE=false' "${ENV_FILE}"; then
    sed -i 's|^COOKIE_SECURE=false|COOKIE_SECURE=true|' "${ENV_FILE}"
    systemctl restart "${SERVICE_NAME}" 2>/dev/null || true
    log "HTTPS erkannt – COOKIE_SECURE auf true gesetzt und Service neu gestartet."
  fi
fi

# -----------------------------------------------------------------------------
# Installierten Commit merken für spätere automatische Update-Prüfungen.
if command -v curl >/dev/null 2>&1; then
  INSTALLED_SHA="$(curl -fsSL --max-time 15 \
    "https://api.github.com/repos/Lokrogaming/lokro-control-center/commits/main" 2>/dev/null \
    | grep -o '"sha": *"[a-z0-9]*"' | head -1 \
    | sed 's/.*"sha": *"\([a-z0-9]*\)".*/\1/')" || true
  if [ -n "${INSTALLED_SHA}" ]; then
    echo "${INSTALLED_SHA}" > "${APP_DIR}/.installed_commit"
    chown "${APP_USER}:${APP_USER}" "${APP_DIR}/.installed_commit" 2>/dev/null || true
    log "Installierte Version merken: ${INSTALLED_SHA:0:7}"
  fi
fi

# -----------------------------------------------------------------------------
# 9. Fertig
# -----------------------------------------------------------------------------
log "------------------------------------------------------------"
log "Installation abgeschlossen."
log "Backend:  systemctl status ${SERVICE_NAME}"
log "Nginx:    ${NGINX_SITE_AVAILABLE}"
log "Daten:    ${DATA_DIR}/lokro.json"
log "Umgebung: ${ENV_FILE}"
log "URL:      https://${DOMAIN}  (nach erfolgreichem Certbot-Lauf)"
log "------------------------------------------------------------"
