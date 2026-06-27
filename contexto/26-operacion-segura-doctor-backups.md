# 26 - Operación segura: migraciones, doctor, checksums, health y backups

Se implementó una capa operativa para producción:

- Migraciones SQLite versionadas con tabla `schema_migrations`.
- Respaldo pre-migración en `data/backups/migrations/` antes de aplicar cambios de esquema.
- Comando `pangolite doctor` para diagnosticar SQLite, migraciones, rutas, Traefik, puertos y servicios.
- Validación de configuración Traefik con rollback de archivos si `traefik check` falla.
- `install.sh` verifica `checksums.txt` del release antes de extraer el paquete.
- Releases generan clientes descargables Linux amd64, arm64, 386, armv7 y Windows amd64.
- Comando Linux de cliente de sistema detecta arquitectura con `uname -m` y descarga el cliente correcto.
- Health checks por recurso muestran estado y latencia.
- Backups automáticos configurables con `PANGOLITE_BACKUP_INTERVAL_HOURS` y `PANGOLITE_BACKUP_RETENTION_DAYS`.

Pendientes fuera de este bloque: logs por recurso, rate limit avanzado para protección web, restauración asistida, roles/permisos y hardening adicional.
