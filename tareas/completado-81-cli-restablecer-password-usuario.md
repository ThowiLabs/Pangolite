# Tarea 81 — CLI para restablecer contraseña de usuario

## Objetivo
Permitir recuperar localmente el acceso de un usuario desde el mismo binario de Pangolite, con una interfaz que pueda crecer a administración multiusuario.

## Criterios
- Añadir `pangolite user reset-password USUARIO` sin modificar el comportamiento de `serve`.
- No aceptar la contraseña nueva como argumento posicional.
- Solicitar contraseña oculta y confirmación en Linux.
- Admitir `--password-stdin` para automatización.
- Usar `PANGOLITE_DATA`/`--data` para abrir la misma base SQLite.
- Validar usuario y contraseña con las reglas existentes.
- Actualizar el hash bcrypt dentro de una transacción.
- Invalidar sesiones y tokens de recuperación anteriores.
- Registrar auditoría del reset CLI.
- Añadir cobertura de regresión del Store y del subcomando.

## Resultado
Completado: el binario principal dispone de una jerarquía `user` extensible y puede restablecer contraseñas de usuarios existentes sin iniciar el servidor ni mantener un segundo ejecutable.
