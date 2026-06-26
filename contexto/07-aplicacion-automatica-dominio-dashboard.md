# Fecha
2026-06-26

# Objetivo
Eliminar mensajes heredados que pedían renderizar Traefik manualmente al guardar el dominio del dashboard y forzar la aplicación automática de cambios ACME/dominio.

# Decisiones tomadas
- Al actualizar ajustes del dashboard, Pangolite reinicia Traefik de forma controlada si cambia el dominio, el correo ACME o el estado de ACME.
- La UI muestra el resultado real devuelto por el backend en `data.traefik.message`.
- El usuario final ya no debe ver instrucciones de “renderizar Traefik” después de guardar ajustes.

# Arquitectura actual
- Pangolite corre en `0.0.0.0:2424`.
- Traefik corre como servicio del sistema.
- La configuración dinámica HTTP/HTTPS se recarga automáticamente.
- La configuración estática se regenera y Traefik se reinicia solo cuando cambian opciones que viven en `traefik.yml`, como ACME o entrypoints TCP/UDP.

# Librerías usadas
- Go standard library.
- `modernc.org/sqlite`.
- `golang.org/x/crypto/bcrypt`.

# Archivos importantes modificados
- `internal/app/server.go`.
- `internal/app/ui.go`.
- `README.md`.

# Problemas encontrados
El mensaje heredado pedía aplicar Traefik manualmente después de guardar el dominio del panel y confundía al administrador. Además, si el dominio cambiaba pero ACME ya estaba activo y el correo era el mismo, el backend solo aplicaba configuración dinámica.

# Soluciones implementadas
- Se considera cambio estático cuando cambia el dominio del dashboard, el correo ACME o el estado de ACME.
- En esos casos se ejecuta `applyTraefikStaticAndRestart()` automáticamente.
- El mensaje visual usa el resultado real de Traefik.

# Pendientes
- Agregar pantalla de diagnóstico ACME con estado de DNS, puertos 80/443, acme.json y últimos errores de Traefik.

# Próximos pasos
Validar en VPS guardando nuevamente el dominio del dashboard y revisar que el panel muestre “Traefik actualizado automaticamente”.
