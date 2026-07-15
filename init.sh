#!/usr/bin/env bash
set -euo pipefail

APP_NAME="pangolite"
INSTALL_DIR="/opt/pangolite"
DATA_DIR="$INSTALL_DIR/data"
BIN_PATH="$INSTALL_DIR/pangolite"
CLIENT_BIN_PATH="$INSTALL_DIR/pangolite-client"
PUBLIC_DIR="$INSTALL_DIR/public"
CLIENT_PUBLIC_BIN="$PUBLIC_DIR/pangolite-client-linux-amd64"
CLIENT_PUBLIC_WINDOWS_BIN="$PUBLIC_DIR/pangolite-client-windows-amd64.exe"
ENV_FILE="$INSTALL_DIR/pangolite.env"
TRAEFIK_DIR="/etc/traefik"
TRAEFIK_VERSION="3.7.5"
GO_VERSION="1.26.4"
PANEL_ADDR="0.0.0.0:2424"
HEALTH_URL="http://127.0.0.1:2424/healthz"
SERVER_IP=""
TMP_DIR=""
GO_BIN=""
INIT_SYSTEM=""

log() { printf '\n[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
warn() { printf '\nADVERTENCIA: %s\n' "$*" >&2; }
fail() { printf '\nERROR: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }
cleanup() {
  if [ -n "${TMP_DIR:-}" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}
trap cleanup EXIT

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    fail "ejecuta este instalador como root: sudo bash init.sh"
  fi
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "falta dependencia requerida: $1"
}

detect_init_system() {
  if have systemctl && [ -d /run/systemd/system ]; then
    INIT_SYSTEM="systemd"
  elif have rc-service && have rc-update; then
    INIT_SYSTEM="openrc"
  elif have sv; then
    INIT_SYSTEM="runit"
  elif [ -d /etc/init.d ] && have service; then
    INIT_SYSTEM="sysvinit"
  else
    fail "no se detecto gestor de servicios compatible: systemd, OpenRC, SysVinit o runit"
  fi
  log "Gestor de arranque detectado: $INIT_SYSTEM"
}

service_exists() {
  local name="$1"
  case "$INIT_SYSTEM" in
    systemd) systemctl list-unit-files 2>/dev/null | grep -q "^${name}\.service" || [ -f "/etc/systemd/system/$name.service" ] ;;
    openrc|sysvinit) [ -x "/etc/init.d/$name" ] ;;
    runit) [ -d "/etc/sv/$name" ] || [ -d "/etc/service/$name" ] || [ -d "/var/service/$name" ] ;;
    *) return 1 ;;
  esac
}

service_stop() {
  local name="$1"
  case "$INIT_SYSTEM" in
    systemd) systemctl stop "$name" >/dev/null 2>&1 || true ;;
    openrc) rc-service "$name" stop >/dev/null 2>&1 || true ;;
    sysvinit) service "$name" stop >/dev/null 2>&1 || "/etc/init.d/$name" stop >/dev/null 2>&1 || true ;;
    runit) sv stop "$name" >/dev/null 2>&1 || true ;;
  esac
}

service_enable_start() {
  local name="$1"
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
      service "$name" restart || service "$name" start || "/etc/init.d/$name" restart || "/etc/init.d/$name" start
      ;;
    runit)
      mkdir -p /etc/service
      ln -sfn "/etc/sv/$name" "/etc/service/$name"
      sv restart "$name" || sv start "$name"
      ;;
  esac
}

service_restart() {
  local name="$1"
  case "$INIT_SYSTEM" in
    systemd) systemctl restart "$name" ;;
    openrc) rc-service "$name" restart ;;
    sysvinit) service "$name" restart || "/etc/init.d/$name" restart ;;
    runit) sv restart "$name" ;;
  esac
}

