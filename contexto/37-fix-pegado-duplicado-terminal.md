# Fecha
2026-06-28

# Objetivo
Corregir que al pegar texto con Ctrl+V dentro de la terminal web el contenido se enviara duplicado al shell remoto.

# Decisiones tomadas
- Mantener una sola ruta de pegado para Ctrl+V: la ruta nativa de xterm.js mediante `term.onData`.
- Eliminar el listener propio de `paste` sobre el contenedor de la terminal, porque interceptaba el mismo pegado que xterm.js ya procesaba.
- Conservar el pegado manual del menú contextual personalizado y Shift+Insert mediante `navigator.clipboard.readText()`.

# Arquitectura actual
La terminal web recibe entrada desde xterm.js y la envía al WebSocket mediante `term.onData(data => sendBytes(data))`.
El menú contextual personalizado usa `pasteFromClipboard()` para enviar texto cuando el usuario elige Pegar.
Ctrl+V queda a cargo de xterm.js para evitar doble envío.

# Librerías usadas
- xterm.js
- WebSocket nativo del navegador
- Clipboard API nativa del navegador

# Archivos importantes modificados
- `internal/app/assets/app/terminal.js`

# Problemas encontrados
`Ctrl+V` activaba dos caminos simultáneos:
1. El manejo nativo de pegado de xterm.js, que terminaba emitiendo datos por `onData`.
2. Un listener propio `box.addEventListener('paste', ...)` que también enviaba el mismo texto con `sendBytes()`.

Por eso una URL o comando pegado con Ctrl+V aparecía duplicado. El pegado desde el menú contextual no se duplicaba porque solo usaba la ruta manual.

# Soluciones implementadas
- Se retiró la instalación de `installTerminalClipboard(box)`.
- Se eliminó la función `installTerminalClipboard`.
- Se conserva `pasteFromClipboard()` para el menú contextual y Shift+Insert.
- Se eliminó la función auxiliar `pasteText()` porque quedó sin uso tras quitar el listener duplicado.

# Pendientes
- Probar en Chrome/Edge con Ctrl+V, Shift+Insert y menú contextual.
- Si se requiere soporte especial para navegadores sin pegado nativo de xterm.js, agregar un fallback condicionado, no un listener global duplicado.

# Próximos pasos
Ejecutar pruebas manuales pegando una URL larga y un comando con Ctrl+V para confirmar que se envía una sola vez.
