# Fecha
2026-08-17
# Objetivo
Permitir operar varios recursos o clientes de un proyecto sin repetir acciones manuales ni aplicar Traefik una vez por elemento.
# Decisiones tomadas
- Selección máxima de 200 elementos por operación.
- Recursos: activar, suspender y comprobar health en lote.
- Clientes: mantenimiento total, reactivar y forzar reconexión en lote.
- La API valida que todos los IDs pertenezcan al proyecto antes de modificar datos.
- Los cambios de recursos se agrupan en transacción y Traefik se aplica una sola vez por lote.
- Las operaciones que pueden interrumpir tráfico o sesiones exigen confirmación explícita en la UI.
# Arquitectura actual
Endpoints POST por proyecto en `/api/projects/{id}/resources/bulk` y `/api/projects/{id}/agents/bulk`, con toolbar contextual compacta en las tablas.
# Librerías usadas
Ninguna nueva.
# Archivos importantes modificados
`internal/app/bulk_actions.go`, `internal/app/assets/app/resources.js`, templates de recursos/clientes y `panel.css`.
# Problemas encontrados
No existe todavía un mecanismo seguro de autoactualización remota de binarios del cliente; por ello no se simuló una acción masiva de actualización que no pudiera verificarse.
# Soluciones implementadas
Operaciones disponibles sobre capacidades reales existentes: mantenimiento, reconexión, estado y health. Las acciones que pueden cortar tráfico, SSH o túneles requieren confirmación explícita en la UI.
# Pendientes
Si en el futuro existe un protocolo de actualización firmado del cliente, integrarlo como nueva acción masiva.
# Próximos pasos
Mantener los lotes acotados y registrar toda nueva operación masiva en auditoría.
