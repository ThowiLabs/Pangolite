# 68 - Cambio de nombre de usuario

# Fecha
2026-08-12

# Objetivo
Permitir cambiar el nombre de inicio de sesión del administrador desde Perfil y seguridad sin crear tablas nuevas ni invalidar la sesión actual.

# Decisiones tomadas
- Reutilizar `NormalizeUsername` y `ValidateUsername`; no duplicar reglas de formato.
- Guardar el nombre normalizado en minúsculas para mantener el mismo comportamiento que el login existente.
- Exigir la contraseña actual cuando el valor realmente cambia.
- Mantener el cambio de correo en su endpoint actual y crear un endpoint pequeño para username, evitando actualizaciones parciales de perfil.
- Mantener sesiones por `user_id`; al renombrar, la sesión actual sigue siendo válida y en la siguiente lectura ya refleja el nuevo username.
- Registrar la operación en auditoría sin almacenar contraseñas.

# Arquitectura actual
`PATCH /api/profile/username` recibe `username` y `currentPassword`, pasa por `requireAuth`/CSRF, valida la contraseña y persiste el nuevo nombre en `users`. La UI de `/profile` actualiza `appBoot.user` y el nombre mostrado en el shell del panel sin requerir logout.

# Librerías usadas
- SQLite ya existente.
- bcrypt ya existente.
- JavaScript nativo.
- No se agregaron dependencias.

# Archivos importantes modificados
- `internal/app/store.go`
- `internal/app/server.go`
- `internal/app/templates/pages/profile.html`
- `internal/app/assets/app/profile.js`
- `internal/app/store_test.go`
- `README.md`
- `contexto/00-resumen-proyecto.md`

# Problemas encontrados
El perfil solo permitía modificar correo y contraseña aunque `users.username` ya era un campo único y el login estaba preparado para resolverlo dinámicamente.

# Soluciones implementadas
Se añadió `Store.UpdateUsername`, validación de unicidad, comprobación bcrypt de contraseña actual, endpoint dedicado, formulario de perfil y prueba de normalización/autenticación con el nuevo nombre.

# Pendientes
- Ejecutar la suite completa con Go 1.26 en un entorno con el toolchain/dependencias disponibles.

# Próximos pasos
Continuar con transferencia de archivos desde la terminal sin cargar archivos completos en memoria.
