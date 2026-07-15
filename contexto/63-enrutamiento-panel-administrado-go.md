# 63 - Enrutamiento del panel administrado por Go

## Problema
El panel mantenía dos fuentes de verdad para la navegación:

- Go seleccionaba una plantilla según la URL.
- `agents.js` volvía a interpretar `location.pathname`, cambiaba encabezados y redirigía rutas desconocidas a `/projects`.

Esto provocó la redirección incorrecta de `/ssh` y hacía posible que una ruta nueva funcionara en el backend pero fuera rechazada por el frontend. Además, Go convertía cualquier URL desconocida en la página de proyectos en lugar de devolver `404`.

## Solución aplicada
- Las páginas administrativas se registran explícitamente en `internal/app/panel_routes.go` usando patrones de `http.ServeMux`.
- Cada ruta entrega directamente su `panelPage`, plantilla, `PageKey`, título y breadcrumb.
- La autenticación y el cambio obligatorio de contraseña se validan en Go antes de renderizar.
- Las rutas con proyecto validan el ID o slug en Go; un proyecto inexistente responde `404`.
- `/terminal` valida en Go el proyecto, el cliente solicitado y combinaciones de parámetros incompatibles.
- Las rutas desconocidas ya no cargan `/projects`; responden `404`.
- Las rutas API desconocidas conservan respuesta JSON `404`.
- Se eliminó por completo `route()` de `agents.js`.
- El frontend ya no inspecciona `location.pathname` ni decide redirecciones.
- `page-init.js` solo hidrata la plantilla que Go ya autorizó, usando `pageKey` y `currentProject` emitidos por el servidor.

## Equivalencia conceptual con Laravel
- `panelRouteDefinitions` equivale al archivo `routes/web.php`.
- `panelRouteHandler` equivale al grupo de middleware `auth` y validación de contraseña temporal.
- `validateProjectPanelRoute` equivale a route model binding con `findOrFail`.
- `panelData` y `renderUIPage` equivalen al controlador que devuelve una vista con datos.
- `page-init.js` equivale al JavaScript específico de la vista, no a un router SPA.

## Archivos principales
- `internal/app/panel_routes.go`
- `internal/app/server.go`
- `internal/app/ui.go`
- `internal/app/assets/app/agents.js`
- `internal/app/assets/app/page-init.js`
- `internal/app/assets/app/settings.js`
- `internal/app/templates/layouts/panel.html`
- `internal/app/ui_test.go`
