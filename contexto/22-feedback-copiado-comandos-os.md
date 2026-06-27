# 22 - Feedback de copiado y comandos por sistema operativo

## Motivo

Después de agregar auditoría, respaldos y la paleta visual, se detectaron dos detalles de UX en el panel:

1. En los bloques `code`, al copiar un comando se mostraba el alert global superior. Eso provocaba que la vista se desplazara hacia arriba y hacía incómodo copiar varios valores/comandos seguidos.
2. En `projects/{id}/agents/create`, al crear un cliente de sistema se mostraban juntos los comandos de Linux y Windows. Eso podía confundir al usuario final, especialmente cuando el servidor remoto solo usa un sistema operativo.

## Cambios realizados

### Copiado sin salto de pantalla

- El botón `Copiar` ya no usa el alert global para confirmar la acción.
- Al copiar correctamente, el mismo botón cambia temporalmente a `Copiado` con palomita.
- Si el navegador no permite copiar automáticamente, el botón cambia temporalmente a `Error`.
- Se conserva la posición de scroll antes y después del copiado.
- El fallback con `textarea` oculto usa `preventScroll` y restaura el foco previo cuando es posible.

### Selector de sistema operativo en creación de cliente

- Se agregó un selector `Sistema operativo del servidor` en `agents/create`.
- Opciones actuales:
  - Linux / systemd / OpenRC
  - Windows / PowerShell
- El sistema elegido define qué comandos se muestran después de crear el cliente.
- El panel de credenciales también incluye un selector para cambiar entre comandos Linux/Windows después de generar o rotar token.
- ID y token siempre se muestran, pero los comandos se filtran por sistema operativo.

## Archivos modificados

- `internal/app/ui.go`
- `contexto/22-feedback-copiado-comandos-os.md`

## Validaciones esperadas

Probar manualmente:

1. Crear cliente de sistema con Linux seleccionado.
2. Confirmar que solo aparecen comandos Linux.
3. Cambiar el selector dentro del panel de credenciales a Windows.
4. Confirmar que aparecen comandos Windows.
5. Copiar ID, token y comandos.
6. Confirmar que la página no se desplaza arriba.
7. Confirmar que el botón cambia temporalmente a `Copiado`.

## Nota

`copyLogs()` conserva el alert global porque no corresponde a bloques `code` ni comandos operativos; es una acción general de la vista de logs.
