# 52 - Scrollbar unificado en terminal web

## Motivo

En la consola web de xterm.js aparecía una barra de desplazamiento interna con el estilo por defecto del navegador, especialmente visible en Windows/Chrome como una barra clara dentro del área negra de la terminal.

La barra existe porque xterm.js crea un `viewport` interno para manejar el historial de scrollback de la terminal. Aunque la página ya tenga su propio scroll global, la terminal necesita su propio contenedor de desplazamiento para poder revisar la salida previa sin mover todo el panel.

## Cambio

Se centralizaron variables visuales para barras de desplazamiento internas del panel:

- `--pg-scrollbar-size`
- `--pg-scrollbar-track`
- `--pg-scrollbar-thumb`
- `--pg-scrollbar-thumb-hover`
- `--pg-scrollbar-border`

La terminal ahora aplica el mismo estilo oscuro/redondeado al contenedor `.xterm-viewport`, evitando que se vea la barra blanca nativa.

También se reutilizan las mismas variables en modales con scroll interno y en el cuerpo del modal de credenciales, para mantener consistencia visual.

## Alcance

- No se modifica el scroll global del contenido central.
- No se elimina el scrollback de xterm.js.
- No se cambia la lógica de conexión, resize, pantalla completa ni WebSocket.

## Archivos

- `internal/app/assets/app/panel.css`
