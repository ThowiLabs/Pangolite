# Fecha
2026-06-29

# Objetivo
Unificar la suspensión/reactivación de mantenimiento para recursos individuales y clientes de sistema, conservando las opciones web existentes: 403, oculto como si no existiera, plantilla existente y HTML personalizado.

# Decisiones tomadas
- Se conserva el flujo actual de recursos web para elegir respuesta de suspensión.
- Se agrega mantenimiento por cliente con selección de alcance: Web HTTP/HTTPS, TCP, UDP o combinaciones.
- TCP/UDP no usa plantilla porque no puede mostrar página; solo deja de publicarse hasta reactivarse.
- La UI advierte cuando se selecciona TCP/UDP porque puede cortar SSH, bases de datos o túneles privados.
- La reactivación solo restaura recursos que Pangolite suspendió desde el componente de mantenimiento.

# Arquitectura actual
- El modal `resource_action_modal` queda como componente reutilizable para recursos y clientes.
- Recursos individuales siguen usando `PATCH /api/resources/{id}`.
- Clientes usan `POST /api/agents/{id}/maintenance`.
- El endpoint anterior `POST /api/agents/{id}/web-maintenance` queda como compatibilidad y delega al flujo nuevo con alcance web.

# Librerías usadas
No se agregaron dependencias.

# Archivos importantes modificados
- `internal/app/model.go`
- `internal/app/store.go`
- `internal/app/server.go`
- `internal/app/assets/app/templates.js`
- `internal/app/assets/app/resources.js`
- `internal/app/assets/app/forms.js`
- `internal/app/assets/app/panel.css`
- `internal/app/templates/components/modal_resource_action.html`
- `internal/app/templates/pages/agents.html`
- `internal/app/store_test.go`

# Problemas encontrados
La implementación anterior solo contemplaba mantenimiento web por cliente. No permitía suspender TCP/UDP desde cliente ni reutilizaba completamente el sistema de opciones web de recursos.

# Soluciones implementadas
- Se agregan tablas genéricas `agent_maintenance` y `agent_maintenance_resources`.
- Se migra el mantenimiento web previo a las nuevas tablas sin eliminar las tablas anteriores.
- Se agregan métodos de store para suspender y reactivar por alcance.
- Se actualiza el listado de agentes para reportar conteos web/tcp/udp activos y suspendidos.
- Se unifica el modal de mantenimiento en frontend.

# Pendientes
- Agregar bitácora visual más detallada por batch de mantenimiento.
- Integrar el futuro panel de salud para mostrar mantenimientos activos.

# Próximos pasos
Probar en un cliente con recursos HTTP/HTTPS y TCP/UDP: suspender solo web, solo TCP/UDP, ambos y reactivar parcialmente.
