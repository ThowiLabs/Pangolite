# Fecha
2026-06-29

# Objetivo
Agregar correo de recuperación a la cuenta del panel y habilitar restablecimiento de contraseña desde el login cuando SMTP esté configurado y validado.

# Decisiones tomadas
- Se agregó correo de recuperación al usuario.
- El cambio/primer guardado de contraseña ahora pide correo de recuperación.
- La recuperación en login solo se muestra cuando SMTP está habilitado, validado y existe al menos una cuenta con correo.
- La configuración SMTP se guarda en ajustes y se valida al habilitarla.
- La contraseña SMTP se conserva si el campo queda vacío al guardar ajustes.
- Los tokens de recuperación se guardan hasheados y vencen en 20 minutos.
- Al consumir un token se invalidan las sesiones existentes del usuario.

# Arquitectura actual
- `users.email` guarda el correo de recuperación.
- `password_reset_tokens` guarda tokens hasheados de restablecimiento.
- `app_settings` guarda configuración SMTP.
- `/api/password-reset/status` informa si el login debe mostrar recuperación.
- `/api/password-reset/request` solicita el correo de recuperación sin revelar si existe la cuenta.
- `/api/password-reset/confirm` consume el token y actualiza la contraseña.
- `/api/settings/smtp/test` envía un correo de prueba al correo del usuario autenticado.

# Librerías usadas
- Standard library de Go: `net/smtp`, `crypto/tls`, `net/mail`.
- No se agregaron dependencias nuevas.

# Archivos importantes modificados
- `internal/app/model.go`
- `internal/app/store.go`
- `internal/app/server.go`
- `internal/app/smtp.go`
- `internal/app/templates/pages/login.html`
- `internal/app/templates/pages/password.html`
- `internal/app/templates/pages/reset.html`
- `internal/app/templates/pages/settings.html`
- `internal/app/assets/app/login.js`
- `internal/app/assets/app/password.js`
- `internal/app/assets/app/reset.js`
- `internal/app/assets/app/settings.js`
- `internal/app/assets/app/auth.css`
- `internal/app/assets/app/panel.css`
- `internal/app/store_test.go`

# Problemas encontrados
- Antes no había correo de cuenta, por lo que no existía forma segura de restablecer contraseña por correo.
- La recuperación no debe aparecer si SMTP no está configurado o si no hay correo en la cuenta.
- Guardar SMTP sin feedback real podía dejar una configuración inválida.

# Soluciones implementadas
- Correo obligatorio al cambiar/confirmar contraseña.
- Configuración SMTP en Ajustes con host, puerto, seguridad, usuario, contraseña, remitente y nombre.
- Validación de conectividad SMTP al guardar con SMTP habilitado.
- Botón de prueba SMTP.
- Pantalla `/reset` para definir nueva contraseña con token.
- Rate limit simple para solicitudes de recuperación usando el mismo mecanismo del login.

# Pendientes
- Considerar cifrar la contraseña SMTP en reposo si se agrega una llave maestra local.
- Agregar bitácora visible de envío de correos y recuperaciones cuando exista panel de eventos más completo.

# Próximos pasos
- Probar con SMTP real: STARTTLS 587 y TLS 465.
- Configurar correo de recuperación de la cuenta antes de depender del restablecimiento por login.