service_status_hint() {
  local name="$1"
  case "$INIT_SYSTEM" in
    systemd) systemctl status "$name" --no-pager || true; journalctl -u "$name" -n 100 --no-pager || true ;;
    openrc) rc-service "$name" status || true; tail -n 120 "$DATA_DIR/$name.err" "$DATA_DIR/$name.log" 2>/dev/null || true ;;
    sysvinit) service "$name" status || true; tail -n 120 "$DATA_DIR/$name.log" 2>/dev/null || true ;;
    runit) sv status "$name" || true ;;
  esac
}

arch_go() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) fail "arquitectura no soportada para descarga temporal de Go: $(uname -m)" ;;
  esac
}

arch_traefik() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    armv7l|armv7|armv8l) echo "armv7" ;;
    i386|i686) echo "386" ;;
    *) fail "arquitectura no soportada para Traefik: $(uname -m)" ;;
  esac
}

version_ok() {
  local v raw major minor
  raw="$($1 version 2>/dev/null || true)"
  v="$(printf '%s' "$raw" | awk '{print $3}' | sed 's/^go//')"
  major="${v%%.*}"
  minor="${v#*.}"; minor="${minor%%.*}"
  [ -n "$major" ] && [ -n "$minor" ] || return 1
  if [ "$major" -gt 1 ]; then return 0; fi
  if [ "$major" -eq 1 ] && [ "$minor" -ge 24 ]; then return 0; fi
  return 1
}

ensure_go() {
  if command -v go >/dev/null 2>&1 && version_ok "$(command -v go)"; then
    GO_BIN="$(command -v go)"
    log "Go detectado: $($GO_BIN version)"
    return
  fi
  log "Go >= 1.24 no esta instalado; descargando Go $GO_VERSION temporalmente"
  need_cmd curl
  need_cmd tar
  local goarch url tarball
  goarch="$(arch_go)"
  TMP_DIR="$(mktemp -d)"
  tarball="$TMP_DIR/go.tar.gz"
  url="https://go.dev/dl/go${GO_VERSION}.linux-${goarch}.tar.gz"
  curl -fsSL "$url" -o "$tarball"
  tar -C "$TMP_DIR" -xzf "$tarball"
  GO_BIN="$TMP_DIR/go/bin/go"
  [ -x "$GO_BIN" ] || fail "no se pudo preparar Go temporal"
  log "Go temporal listo: $($GO_BIN version)"
}

write_systemd_service() {
  local name="$1" desc="$2" exec_line="$3" env_line="$4" workdir="$5"
  cat > "/etc/systemd/system/$name.service" <<SERVICE
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
WorkingDirectory=$workdir
NoNewPrivileges=true
PrivateTmp=true
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
SERVICE
}

write_openrc_service() {
  local name="$1" desc="$2" run_cmd="$3" workdir="$4"
  local logfile="$DATA_DIR/$name.log" errfile="$DATA_DIR/$name.err"
  cat > "/etc/init.d/$name" <<SERVICE
#!/sbin/openrc-run
# managed by Pangolite
name="$name"
description="$desc"
pidfile="/run/$name.pid"
command="/bin/sh"
command_args="-c '$run_cmd'"
command_background="yes"
directory="$workdir"
output_log="$logfile"
error_log="$errfile"

start_pre() {
  checkpath --directory --mode 0755 "$workdir"
  checkpath --directory --mode 0700 "$DATA_DIR"
}

depend() {
  need net
  after firewall
}
SERVICE
  chmod 0755 "/etc/init.d/$name"
}

