# 18 - Corrección de formularios del panel

Fecha: 2026-06-27

## Objetivo

Corregir el flujo de creación de proyectos y endurecer los formularios principales del panel para evitar que el navegador ejecute un submit HTML normal.

## Problema detectado

Al crear un proyecto, el formulario navegaba a `/projects?` y no creaba el registro. Eso indicaba que el `submit` del formulario no estaba interceptado por el JavaScript de la SPA y el navegador lo estaba tratando como un formulario GET normal.

## Decisiones tomadas

- Registrar manejadores explícitos para los formularios principales desde `setupForms()`.
- Forzar `action="javascript:void(0)"` como protección adicional contra submit HTML accidental.
- Crear proyectos mediante `POST /api/projects` con JSON y CSRF.
- Actualizar dashboard, tabla y selector de proyecto inmediatamente después de crear.
- Enviar al usuario al proyecto recién creado.
- Mantener modales de espera y mensajes visuales de éxito/error.

## Archivos modificados

- `internal/app/ui.go`
- `contexto/18-fix-formularios-proyecto.md`

## Validaciones

- `gofmt`
- `sh -n init.sh`
- `node --check` sobre el script principal del panel

## Pendientes

- Añadir pruebas E2E de flujos de UI cuando se defina una herramienta de navegador/headless para el proyecto.
