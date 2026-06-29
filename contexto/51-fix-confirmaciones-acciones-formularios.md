# 51. Fix de confirmaciones en acciones y formularios

## Problema

En producción, al intentar eliminar un recurso desde la tabla de Recursos, el navegador arrojaba:

```text
ReferenceError: confirmAction is not defined
```

El flujo afectado era `deleteResource(...)` en `forms.js`, que depende del helper global `confirmAction(...)` para abrir el modal de confirmación.

## Causa

Aunque `confirmAction` vive en `modals.js`, las acciones críticas del panel dependían directamente de que ese archivo estuviera cargado y expusiera sus helpers globales sin fallback. Si el helper no quedaba disponible en el scope global, las acciones con confirmación fallaban antes de llamar al backend.

## Cambios

- Se agregaron fallbacks seguros en `core.js` para helpers globales críticos:
  - `confirmAction`
  - `showNotice`
  - `showBusy`
  - `hideBusy`
  - `confirmInputAction`
  - `confirmPasswordAction`
- Los fallbacks usan los modales del panel si existen y, si por alguna razón no están disponibles, degradan a `window.confirm`, `window.alert` o `window.prompt`.
- `modals.js` ahora expone explícitamente sus helpers principales en `window` para evitar problemas con handlers inline y acciones generadas dinámicamente.
- Se revisaron referencias inline de botones/formularios contra funciones JS disponibles.
- `checkResourceHealth` ahora acepta el botón como argumento opcional y usa `withActionLoading` para mostrar estado de carga al probar health de recursos.

## Resultado

Las acciones de eliminación, suspensión, activación, rotación, configuración y formularios dejan de depender frágilmente del orden/scope de `modals.js`. El modal visual sigue siendo el camino normal, pero existe un fallback seguro para evitar romper acciones críticas en producción.
