# 48 - Perfil de usuario y dropdown superior

## Objetivo

Mejorar la UX de cuenta moviendo las opciones personales fuera de Ajustes/SMTP y fuera del flujo forzado de contraseña. El panel ahora tiene un menú de usuario en la esquina superior derecha y una página `/perfil` para gestionar datos de la cuenta actual.

## Cambios

- Se agrega ruta de panel `/perfil` con `PageKey` `profile`.
- El nombre de usuario del topbar ahora abre un dropdown con:
  - Mi perfil.
  - Nota de preparación para múltiples usuarios y roles en una fase futura.
  - Cerrar sesión.
- Se agrega página `profile.html` con dos formularios separados:
  - Correo de recuperación.
  - Cambio de contraseña.
- Se agrega `profile.js` para inicializar los datos desde `appBootstrap.user`, guardar correo por endpoint dedicado y cambiar contraseña usando el flujo existente.
- Se agrega endpoint autenticado `PATCH /api/profile` para actualizar solo el correo de recuperación de la cuenta actual.
- Se conserva `/password` como flujo de cambio forzado/inicial para no romper la instalación actual.

## Decisiones

- No se implementan aún múltiples usuarios ni roles. Solo se deja la UI preparada con lenguaje claro de “próximamente”.
- El correo de recuperación se separa de SMTP: SMTP es configuración global del sistema; el correo pertenece a la cuenta.
- El cambio de contraseña en `/perfil` siempre exige contraseña actual.
- La actualización de correo registra auditoría `user.profile.update`.

## Pruebas esperadas

1. Iniciar sesión en el panel.
2. Abrir el dropdown del usuario arriba a la derecha.
3. Entrar a `/perfil`.
4. Guardar correo de recuperación.
5. Cerrar sesión y confirmar que, si SMTP está listo, aparece el vínculo de recuperación en login.
6. Cambiar contraseña desde `/perfil` con contraseña actual.
7. Confirmar que `/password` sigue funcionando para el flujo forzado inicial.
