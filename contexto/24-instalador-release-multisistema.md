# 24 - Instalador release y soporte multi-init

Se agrega `install.sh` como instalador principal basado en releases de GitHub.

## Objetivo

Evitar compilar en el VPS y permitir instalación portable en distribuciones con distintos gestores de arranque.

## Instalador

`install.sh`:

- Descarga la última versión desde GitHub Releases.
- Permite instalar una versión específica con `--version X.Y`.
- Detecta arquitectura Linux: `amd64`, `arm64`, `386`, `armv7`.
- Detecta gestor de arranque: `systemd`, `OpenRC`, `SysVinit` y `runit`.
- Preserva `pangolite.env`, base SQLite, respaldos y datos si ya existe una instalación.
- Crea servicios según el init detectado.
- Descarga/instala Traefik como binario oficial si no existe.

## Workflow manual

Se agrega `.github/workflows/release.yml` manual con `workflow_dispatch`.

- Si se especifica versión, crea `vX.Y`.
- Si no se especifica, busca el último tag `vX.Y` e incrementa el número menor.
- Publica assets para Linux `amd64`, `arm64`, `386`, `armv7`.
- Incluye cliente Linux amd64 y Windows amd64 para descargas desde el panel.
- Genera `checksums.txt`.

## Versión de binarios

La versión se inyecta con ldflags en `internal/app.Version` para `pangolite version` y heartbeat del cliente NAT.
