# Fecha
2026-08-27

# Objetivo
Corregir el control de acceso administrativo por IP para que un cambio de IP invalide la sesión actual y solicite nuevamente la contraseña, sin bloquear el propio login en el modo `learn`.

# Problema encontrado
- El hardening anterior comprobaba `adminIPAllowed` antes de autenticar `/api/login`.
- Después de aprender una primera red, una IP de otra red recibía `403` antes de poder validar usuario y contraseña.
- Las sesiones no almacenaban la IP con la que se habían creado. El control era global por redes aprendidas, por lo que no detectaba un cambio exacto de IP dentro de un mismo `/24` o `/64`.

# Solución implementada
- SQLite incorpora migración v11 con `sessions.client_ip`.
- Cada login persiste la IP real obtenida mediante la cadena de proxies confiables ya existente.
- En `learn`, `/api/login` acepta cualquier IP válida para llegar a la comprobación de contraseña; solo después de autenticar se registra la red conocida.
- `currentSession` compara la IP exacta de la sesión con la IP actual. Si cambió, la sesión deja de autorizar y las páginas del panel redirigen a `/login`.
- Después de introducir correctamente la contraseña desde la IP nueva se crea otra sesión ligada a esa IP y la navegación continúa normalmente.
- `allowlist` conserva el bloqueo estricto de login desde redes no autorizadas.
- `off` conserva la capacidad de desactivar esta protección.
- Las sesiones creadas antes de la migración no tienen IP asociada y por seguridad requerirán iniciar sesión nuevamente una vez después de actualizar.

# Pruebas añadidas
- Persistencia de la IP dentro de una sesión.
- Login válido desde una red nueva en modo `learn` aunque todavía no esté aprendida.
- Redirección a `/login` cuando la IP exacta cambia, incluso dentro del mismo `/24`.
- Reautenticación correcta desde la nueva IP y emisión de una sesión nueva.
- Conservación del bloqueo de redes desconocidas en `allowlist`.

# Archivos principales
- `internal/app/model.go`
- `internal/app/store.go`
- `internal/app/security.go`
- `internal/app/server.go`
- `internal/app/store_test.go`
- `internal/app/server_test.go`
- `README.md`
- `init.sh`
- `install.sh`
- `contexto/00-resumen-proyecto.md`

# Validación
- `gofmt` aplicado.
- `bash -n init.sh`, `bash -n install.sh` y `sh -n install.sh`.
- `git diff --check`.
- La suite Go completa requiere Go 1.26 y módulos disponibles; el entorno actual tiene Go 1.23.2 local y no puede descargar el toolchain por falta de salida DNS.
