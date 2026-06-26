# Fecha
2026-06-26

# Objetivo
Corregir el flujo de certificados ACME del dashboard cuando Traefik publica el panel por HTTPS y el navegador muestra `TRAEFIK DEFAULT CERT`.

# Decisiones tomadas
- La configuración estática de Traefik solo declara `certificatesResolvers.letsencrypt` cuando hay dominio público y correo ACME válidos.
- La configuración dinámica del dashboard declara explícitamente `tls.domains.main` para el dominio del panel.
- Si el correo ACME cambia o ACME pasa de desactivado a activado, Pangolite escribe configuración estática y reinicia Traefik de forma controlada.
- Si solo cambia la ruta HTTP/HTTPS dinámica, Pangolite mantiene recarga dinámica sin reiniciar Traefik.

# Arquitectura actual
Pangolite corre en `0.0.0.0:2424`. Traefik del sistema escucha `:80` y `:443`, obtiene certificados por HTTP-01 y enruta el dominio del dashboard hacia `http://127.0.0.1:2424`.

# Librerías usadas
Sin librerías nuevas.

# Archivos importantes modificados
- `internal/app/traefik.go`
- `internal/app/server.go`
- `README.md`

# Problemas encontrados
El navegador podía recibir el certificado por defecto de Traefik cuando ACME no había emitido todavía o cuando la configuración estática no se actualizaba al activar/cambiar el resolver.

# Soluciones implementadas
- `ACMEEnabled` ahora controla si se escribe `certificatesResolvers`.
- El dashboard HTTPS ahora incluye `tls.domains` explícito.
- Cambios de correo ACME o activación de ACME disparan escritura estática y reinicio controlado de Traefik.
- `acme.json` se conserva con permisos `0600`.

# Pendientes
Agregar vista visual de estado de certificado desde el panel.

# Próximos pasos
Implementar diagnóstico de ACME en UI: DNS, puerto 80, último error de Traefik y presencia del dominio dentro de `acme.json`.
