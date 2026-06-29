# 56 - Fix CSP en recursos HTTP publicados por cliente remoto

## Problema

Tras corregir que rutas como `/login`, `/api` o `/assets` de un recurso HTTP remoto no cayeran en el panel administrativo, el recurso ya respondía correctamente, pero el navegador bloqueaba assets externos de la aplicación publicada.

Ejemplo observado en `jyv.admvo.org/login`:

- `https://cdn.tailwindcss.com/` bloqueado por `Content-Security-Policy`.
- `https://fonts.googleapis.com/...` bloqueado por `Content-Security-Policy`.
- `https://cdnjs.cloudflare.com/...` bloqueado por `Content-Security-Policy`.
- `tailwind is not defined` porque el script CDN fue bloqueado.

La CSP que aparecía era la del panel de Pangolite:

```txt
default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; img-src 'self' data:; font-src 'self' https://cdn.jsdelivr.net data:; connect-src 'self' https://cdn.jsdelivr.net; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'
```

## Causa

`Handler()` envolvía todo con `securityHeaders()`:

```go
return securityHeaders(s.recoverRequests(s.logRequests(s.publicResourceGateway(s.mux))))
```

Eso significa que Pangolite agregaba headers del panel administrativo a absolutamente todas las respuestas, incluyendo recursos publicados mediante proxy/agente.

En recursos HTTP publicados, Pangolite debe comportarse como proxy transparente. No debe imponer CSP, `X-Frame-Options`, ni otros headers propios del panel sobre la aplicación remota.

## Solución

Se cambió el orden del middleware para que `publicResourceGateway()` se ejecute antes de `securityHeaders()`.

Ahora el flujo es:

```txt
recover -> log -> publicResourceGateway -> securityHeaders -> mux panel
```

Código final:

```go
func (s *Server) Handler() http.Handler {
	return s.recoverRequests(s.logRequests(s.publicResourceGateway(securityHeaders(s.mux))))
}
```

Con esto:

- Si el host pertenece a un recurso publicado, se atiende/proxy directamente y no recibe la CSP del panel.
- Si el host/ruta pertenece al panel, sí pasa por `securityHeaders()`.
- El panel conserva sus headers de seguridad.
- Las apps publicadas conservan sus propios headers.

## Prueba agregada

Se agregó `TestPublicAgentResourceDoesNotReceivePanelCSP` para validar que un recurso HTTP remoto por agente:

- recibe `/login` como job del agente,
- mantiene `TargetHost = 127.0.0.1`,
- mantiene `TargetPort = 8181`,
- responde con el HTML remoto,
- no hereda `Content-Security-Policy` del panel,
- no hereda `X-Frame-Options` del panel,
- conserva headers propios de la app remota.

## Nota importante

El valor `http://127.0.0.1:2424` en Traefik sigue siendo correcto para recursos por agente.

Ese puerto representa:

```txt
Traefik -> Pangolite :2424 -> agente remoto -> backendHost/backendPort configurados
```

No significa que Pangolite haya reemplazado el puerto del backend remoto.
