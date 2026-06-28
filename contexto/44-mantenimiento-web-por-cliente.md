# Fecha
2026-06-29

# Objetivo
Agregar una acción de mantenimiento por cliente de sistema para suspender únicamente sus recursos web HTTP/HTTPS sin afectar túneles TCP/UDP.

# Decisiones tomadas
- La suspensión por cliente solo afecta recursos con `mode = http` y `agent_id` del cliente seleccionado.
- Los recursos TCP/UDP quedan intactos para mantener acceso administrativo por SSH u otros túneles NAT.
- Se guarda una tabla de mantenimiento para recordar cuáles recursos web estaban activos al momento de suspender.
- La reactivación solo vuelve a encender recursos que Pangolite suspendió en esa operación; los recursos que ya estaban apagados antes no se activan accidentalmente.
- La respuesta por defecto de mantenimiento usa HTML seguro embebido con estado 200.

# Arquitectura actual
- Backend expone `POST /api/agents/{id}/web-maintenance` con `suspended: true|false`.
- Store agrega tablas `agent_web_maintenance` y `agent_web_maintenance_resources` mediante migración v6.
- Traefik se regenera después de suspender o reactivar recursos web.
- La UI de Clientes muestra acción contextual `Suspender web` o `Reactivar web`.

# Librerías usadas
- No se agregaron dependencias nuevas.

# Archivos importantes modificados
- `internal/app/model.go`
- `internal/app/store.go`
- `internal/app/server.go`
- `internal/app/store_test.go`
- `internal/app/assets/app/agents.js`
- `internal/app/assets/app/resources.js`
- `internal/app/templates/pages/agents.html`

# Problemas encontrados
- Suspender todos los recursos web de un cliente no podía implementarse solo con `enabled=false` sin riesgo de reactivar después recursos que ya estaban apagados manualmente.

# Soluciones implementadas
- Se agregó tracking de recursos afectados por mantenimiento para restaurar solo esos recursos.
- Se agregaron contadores web por cliente para mostrar estado de mantenimiento en la UI.
- Se dejó TCP/UDP sin cambios durante suspensión y reactivación web.

# Pendientes
- Permitir elegir plantilla de suspensión desde el modal de suspensión por cliente.
- Mostrar historial de mantenimientos por cliente.

# Próximos pasos
- Implementar panel de salud del sistema y validación fuerte antes de cambiar dominio principal.
