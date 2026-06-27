#!/bin/sh
set -eu

APP_NAME="pangolite"
REPO="${PANGOLITE_REPO:-thowilabs/pangolite}"
REQUESTED_VERSION=""
INSTALL_DIR="${PANGOLITE_INSTALL_DIR:-/opt/pangolite}"
DATA_DIR="${PANGOLITE_DATA_DIR:-$INSTALL_DIR/data}"
PUBLIC_DIR="$INSTALL_DIR/public"
BIN_PATH="$INSTALL_DIR/pangolite"
CLIENT_BIN_PATH="$INSTALL_DIR/pangolite-client"
ENV_FILE="$INSTALL_DIR/pangolite.env"
TRAEFIK_DIR="${PANGOLITE_TRAEFIK_DIR:-/etc/traefik}"
TRAEFIK_VERSION="${TRAEFIK_VERSION:-3.7.5}"
PANEL_ADDR="${PANGOLITE_ADDR:-0.0.0.0:2424}"
HEALTH_URL="http://127.0.0.1:2424/healthz"
SERVER_IP=""
INIT_SYSTEM=""
OS_ID=""
OS_NAME=""
ARCH=""
TMP_DIR=""
SKIP_TRAEFIK="0"
ASSUME_YES="0"

log() { printf '\n[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
warn() { printf '\nADVERTENCIA: %s\n' "$*" >&2; }
fail() { printf '\nERROR: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

cleanup() {
  if [ -n "${TMP_DIR:-}" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}
trap cleanup EXIT INT TERM

usage() {
  cat <<USAGE
Pangolite installer

Uso:
  sh install.sh [opciones]

Opciones:
  --version X.Y       Instala una version especifica del release, por ejemplo 0.1 o v0.1.
  --repo OWNER/REPO   Repo de GitHub a usar. Default: $REPO
  --install-dir DIR   Directorio de instalacion. Default: $INSTALL_DIR
  --yes              No pedir confirmacion cuando ya exista una instalacion.
  --skip-traefik      Instala Pangolite sin instalar/configurar Traefik.
  -h, --help          Muestra esta ayuda.

Ejemplos:
  sh install.sh
  sh install.sh --version 0.3
  PANGOLITE_ADDR=0.0.0.0:2424 sh install.sh --version v0.3
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || fail "--version requiere un valor"
      REQUESTED_VERSION="$2"
      shift 2
      ;;
    --version=*)
      REQUESTED_VERSION="${1#--version=}"
      shift
      ;;
    --repo)
      [ "$#" -ge 2 ] || fail "--repo requiere OWNER/REPO"
      REPO="$2"
      shift 2
      ;;
    --repo=*)
      REPO="${1#--repo=}"
      shift
      ;;
    --install-dir)
      [ "$#" -ge 2 ] || fail "--install-dir requiere una ruta"
      INSTALL_DIR="$2"
      DATA_DIR="${PANGOLITE_DATA_DIR:-$INSTALL_DIR/data}"
      PUBLIC_DIR="$INSTALL_DIR/public"
      BIN_PATH="$INSTALL_DIR/pangolite"
      CLIENT_BIN_PATH="$INSTALL_DIR/pangolite-client"
      ENV_FILE="$INSTALL_DIR/pangolite.env"
      shift 2
      ;;
    --install-dir=*)
      INSTALL_DIR="${1#--install-dir=}"
      DATA_DIR="${PANGOLITE_DATA_DIR:-$INSTALL_DIR/data}"
      PUBLIC_DIR="$INSTALL_DIR/public"
      BIN_PATH="$INSTALL_DIR/pangolite"
      CLIENT_BIN_PATH="$INSTALL_DIR/pangolite-client"
      ENV_FILE="$INSTALL_DIR/pangolite.env"
      shift
      ;;
    --yes|-y)
      ASSUME_YES="1"
      shift
      ;;
    --skip-traefik)
      SKIP_TRAEFIK="1"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "opcion desconocida: $1"
      ;;
  esac
done

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    fail "ejecuta como root: sudo sh install.sh"
  fi
}