write_sysv_service() {
  local name="$1" desc="$2" run_cmd="$3"
  local logfile="$DATA_DIR/$name.log" pidfile="/var/run/$name.pid"
  cat > "/etc/init.d/$name" <<SERVICE
#!/bin/sh
### BEGIN INIT INFO
# Provides:          $name
# Required-Start:    \$remote_fs \$syslog \$network
# Required-Stop:     \$remote_fs \$syslog \$network
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: $desc
### END INIT INFO
# managed by Pangolite
PIDFILE="$pidfile"
LOGFILE="$logfile"
start() {
  if [ -f "\$PIDFILE" ] && kill -0 "\$(cat \$PIDFILE)" 2>/dev/null; then echo "$name ya esta ejecutandose"; return 0; fi
  echo "Starting $name"
  start-stop-daemon --start --background --make-pidfile --pidfile "\$PIDFILE" --exec /bin/sh -- -c "$run_cmd >>\$LOGFILE 2>&1"
}
stop() {
  echo "Stopping $name"
  start-stop-daemon --stop --pidfile "\$PIDFILE" --retry TERM/10/KILL/5 2>/dev/null || true
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
  chmod 0755 "/etc/init.d/$name"
}

write_runit_service() {
  local name="$1" run_cmd="$2"
  mkdir -p "/etc/sv/$name"
  cat > "/etc/sv/$name/run" <<SERVICE
#!/bin/sh
# managed by Pangolite
exec /bin/sh -c "$run_cmd"
SERVICE
  chmod 0755 "/etc/sv/$name/run"
}

write_managed_service() {
  local name="$1" desc="$2" run_cmd="$3" systemd_exec="$4" env_line="$5" workdir="$6"
  case "$INIT_SYSTEM" in
    systemd) write_systemd_service "$name" "$desc" "$systemd_exec" "$env_line" "$workdir" ;;
    openrc) write_openrc_service "$name" "$desc" "$run_cmd" "$workdir" ;;
    sysvinit) write_sysv_service "$name" "$desc" "$run_cmd" ;;
    runit) write_runit_service "$name" "$run_cmd" ;;
  esac
}

ensure_traefik() {
  mkdir -p "$TRAEFIK_DIR" "$TRAEFIK_DIR/dynamic" "$DATA_DIR"
  touch "$TRAEFIK_DIR/acme.json"
  chmod 600 "$TRAEFIK_DIR/acme.json"

  local traefik_bin=""
  if command -v traefik >/dev/null 2>&1; then
    traefik_bin="$(command -v traefik)"
    log "Traefik detectado: $(traefik version 2>/dev/null | head -1 || true)"
  else
    log "Traefik no esta instalado; descargando Traefik v$TRAEFIK_VERSION"
    need_cmd curl
    need_cmd tar
    local arch url tarball work
    arch="$(arch_traefik)"
    work="$(mktemp -d)"
    tarball="$work/traefik.tar.gz"
    url="https://github.com/traefik/traefik/releases/download/v${TRAEFIK_VERSION}/traefik_v${TRAEFIK_VERSION}_linux_${arch}.tar.gz"
    curl -fsSL "$url" -o "$tarball"
    tar -C "$work" -xzf "$tarball"
    install -m 0755 "$work/traefik" /usr/local/bin/traefik
    rm -rf "$work"
    traefik_bin="/usr/local/bin/traefik"
    log "Traefik instalado en $traefik_bin"
  fi

  local traefik_run="exec $traefik_bin --configFile=$TRAEFIK_DIR/traefik.yml"
  write_managed_service "traefik" "Traefik reverse proxy" "$traefik_run" "$traefik_bin --configFile=$TRAEFIK_DIR/traefik.yml" "" "$TRAEFIK_DIR"
  log "Servicio $INIT_SYSTEM de Traefik creado/actualizado"
}

detect_server_ip() {
  local ip_value=""
  ip_value="$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
  if [ -z "$ip_value" ]; then ip_value="$(curl -fsS --max-time 5 https://ifconfig.me/ip 2>/dev/null || true)"; fi
  if [ -z "$ip_value" ] && command -v ip >/dev/null 2>&1; then ip_value="$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++){if($i=="src"){print $(i+1); exit}}}' || true)"; fi
  if [ -z "$ip_value" ]; then ip_value="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"; fi
  SERVER_IP="$ip_value"
  if [ -n "$SERVER_IP" ]; then log "IP del servidor detectada: $SERVER_IP"; else log "No se pudo detectar IP del servidor; el panel seguira en 0.0.0.0:2424"; fi
}

