# 16 - Eliminación estable de recursos y reinicio agrupado de Traefik

## Fecha
2026-06-27

## Objetivo
Corregir errores intermitentes tipo `Failed to fetch` al eliminar recursos TCP/UDP desde el panel y evitar que la tabla de recursos quede desactualizada después de una eliminación correcta.

## Decisiones tomadas
- La eliminación de recursos ahora es idempotente: si el recurso ya no existe, la API responde como operación completada para tolerar doble clics, reintentos del navegador o acciones repetidas desde la UI.
- La tabla de recursos se actualiza localmente inmediatamente después de eliminar un recurso para que el usuario vea el cambio aunque Traefik se reinicie segundos después.
- El refresco completo de proyectos/recursos se mantiene, pero si falla por una reconexión temporal se reintenta en segundo plano.
- Los reinicios de Traefik por cambios de entrypoints TCP/UDP se agrupan con debounce de 15 segundos para evitar múltiples reinicios seguidos al eliminar varios recursos.

## Arquitectura actual
- HTTP/HTTPS se sigue aplicando por configuración dinámica de Traefik.
- TCP/UDP sigue requiriendo actualización de entrypoints estáticos.
- Pangolite programa un reinicio controlado de Traefik solo para cambios TCP/UDP, pero ahora lo agrupa para reducir cortes del panel cuando se accede por dominio detrás de Traefik.

## Archivos importantes modificados
- `internal/app/server.go`
- `internal/app/ui.go`

## Problemas encontrados
- El log mostraba eliminación exitosa con `status=200`, seguida de un segundo `DELETE` al mismo recurso con `status=404`.
- El usuario podía ver `Failed to fetch` si Traefik reiniciaba mientras el navegador intentaba refrescar la tabla.
- Al eliminar varios recursos TCP/UDP seguidos, cada operación podía programar un reinicio independiente de Traefik.

## Soluciones
- `DELETE /api/resources/{id}` ahora considera “recurso no encontrado” como operación ya completada.
- La UI evita operaciones duplicadas por recurso con `deletingResources`.
- La UI elimina el recurso del arreglo local y repinta la tabla antes de intentar refrescar desde API.
- Se agregó `refreshCurrentProjectSoft()` para reintentar refrescos si hay reconexión temporal.
- `scheduleTraefikRestart()` ahora usa `time.AfterFunc` con debounce y reprograma el reinicio si llegan más cambios antes de ejecutarse.

## Pendientes
- Mostrar en el panel un indicador persistente de “Traefik aplicando cambios TCP/UDP”.
- Implementar historial visible de reinicios/programaciones de Traefik desde `/logs` con filtros por tipo.
- Evaluar un modo de mantenimiento que cierre entrypoints antiguos en horarios definidos, en lugar de reiniciar inmediatamente tras eliminar recursos.

## Próximos pasos
- Probar eliminación masiva de recursos TCP/UDP desde el panel usando el dominio HTTPS.
- Verificar que la tabla se actualiza sin recargar manualmente.
- Confirmar que el log agrupa varios cambios como reinicio reprogramado.
