# Fecha
2026-08-13

# Objetivo
Sustituir el import WebSocket deprecado por el repositorio mantenido por Coder sin cambiar el comportamiento de los túneles.

# Decisiones tomadas
- `nhooyr.io/websocket v1.8.17` se reemplaza por `github.com/coder/websocket v1.8.15`.
- La tarea se limita al cambio de módulo/import path; no se mezclan refactors del protocolo, framing, keepalive, timeouts ni concurrencia.
- Se conserva la API ya usada por Pangolite (`Accept`, `Dial`, `Read`, `Write`, `Ping`, `Close`, `CloseNow`, `SetReadLimit`).
- No se agrega ninguna otra dependencia.

# Arquitectura actual
Servidor, cliente/agente, terminal, streaming HTTP/TCP y descarga de terminal siguen usando la misma arquitectura WebSocket. Solo cambia el módulo que provee la implementación.

# Librerías usadas
- `github.com/coder/websocket v1.8.15`.
- Resto del stack sin cambios.

# Archivos importantes modificados
- `go.mod`
- `internal/app/stream_bridge.go`
- `internal/app/agent_client.go`
- `internal/app/server.go`
- `internal/app/terminal.go`
- `internal/app/terminal_download.go`
- `contexto/00-resumen-proyecto.md`
- `contexto/72-descargas-terminal-y-toolbar-compacto.md`

# Problemas encontrados
El módulo anterior está deprecado y remite al repositorio mantenido por Coder.

# Soluciones implementadas
Migración directa de import path y versión mantenida, evitando cambios simultáneos de comportamiento para reducir riesgo.

# Pendientes
- Ejecutar `go mod tidy`, `go test ./...`, `go vet ./...` y pruebas de integración con Go 1.26 en un entorno con acceso al proxy de módulos.
- Generar/versionar `go.sum` cuando se ejecute `go mod tidy` con red disponible.

# Próximos pasos
Validar terminal local/remota, HTTP streaming, SSH/SFTP, subida/descarga de terminal y reconexiones de agente después del despliegue.
