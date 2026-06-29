# 55 - Fix recurso HTTP remoto interceptado por rutas del panel

## Problema

Al publicar un recurso HTTP/HTTPS usando un cliente de sistema remoto, Traefik enruta el dominio al puerto interno del panel Pangolite (`127.0.0.1:2424`) para que Pangolite pueda actuar como intermediario y reenviar la solicitud al agente remoto.

Eso es correcto para recursos remotos, suspendidos o protegidos. El problema era que Pangolite solo intentaba resolver el host público del recurso desde el catch-all `/`. Si la aplicación remota redirigía o cargaba rutas como `/login`, `/api`, `/assets`, `/password`, etc., el `ServeMux` del panel atendía esas rutas antes del catch-all y mostraba el acceso administrativo de Pangolite.

## Solución

Se agregó un middleware de entrada `publicResourceGateway` delante del mux principal. Ahora, antes de resolver rutas internas del panel, Pangolite verifica si el `Host` corresponde a un recurso HTTP que debe pasar por Pangolite:

- recurso remoto por cliente de sistema,
- recurso suspendido,
- recurso protegido por contraseña o sesión.

Si coincide, Pangolite atiende/proxyfica ese recurso para cualquier path, incluyendo `/login`, `/api` y `/assets`. Si no coincide, la petición continúa normalmente hacia el panel administrativo.

## Archivos modificados

- `internal/app/server.go`
- `internal/app/server_test.go`

## Nota sobre Traefik

Cuando el JSON dinámico muestra `url: "http://127.0.0.1:2424"` en un recurso remoto por cliente de sistema, no significa que el backend real sea `2424`. Significa que Traefik entrega la petición a Pangolite, y Pangolite usa los campos `backendHost`/`backendPort` del recurso para enviar el trabajo al cliente remoto.

Para una app remota en `127.0.0.1:8181` desde la perspectiva del cliente de sistema, el `backendPort` debe permanecer en `8181`; el `2424` solo aparece en Traefik como intermediario interno.

## Pruebas

Se agregó una prueba para confirmar que `/login` en un dominio de recurso público no renderiza el login administrativo.

En este entorno no se pudo ejecutar `go test ./internal/app` porque el proyecto requiere Go 1.24.0 y el contenedor no tiene internet para descargar el toolchain desde `proxy.golang.org`.
