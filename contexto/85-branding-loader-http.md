# 85 - Branding de carga opcional por recurso HTTP

## Objetivo
Permitir que un administrador active un loader de marca Thowilabs únicamente en recursos web concretos. La función debe permanecer desactivada por defecto y no debe alterar APIs, descargas ni otros recursos.

## Diseño
- `resources.branding_loader_enabled` se incorpora en la migración SQLite v13 con default `0`.
- La opción solo es válida para recursos `http` que no sean redirects permanentes. Al cambiar a TCP/UDP o redirect se normaliza automáticamente a `false`.
- Cuando está activa, el servicio HTTP de Traefik para ese recurso apunta a Pangolite; los recursos HTTP sin branding siguen pudiendo ir directos al backend.
- Pangolite inyecta el loader únicamente en navegaciones GET que devuelvan `200 text/html`, sin `Content-Disposition: attachment`, sin `Content-Range` y sin una codificación de contenido que no pueda transformar de forma segura.
- Para evitar respuestas `304` sin cuerpo durante una navegación branded se eliminan validadores condicionales hacia el backend y se solicita `Accept-Encoding: identity`.
- La transformación elimina `Content-Length`, `ETag`, `Last-Modified`, `Content-MD5`, `Digest` y `Accept-Ranges`, ya que dejarían de describir el cuerpo entregado, y fuerza revalidación de cache para que desactivar branding no deje HTML modificado considerado fresco.
- CSS y logo se sirven desde `<pathPrefix>/.pangolite/branding/`. La ruta queda reservada únicamente cuando branding está activo en ese recurso.
- El loader usa CSS puro y desaparece automáticamente; no se inyecta JavaScript.
- Si el CSP efectivo no permite hojas de estilo same-origin, Pangolite omite la inyección y conserva la página intacta; no añade `unsafe-inline`, no elimina CSP y no amplía políticas del backend.
- El parser de prefijo ignora comentarios y bloques `script/style` antes de localizar el `<body>`, evitando inyectar sobre cadenas que contengan texto parecido a etiquetas HTML.

## UI
Crear/editar recurso web incluye el switch **Mostrar loader Thowilabs** dentro de las opciones de aplicación. Está apagado inicialmente y se oculta/desactiva para redirect, TCP y UDP. La lista de recursos muestra una etiqueta `Thowilabs` cuando la función está activa.

## Compatibilidad
La actualización es automática: la migración v13 agrega una sola columna booleana y realiza backup pre-migración mediante el mecanismo existente. Todos los recursos previos reciben `0`, por lo que no hay cambio visual ni de routing hasta que el administrador active el switch de un recurso específico.


## Validación
- `gofmt`, `git diff --check`, `node --check` para los assets JavaScript y validación de sintaxis de instaladores pasan.
- Un harness Go aislado con dependencias únicamente de stdlib ejecuta el motor de branding y pasa pruebas de inyección, identidad/validadores, CSP estricto, CSP limitado por ruta, parser de `script` y exclusión de respuestas no documentales.
- La suite completa `go test ./...` queda para CI porque el repositorio exige Go 1.26 y este host tiene Go 1.23.2 sin acceso DNS para descargar toolchain/dependencias.
