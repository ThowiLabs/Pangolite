# Fecha
2026-08-12

# Objetivo
Estabilizar los recursos TCP remotos, en especial SSH/SFTP/WinSCP, y eliminar el límite práctico de tamaño del túnel HTTP sin aumentar el consumo de memoria del servidor.

# Decisiones tomadas
- Separar el tiempo máximo para que un agente adjunte un stream de la vida útil del stream ya conectado.
- Mantener 30 segundos como timeout de handshake/adjunto, pero no aplicar un deadline global a SSH/SFTP/TCP.
- Añadir keepalive WebSocket y TCP, `TCP_NODELAY` y escritura completa ante short writes.
- Limitar globalmente los streams TCP remotos con `PANGOLITE_AGENT_STREAM_CONCURRENCY`; valor por defecto 16 para favorecer VPS pequeños.
- Implementar HTTP streaming como protocolo de capacidad `http-stream-v1` en vez de aumentar el límite del protocolo JSON/base64 existente.
- Mantener compatibilidad hacia atrás: un agente que no anuncie `http-stream-v1` continúa usando el transporte HTTP legado con máximo de 16 MiB.
- Mantener `PANGOLITE_AGENT_HTTP_CONCURRENCY=4` como protección independiente para requests HTTP remotos.
- No añadir Redis, colas externas ni nuevas dependencias.

# Arquitectura actual
- Los recursos TCP públicos aceptados por `BridgeManager` generan un `AgentStreamJob` y esperan como máximo 30 s a que el agente abra el WebSocket.
- Tras el adjunto, la sesión queda ligada al socket/stream real y no a un timer artificial.
- El puente WebSocket/TCP trabaja en ambos sentidos con buffers de 32 KiB y keepalive ligero.
- Los agentes actuales anuncian `http-stream-v1` en sus requests de control.
- Para HTTP streaming, el servidor crea un stream por request, escribe la petición HTTP/1.1 directamente al backend remoto a través del WebSocket y copia la respuesta progresivamente al cliente.
- El cuerpo no se acumula completo en RAM. Las cabeceras de respuesta del backend se mantienen limitadas a 64 KiB.
- Agentes antiguos usan `AgentJob`/`AgentResponse` y el límite legado de 16 MiB hasta ser actualizados.

# Librerías usadas
- Librería estándar Go: `net`, `net/http`, `net.Pipe`, `bufio`, `io`, `crypto/tls`, `context`.
- `nhooyr.io/websocket` ya existente; no se agregaron dependencias.
- SQLite/Traefik permanecen sin cambios arquitectónicos para este trabajo.

# Archivos importantes modificados
- `internal/app/tunnel.go`
- `internal/app/stream_bridge.go`
- `internal/app/stream_bridge_test.go`
- `internal/app/agent_client.go`
- `internal/app/server.go`
- `internal/app/config.go`
- `internal/app/tunnel_test.go`
- `internal/app/server_test.go`
- `init.sh`
- `install.sh`
- `README.md`
- `contexto/00-resumen-proyecto.md`
- `tareas/completado-66-streaming-http-estabilidad-tcp.md`

# Problemas encontrados
- El puente TCP creaba `context.WithTimeout(..., 20s)` antes de `SubmitStream`. Como `SubmitStream` esperaba hasta el final de la sesión, ese mismo contexto cancelaba SSH/SFTP aproximadamente a los 20 segundos aunque el agente estuviera sano.
- La consola web no pasaba por ese timeout, lo que explica que permaneciera estable mientras SSH/WinSCP se desconectaban.
- El túnel HTTP legado lee el request y response completos y los serializa dentro de JSON/base64; elevar el límite fijo aumenta proporcionalmente la presión de RAM.
- Una escritura `net.Conn.Write` no garantiza por contrato escribir todos los bytes solicitados; el puente no completaba explícitamente short writes.
- Los streams TCP públicos no tenían un límite global de concurrencia independiente.

# Soluciones implementadas
- `TunnelHub.StartStream` aplica el timeout solo hasta `AttachStream`; `WaitStream` ya no hereda un deadline artificial cuando el caller usa un contexto sin deadline.
- `BridgeManager.handleTCPConn` deja vivir SSH/SFTP hasta que cierre uno de los extremos.
- Se añadieron keepalive WebSocket periódico, keepalive TCP, `TCP_NODELAY`, cierre inmediato y propagación de cancelación.
- Se añadió `writeFull` para completar escrituras parciales.
- Se añadió un semáforo global de streams TCP con valor por defecto 16.
- Se añadió negociación simple de capacidades por `X-Pangolite-Capabilities` con TTL corto en memoria.
- Se añadió HTTP streaming v1 con backpressure natural de `net.Pipe`/WebSocket y buffers de 32 KiB.
- Se añadieron pruebas de regresión para demostrar que un stream adjuntado sobrevive al antiguo timeout y que el timeout solo actúa antes del handshake.
- Se añadieron pruebas de upload/download HTTP mayores de 16 MiB sin construir el contenido completo en memoria y de respuesta temprana del backend durante un upload grande.

# Pendientes
- Ejecutar `go test ./...` y `go vet ./...` con Go 1.26 y dependencias disponibles; el entorno de edición actual no puede descargar el toolchain por bloqueo DNS.
- HTTP streaming v1 no implementa aún `101 Switching Protocols`/WebSocket público a través del recurso HTTP remoto; devuelve error explícito en ese caso. El transporte HTTP legado tampoco ofrecía un túnel bidireccional real para ese upgrade.
- Migrar en una ventana posterior `nhooyr.io/websocket` a `github.com/coder/websocket` con suite completa disponible.

# Próximos pasos
- Desplegar primero el servidor actualizado: la corrección del timeout TCP beneficia inmediatamente a clientes existentes.
- Actualizar después los agentes para activar keepalive mejorado y `http-stream-v1`.
- En el VPS de ~500 MiB/1 vCPU mantener inicialmente `PANGOLITE_AGENT_HTTP_CONCURRENCY=4` y `PANGOLITE_AGENT_STREAM_CONCURRENCY=16`; bajar streams a 8 solo si se observa presión sostenida o se exponen muchos puertos públicos.
- Validar una sesión SSH de al menos 10 minutos, transferencia SFTP/WinSCP grande y upload/download HTTP mayor de 16 MiB.