confirm_existing_install() {
  if [ -x "$BIN_PATH" ] || [ -f "$ENV_FILE" ] || service_exists "$APP_NAME"; then
    log "Instalacion existente detectada en $INSTALL_DIR"
    if [ -x "$BIN_PATH" ]; then
      "$BIN_PATH" version 2>/dev/null || true
    fi
    if [ "$ASSUME_YES" = "1" ]; then
      log "Continuando por --yes; se preservaran data/env y se reemplazaran binarios."
      return
    fi
    printf 'Deseas actualizar/reinstalar preservando datos y configuracion? [s/N]: '
    read ans || ans=""
    case "$ans" in
      s|S|si|SI|y|Y|yes|YES) ;;
      *) fail "instalacion cancelada" ;;
    esac
  fi
}

detect_os() {
  if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    OS_ID="${ID:-desconocido}"
    OS_NAME="${PRETTY_NAME:-${NAME:-$OS_ID}}"
  else
    OS_ID="desconocido"
    OS_NAME="$(uname -s 2>/dev/null || echo Linux)"
  fi
  log "Sistema detectado: $OS_NAME"
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    i386|i486|i586|i686) ARCH="386" ;;
    armv7l|armv7) ARCH="armv7" ;;
    *) fail "arquitectura no soportada: $(uname -m)" ;;
  esac
  log "Arquitectura detectada: linux/$ARCH"
}

detect_init_system() {
  if have systemctl && [ -d /run/systemd/system ]; then
    INIT_SYSTEM="systemd"
  elif have rc-service && have rc-update; then
    INIT_SYSTEM="openrc"
  elif have sv && { [ -d /etc/sv ] || [ -d /var/service ] || [ -d /service ]; }; then
    INIT_SYSTEM="runit"
  elif [ -d /etc/init.d ] && { have service || have update-rc.d || have chkconfig || have insserv; }; then
    INIT_SYSTEM="sysvinit"
  else
    INIT_SYSTEM="none"
  fi
  log "Gestor de arranque detectado: $INIT_SYSTEM"
  if [ "$INIT_SYSTEM" = "none" ]; then
    fail "no se detecto systemd, OpenRC, SysVinit ni runit; instala manualmente o ejecuta en un sistema con gestor de servicios compatible"
  fi
}

package_install() {
  [ "$#" -gt 0 ] || return 0
  if have apk; then
    apk add --no-cache "$@"
  elif have apt-get; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y "$@"
  elif have dnf; then
    dnf install -y "$@"
  elif have yum; then
    yum install -y "$@"
  elif have pacman; then
    pacman -Sy --noconfirm "$@"
  elif have zypper; then
    zypper --non-interactive install "$@"
  else
    fail "faltan dependencias y no se detecto gestor de paquetes compatible"
  fi
}

ensure_base_deps() {
  missing=""
  if ! have curl && ! have wget; then missing="$missing curl"; fi
  if ! have tar; then missing="$missing tar"; fi
  if ! have gzip; then missing="$missing gzip"; fi
  if ! have sha256sum; then missing="$missing coreutils"; fi
  if [ -n "$missing" ]; then
    log "Instalando dependencias base:$missing"
    # ca-certificates es necesario para GitHub en sistemas minimos como Alpine.
    package_install ca-certificates $missing
  fi
}

download_file() {
  url="$1"
  out="$2"
  if have curl; then
    curl -fL --retry 3 --connect-timeout 20 "$url" -o "$out"
  elif have wget; then
    wget -O "$out" "$url"
  else
    fail "se requiere curl o wget para descargar $url"
  fi
}

http_ok() {
  url="$1"
  if have curl; then
    curl -fsS --max-time 2 "$url" >/dev/null 2>&1
  elif have wget; then
    wget -q -T 2 -O /dev/null "$url" >/dev/null 2>&1
  else
    return 1
  fi
}

normalize_version() {
  v="$1"
  v="${v#v}"
  case "$v" in
    ''|*[!0-9.]*|*.*.*|.*|*.) fail "version invalida: $1. Usa formato X.Y, ejemplo 0.3" ;;
    *.*) printf '%s' "$v" ;;
    *) fail "version invalida: $1. Usa formato X.Y, ejemplo 0.3" ;;
  esac
}

