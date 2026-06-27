# 28. Refactor frontend a templates y assets

## Objetivo

Separar el frontend del archivo `internal/app/ui.go`, que había crecido demasiado por contener HTML, CSS y JavaScript embebidos como strings Go.

## Decisión

Se movió la interfaz a archivos físicos embebidos con `embed.FS`, manteniendo la arquitectura simple sin agregar Node, Vite, React ni dependencias nuevas.

Estructura nueva:

```txt
internal/app/templates/
  layouts/
    auth.html
    panel.html
  components/
    auth_brand.html
    footer.html
  pages/
    login.html
    password.html
    panel.html

internal/app/assets/app/
  auth.css
  panel.css
  login.js
  password.js
  app.js
```

## Navegación

Se quitó la navegación SPA que interceptaba enlaces internos con `history.pushState`. Ahora los enlaces usan rutas normales del navegador:

```txt
/projects
/projects/{id}
/projects/{id}/resources
/projects/{id}/resources/create
/projects/{id}/agents
/projects/{id}/agents/create
/settings
/logs
/maintenance
```

El servidor sigue entregando el shell protegido para esas rutas y el JS solo hidrata la vista correspondiente según `location.pathname`.

## Motivo Ponytail

No se agregó framework frontend ni toolchain de build. La separación se hizo con herramientas nativas de Go, HTML, CSS y JavaScript simple.

Esto reduce el tamaño de `ui.go`, facilita editar layout/componentes y mantiene binarios autocontenidos.

## Límite conocido

El panel aún conserva renderizado dinámico en `app.js` para tablas y formularios que dependen de APIs. No se migró todo a SSR completo porque implicaría reescribir de golpe flujos grandes y aumentaría riesgo.

ponytail: renderizado híbrido servidor + JS, convertir vistas puntuales a SSR cuando ya estén estables y convenga reducir más JavaScript.
