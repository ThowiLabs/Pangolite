# 69 - Transferencia de archivos desde terminal

# Fecha
2026-08-12

# Objetivo
Permitir subir archivos desde el navegador al directorio actual de una terminal local o de un cliente remoto, con arrastrar y soltar, selector compatible con Android, barra de progreso y consumo de memoria acotado.

# Decisiones tomadas
- Reutilizar el WebSocket autenticado de la terminal; no crear un servidor SFTP, endpoint multipart ni dependencia nueva.
- Transportar cada archivo en bloques binarios pequeños de 24 KiB desde el navegador; el receptor acepta como máximo 64 KiB por frame de upload.
- Mantener backpressure en el navegador y no permitir que el buffer pendiente del WebSocket crezca libremente.
- Resolver el directorio actual desde el proceso en primer plano del PTY mediante `/proc/<pid>/cwd` en Linux. Esto permite que `cd` determine el destino real sin aceptar una ruta arbitraria del navegador.
- Sanitizar el nombre a basename, rechazar controles/NUL y limitarlo a 240 bytes.
- Escribir primero a `.pangolite-upload-*.part` dentro del mismo directorio y publicar al final sin reemplazar un archivo existente. Si el nombre está ocupado se genera `nombre (N).ext`.
- Limpiar parciales cuando el usuario cancela o cuando termina la sesión de terminal.
- Procesar una transferencia del navegador a la vez para reducir presión de RAM, disco y CPU en VPS pequeños; se permiten varios archivos en cola.
- Mantener el mismo protocolo tanto en terminal local como remota; el servidor retransmite frames de control/datos al agente remoto solo si este anuncia `terminal-upload-v1`.
- Conservar compatibilidad con agentes anteriores: si no anuncian la capacidad, el servidor rechaza la subida antes de enviar bytes al PTY remoto y pide actualizar el cliente.

# Arquitectura actual
El navegador envía `upload.start` como control de terminal. El proceso que posee el PTY resuelve su cwd, crea un temporal y responde `upload.ready`. Los datos se envían como frames binarios con magic propio, identificador y longitud; `terminalUploadFrameFilter` tolera fragmentación del stream remoto. Al recibir `upload.finish`, el receptor valida que los bytes escritos coincidan con el tamaño declarado y publica el archivo. Las respuestas `upload.progress`, `upload.done` y `upload.error` vuelven al navegador por el mismo canal.

Para terminales remotas, los controles viajan en el framing interno ya existente del túnel de terminal y el binario de upload cruza el stream sin base64. En el agente se extrae antes de escribir al PTY, por lo que los bytes del archivo nunca aparecen como entrada del shell.

# Librerías usadas
- Standard library de Go (`os`, `filepath`, `syscall`, `encoding/json`).
- WebSocket ya existente en Pangolite.
- JavaScript nativo y File/Blob API del navegador.
- No se agregaron dependencias.

# Archivos importantes modificados
- `internal/app/terminal_upload.go`
- `internal/app/tunnel.go`
- `internal/app/terminal.go`
- `internal/app/terminal_process_linux.go`
- `internal/app/agent_client.go`
- `internal/app/templates/pages/terminal.html`
- `internal/app/assets/app/terminal.js`
- `internal/app/assets/app/panel.css`
- `internal/app/terminal_upload_test.go`
- `internal/app/terminal_process_linux_test.go`
- `internal/app/ui_test.go`
- `README.md`

# Problemas encontrados
- No existía un canal de archivos; enviar base64 o un multipart separado habría duplicado memoria y perdido el contexto del directorio actual de la terminal.
- El navegador no puede conocer de forma segura el cwd real del shell remoto.
- Una simple comprobación de existencia seguida de `rename` podría sobrescribir un archivo creado concurrentemente entre ambas operaciones.
- Un error de disco recibido durante el upload debía detener el envío del navegador en vez de continuar transmitiendo bytes inútiles.

# Soluciones implementadas
- Framing binario incremental y buffers acotados.
- Resolución del foreground process group del PTY y `/proc/<pid>/cwd`, con fallback al proceso de shell.
- Temporal oculto y publicación con hard link dentro del mismo directorio para obtener semántica de no sobrescritura en Linux; después se elimina el nombre temporal.
- Cola secuencial, progreso visual, selector múltiple y drag & drop.
- Propagación inmediata de errores para detener el envío restante.
- Read limit de 128 KiB también en el WebSocket de terminal del agente.
- Capability `terminal-upload-v1` anunciada por clientes nuevos y validada antes de permitir una subida remota.
- Pruebas de sanitización, no sobrescritura, limpieza de parcial, framing fragmentado, límite de chunk y seguimiento del cwd tras `cd`.

# Pendientes
- Ejecutar `go test ./...` y `go vet ./...` con Go 1.26 en un entorno con el toolchain y dependencias disponibles.
- La función implementada es subida navegador -> terminal. Una descarga iniciada desde la UI puede añadirse después si existe una necesidad real; no se incluyó especulativamente.

# Próximos pasos
Probar en producción archivos pequeños y grandes tanto contra la terminal del servidor como contra un cliente remoto, incluyendo Android, cambio de directorio con `cd`, cancelación/desconexión y nombres repetidos.