release_url_for() {
  asset="$1"
  if [ -n "$REQUESTED_VERSION" ]; then
    v="$(normalize_version "$REQUESTED_VERSION")"
    printf 'https://github.com/%s/releases/download/v%s/%s' "$REPO" "$v" "$asset"
  else
    printf 'https://github.com/%s/releases/latest/download/%s' "$REPO" "$asset"
  fi
}

fetch_release_archive() {
  TMP_DIR="$(mktemp -d)"
  archive="$TMP_DIR/pangolite_linux_${ARCH}.tar.gz"
  asset="pangolite_linux_${ARCH}.tar.gz"
  url="$(release_url_for "$asset")"
  log "Descargando Pangolite: $url"
  download_file "$url" "$archive"

  checksums_url="$(release_url_for checksums.txt)"
  checksums="$TMP_DIR/checksums.txt"
  if download_file "$checksums_url" "$checksums" >/dev/null 2>&1; then
    if have sha256sum; then
      log "Verificando checksum"
      match="$TMP_DIR/checksum-$asset.txt"
      if grep "  $asset\$" "$checksums" > "$match"; then
        (cd "$TMP_DIR" && sha256sum -c "$match") || fail "checksum invalido para $asset"
      else
        warn "checksums.txt no contiene $asset; se continuara sin verificacion sha256"
      fi
    fi
  else
    warn "no se pudo descargar checksums.txt; se continuara sin verificacion sha256"
  fi

  mkdir -p "$TMP_DIR/extract"
  tar -xzf "$archive" -C "$TMP_DIR/extract"
  [ -x "$TMP_DIR/extract/pangolite" ] || fail "el release no contiene binario pangolite ejecutable"
  [ -x "$TMP_DIR/extract/pangolite-client" ] || fail "el release no contiene binario pangolite-client ejecutable"
}

service_exists() {
  name="$1"
  case "${INIT_SYSTEM:-}" in
    systemd) systemctl list-unit-files 2>/dev/null | grep -q "^${name}\.service" || [ -f "/etc/systemd/system/$name.service" ] ;;
    openrc) [ -x "/etc/init.d/$name" ] ;;
    sysvinit) [ -x "/etc/init.d/$name" ] ;;
    runit) [ -d "/etc/sv/$name" ] || [ -e "/var/service/$name" ] || [ -e "/service/$name" ] ;;
    *) return 1 ;;
  esac
}

service_stop() {
  name="$1"
  case "$INIT_SYSTEM" in
    systemd) systemctl stop "$name" >/dev/null 2>&1 || true ;;
    openrc) rc-service "$name" stop >/dev/null 2>&1 || true ;;
    sysvinit) service "$name" stop >/dev/null 2>&1 || "/etc/init.d/$name" stop >/dev/null 2>&1 || true ;;
    runit) sv down "$name" >/dev/null 2>&1 || true ;;
  esac
}

service_restart() {
  name="$1"
  case "$INIT_SYSTEM" in
    systemd) systemctl restart "$name" ;;
    openrc) rc-service "$name" restart ;;
    sysvinit) service "$name" restart || "/etc/init.d/$name" restart ;;
    runit) sv restart "$name" || sv up "$name" ;;
  esac
}

service_enable_start() {
  name="$1"
  case "$INIT_SYSTEM" in
    systemd)
      systemctl daemon-reload
      systemctl enable --now "$name"
      ;;
    openrc)
      rc-update add "$name" default >/dev/null 2>&1 || true
      rc-service "$name" restart || rc-service "$name" start
      ;;
    sysvinit)
      if have update-rc.d; then update-rc.d "$name" defaults >/dev/null 2>&1 || true; fi
      if have chkconfig; then chkconfig "$name" on >/dev/null 2>&1 || true; fi
      service "$name" restart || service "$name" start || "/etc/init.d/$name" restart || "/etc/init.d/$name" start
      ;;
    runit)
      mkdir -p /etc/sv
      if [ -d /var/service ]; then ln -sfn "/etc/sv/$name" "/var/service/$name"; fi
      if [ -d /service ]; then ln -sfn "/etc/sv/$name" "/service/$name"; fi
      sv up "$name" || true
      ;;
  esac
}

