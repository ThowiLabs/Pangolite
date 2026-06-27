# 21 - Fix respaldos: modal reutilizable, descarga y paleta de credenciales

## Motivo

Después de agregar auditoría y respaldos se detectaron detalles de UX y navegación:

- La creación de respaldos usaba `prompt()`, un componente nativo del navegador que no seguía la interfaz de Pangolite.
- Al presionar `Esc` o cancelar en el prompt, el respaldo se creaba de todos modos porque `null` se convertía a cadena vacía.
- El enlace de descarga de respaldos era interceptado por el router SPA y terminaba mandando al dashboard.
- El contenedor que muestra ID, token y comandos de instalación de un cliente de sistema usaba `alert-warning` de Bootstrap, generando un fondo color piel/beige que no combina con la paleta oscura y reduce legibilidad.

## Cambios aplicados

### Modal reutilizable para entradas sensibles o confirmadas

Se reutilizó el modal que ya existía para confirmar contraseña al eliminar clientes, pero se generalizó con `confirmInputAction()`.

Ahora el mismo modal soporta:

- Título dinámico.
- Texto descriptivo dinámico.
- Etiqueta del input dinámica.
- Tipo de input `password` o `text`.
- Placeholder.
- Botón primario o peligroso.
- Icono del botón.

`confirmPasswordAction()` quedó como wrapper especializado para contraseña, usado por la eliminación de clientes.

### Creación de respaldos

`createBackup()` ya no usa `prompt()`.

Ahora abre el modal Pangolite y pide:

- Prefijo opcional del respaldo.
- Botón `Crear respaldo`.

Si el usuario presiona `Esc`, clic fuera del modal o `Cancelar`, el resultado es `null` y no se llama a `POST /api/backups`.

Si el usuario confirma con el campo vacío, sí se crea un respaldo con nombre automático.

### Descarga de respaldos

Se corrigió el router SPA para no interceptar enlaces que:

- Tengan atributo `download`.
- Apunten a `/api/...`.
- Tengan `target`.

Además, el botón de descarga de respaldo ahora incluye `download`.

Esto permite que `/api/backups/{name}/download` sea atendido por el servidor y no por el router del dashboard.

### Paleta Pangolite para bloques reutilizables

Se agregaron clases utilitarias compatibles con la paleta actual:

- `pl-primary`
- `pl-secondary`
- `pl-secundary` como alias tolerante por el typo común.
- `pl-surface`
- `pl-success`
- `pl-warning`
- `pl-danger`
- `pl-callout`
- `pl-callout-primary`

Los contenedores de credenciales y comandos de cliente ahora usan:

```html
pl-callout pl-primary token-box
```

Esto elimina el fondo beige de Bootstrap y mantiene contraste con la interfaz oscura.

## Archivos modificados

- `internal/app/ui.go`
- `contexto/21-fix-respaldos-modal-descarga-paleta.md`

## Validaciones

Ejecutado:

```bash
gofmt -w internal/app/ui.go
sh -n init.sh
node --check sobre scripts embebidos en internal/app/ui.go
```

Pendiente en entorno con internet o `go.sum` presente:

```bash
go mod tidy
go test ./...
```

## Riesgos

- El modal reutilizable conserva los mismos IDs para no reestructurar toda la UI, pero ahora su comportamiento es más general. Debe cuidarse no abrir dos confirmaciones simultáneas.
- La descarga depende de la sesión autenticada; si la sesión expira, el endpoint puede redirigir según middleware.

## Commit sugerido

```text
Corregir respaldos y paleta de credenciales
```

Descripción:

```text
Se reemplaza el prompt nativo de respaldos por el modal reutilizable del panel, se evita crear respaldos al cancelar, se corrige la descarga para que no sea interceptada por el router SPA y se agrega una paleta utilitaria Pangolite para mejorar la legibilidad de los bloques de credenciales y comandos.
```
