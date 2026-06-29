# 49 - Fix ruta de perfil sin redirección

## Problema

Al entrar a `/perfil`, la página cargaba correctamente al inicio, pero después de unos segundos el frontend redirigía a `/projects`.

La causa era que el router frontend global no tenía un caso explícito para `/perfil`. Después de inicializar datos asincrónicos, el router intentaba resolver la ruta como si fuera una ruta de proyecto. Al no coincidir con `/projects/...`, ejecutaba el fallback `go('/projects')`.

## Solución

Se agregó soporte explícito para `/perfil` en el router frontend:

- Mantiene la página de perfil activa.
- Actualiza el encabezado superior a `Mi perfil / Perfil y seguridad`.
- Marca el vínculo de perfil en el menú de usuario.
- Reejecuta la inicialización de perfil de forma segura si la función está disponible.

## Archivos afectados

- `internal/app/assets/app/agents.js`

## Resultado

`/perfil` ya no es tratado como ruta desconocida y deja de redireccionar automáticamente a `/projects`.