set_env_value() {
  local key="$1" value="$2"
  if grep -q "^${key}=" "$ENV_FILE" 2>/dev/null; then sed -i "s|^${key}=.*|${key}=${value}|" "$ENV_FILE"; else printf '%s=%s\n' "$key" "$value" >> "$ENV_FILE"; fi
}

write_env_file() {
  mkdir -p "$INSTALL_DIR" "$DATA_DIR"
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
PANGOLITE_BACKUP_INTERVAL_HOURS=24
PANGOLITE_BACKUP_RETENTION_DAYS=14
PANGOLITE_SUSPENSION_TEMPLATE_DIR=$DATA_DIR/templates/suspension
PANGOLITE_SESSION_DAYS=30
PANGOLITE_AUTO_TRAEFIK=1
PANGOLITE_CLIENT_LINUX_AMD64=$CLIENT_PUBLIC_BIN
PANGOLITE_CLIENT_WINDOWS_AMD64=$CLIENT_PUBLIC_WINDOWS_BIN
# Opcional: configura dominio/correo para que Traefik publique el panel por HTTP/HTTPS.
# PANGOLITE_DASHBOARD_DOMAIN=panel.midominio.com
# PANGOLITE_LETSENCRYPT_EMAIL=admin@midominio.com
ENV
    chmod 600 "$ENV_FILE"
  fi
  set_env_value PANGOLITE_CLIENT_LINUX_AMD64 "$CLIENT_PUBLIC_BIN"
  set_env_value PANGOLITE_CLIENT_WINDOWS_AMD64 "$CLIENT_PUBLIC_WINDOWS_BIN"
  set_env_value PANGOLITE_LOG_FILE "$DATA_DIR/pangolite.log"
  set_env_value PANGOLITE_BACKUP_DIR "$DATA_DIR/backups"
  set_env_value PANGOLITE_BACKUP_INTERVAL_HOURS "${PANGOLITE_BACKUP_INTERVAL_HOURS:-24}"
  set_env_value PANGOLITE_BACKUP_RETENTION_DAYS "${PANGOLITE_BACKUP_RETENTION_DAYS:-14}"
  set_env_value PANGOLITE_SUSPENSION_TEMPLATE_DIR "$DATA_DIR/templates/suspension"
  if [ -n "$SERVER_IP" ]; then set_env_value PANGOLITE_PUBLIC_IP "$SERVER_IP"; fi
}

prepare_runtime_dirs() {
  log "Preparando directorios de ejecucion"
  mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$TRAEFIK_DIR" "$PUBLIC_DIR"
  chmod 755 "$INSTALL_DIR"
  chmod 700 "$DATA_DIR"
  chmod 755 "$TRAEFIK_DIR"
}

build_and_install() {
  log "Resolviendo modulos Go"
  "$GO_BIN" mod tidy
  log "Ejecutando pruebas"
  "$GO_BIN" test -timeout 2m ./...
  log "Compilando binario"
  CGO_ENABLED=0 "$GO_BIN" build -buildvcs=false -trimpath -ldflags='-s -w' -o "$BIN_PATH.tmp" ./cmd/pangolite
  install -m 0755 "$BIN_PATH.tmp" "$BIN_PATH"
  rm -f "$BIN_PATH.tmp"
  log "Binario instalado en $BIN_PATH"
  log "Compilando cliente NAT estatico"
  CGO_ENABLED=0 "$GO_BIN" build -buildvcs=false -trimpath -ldflags='-s -w' -o "$CLIENT_BIN_PATH.tmp" ./cmd/pangolite-client
  install -m 0755 "$CLIENT_BIN_PATH.tmp" "$CLIENT_BIN_PATH"
  mkdir -p "$PUBLIC_DIR"
  install -m 0755 "$CLIENT_BIN_PATH.tmp" "$CLIENT_PUBLIC_BIN"
  rm -f "$CLIENT_BIN_PATH.tmp"
  log "Compilando cliente NAT Windows amd64"
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 "$GO_BIN" build -buildvcs=false -trimpath -ldflags='-s -w' -o "$CLIENT_PUBLIC_WINDOWS_BIN.tmp" ./cmd/pangolite-client
  install -m 0755 "$CLIENT_PUBLIC_WINDOWS_BIN.tmp" "$CLIENT_PUBLIC_WINDOWS_BIN"
  rm -f "$CLIENT_PUBLIC_WINDOWS_BIN.tmp"
  log "Cliente NAT instalado en $CLIENT_BIN_PATH y publicado para Linux/Windows"
}

