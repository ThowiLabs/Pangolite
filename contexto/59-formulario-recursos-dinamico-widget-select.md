# 59 - Formulario dinámico de recursos y widget de selectores

## Objetivo
Separar visualmente los modos de creación/edición de recursos HTTP para evitar que opciones incompatibles se mezclen. En especial, una redirección permanente no debe mostrar cliente, backend, protección ni opción de devolver 404 cuando el backend/cliente no responde.

## Cambios
- Se agregó un selector de `Tipo de recurso web` con tres modos:
  - Aplicación local/directa.
  - Aplicación por cliente/agente.
  - Redirección permanente.
- El formulario ahora muestra/oculta secciones según el tipo elegido.
- En modo redirección permanente solo se muestran dominio origen, path, SSL, destino y código 301/308.
- En modo aplicación local/agente sí se muestran backend, protección y `Devolver 404 oculto`.
- Para TCP/UDP se mantiene selector de ubicación local/remota y cliente cuando aplique.
- Se agregó un widget reutilizable para selectores con apariencia similar al selector de proyecto.
- El selector de cliente muestra nombre, ID completo y estado online/offline con punto verde/rojo.
- El backend normaliza defensivamente recursos HTTP con redirect para limpiar `agentId`, protección y `hideWhenUnavailable` aunque un cliente mande esos campos manualmente.

## Archivos principales
- `internal/app/templates/pages/resource_create.html`
- `internal/app/templates/components/modal_resource_edit.html`
- `internal/app/assets/app/forms.js`
- `internal/app/assets/app/projects.js`
- `internal/app/assets/app/templates.js`
- `internal/app/assets/app/panel.css`
- `internal/app/model.go`

## Nota
No cambia el protocolo de `pangolite-client`; no requiere actualizar clientes remotos.
