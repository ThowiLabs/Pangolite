# 57 - Hardening del proxy HTTP por agente

## Problema observado

Después de corregir el enrutamiento de dominios públicos y evitar que los recursos publicados heredaran la CSP del panel, las aplicaciones remotas cargaban sus assets, pero los flujos de login podían regresar a la misma página al enviar el formulario.

La causa no debía tratarse como un arreglo puntual de `/login`, porque el túnel HTTP por agente debe funcionar como reverse proxy para cualquier método y flujo web moderno: GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, cookies, CSRF, redirecciones y headers `X-Forwarded-*`.

## Hallazgos

- El recurso HTTP por agente conserva método, path, query y body, pero el agente usaba `http.Client` normal para hablar con el backend local.
- `http.Client` sigue redirecciones automáticamente. En aplicaciones como Laravel, un `POST /login` exitoso suele responder `302` con `Set-Cookie`. Si el agente sigue ese redirect internamente, el navegador no recibe el `302` original ni la cookie en el momento correcto.
- El backend local recibía el request contra `127.0.0.1:puerto`; sin conservar `Host` público y `X-Forwarded-Proto`, frameworks que dependen de URL pública, sesión segura, CSRF o trusted proxies pueden comportarse mal.
- El filtro de headers eliminaba `X-CSRF-Token`, que puede ser un header de la app publicada y no de Pangolite.
- El filtrado de hop-by-hop headers no consideraba los nombres dinámicos declarados en `Connection`.

## Cambios aplicados

- El gateway de recursos públicos se mantiene antes del mux del panel para que rutas como `/login`, `/api`, `/assets` o cualquier ruta de la app publicada no caigan en el panel.
- Los recursos publicados ya no heredan `Content-Security-Policy` ni otros headers del panel.
- `AgentJob` ahora transporta `PublicScheme` y `PublicHost`.
- El servidor envía al agente los headers correctos:
  - `Host` público lógico mediante `PublicHost`.
  - `X-Forwarded-Host`.
  - `X-Forwarded-Proto`.
  - `X-Forwarded-Port`.
  - `X-Real-IP`.
  - `X-Forwarded-For` si no existía previamente.
  - `Forwarded` si no existía previamente.
- El agente ya no sigue redirects del backend cuando ejecuta jobs HTTP; devuelve el `302/301/303/307/308` al navegador real.
- Se conserva `X-CSRF-Token` y `X-XSRF-Token`.
- Se filtran únicamente headers internos `X-Pangolite-*` y cookies internas `pangolite_resource_*`/`pangolite_session`.
- Se reescriben `Location` absolutos que apunten al backend interno para devolverlos con el dominio público.
- Se normalizan cookies con `Domain=127.0.0.1`, `Domain=localhost` o dominio interno del backend, eliminando ese atributo para que el navegador las acepte en el dominio público.
- Se filtran headers hop-by-hop fijos y dinámicos declarados en `Connection`.

## Validaciones agregadas

- Un recurso público por agente en `/login` no hereda CSP del panel.
- `POST /login` conserva body, cookies de Laravel y headers CSRF.
- `POST /login` conserva host/proto/port públicos.
- Redirecciones absolutas internas se reescriben al dominio público.
- `Set-Cookie` con dominio interno se normaliza.
- `DELETE` se proxy-pasa correctamente.
- El agente no sigue redirects del backend.
- `PUT` conserva método, body, `Content-Type`, `Host` público y headers forwarded.
- Headers hop-by-hop dinámicos se eliminan.

## Nota operativa

Este cambio modifica tanto el servidor como el cliente/agente. Al desplegarlo, hay que reconstruir Pangolite y también actualizar el binario `pangolite-client` en las máquinas remotas que publican recursos HTTP por agente.
