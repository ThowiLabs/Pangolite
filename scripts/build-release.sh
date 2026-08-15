#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

VERSION="${1:-${VERSION:-}}"
VERSION_CODE="${2:-${VERSION_CODE:-}}"

if [[ -z "$VERSION" || -z "$VERSION_CODE" ]]; then
  echo "Uso: $0 VERSION VERSION_CODE" >&2
  exit 2
fi
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "Version invalida: $VERSION" >&2
  exit 2
fi
if [[ ! "$VERSION_CODE" =~ ^[0-9]+$ ]]; then
  echo "Codigo de version invalido: $VERSION_CODE" >&2
  exit 2
fi

DIST_DIR="${DIST_DIR:-dist}"
BUILD_DIR="${BUILD_DIR:-build/release}"
PUBLIC_DIR="$BUILD_DIR/public"
LDFLAGS="-s -w -X github.com/thowilabs/pangolite/internal/app.Version=${VERSION} -X github.com/thowilabs/pangolite/internal/app.VersionCode=${VERSION_CODE}"

rm -rf "$DIST_DIR" "$BUILD_DIR"
mkdir -p "$DIST_DIR" "$PUBLIC_DIR"

build_bin() {
  local goos="$1"
  local goarch="$2"
  local goarm="${3:-}"
  local pkg="$4"
  local out="$5"
  echo "==> build ${pkg} ${goos}/${goarch}${goarm:+/v$goarm}"
  env GOOS="$goos" GOARCH="$goarch" GOARM="$goarm" CGO_ENABLED=0 \
    go build -buildvcs=false -trimpath -ldflags="$LDFLAGS" -o "$out" "$pkg"
  [[ -s "$out" ]] || { echo "Artefacto vacio: $out" >&2; exit 1; }
}

build_bin linux amd64 "" ./cmd/pangolite-client "$PUBLIC_DIR/pangolite-client-linux-amd64"
build_bin linux arm64 "" ./cmd/pangolite-client "$PUBLIC_DIR/pangolite-client-linux-arm64"
build_bin linux 386 "" ./cmd/pangolite-client "$PUBLIC_DIR/pangolite-client-linux-386"
build_bin linux arm 7 ./cmd/pangolite-client "$PUBLIC_DIR/pangolite-client-linux-armv7"
build_bin windows amd64 "" ./cmd/pangolite-client "$PUBLIC_DIR/pangolite-client-windows-amd64.exe"

package_linux() {
  local arch="$1"
  local goarch="$2"
  local goarm="${3:-}"
  local name="pangolite_linux_${arch}"
  local pkgdir="$BUILD_DIR/$name"
  rm -rf "$pkgdir"
  mkdir -p "$pkgdir/public"

  build_bin linux "$goarch" "$goarm" ./cmd/pangolite "$pkgdir/pangolite"
  build_bin linux "$goarch" "$goarm" ./cmd/pangolite-client "$pkgdir/pangolite-client"

  cp "$PUBLIC_DIR/pangolite-client-linux-amd64" "$pkgdir/public/pangolite-client-linux-amd64"
  cp "$PUBLIC_DIR/pangolite-client-linux-arm64" "$pkgdir/public/pangolite-client-linux-arm64"
  cp "$PUBLIC_DIR/pangolite-client-linux-386" "$pkgdir/public/pangolite-client-linux-386"
  cp "$PUBLIC_DIR/pangolite-client-linux-armv7" "$pkgdir/public/pangolite-client-linux-armv7"
  cp "$PUBLIC_DIR/pangolite-client-windows-amd64.exe" "$pkgdir/public/pangolite-client-windows-amd64.exe"
  cp install.sh README.md LICENSE "$pkgdir/"
  printf '%s\n' "$VERSION" > "$pkgdir/VERSION"
  printf '%s\n' "$VERSION_CODE" > "$pkgdir/VERSION_CODE"

  tar -C "$pkgdir" -czf "$DIST_DIR/${name}.tar.gz" .
  tar -tzf "$DIST_DIR/${name}.tar.gz" >/dev/null
}

package_linux amd64 amd64
package_linux arm64 arm64
package_linux 386 386
package_linux armv7 arm 7

cp "$PUBLIC_DIR/pangolite-client-linux-amd64" "$DIST_DIR/pangolite-client_linux_amd64"
cp "$PUBLIC_DIR/pangolite-client-linux-arm64" "$DIST_DIR/pangolite-client_linux_arm64"
cp "$PUBLIC_DIR/pangolite-client-linux-386" "$DIST_DIR/pangolite-client_linux_386"
cp "$PUBLIC_DIR/pangolite-client-linux-armv7" "$DIST_DIR/pangolite-client_linux_armv7"
cp "$PUBLIC_DIR/pangolite-client-windows-amd64.exe" "$DIST_DIR/pangolite-client_windows_amd64.exe"

required_assets=(
  "$DIST_DIR/pangolite_linux_amd64.tar.gz"
  "$DIST_DIR/pangolite_linux_arm64.tar.gz"
  "$DIST_DIR/pangolite_linux_386.tar.gz"
  "$DIST_DIR/pangolite_linux_armv7.tar.gz"
  "$DIST_DIR/pangolite-client_linux_amd64"
  "$DIST_DIR/pangolite-client_linux_arm64"
  "$DIST_DIR/pangolite-client_linux_386"
  "$DIST_DIR/pangolite-client_linux_armv7"
  "$DIST_DIR/pangolite-client_windows_amd64.exe"
)
for asset in "${required_assets[@]}"; do
  [[ -s "$asset" ]] || { echo "No se genero el artefacto requerido: $asset" >&2; exit 1; }
done

if [[ "$(uname -s)" == "Linux" && "$(uname -m)" == "x86_64" ]]; then
  echo "==> smoke tests de binarios amd64"
  server_version="$("$BUILD_DIR/pangolite_linux_amd64/pangolite" --version)"
  client_version="$("$PUBLIC_DIR/pangolite-client-linux-amd64" --version)"
  [[ "$server_version" == "pangolite ${VERSION} (code ${VERSION_CODE})" ]] || {
    echo "Version inesperada del servidor: $server_version" >&2
    exit 1
  }
  [[ "$client_version" == "pangolite-client ${VERSION} (code ${VERSION_CODE})" ]] || {
    echo "Version inesperada del cliente: $client_version" >&2
    exit 1
  }
fi

(
  cd "$DIST_DIR"
  sha256sum pangolite_linux_*.tar.gz pangolite-client_* > checksums.txt
)
[[ -s "$DIST_DIR/checksums.txt" ]] || { echo "checksums.txt no fue generado" >&2; exit 1; }

echo "==> build de release completado: ${VERSION} (code ${VERSION_CODE})"
