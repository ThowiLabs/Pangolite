# 61 - Directorio global de Conexiones SSH

## Objetivo
Agregar un acceso global y visual para abrir la terminal del servidor Pangolite o de cualquier cliente de sistema sin tener que entrar primero al proyecto y después al listado de clientes.

## Experiencia implementada
- Nueva ruta administrativa `/ssh` y entrada **Conexiones SSH** en el sidebar.
- El servidor Pangolite aparece como la primera tarjeta del directorio.
- Los demás destinos son todos los clientes registrados, sin importar a qué proyecto pertenecen.
- Cada cliente muestra una etiqueta con el nombre de su proyecto para identificarlo rápidamente.
- Las tarjetas incluyen estado, sistema/arquitectura, hostname, dirección y última actividad.
- Los clientes offline, inactivos o Windows aparecen visibles, pero con la conexión deshabilitada y un estado explicativo.
- Cada tarjeta disponible abre `/terminal` con el destino ya seleccionado y conexión automática mediante `autoconnect=1`.
- La terminal incluye un botón para regresar al directorio global.

## Búsqueda y paginación
- Búsqueda instantánea por nombre, proyecto, slug, hostname, IP, sistema, arquitectura o ID del cliente.
- Normalización de mayúsculas, minúsculas y acentos para mejorar coincidencias.
- Paginación cliente con tamaños de 9, 18 o 30 conexiones por página.
- Controles anterior/siguiente y páginas compactadas con puntos suspensivos.
- El estado de búsqueda, página y tamaño se conserva en la URL.
- Estado vacío con acción directa para limpiar la búsqueda.

## Responsive y diseño
- Grid de tres columnas en escritorio, dos en pantallas medianas y una en móvil.
- Tarjeta local diferenciada visualmente como servidor principal.
- Resumen superior con destinos totales, conexiones disponibles y clientes registrados.
- Buscador, paginación y acciones adaptables a móvil.

## Seguridad y comportamiento
- La ruta reutiliza autenticación administrativa y bloqueo por contraseña temporal.
- No se agregaron credenciales SSH ni un protocolo nuevo: se reutiliza el canal de terminal WebSocket/PTY existente.
- Solo se considera disponible un cliente habilitado, online y con sistema compatible.
- La terminal local y remota Windows siguen deshabilitadas por la limitación ya documentada.
- Si un cliente deja de estar disponible entre el clic y la carga de la consola, la terminal muestra un aviso y nunca conecta accidentalmente al servidor local.

## Archivos modificados
- `README.md`
- `internal/app/ui.go`
- `internal/app/ui_test.go`
- `internal/app/templates/layouts/panel.html`
- `internal/app/templates/pages/terminal.html`
- `internal/app/templates/pages/ssh_connections.html`
- `internal/app/assets/app/terminal.js`
- `internal/app/assets/app/ssh-connections.js`
- `internal/app/assets/app/panel.css`
- `contexto/61-directorio-global-conexiones-ssh.md`

## Validación pendiente
Ejecutar en el equipo del proyecto, con Go 1.24 y dependencias disponibles:

```bash
go test ./...
```
