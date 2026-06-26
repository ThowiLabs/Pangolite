# Fecha
2026-06-26

# Objetivo
Corregir fallo de arranque en systemd cuando `/etc/traefik` no existe durante la preparación del namespace.

# Decisiones tomadas
- `init.sh` crea `/opt/pangolite`, `/opt/pangolite/data` y `/etc/traefik` antes de compilar/instalar el servicio.
- La unidad `pangolite.service` deja de usar `ReadWritePaths` porque ese endurecimiento provoca error `226/NAMESPACE` si una ruta no existe antes del arranque.
- Se conserva `NoNewPrivileges`, `PrivateTmp`, `ProtectHome`, `UMask=0077` y `LimitNOFILE=65535`.
- `init.sh` detiene un servicio previo antes de reescribir la unidad systemd para evitar loops de reinicio durante actualizaciones.

# Arquitectura actual
- Pangolite corre como servicio systemd directo.
- Binario: `/opt/pangolite/pangolite`.
- SQLite: `/opt/pangolite/data/pangolite.db`.
- Traefik del sistema usa `/etc/traefik`.

# Librerías usadas
Sin cambios.

# Archivos importantes modificados
- `init.sh`
- `README.md`

# Problemas encontrados
El servicio fallaba con `Failed to set up mount namespacing: /run/systemd/unit-root/etc/traefik: No such file or directory` y `status=226/NAMESPACE`.

# Soluciones implementadas
Crear directorios antes del arranque y simplificar la unidad systemd para evitar dependencia de namespace sobre rutas opcionales.

# Pendientes
Validar de nuevo en VPS con `sudo bash init.sh`.

# Próximos pasos
Revisar salida del instalador y continuar con TCP remoto por cliente de sistema.
