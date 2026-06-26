#!/usr/bin/env bash
set -euo pipefail

APP_NAME="pangolite"
MODULE="github.com/thowilabs/pangolite"
INSTALL_DIR="/opt/pangolite"
DATA_DIR="$INSTALL_DIR/data"
BIN_PATH="$INSTALL_DIR/pangolite"
ENV_FILE="$INSTALL_DIR/pangolite.env"
SERVICE_FILE="/etc/systemd/system/pangolite.service"
TRAEFIK_DIR="/etc/traefik"
TRAEFIK_VERSION="3.7.5"
GO_VERSION="1.26.4"
PANEL_ADDR="0.0.0.0:2424"
HEALTH_URL="http://127.0.0.1:2424/healthz"
SERVER_IP=""
TMP_DIR=""
GO_BIN=""

log() { printf '\n[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
fail() { printf '\nERROR: %s\n' "$*" >&2; exit 1; }
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
    armv7l|armv7) echo "armv7" ;;
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
  if [ "$major" -eq 1 ] && [ "$minor" -ge 23 ]; then return 0; fi
  return 1
}

ensure_go() {
  if command -v go >/dev/null 2>&1 && version_ok "$(command -v go)"; then
    GO_BIN="$(command -v go)"
    log "Go detectado: $($GO_BIN version)"
    return
  fi
  log "Go >= 1.23 no esta instalado; descargando Go $GO_VERSION temporalmente"
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


ensure_traefik() {
  mkdir -p "$TRAEFIK_DIR" "$TRAEFIK_DIR/dynamic"
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

  if ! systemctl list-unit-files 2>/dev/null | grep -q '^traefik\.service'; then
    cat > /etc/systemd/system/traefik.service <<SERVICE
[Unit]
Description=Traefik reverse proxy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$traefik_bin --configFile=$TRAEFIK_DIR/traefik.yml
Restart=always
RestartSec=3
LimitNOFILE=1048576
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
SERVICE
    systemctl daemon-reload
    log "Servicio systemd de Traefik creado"
  fi
}


detect_server_ip() {
  local ip_value=""
  ip_value="$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
  if [ -z "$ip_value" ]; then
    ip_value="$(curl -fsS --max-time 5 https://ifconfig.me/ip 2>/dev/null || true)"
  fi
  if [ -z "$ip_value" ] && command -v ip >/dev/null 2>&1; then
    ip_value="$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++){if($i=="src"){print $(i+1); exit}}}' || true)"
  fi
  if [ -z "$ip_value" ]; then
    ip_value="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  fi
  SERVER_IP="$ip_value"
  if [ -n "$SERVER_IP" ]; then
    log "IP del servidor detectada: $SERVER_IP"
  else
    log "No se pudo detectar IP del servidor; el panel seguira en 0.0.0.0:2424"
  fi
}

set_env_value() {
  local key="$1" value="$2"
  if grep -q "^${key}=" "$ENV_FILE" 2>/dev/null; then
    sed -i "s|^${key}=.*|${key}=${value}|" "$ENV_FILE"
  else
    printf '%s=%s\n' "$key" "$value" >> "$ENV_FILE"
  fi
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
PANGOLITE_SESSION_DAYS=30
PANGOLITE_AUTO_TRAEFIK=1
# Opcional: configura dominio/correo para que Traefik publique el panel por HTTP/HTTPS.
# PANGOLITE_DASHBOARD_DOMAIN=pangolin.yahirex.us.kg
# PANGOLITE_LETSENCRYPT_EMAIL=admin@yahirex.us.kg
ENV
    chmod 600 "$ENV_FILE"
  fi
  if [ -n "$SERVER_IP" ]; then
    set_env_value PANGOLITE_PUBLIC_IP "$SERVER_IP"
  fi
}

prepare_runtime_dirs() {
  log "Preparando directorios de ejecucion"
  mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$TRAEFIK_DIR"
  chmod 755 "$INSTALL_DIR"
  chmod 700 "$DATA_DIR"
  chmod 755 "$TRAEFIK_DIR"
}

build_and_install() {
  log "Resolviendo modulos Go"
  "$GO_BIN" mod tidy
  log "Ejecutando pruebas"
  "$GO_BIN" test ./...
  log "Compilando binario"
  "$GO_BIN" build -buildvcs=false -trimpath -ldflags='-s -w' -o "$BIN_PATH.tmp" ./cmd/pangolite
  install -m 0755 "$BIN_PATH.tmp" "$BIN_PATH"
  rm -f "$BIN_PATH.tmp"
  log "Binario instalado en $BIN_PATH"
}

write_service() {
  systemctl stop pangolite >/dev/null 2>&1 || true
  cat > "$SERVICE_FILE" <<SERVICE
[Unit]
Description=Pangolite control plane
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=$ENV_FILE
ExecStart=$BIN_PATH serve
Restart=always
RestartSec=3
WorkingDirectory=$INSTALL_DIR
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
UMask=0077
LimitNOFILE=65535
# No se usa ReadWritePaths aqui: si /etc/traefik no existe, systemd falla en NAMESPACE antes de ejecutar el binario.
# init.sh crea las rutas necesarias y Pangolite valida permisos al renderizar Traefik.

[Install]
WantedBy=multi-user.target
SERVICE
  systemctl daemon-reload
  systemctl enable --now pangolite
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
  systemctl status pangolite --no-pager || true
  journalctl -u pangolite -n 100 --no-pager || true
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
  systemctl daemon-reload
  systemctl enable --now traefik >/dev/null 2>&1 || true
  if ! systemctl is-active --quiet traefik; then
    systemctl restart traefik || {
      journalctl -u traefik -n 120 --no-pager || true
      fail "Traefik no pudo iniciar. Revisa puertos ocupados o configuracion previa."
    }
  else
    systemctl restart traefik || {
      journalctl -u traefik -n 120 --no-pager || true
      fail "Traefik no pudo recargar configuracion estatica inicial."
    }
  fi
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
  need_cmd systemctl
  need_cmd curl
  need_cmd tar
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
  SQLite: $DATA_DIR/pangolite.db
  Password temporal: $DATA_DIR/admin-password.txt
  Env: $ENV_FILE

Comandos utiles:
  systemctl status pangolite --no-pager
  journalctl -u pangolite -f
  $BIN_PATH render-traefik # normalmente la UI aplica cambios automaticamente

INFO
}

main "$@"
