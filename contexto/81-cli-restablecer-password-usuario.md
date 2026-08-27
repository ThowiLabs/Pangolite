# Fecha
2026-08-27

# Objetivo
Añadir una vía local de recuperación de contraseñas dentro del mismo binario `pangolite`, sin alterar el modo servidor ni crear un ejecutable administrativo separado.

# Diseño
- Nueva jerarquía `pangolite user ...`, preparada para futuras operaciones multiusuario.
- Comando principal: `pangolite user reset-password USUARIO`.
- Alias corto: `pangolite user passwd USUARIO`.
- La nueva contraseña no se acepta como argumento posicional para evitar exposición en historial de shell y listas de procesos.
- En Linux, el modo interactivo abre `/dev/tty`, desactiva echo y solicita la contraseña dos veces.
- Para automatización existe `--password-stdin`.
- `--data` permite seleccionar explícitamente la base SQLite; por defecto se usa `PANGOLITE_DATA`.
- `--require-change` marca la nueva contraseña como temporal para obligar a cambiarla tras iniciar sesión.

# Seguridad y persistencia
- Se reutilizan `NormalizeUsername`, `ValidateUsername` y `ValidatePassword` del backend.
- El hash usa el mismo bcrypt que el login web.
- El cambio se ejecuta en una transacción SQLite.
- Se invalidan todas las sesiones existentes del usuario.
- Se invalidan todos sus tokens de recuperación pendientes.
- Se registra un evento de auditoría `user.password.reset_cli` con actor `system`.
- El comando no levanta HTTP, Traefik, túneles, backups periódicos ni ninguna tarea de `serve`.

# Compatibilidad
- Ejecutar `pangolite` sin subcomando continúa entrando en `serve` como antes.
- Los subcomandos existentes no cambian.
- SQLite usa WAL y `busy_timeout`, por lo que el reset puede ejecutarse con el servicio activo.
- En plataformas no Linux se exige `--password-stdin` para no introducir una implementación de consola insegura o dependencias adicionales.

# Futuro multiusuario
La jerarquía permite agregar más adelante, sin rediseñar la CLI:
- `pangolite user list`
- `pangolite user create`
- `pangolite user disable`
- `pangolite user delete`
- `pangolite user revoke-sessions`
- roles/permisos cuando exista un modelo de autorización multiusuario.

# Archivos principales
- `cmd/pangolite/main.go`
- `cmd/pangolite/password_linux.go`
- `cmd/pangolite/password_other.go`
- `cmd/pangolite/main_test.go`
- `internal/app/store.go`
- `internal/app/store_test.go`
- `README.md`
- `contexto/00-resumen-proyecto.md`
