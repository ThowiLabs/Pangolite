# 36 - Fix pegado duplicado en terminal

# Fecha
2026-06-28

# Objetivo
Corregir el pegado de la consola web antes de hacer commit: `Ctrl+V` estaba enviando el portapapeles duplicado y el modal de pegado no retenía correctamente el texto porque el evento terminaba siendo capturado por la terminal.

# Decisiones tomadas
- El pegado directo de la terminal debe procesarse una sola vez en el listener propio del contenedor.
- Cuando el panel/modal de pegado está abierto, los eventos `paste` dentro del textarea pertenecen al modal, no a xterm ni al WebSocket.
- El modal ya no envía automáticamente al pegar; primero deja el texto en el textarea y el usuario confirma con `Enviar` o `Ctrl+Enter`.
- Se mantiene el fallback del modal porque en HTTP o contextos sin permiso el navegador puede bloquear la lectura directa del portapapeles.

# Arquitectura actual
- `terminal.js` maneja el pegado directo con `installTerminalClipboard` en fase capture.
- El listener hace `preventDefault`, `stopPropagation` y `stopImmediatePropagation` para evitar que xterm procese el mismo `paste` una segunda vez.
- `openPasteCapture` crea el panel de pegado y protege su textarea para que el contenido se pegue ahí y no en la consola.

# Librerías usadas
- JavaScript nativo.
- No se agregaron dependencias.

# Archivos importantes modificados
- `internal/app/assets/app/terminal.js`
- `contexto/36-fix-pegado-terminal-duplicado.md`

# Problemas encontrados
- `Ctrl+V` podía duplicar el contenido porque el listener propio enviaba el texto y el manejador interno de xterm también podía procesar el mismo evento.
- El fallback de pegado estaba dentro del mismo contenedor de la terminal; por eso el listener global del contenedor podía interceptar el `paste` del textarea del modal.
- El modal autoenviaba al pegar, lo cual hacía que el usuario no pudiera revisar o editar el texto antes de enviarlo.

# Soluciones implementadas
- Si el evento `paste` ocurre dentro del modal de pegado, `installTerminalClipboard` lo ignora.
- El evento de pegado directo en la terminal detiene propagación normal e inmediata después de enviar una sola vez.
- El textarea del modal detiene la propagación de su propio evento `paste`.
- Se eliminó el autoenvío al pegar dentro del modal; ahora se requiere `Enviar` o `Ctrl+Enter`.

# Pendientes
- Probar en navegador real con `Ctrl+V` directo en terminal: debe enviarse una sola vez.
- Probar clic derecho → Pegar en HTTP: debe abrir modal; `Ctrl+V` debe llenar el textarea, no enviar a terminal.
- Probar botón `Enviar` y `Ctrl+Enter` desde el modal.

# Próximos pasos
- Ejecutar `node --check internal/app/assets/app/terminal.js`.
- Ejecutar `go test ./...` en entorno con Go 1.24+.
- Validar manualmente desde `/terminal?agentId=...` antes de commitear el bloque de terminal.
