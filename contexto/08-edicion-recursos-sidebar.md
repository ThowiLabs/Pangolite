# Fecha
2026-06-26

# Objetivo
Mejorar la experiencia enterprise del panel agregando edición completa de recursos, recarga automática de Traefik y un selector de proyecto en el sidebar inspirado en SB Admin Pro.

# Decisiones tomadas
- Los recursos ahora se editan desde un modal con botón `Editar` por fila.
- El endpoint `PATCH /api/resources/{id}` soporta edición completa y mantiene compatibilidad con el control de suspensión.
- Pangolite compara el estado de recursos antes/después para decidir si Traefik solo debe recargar configuración dinámica o si requiere reinicio controlado por cambio de entrypoints TCP/UDP.
- El sidebar ya no lista todos los proyectos directamente; usa un selector desplegable con búsqueda para evitar ruido visual cuando haya muchos clientes.
- El sidebar puede ocultarse/mostrarse desde un botón en el topbar.

# Arquitectura actual
- Go estándar para HTTP y UI server-side ligera.
- SQLite embebido para usuarios, sesiones, proyectos, dominios, agentes y recursos.
- Traefik del sistema con provider HTTP/file para recarga automática.
- Recursos HTTP/HTTPS se actualizan sin reiniciar Traefik.
- TCP/UDP solo reinicia Traefik si cambia la firma de puertos públicos.

# Librerías usadas
- `golang.org/x/crypto/bcrypt` para contraseñas.
- `modernc.org/sqlite` para SQLite sin CGO.
- SB Admin Pro CSS local y Bootstrap Icons por CDN para iconografía.

# Archivos importantes modificados
- `internal/app/server.go`
- `internal/app/store.go`
- `internal/app/ui.go`
- `README.md`
- `contexto/08-edicion-recursos-sidebar.md`

# Problemas encontrados
- La UI no tenía edición completa por recurso; solo permitía suspensión o eliminación.
- El sidebar podía saturarse al listar todos los proyectos.
- Cambios de recursos debían aplicar Traefik automáticamente sin afectar otros recursos cuando no cambian entrypoints.

# Soluciones implementadas
- Agregado `Store.ResourceByID` y `Store.UpdateResource`.
- Agregada validación de puertos en edición, excluyendo el propio recurso.
- Agregado modal de edición de recurso.
- Agregado widget dropdown con búsqueda para cambiar de proyecto.
- Agregado sidebar colapsable persistente por `localStorage`.

# Pendientes
- Implementar TCP remoto real por cliente de sistema.
- Empaquetar Bootstrap Icons localmente si se quiere operar sin CDN.
- Agregar auditoría de cambios por usuario para recursos críticos.

# Próximos pasos
Validar en VPS con `sudo bash init.sh`, probar edición HTTP sin reiniciar Traefik y probar edición TCP cambiando puerto para confirmar reinicio controlado.
