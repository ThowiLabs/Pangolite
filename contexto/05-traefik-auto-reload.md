# Fecha
2026-06-26

# Objetivo
Eliminar la necesidad de que el usuario renderice Traefik manualmente al configurar dominios del dashboard, recursos HTTP/HTTPS o suspensión de recursos, y hacer que init.sh instale Traefik si falta.

# Decisiones tomadas
- Traefik del sistema se instala automáticamente desde release oficial si no existe el binario `traefik`.
- La configuración estática se mantiene en `/etc/traefik/traefik.yml`.
- La configuración dinámica del dashboard se mueve a `/etc/traefik/dynamic/pangolite-dashboard.yml`.
- Se habilita `providers.file.watch=true` para que Traefik recargue cambios dinámicos sin reiniciar.
- Los recursos HTTP/HTTPS siguen entregándose por `providers.http` desde `/api/v1/traefik-config`, con polling de 5s.
- Los puertos TCP/UDP siguen requiriendo entrypoints estáticos; Pangolite detecta cambios de puertos y reinicia Traefik automáticamente de forma controlada.

# Arquitectura actual
- Pangolite corre como systemd en `0.0.0.0:2424`.
- SQLite vive en `/opt/pangolite/data/pangolite.db`.
- Traefik corre como servicio systemd y consume:
  - `/etc/traefik/traefik.yml` para configuración estática.
  - `/etc/traefik/dynamic/` para rutas dinámicas del dashboard.
  - `http://127.0.0.1:2424/api/v1/traefik-config` para recursos HTTP/HTTPS/TCP/UDP.

# Librerías usadas
- Librería estándar de Go.
- `golang.org/x/crypto/bcrypt`.
- `modernc.org/sqlite`.

# Archivos importantes modificados
- `init.sh`.
- `internal/app/traefik.go`.
- `internal/app/server.go`.
- `internal/app/config.go`.
- `cmd/pangolite/main.go`.
- `internal/app/ui.go`.
- `README.md`.

# Problemas encontrados
El flujo anterior mostraba al usuario mensajes como “renderizar Traefik”, lo cual era confuso y obligaba a aplicar manualmente cambios que deberían ser automáticos.

# Soluciones implementadas
- `init.sh` instala Traefik si falta y crea servicio systemd cuando no existe.
- Pangolite escribe configuración dinámica para el dashboard y Traefik la recarga automáticamente.
- Crear, borrar o suspender recursos HTTP/HTTPS ya no requiere acción manual.
- Crear o eliminar recursos TCP/UDP dispara render estático y reinicio automático de Traefik cuando cambia la lista de puertos públicos.

# Pendientes
- Implementar TCP/UDP remoto por cliente de sistema con streams persistentes.
- Mejorar el reinicio controlado de Traefik para entrypoints TCP/UDP con validación previa más visible en UI.

# Próximos pasos
Ejecutar `sudo bash init.sh` en VPS y validar que Traefik se instale o detecte, que el panel arranque y que el dominio del dashboard se aplique sin pasos manuales.