write_service() {
  service_stop pangolite
  local pangolite_run="cd $INSTALL_DIR; set -a; . $ENV_FILE; set +a; exec $BIN_PATH serve"
  write_managed_service "pangolite" "Pangolite control plane" "$pangolite_run" "$BIN_PATH serve" "EnvironmentFile=$ENV_FILE" "$INSTALL_DIR"
  service_enable_start pangolite
}

wait_health() {
  log "Esperando salud del panel en $HEALTH_URL"
  for _ in $(seq 1 40); do
    if curl -fsS "$HEALTH_URL" >/dev/null 2>&1; then
      curl -fsS "$HEALTH_URL"; printf '\n'
      return
    fi
    sleep 1
  done
  service_status_hint pangolite
  fail "Pangolite no respondio en $HEALTH_URL"
}

configure_traefik_if_available() {
  mkdir -p "$TRAEFIK_DIR" "$TRAEFIK_DIR/dynamic"
  if [ -f "$TRAEFIK_DIR/traefik.yml" ] && ! grep -q 'managed by Pangolite' "$TRAEFIK_DIR/traefik.yml"; then
    local backup="$TRAEFIK_DIR/traefik.yml.backup-$(date +%Y%m%d%H%M%S)"
    cp "$TRAEFIK_DIR/traefik.yml" "$backup"
    log "Backup de Traefik creado: $backup"
  fi
  log "Escribiendo configuracion de Traefik con file provider watch"
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
  "$BIN_PATH" render-traefik
  touch "$TRAEFIK_DIR/acme.json"
  chmod 600 "$TRAEFIK_DIR/acme.json"
  service_enable_start traefik
  service_restart traefik || {
    service_status_hint traefik
    fail "Traefik no pudo reiniciarse. Revisa puertos ocupados o configuracion previa."
  }
  log "Traefik activo; HTTP/HTTPS se actualizara automaticamente por configuracion dinamica"
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

main() {
  require_root
  log "Validando dependencias base"
  need_cmd curl
  need_cmd tar
  detect_init_system
  ensure_go
  detect_server_ip
  write_env_file
  prepare_runtime_dirs
  ensure_traefik
  build_and_install
  write_service
  wait_health
  configure_traefik_if_available
  print_credentials
  log "Instalacion completada"
  cat <<INFO

Panel directo sin redireccion HTTPS:
  http://${SERVER_IP:-IP_DEL_SERVIDOR}:2424

Archivos:
  Binario: $BIN_PATH
  Cliente NAT: $CLIENT_BIN_PATH
  Descarga cliente Linux: http://${SERVER_IP:-IP_DEL_SERVIDOR}:2424/download/pangolite-client-linux-amd64
  Descarga cliente Windows: http://${SERVER_IP:-IP_DEL_SERVIDOR}:2424/download/pangolite-client-windows-amd64.exe
  SQLite: $DATA_DIR/pangolite.db
  Password temporal: $DATA_DIR/admin-password.txt
  Env: $ENV_FILE

Comandos utiles:
  pangolite doctor
  $BIN_PATH render-traefik # normalmente la UI aplica cambios dinamicos automaticamente

INFO
}

main "$@"
