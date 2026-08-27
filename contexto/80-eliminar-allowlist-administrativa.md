# Fecha
2026-08-27

# Objetivo
Eliminar por completo el modo administrativo `allowlist` y la lista de CIDR administrativos, conservando únicamente la reautenticación por cambio de IP y la opción de desactivar ese enlace.

# Motivo
- El comportamiento esperado es que un cambio de IP invalide la sesión y solicite nuevamente la contraseña.
- Bloquear el propio login por una lista de redes añade riesgo de perder acceso y no aporta valor al flujo esperado.
- Tras ligar cada sesión a su IP exacta, las redes aprendidas dejaron de participar en la autorización efectiva del modo `learn`.

# Solución implementada
- `PANGOLITE_ADMIN_ACCESS_MODE` acepta únicamente `learn` y `off`.
- Se eliminó `PANGOLITE_ADMIN_ALLOWED_CIDRS` de la configuración de la aplicación y de la documentación activa.
- `/api/login` ya no tiene ninguna barrera de CIDR administrativa previa a la contraseña.
- En `learn`, cada sesión sigue ligada a la IP exacta con la que se autenticó. Si cambia, el panel exige iniciar sesión otra vez y crea una nueva sesión para la IP actual.
- En `off`, la sesión no se invalida por un cambio de IP.
- Se eliminaron del servidor las redes administrativas configuradas/aprendidas y toda la lógica para recordarlas o consultarlas.
- SQLite incorpora migración v12, que elimina la tabla histórica `trusted_admin_networks`.
- Los instaladores eliminan la variable antigua `PANGOLITE_ADMIN_ALLOWED_CIDRS` de entornos existentes y normalizan modos antiguos a `learn`, salvo que el usuario tenga `off`.
- `PANGOLITE_TRUSTED_PROXY_CIDRS` se conserva porque controla qué proxies pueden suministrar de forma confiable la IP real y no es una lista de acceso administrativo.

# Regresiones
- Configuración: solo `learn` y `off` son válidos; el modo retirado se rechaza.
- Migración: el esquema llega a v12 y `trusted_admin_networks` deja de existir.
- Se conservan las pruebas de login desde una IP nueva y reautenticación cuando cambia la IP exacta.

# Archivos principales
- `internal/app/config.go`
- `internal/app/security.go`
- `internal/app/server.go`
- `internal/app/store.go`
- `internal/app/config_test.go`
- `internal/app/server_test.go`
- `internal/app/store_test.go`
- `README.md`
- `init.sh`
- `install.sh`
- `contexto/00-resumen-proyecto.md`
