# Fecha
2026-06-28

# Objetivo
Implementar una solución simple para clientes instalados cuando cambia el dominio del panel: si la URL principal deja de responder, el cliente intenta recuperar la URL vigente consultando un endpoint por IP del VPS. Aprovechar el cambio para auditar y reforzar puntos de seguridad en la conexión cliente-servidor y en el proxy del panel.

# Decisiones tomadas
- Se agrega `fallback_url` a los clientes de sistema.
- Los comandos de instalación nuevos incluyen `--fallback-url http://IP_DEL_VPS:2424` cuando el servidor conoce `PANGOLITE_PUBLIC_IP`.
- Se agrega el endpoint `POST /api/agent/discover` autenticado con el mismo ID/token del cliente.
- El endpoint de descubrimiento devuelve la URL principal actual del panel; si no hay dominio configurado, devuelve la URL por IP del VPS.
- El cliente mantiene en memoria el endpoint activo. Si el dominio falla, consulta el fallback por IP y prueba la URL principal devuelta; si no responde, usa la IP como endpoint operativo.
- Se elimina el bloqueo global que impedía borrar dominios administrados cuando existía cualquier cliente. Ahora se bloquea por riesgo real: dominio principal, recursos asociados o clientes vinculados por `server_url`/`domain_id` que todavía no tienen `fallback_url`.
- Se filtran cookies y headers internos de Pangolite antes de reenviar requests a backends locales o remotos.

# Arquitectura actual
- Cliente:
  - `PANGOLITE_SERVER_URL`: endpoint principal preferido.
  - `PANGOLITE_FALLBACK_URL`: endpoint directo por IP para redescubrimiento.
  - `agentEndpointManager`: mantiene endpoint activo y ejecuta recuperación.
- Servidor:
  - `/api/agent/discover`: endpoint público solo para clientes autenticados.
  - `publicBaseURL`: usa dominio principal si existe; si no, usa IP pública.
  - `publicIPBaseURL`: construye `http://IP:PUERTO` usando `PANGOLITE_PUBLIC_IP` y el puerto interno del panel.

# Librerías usadas
- Solo librería estándar de Go.
- No se agregaron dependencias nuevas.

# Archivos importantes modificados
- `internal/app/agent_client.go`
- `internal/app/model.go`
- `internal/app/store.go`
- `internal/app/server.go`
- `internal/app/headers.go`
- `cmd/pangolite/main.go`
- `cmd/pangolite-client/main.go`
- `cmd/pangolite-client/install_unix.go`
- `cmd/pangolite-client/install_windows.go`
- `internal/app/agent_test.go`
- `internal/app/store_test.go`

# Problemas encontrados
- El bloqueo anterior de eliminación de dominios era demasiado conservador: impedía borrar cualquier dominio si existía cualquier cliente, aunque ese cliente no usara el dominio.
- Los clientes instalados solo tenían una URL principal. Si esa URL dejaba de resolver, no podían descubrir el dominio nuevo.
- El proxy podía reenviar cookies internas de Pangolite o headers `X-Pangolite-*` a backends, lo cual no debe ocurrir.
- En recursos protegidos por Pangolite con Basic/Auth, el header `Authorization` podía representar credenciales del protector y no debe pasar al backend.

# Soluciones implementadas
- `fallback_url` persistente en SQLite con migración v5.
- Flag/env `--fallback-url` / `PANGOLITE_FALLBACK_URL` en cliente Linux/Windows y modo agent del binario principal.
- Descubrimiento autenticado por IP.
- Fallback automático con prueba de `/healthz` para elegir entre dominio principal actual e IP.
- Filtrado de cookies internas (`pangolite_session`, `pangolite_resource_*`) y headers internos antes de proxy.
- Eliminación de `Authorization` hacia backend cuando el recurso usa protección de Pangolite.
- Eliminación física de dominios heredados permitida cuando los clientes vinculados ya tienen fallback por IP; si no lo tienen, se bloquea para evitar clientes huérfanos.

# Pendientes
- Los clientes ya instalados antes de este cambio no tendrán `PANGOLITE_FALLBACK_URL` hasta reinstalarlos o actualizar su archivo `.env`.
- Si se usa IP directa por HTTP, el tráfico del cliente no tiene TLS. Es aceptable como fallback simple, pero en producción conviene mantener un dominio válido siempre que sea posible.

# Próximos pasos
- Probar cambio de dominio en un cliente Linux nuevo.
- Confirmar que el cliente recupera conexión por IP si el dominio viejo deja de responder.
- Considerar una acción futura en UI para mostrar clientes sin fallback configurado.
