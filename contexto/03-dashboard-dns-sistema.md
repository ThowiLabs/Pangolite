# Fecha
2026-06-26

# Objetivo
Agregar configuración del dominio público del dashboard desde el panel y validar que el DNS apunte a la IP del servidor.

# Decisiones tomadas
- El dominio del dashboard y el correo ACME se guardan en SQLite en `app_settings`.
- `PANGOLITE_PUBLIC_IP` queda como fuente de verdad operativa para validar DNS sin depender siempre de servicios externos.
- `init.sh` detecta la IP real del servidor y la escribe en `/opt/pangolite/pangolite.env`.
- El panel directo sigue escuchando en `0.0.0.0:2424` sin redirección HTTPS inicial.
- `render-traefik` usa ajustes efectivos: SQLite tiene prioridad sobre variables de entorno.

# Arquitectura actual
- Pangolite corre como servicio systemd en `/opt/pangolite/pangolite`.
- SQLite vive en `/opt/pangolite/data/pangolite.db`.
- Traefik del sistema consume configuración generada en `/etc/traefik`.
- Ajustes del dashboard se editan desde `/settings`.

# Librerías usadas
- Go standard library para detección de red, DNS y HTTP.
- `golang.org/x/crypto/bcrypt` para contraseñas.
- `modernc.org/sqlite` para SQLite sin CGO.

# Archivos importantes modificados
- `init.sh`
- `README.md`
- `internal/app/config.go`
- `internal/app/model.go`
- `internal/app/network.go`
- `internal/app/server.go`
- `internal/app/store.go`
- `internal/app/ui.go`
- `cmd/pangolite/main.go`

# Problemas encontrados
- La configuración de Traefik dependía solo de variables de entorno, lo que hacía incómodo editar el dominio del dashboard desde el panel.
- El instalador imprimía `IP_DEL_SERVIDOR:2424` en vez de una IP real.

# Soluciones implementadas
- Tabla `app_settings` para `dashboard_domain` y `lets_encrypt_email`.
- Validación DNS contra la IP pública detectada/definida.
- Endpoint `/api/settings` para leer/guardar ajustes.
- Endpoint `/api/system/network` para diagnóstico de IP/DNS.
- UI en Ajustes para configurar dominio del dashboard y correo ACME.
- `init.sh` detecta IP pública y la guarda como `PANGOLITE_PUBLIC_IP`.

# Pendientes
- Mejorar flujo de aplicación automática de Traefik desde UI con reinicio seguro del servicio.
- Implementar streams TCP/UDP remotos por cliente de sistema.

# Próximos pasos
Validar en VPS con `sudo bash init.sh`, configurar `panel.midominio.com` desde Ajustes y confirmar que Traefik se aplique automáticamente.
