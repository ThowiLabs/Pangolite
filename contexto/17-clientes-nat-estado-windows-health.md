# 17 - Clientes NAT: estado operativo, Windows y health checks

## Fecha
2026-06-27

## Objetivo
Fortalecer la operación del cliente NAT agregando visibilidad real en el panel, soporte de instalación/eliminación en Windows y validaciones de estado por recurso.

## Decisiones tomadas
- El cliente NAT ahora reporta metadatos en cada poll: sistema operativo, arquitectura, hostname, IP privada, versión y última conexión.
- El servidor guarda esos datos en SQLite y calcula `online` si el último heartbeat es reciente.
- El panel muestra estado online/offline, versión, sistema, hostname/IP y recursos asociados por cliente.
- Se agregó detalle modal de cliente con recursos asociados para depuración operativa.
- El instalador compila y publica cliente Linux amd64 y Windows amd64.
- El cliente Windows implementa `--install`, `--remove` y `--service` usando el administrador de servicios de Windows.
- Se agregó API de health checks por recurso para validar disponibilidad básica de HTTP/TCP y estado de cliente NAT.

## Arquitectura actual
- `/api/agents` lista clientes con estado operativo.
- `/api/agents/{id}` devuelve detalle y recursos asociados.
- `/api/resources/health` ejecuta checks básicos por recurso.
- `/download/pangolite-client-linux-amd64` descarga cliente Linux.
- `/download/pangolite-client-windows-amd64.exe` descarga cliente Windows.
- `cmd/pangolite-client` ahora usa archivos por plataforma para instalación Linux/OpenRC/systemd y Windows Service.

## Librerías usadas
- Go estándar para Linux, networking y servicio principal.
- `golang.org/x/sys/windows/svc` para integrar el cliente con Windows Service Control Manager.

## Archivos importantes modificados
- `cmd/pangolite-client/main.go`
- `cmd/pangolite-client/install_unix.go`
- `cmd/pangolite-client/install_windows.go`
- `internal/app/agent_client.go`
- `internal/app/model.go`
- `internal/app/server.go`
- `internal/app/store.go`
- `internal/app/ui.go`
- `init.sh`
- `go.mod`

## Problemas resueltos
- El panel no distinguía entre cliente habilitado y cliente realmente conectado.
- No existía binario Windows instalable.
- El usuario debía inferir qué recursos pertenecían a cada cliente.
- No había health check rápido desde el panel.

## Pendientes
- Logs remotos del cliente enviados al servidor.
- Auto-update del cliente.
- Binarios arm64 Linux/Windows/macOS.
- Cliente de usuario para recursos privados zero-trust.
