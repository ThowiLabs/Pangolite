# Fecha
2026-06-26

# Objetivo
Corregir la compilación desde ZIP limpio cuando Go intenta leer metadata de Git y el repositorio no incluye `.git`.

# Decisiones tomadas
- Compilar con `-buildvcs=false` en `init.sh`.
- Actualizar `Makefile` para usar el mismo flag.
- Documentar el flag en README para builds reproducibles desde ZIP.

# Arquitectura actual
Pangolite se instala como binario del sistema en `/opt/pangolite/pangolite`, con SQLite en `/opt/pangolite/data/pangolite.db` y servicio systemd.

# Librerías usadas
Sin librerías nuevas.

# Archivos importantes modificados
- `init.sh`
- `Makefile`
- `README.md`

# Problemas encontrados
Go 1.26 puede intentar incrustar información VCS durante `go build`. En un ZIP sin `.git`, el build falló con `error obtaining VCS status: exit status 128`.

# Soluciones implementadas
Se agregó `-buildvcs=false` al build del instalador y al build manual del Makefile.

# Pendientes
Ejecutar `sudo bash init.sh` en el VPS y confirmar que el servicio inicia correctamente.

# Próximos pasos
Después de validar instalación, continuar con mejoras de UX y TCP remoto real por cliente de sistema.
