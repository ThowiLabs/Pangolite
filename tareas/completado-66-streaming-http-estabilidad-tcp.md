# Fecha
2026-08-12

# Objetivo
Eliminar los cortes periódicos de SSH/SFTP en recursos TCP remotos y convertir el túnel HTTP de agentes a streaming para soportar cargas/descargas grandes con memoria acotada en VPS pequeños.

# Alcance completado
- Separar el timeout de adjunto del agente de la vida útil de una sesión TCP.
- Añadir keepalive ligero, `TCP_NODELAY` y escritura completa en puentes WebSocket/TCP.
- Reutilizar el cliente HTTP configurado en el handshake WebSocket del agente.
- Añadir túnel HTTP streaming compatible con clientes nuevos sin romper clientes anteriores.
- Mantener fallback al protocolo HTTP legado de 16 MiB cuando el agente no anuncia la capacidad nueva.
- Soportar respuesta temprana del backend mientras continúa un upload sin bloquear el túnel.
- Acotar concurrencia TCP remota para proteger RAM/CPU.
- Actualizar pruebas, README, instaladores y contexto.

# Validación
- `gofmt`: correcto.
- `bash -n init.sh`: correcto.
- `bash -n install.sh`: correcto.
- `sh -n install.sh`: correcto.
- `git diff --check`: correcto.
- Pruebas aisladas del `TunnelHub` con `go test -race`: correctas; incluyen regresión del timeout tras adjunto y expiración/downgrade de capacidades.
- `go test ./...` y `go vet ./...` completos no pudieron ejecutarse porque el entorno disponible tiene Go 1.23.2, el repositorio requiere Go >=1.26.0 y la red del contenedor no permite descargar el toolchain/dependencias.

# Estado
Completado y listo para commit/entrega.