service_status_hint() {
  name="$1"
  case "$INIT_SYSTEM" in
    systemd) printf 'systemctl status %s --no-pager\njournalctl -u %s -f\n' "$name" "$name" ;;
    openrc) printf 'rc-service %s status\ntail -f %s/%s.log\n' "$name" "$DATA_DIR" "$name" ;;
    sysvinit) printf 'service %s status\ntail -f %s/%s.log\n' "$name" "$DATA_DIR" "$name" ;;
    runit) printf 'sv status %s\n' "$name" ;;
  esac
}

backup_existing_binaries() {
  ts="$(date +%Y%m%d%H%M%S)"
  if [ -x "$BIN_PATH" ]; then
    cp "$BIN_PATH" "$BIN_PATH.bak-$ts" || true
    log "Backup del binario anterior: $BIN_PATH.bak-$ts"
  fi
  if [ -x "$CLIENT_BIN_PATH" ]; then
    cp "$CLIENT_BIN_PATH" "$CLIENT_BIN_PATH.bak-$ts" || true
  fi
}

install_binaries() {
  log "Instalando binarios en $INSTALL_DIR"
  service_stop "$APP_NAME"
  backup_existing_binaries
  mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$PUBLIC_DIR"
  chmod 755 "$INSTALL_DIR"
  chmod 700 "$DATA_DIR"
  cp "$TMP_DIR/extract/pangolite" "$BIN_PATH"
  chmod 0755 "$BIN_PATH"
  cp "$TMP_DIR/extract/pangolite-client" "$CLIENT_BIN_PATH"
  chmod 0755 "$CLIENT_BIN_PATH"

  if [ -d "$TMP_DIR/extract/public" ]; then
    cp -R "$TMP_DIR/extract/public/." "$PUBLIC_DIR/"
    chmod 0755 "$PUBLIC_DIR"/* 2>/dev/null || true
  else
    cp "$TMP_DIR/extract/pangolite-client" "$PUBLIC_DIR/pangolite-client-linux-$ARCH"
    chmod 0755 "$PUBLIC_DIR/pangolite-client-linux-$ARCH"
  fi

  if [ -f "$TMP_DIR/extract/VERSION" ]; then
    cp "$TMP_DIR/extract/VERSION" "$INSTALL_DIR/VERSION"
  elif [ -n "$REQUESTED_VERSION" ]; then
    normalize_version "$REQUESTED_VERSION" > "$INSTALL_DIR/VERSION"
  else
    echo "latest" > "$INSTALL_DIR/VERSION"
  fi
}

set_env_value() {
  key="$1"
  value="$2"
  tmp="$ENV_FILE.tmp.$$"
  if [ -f "$ENV_FILE" ] && grep -q "^${key}=" "$ENV_FILE"; then
    sed "s|^${key}=.*|${key}=${value}|" "$ENV_FILE" > "$tmp"
    mv "$tmp" "$ENV_FILE"
  else
    printf '%s=%s\n' "$key" "$value" >> "$ENV_FILE"
  fi
}

detect_server_ip() {
  ip_value=""
  if have curl; then
    ip_value="$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
    [ -n "$ip_value" ] || ip_value="$(curl -fsS --max-time 5 https://ifconfig.me/ip 2>/dev/null || true)"
  elif have wget; then
    ip_value="$(wget -q -T 5 -O - https://api.ipify.org 2>/dev/null || true)"
  fi
  if [ -z "$ip_value" ] && have ip; then
    ip_value="$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++){if($i=="src"){print $(i+1); exit}}}' || true)"
  fi
  if [ -z "$ip_value" ]; then
    ip_value="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  fi
  SERVER_IP="$ip_value"
  if [ -n "$SERVER_IP" ]; then
    log "IP del servidor detectada: $SERVER_IP"
  else
    warn "no se pudo detectar IP publica; el panel seguira escuchando en $PANEL_ADDR"
  fi
}

write_env_file() {
  mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$PUBLIC_DIR"
  chmod 700 "$DATA_DIR"
  if [ ! -f "$ENV_FILE" ]; then
    cat > "$ENV_FILE" <<ENV
PANGOLITE_ADDR=$PANEL_ADDR
PANGOLITE_DATA=$DATA_DIR/pangolite.db
PANGOLITE_TRAEFIK_DIR=$TRAEFIK_DIR
PANGOLITE_PUBLIC_IP=$SERVER_IP
PANGOLITE_INITIAL_ADMIN_USER=admin
PANGOLITE_INITIAL_PASSWORD_FILE=$DATA_DIR/admin-password.txt
PANGOLITE_LOG_FILE=$DATA_DIR/pangolite.log
PANGOLITE_BACKUP_DIR=$DATA_DIR/backups
PANGOLITE_SUSPENSION_TEMPLATE_DIR=$DATA_DIR/templates/suspension
PANGOLITE_SESSION_DAYS=30
PANGOLITE_AUTO_TRAEFIK=1
PANGOLITE_CLIENT_LINUX_AMD64=$PUBLIC_DIR/pangolite-client-linux-amd64
PANGOLITE_CLIENT_WINDOWS_AMD64=$PUBLIC_DIR/pangolite-client-windows-amd64.exe
# Opcional: configura dominio/correo para que Traefik publique el panel por HTTP/HTTPS.
# PANGOLITE_DASHBOARD_DOMAIN=panel.midominio.com
# PANGOLITE_LETSENCRYPT_EMAIL=admin@midominio.com
ENV
    chmod 600 "$ENV_FILE"
  fi
  set_env_value PANGOLITE_DATA "$DATA_DIR/pangolite.db"
  set_env_value PANGOLITE_TRAEFIK_DIR "$TRAEFIK_DIR"
  set_env_value PANGOLITE_LOG_FILE "$DATA_DIR/pangolite.log"
  set_env_value PANGOLITE_BACKUP_DIR "$DATA_DIR/backups"
  set_env_value PANGOLITE_SUSPENSION_TEMPLATE_DIR "$DATA_DIR/templates/suspension"
  set_env_value PANGOLITE_CLIENT_LINUX_AMD64 "$PUBLIC_DIR/pangolite-client-linux-amd64"
  set_env_value PANGOLITE_CLIENT_WINDOWS_AMD64 "$PUBLIC_DIR/pangolite-client-windows-amd64.exe"
  [ -n "$SERVER_IP" ] && set_env_value PANGOLITE_PUBLIC_IP "$SERVER_IP"
}

traefik_arch() {
  case "$ARCH" in
    amd64) echo "amd64" ;;
    arm64) echo "arm64" ;;
    386) echo "386" ;;
    armv7) echo "armv7" ;;
    *) fail "arquitectura no soportada para Traefik: $ARCH" ;;
  esac
}

ensure_traefik_binary() {
  mkdir -p "$TRAEFIK_DIR" "$TRAEFIK_DIR/dynamic"
  touch "$TRAEFIK_DIR/acme.json"
  chmod 600 "$TRAEFIK_DIR/acme.json"

  if have traefik; then
    log "Traefik detectado: $(traefik version 2>/dev/null | head -1 || true)"
    return
  fi

  arch="$(traefik_arch)"
  work="$TMP_DIR/traefik"
  mkdir -p "$work"
  tarball="$work/traefik.tar.gz"
  url="https://github.com/traefik/traefik/releases/download/v${TRAEFIK_VERSION}/traefik_v${TRAEFIK_VERSION}_linux_${arch}.tar.gz"
  log "Traefik no esta instalado; descargando $url"
  download_file "$url" "$tarball"
  tar -xzf "$tarball" -C "$work"
  [ -x "$work/traefik" ] || fail "no se pudo extraer Traefik"
  cp "$work/traefik" /usr/local/bin/traefik
  chmod 0755 /usr/local/bin/traefik
  log "Traefik instalado en /usr/local/bin/traefik"
}

write_systemd_service() {
  name="$1"
  desc="$2"
  exec_line="$3"
  env_line="$4"
  file="/etc/systemd/system/$name.service"
  cat > "$file" <<SERVICE
[Unit]
Description=$desc
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
$env_line
ExecStart=$exec_line
Restart=always
RestartSec=3
WorkingDirectory=$INSTALL_DIR
NoNewPrivileges=true
PrivateTmp=true
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
SERVICE
}

write_openrc_service() {
  name="$1"
  desc="$2"
  run_cmd="$3"
  file="/etc/init.d/$name"
  cat > "$file" <<SERVICE
#!/sbin/openrc-run
name="$name"
description="$desc"
pidfile="/run/$name.pid"

start() {
  ebegin "Starting $name"
  start-stop-daemon --start --background --make-pidfile --pidfile "\$pidfile" --exec /bin/sh -- -c "$run_cmd"
  eend \$?
}

stop() {
  ebegin "Stopping $name"
  start-stop-daemon --stop --pidfile "\$pidfile" --retry TERM/10/KILL/5
  eend \$?
}

depend() {
  need net
}
SERVICE
  chmod 0755 "$file"
}

write_sysv_service() {
  name="$1"
  desc="$2"
  run_cmd="$3"
  file="/etc/init.d/$name"
  log_file="$DATA_DIR/$name.log"
  cat > "$file" <<SERVICE
#!/bin/sh
### BEGIN INIT INFO
# Provides:          $name
# Required-Start:    \$remote_fs \$syslog \$network
# Required-Stop:     \$remote_fs \$syslog \$network
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: $desc
### END INIT INFO

PIDFILE=/var/run/$name.pid
LOGFILE=$log_file

start() {
  if [ -f "\$PIDFILE" ] && kill -0 "\$(cat \$PIDFILE)" 2>/dev/null; then
    echo "$name ya esta ejecutandose"
    return 0
  fi
  echo "Starting $name"
  if command -v start-stop-daemon >/dev/null 2>&1; then
    start-stop-daemon --start --background --make-pidfile --pidfile "\$PIDFILE" --exec /bin/sh -- -c "$run_cmd >>\$LOGFILE 2>&1"
  else
    nohup /bin/sh -c "$run_cmd" >>"\$LOGFILE" 2>&1 &
    echo \$! > "\$PIDFILE"
  fi
}

stop() {
  echo "Stopping $name"
  if command -v start-stop-daemon >/dev/null 2>&1; then
    start-stop-daemon --stop --pidfile "\$PIDFILE" --retry TERM/10/KILL/5 || true
  elif [ -f "\$PIDFILE" ]; then
    kill "\$(cat \$PIDFILE)" 2>/dev/null || true
  fi
  rm -f "\$PIDFILE"
}

case "\$1" in
  start) start ;;
  stop) stop ;;
  restart) stop; sleep 1; start ;;
  status) [ -f "\$PIDFILE" ] && kill -0 "\$(cat \$PIDFILE)" 2>/dev/null && echo "$name activo" || echo "$name detenido" ;;
  *) echo "Uso: \$0 {start|stop|restart|status}"; exit 1 ;;
esac
SERVICE
  chmod 0755 "$file"
}

write_runit_service() {
  name="$1"
  run_cmd="$2"
  dir="/etc/sv/$name"
  mkdir -p "$dir"
  cat > "$dir/run" <<SERVICE
#!/bin/sh
exec /bin/sh -c "$run_cmd"
SERVICE
  chmod 0755 "$dir/run"
}

write_service_files() {
  log "Creando servicios para $INIT_SYSTEM"
  pangolite_run="cd '$INSTALL_DIR'; set -a; . '$ENV_FILE'; set +a; exec '$BIN_PATH' serve"
  traefik_bin="$(command -v traefik || echo /usr/local/bin/traefik)"
  traefik_run="exec '$traefik_bin' --configFile='$TRAEFIK_DIR/traefik.yml'"

  case "$INIT_SYSTEM" in
    systemd)
      write_systemd_service "$APP_NAME" "Pangolite control plane" "$BIN_PATH serve" "EnvironmentFile=$ENV_FILE"
      if [ "$SKIP_TRAEFIK" != "1" ] && ! systemctl list-unit-files 2>/dev/null | grep -q '^traefik\.service'; then
        write_systemd_service "traefik" "Traefik reverse proxy" "$traefik_bin --configFile=$TRAEFIK_DIR/traefik.yml" ""
      fi
      systemctl daemon-reload
      ;;
    openrc)
      write_openrc_service "$APP_NAME" "Pangolite control plane" "$pangolite_run"
      if [ "$SKIP_TRAEFIK" != "1" ] && [ ! -x /etc/init.d/traefik ]; then
        write_openrc_service "traefik" "Traefik reverse proxy" "$traefik_run"
      fi
      ;;
    sysvinit)
      write_sysv_service "$APP_NAME" "Pangolite control plane" "$pangolite_run"
      if [ "$SKIP_TRAEFIK" != "1" ] && [ ! -x /etc/init.d/traefik ]; then
        write_sysv_service "traefik" "Traefik reverse proxy" "$traefik_run"
      fi
      ;;
    runit)
      write_runit_service "$APP_NAME" "$pangolite_run"
      if [ "$SKIP_TRAEFIK" != "1" ]; then
        write_runit_service "traefik" "$traefik_run"
      fi
      ;;
  esac
}

configure_traefik() {
  [ "$SKIP_TRAEFIK" = "1" ] && return 0
  mkdir -p "$TRAEFIK_DIR" "$TRAEFIK_DIR/dynamic"
  if [ -f "$TRAEFIK_DIR/traefik.yml" ] && ! grep -q 'managed by Pangolite' "$TRAEFIK_DIR/traefik.yml"; then
    backup="$TRAEFIK_DIR/traefik.yml.backup-$(date +%Y%m%d%H%M%S)"
    cp "$TRAEFIK_DIR/traefik.yml" "$backup"
    log "Backup de Traefik creado: $backup"
  fi
  log "Renderizando configuracion inicial de Traefik"
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
  "$BIN_PATH" render-traefik
  touch "$TRAEFIK_DIR/acme.json"
  chmod 600 "$TRAEFIK_DIR/acme.json"
  service_enable_start traefik
  service_restart traefik || fail "Traefik no pudo iniciar. Revisa puertos ocupados o configuracion previa."
}

wait_health() {
  log "Esperando salud del panel en $HEALTH_URL"
  i=0
  while [ "$i" -lt 40 ]; do
    if http_ok "$HEALTH_URL"; then
      log "Pangolite responde correctamente"
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  warn "Pangolite no respondio a tiempo. Comandos de diagnostico:"
  service_status_hint "$APP_NAME"
  fail "Pangolite no respondio en $HEALTH_URL"
}

print_credentials() {
  log "Credenciales iniciales"
  if [ -f "$DATA_DIR/admin-password.txt" ]; then
    cat "$DATA_DIR/admin-password.txt"
    printf '\nGuarda esta password y cambiala en el primer acceso.\n'
  else
    printf 'No hay password temporal pendiente. Si ya la cambiaste, esto es correcto.\n'
  fi
}

print_summary() {
  version_installed="$(cat "$INSTALL_DIR/VERSION" 2>/dev/null || echo desconocida)"
  cat <<INFO

Instalacion completada.

Version instalada:
  $version_installed

Panel directo:
  http://${SERVER_IP:-IP_DEL_SERVIDOR}:2424

Archivos:
  Binario: $BIN_PATH
  Cliente local: $CLIENT_BIN_PATH
  SQLite: $DATA_DIR/pangolite.db
  Env: $ENV_FILE
  Backups: $DATA_DIR/backups

Comandos utiles:
$(service_status_hint "$APP_NAME")
INFO
}

main() {
  require_root
  detect_os
  detect_arch
  detect_init_system
  ensure_base_deps
  confirm_existing_install
  fetch_release_archive
  detect_server_ip
  install_binaries
  write_env_file
  if [ "$SKIP_TRAEFIK" != "1" ]; then
    ensure_traefik_binary
  fi
  write_service_files
  service_enable_start "$APP_NAME"
  wait_health
  configure_traefik
  print_credentials
  print_summary
}

main "$@"
