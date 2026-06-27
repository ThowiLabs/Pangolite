# Contexto 19 - Zona de peligro de proyectos y clientes

## Fecha
2026-06-27

## Objetivo
Agregar operaciones destructivas seguras para proyectos, recursos y clientes NAT sin permitir pérdida accidental de datos.

## Decisiones tomadas
- El proyecto se puede renombrar y actualizar con descripción desde su vista principal.
- La eliminación de proyectos vive en una sección llamada "Zona de peligro".
- Un proyecto solo puede eliminarse si no tiene recursos ni clientes vinculados.
- El proyecto `default` / General no se puede eliminar porque es la base de migración y compatibilidad.
- Eliminar un cliente NAT elimina también todos los recursos vinculados a ese cliente.
- La eliminación de cliente NAT requiere escribir la contraseña del administrador actual.

## Arquitectura actual
- `DELETE /api/projects/{id}` elimina proyectos vacíos.
- `PATCH /api/projects/{id}` conserva el estado actual si no se envía `enabled`.
- `DELETE /api/agents/{id}` elimina cliente y recursos asociados dentro de una transacción.
- Después de eliminar recursos por cliente, Pangolite aplica Traefik automáticamente.

## Archivos importantes modificados
- `internal/app/server.go`
- `internal/app/store.go`
- `internal/app/ui.go`
- `README.md`

## Problemas resueltos
- Antes los proyectos no tenían una zona clara para renombrar, describir o eliminar.
- La acción de cliente solo deshabilitaba; ahora existe eliminación real con confirmación fuerte.
- El endpoint de proyecto podía apagar un proyecto por accidente si `enabled` no venía en el JSON.

## Pendientes
- Añadir auditoría detallada por acción destructiva.
- Añadir permisos por rol antes de abrir el panel a múltiples usuarios.
- Añadir restauración desde backup para eliminaciones críticas.

## Próximos pasos
Implementar auditoría por proyecto/recurso/cliente y una pantalla de respaldo/restauración para SQLite.
