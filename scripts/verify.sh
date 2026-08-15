#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

log() {
  printf '\n==> %s\n' "$*"
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

command -v go >/dev/null 2>&1 || fail "Go no esta instalado"
command -v git >/dev/null 2>&1 || fail "Git no esta instalado"
command -v node >/dev/null 2>&1 || fail "Node.js no esta instalado; se usa para validar la sintaxis JavaScript del panel"

log "Comprobando formato Go"
mapfile -d '' go_files < <(find . -type f -name '*.go' -not -path './.git/*' -not -path './vendor/*' -print0)
if ((${#go_files[@]} == 0)); then
  fail "No se encontraron archivos Go"
fi
unformatted="$(gofmt -l "${go_files[@]}")"
if [[ -n "$unformatted" ]]; then
  printf '%s\n' "$unformatted" >&2
  fail "hay archivos Go sin gofmt"
fi

log "Comprobando coherencia de version"
make_version="$(awk '/^VERSION[[:space:]]*\?=/{print $3; exit}' Makefile)"
make_version_code="$(awk '/^VERSION_CODE[[:space:]]*\?=/{print $3; exit}' Makefile)"
go_version="$(sed -n 's/^var Version = "\([^"]*\)"/\1/p' internal/app/version.go | head -n 1)"
go_version_code="$(sed -n 's/^var VersionCode = "\([^"]*\)"/\1/p' internal/app/version.go | head -n 1)"
[[ -n "$make_version" && -n "$make_version_code" && -n "$go_version" && -n "$go_version_code" ]] || fail "no se pudo resolver la version de desarrollo"
[[ "$make_version" == "$go_version" ]] || fail "VERSION del Makefile ($make_version) no coincide con internal/app/version.go ($go_version)"
[[ "$make_version_code" == "$go_version_code" ]] || fail "VERSION_CODE del Makefile ($make_version_code) no coincide con internal/app/version.go ($go_version_code)"
if [[ ! "$make_version" =~ ^([0-9]+)\.([0-9]+)$ ]]; then
  fail "VERSION debe usar formato X.Y"
fi
expected_code=$((10#${BASH_REMATCH[1]} * 100000 + 10#${BASH_REMATCH[2]}))
[[ "$make_version_code" == "$expected_code" ]] || fail "VERSION_CODE esperado para $make_version es $expected_code, no $make_version_code"

log "Comprobando que go.mod/go.sum esten normalizados"
go mod tidy
if ! git diff --exit-code -- go.mod go.sum; then
  fail "go mod tidy modifica go.mod o go.sum; confirma esos cambios antes de publicar"
fi

log "Ejecutando go vet"
go vet ./...

log "Ejecutando pruebas unitarias con cobertura"
mkdir -p build/quality
go test -count=1 -shuffle=on -timeout=5m -covermode=atomic -coverprofile=build/quality/coverage.out ./...
go tool cover -func=build/quality/coverage.out | tail -n 1

log "Ejecutando detector de condiciones de carrera"
CGO_ENABLED=1 go test -race -count=1 -timeout=8m ./...

log "Validando JavaScript embebido"
while IFS= read -r -d '' js_file; do
  node --check "$js_file" >/dev/null
done < <(find internal/app/assets -type f -name '*.js' -print0)

log "Validando scripts shell"
bash -n init.sh install.sh
while IFS= read -r -d '' shell_file; do
  bash -n "$shell_file"
done < <(find scripts -type f -name '*.sh' -print0)

log "Comprobando higiene de artefactos Git"
if git ls-files -z | grep -Eiz '\.zip$' >/dev/null; then
  fail "hay archivos .zip trackeados por Git"
fi
if git rev-list --objects --all | awk '{print $2}' | grep -Ei '\.zip$' >/dev/null; then
  fail "hay archivos .zip alcanzables en el historial Git"
fi

log "Verificacion de calidad completada correctamente"
