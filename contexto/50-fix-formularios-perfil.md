# Fix formularios de perfil

## Problema

En la pagina `/perfil`, al intentar guardar el correo de recuperacion aparecia el error JavaScript:

```text
handler is not a function
```

## Causa

El formulario de perfil usaba `bindAsyncSubmit` con los argumentos invertidos. La firma correcta del helper global es:

```js
bindAsyncSubmit(form, handler, label)
```

pero `profile.js` pasaba primero el texto de carga y despues el callback:

```js
bindAsyncSubmit(emailForm, 'Guardando', async () => { ... })
```

Eso provocaba que el helper intentara ejecutar el string `Guardando` como funcion.

## Correccion

Se corrigio `internal/app/assets/app/profile.js` para usar la firma correcta en:

- Formulario de correo de recuperacion.
- Formulario de cambio de contraseña.

Tambien se ajusto el cambio de contraseña desde `/perfil` para no usar el valor no guardado del campo de correo. El correo se gestiona de forma separada mediante `PATCH /api/profile`, y el cambio de contraseña envia solamente la contraseña actual y la nueva contraseña.

## Resultado esperado

- Guardar correo desde `/perfil` funciona sin error JS.
- Cambiar contraseña desde `/perfil` mantiene separado el flujo de correo.
- Los botones muestran estado de carga usando el helper global.
