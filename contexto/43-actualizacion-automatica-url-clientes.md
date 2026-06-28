# Fecha
2026-06-28

# Objetivo
Permitir que los clientes de sistema cambien automáticamente a la nueva URL del panel cuando se marca otro dominio como principal, sin requerir reinstalación ni reinicio manual del servicio cuando sea posible.

# Decisiones tomadas
- El servidor envía sugerencias autenticadas de endpoint en cada poll de clientes mediante headers HTTP.
- El cliente mantiene la conexión activa con el endpoint actual, pero si el dominio principal sugerido responde, cambia a ese endpoint.
- Cuando el cliente cambia de endpoint, también actualiza su archivo env local para persistir `PANGOLITE_SERVER_URL` y `PANGOLITE_FALLBACK_URL`.
- Si el nuevo dominio todavía no responde, el cliente no corta servicio: sigue usando el endpoint actual y vuelve a intentar en los siguientes polls.
- Si el endpoint configurado falla, el cliente conserva el fallback por IP para consultar `/api/agent/discover` y recuperar el dominio principal vigente.

# Arquitectura actual
- Servidor:
  - `/api/agent/poll`
  - `/api/agent/stream-poll`
  - `/api/agent/discover`
  - `/api/agent/jobs/{id}/response`
  envían headers:
  - `X-Pangolite-Server-URL`
  - `X-Pangolite-Fallback-URL`
  - `X-Pangolite-Domain`
  - `X-Pangolite-Public-IP`

- Cliente:
  - Lee configuración inicial desde variables de entorno o archivo env.
  - Usa `agentEndpointManager` para administrar URL activa y fallback.
  - Aplica hints del servidor si el nuevo dominio responde.
  - Persiste cambios en el archivo env configurado.

# Librerías usadas
- Solo librería estándar de Go.
- No se agregaron dependencias.

# Archivos importantes modificados
- `internal/app/agent_client.go`
- `internal/app/server.go`
- `cmd/pangolite-client/main.go`
- `cmd/pangolite-client/install_unix.go`
- `cmd/pangolite-client/install_windows.go`
- `internal/app/agent_test.go`

# Problemas encontrados
- El fallback por IP anterior solo cambiaba el endpoint en memoria cuando fallaba el dominio, pero no actualizaba el `.env`.
- Si el dominio viejo todavía respondía, el cliente no tenía motivo para consultar discovery y podía seguir conectado al dominio heredado indefinidamente.
- El servidor no notificaba a clientes conectados que había un nuevo dominio principal.

# Soluciones implementadas
- Se agregaron headers de endpoint vigente a respuestas autenticadas del agente.
- El cliente ahora consume esos headers, valida que el dominio sugerido responda y cambia sin cortar servicio.
- El cliente actualiza su archivo env local cuando cambia URL activa o fallback.
- Se agregó `--config-file` y `PANGOLITE_CONFIG_PATH` para controlar qué archivo env debe actualizarse.
- Linux usa por defecto `/opt/pangolite-client/pangolite-client.env`.
- Windows usa por defecto `C:\ProgramData\Pangolite Client\pangolite-client.env`.

# Pendientes
- Mostrar en la UI la URL activa reportada y diferenciarla de la URL configurada originalmente si se requiere más transparencia.
- Probar en Windows real que el reemplazo del env funcione con permisos del servicio.

# Próximos pasos
- Ejecutar `go test -timeout 2m ./...` en servidor con Go 1.24+.
- Cambiar dominio principal y verificar que clientes conectados actualicen su `.env` sin reiniciar.
