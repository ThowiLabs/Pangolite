# Fecha
2026-06-28

# Objetivo
Corregir inconsistencias del ciclo de vida de dominios administrados y el fallback por IP de clientes.

# Decisiones tomadas
- Cambiar dominio principal debe estar permitido sin eliminar el dominio anterior.
- El dominio anterior puede quedar heredado para compatibilidad con clientes antiguos.
- No basta con que el panel haya generado una URL fallback; el cliente debe confirmar que realmente está instalado con fallback.
- La eliminación de dominios con clientes solo se considera segura cuando esos clientes reportan fallback por IP confirmado.

# Arquitectura actual
- El cliente envía en cada solicitud autenticada sus URLs configuradas mediante headers internos:
  - `X-Pangolite-Client-Server-URL`
  - `X-Pangolite-Client-Fallback-URL`
- El servidor registra `fallback_confirmed_at` cuando recibe un fallback válido desde un cliente autenticado.
- `/api/agent/discover` sigue siendo el endpoint de rescate usado por el cliente contra la IP del VPS para obtener el dominio principal vigente.

# Librerías usadas
No se agregaron dependencias nuevas.

# Archivos importantes modificados
- `internal/app/model.go`
- `internal/app/store.go`
- `internal/app/server.go`
- `internal/app/agent_client.go`
- `internal/app/store_test.go`
- `internal/app/assets/app/agents.js`
- `internal/app/assets/app/projects.js`
- `internal/app/templates/pages/settings.html`

# Problemas encontrados
- La UI permitía confusión entre “generé fallback en el comando” y “el cliente ya tiene fallback realmente instalado”.
- Rotar token podía dejar `fallback_url` en base de datos aunque el cliente todavía no hubiera aplicado el nuevo comando.
- El botón para hacer principal un dominio no explicaba que no elimina el anterior.

# Soluciones implementadas
- Se agregó confirmación real de fallback por heartbeat de cliente.
- La eliminación de dominios con clientes se bloquea si hay clientes sin fallback confirmado.
- La UI muestra cuántos clientes siguen sin fallback confirmado.
- El cambio a dominio principal se mantiene como acción independiente de eliminar el dominio anterior.
- El texto de Ajustes explica el flujo de fallback por IP y dominios heredados.

# Pendientes
- Probar en producción con un cliente Linux instalado antes de eliminar un dominio heredado.
- Validar que el firewall permita acceso al puerto fallback por IP cuando se use como mecanismo de rescate.

# Próximos pasos
- Ejecutar `go test -timeout 2m ./...` en el servidor con Go 1.24+.
- Cambiar el dominio principal desde la UI y verificar que los clientes reporten fallback confirmado antes de eliminar el dominio anterior.
