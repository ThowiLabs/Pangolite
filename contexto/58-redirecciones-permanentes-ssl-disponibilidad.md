# 58 - Redirecciones permanentes, SSL seguro y caída oculta

## Objetivo
Agregar soporte por recurso HTTP para redirecciones permanentes de dominio, manejo más seguro del switch SSL y una opción para ocultar caídas del backend/cliente remoto como 404.

## Implementado
- Nuevos campos en `Resource`:
  - `redirectEnabled`
  - `redirectTarget`
  - `redirectStatusCode` (`301` o `308`, por defecto `308`)
  - `hideWhenUnavailable`
- Migración SQLite v9 para agregar columnas sin romper bases existentes.
- Si un recurso HTTP tiene redirección permanente activa, Traefik lo enruta a Pangolite (`127.0.0.1:2424`) para que Pangolite responda el redirect.
- La redirección conserva path y query string cuando el destino es dominio o URL sin path específico.
- Si el destino incluye path específico, se usa esa URL como destino fijo.
- El redirect corre antes de protección o proxy al backend, pero sólo cuando el recurso está activo.
- Si el recurso está suspendido, conserva la respuesta de suspensión configurada.
- Si `hideWhenUnavailable` está activo:
  - error de backend local -> 404 oculto;
  - agente remoto offline/sin respuesta -> 404 oculto;
  - error devuelto por el agente -> 404 oculto.
- Si `hideWhenUnavailable` está desactivado, se conserva el comportamiento anterior (`502/503` con mensaje de proxy).
- El switch SSL ya no bloquea la creación/edición cuando ACME no está configurado: Pangolite lo desactiva automáticamente y avisa.
- Globalmente, antes de generar/aplicar configuración Traefik, si ACME no está disponible se apagan switches `tls` activos para evitar redirects HTTP->HTTPS rotos.
- Si se elimina/desactiva el correo ACME desde Ajustes y existen recursos HTTP con SSL activo, Pangolite ya no bloquea el guardado: guarda ajustes, apaga esos switches y devuelve warning.

## Consideraciones
- Esto no puede detectar de forma perfecta todos los fallos ACME futuros porque Traefik genera certificados de forma asíncrona al recibir tráfico. Se cubren casos seguros: ACME desconfigurado/no disponible y configuración inválida antes de renderizar.
- Para dominios detrás de Cloudflare proxy, si se quiere forzar el destino HTTPS en una redirección de dominio, conviene poner el destino como URL completa `https://nuevo-dominio.tld`.
- La redirección usa por defecto `308 Permanent Redirect` porque conserva método y body. También se permite `301` clásico.

## Pruebas sugeridas
- Crear recurso HTTP antiguo `old.example.com` con redirect hacia `https://new.example.com`.
- Verificar:
  - `GET /` -> `308 Location: https://new.example.com/`
  - `GET /login?x=1` -> `308 Location: https://new.example.com/login?x=1`
- Crear recurso con SSL marcado pero sin correo ACME y confirmar que se guarda con `tls=false` y muestra warning.
- Apagar cliente remoto con `hideWhenUnavailable=true` y confirmar que el dominio responde `404` oculto en lugar de `503 agente no disponible`.
