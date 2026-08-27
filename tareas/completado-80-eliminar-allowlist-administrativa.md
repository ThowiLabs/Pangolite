# Tarea 80 — Eliminar allowlist administrativa

## Objetivo
Simplificar la protección administrativa para que un cambio de IP solo obligue a reautenticarse, sin bloquear el login mediante CIDR.

## Criterios
- Eliminar el modo `allowlist` y `PANGOLITE_ADMIN_ALLOWED_CIDRS` de la configuración activa.
- Mantener `learn`: sesión ligada a IP exacta y contraseña de nuevo al cambiar de IP.
- Mantener `off` para desactivar el enlace de sesión a IP.
- Eliminar redes aprendidas/configuradas del servidor y su persistencia histórica.
- Migrar instalaciones existentes sin dejar variables antiguas activas.
- Conservar `PANGOLITE_TRUSTED_PROXY_CIDRS` para resolución segura de la IP real detrás de proxies confiables.
- Añadir regresiones de configuración y migración.

## Resultado
Completado: el acceso administrativo queda reducido a `learn`/`off`, sin allowlist ni redes administrativas persistentes; SQLite v12 elimina la tabla histórica y los instaladores limpian la configuración antigua.
