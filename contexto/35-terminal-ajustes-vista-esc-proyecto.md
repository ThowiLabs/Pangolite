# 35 - Ajustes finales de terminal: vista, Esc y proyecto activo

# Fecha
2026-06-28

# Objetivo
Corregir detalles detectados antes del primer commit de la terminal remota: la acción de clic derecho "Reiniciar vista" dejaba la consola visualmente inutilizable, Escape en pantalla completa salía del modo fullscreen en vez de llegar a programas como vim, y al abrir Terminal desde Clientes de un proyecto se perdía el proyecto seleccionado en el sidebar.

# Decisiones tomadas
- No usar `term.reset()` desde el menú contextual porque reinicia el emulador xterm y puede dejar la pantalla sin prompt visible aunque el WebSocket siga activo.
- Cambiar la acción a "Reajustar vista": solo recalcula tamaño, manda resize al backend y regresa el foco a la terminal.
- En pantalla completa, `Esc` debe enviarse al PTY remoto cuando la terminal está conectada.
- Evitar depender del Fullscreen API nativo para el botón de la terminal; se usa pantalla completa interna por CSS para que el navegador no secuestre Escape.
- Mantener captura de `keydown` para enviar `\x1b` al WebSocket cuando la terminal está conectada y en pantalla completa interna.
- Preservar el proyecto activo al navegar a `/terminal` usando `projectId` en la URL y resolviendo también el proyecto desde `agentId` cuando aplique.

# Arquitectura actual
- `terminal.js` administra el estado visual de la consola y ahora separa "reajustar vista" de "reiniciar terminal".
- La pantalla completa del botón de terminal usa clase CSS fija en vez de Fullscreen API nativo; así Escape queda disponible para vim/nano/shell.
- La captura de Escape vive en el frontend porque el problema ocurre antes de llegar al WebSocket si el navegador interpreta la tecla como salida de fullscreen.
- `ui.go` resuelve el proyecto activo desde ruta `/projects/...`, desde query `projectId` en `/terminal`, o desde el `agentId` seleccionado.
- Los links de consola desde el listado de clientes incluyen `projectId` para mantener contexto del sidebar.

# Librerías usadas
- JavaScript nativo.
- Go standard library.
- No se agregaron dependencias nuevas.

# Archivos importantes modificados
- `internal/app/assets/app/terminal.js`
- `internal/app/assets/app/projects.js`
- `internal/app/assets/app/resources.js`
- `internal/app/assets/app/agents.js`
- `internal/app/templates/pages/terminal.html`
- `internal/app/templates/pages/agents.html`
- `internal/app/templates/layouts/panel.html`
- `internal/app/ui.go`
- `contexto/35-terminal-ajustes-vista-esc-proyecto.md`

# Problemas encontrados
- `term.reset()` no era una acción segura para una consola remota viva; reinicia modos internos del emulador y puede no repintar el prompt del shell remoto.
- Escape en fullscreen es una tecla especial del navegador; sin Keyboard Lock puede salir de pantalla completa antes de llegar a xterm/vim.
- `/terminal?agentId=...` no llevaba el proyecto, por lo que `panelData` no marcaba `CurrentID` y el sidebar volvía a estado sin proyecto activo.

# Soluciones implementadas
- Menú contextual: `resetTerminalView()` reemplaza a `term.reset()` y solo hace fit/resize/focus.
- Pantalla completa: se usa modo interno por CSS para no salir del fullscreen al presionar Escape.
- Pantalla completa conectada: Escape se intercepta y se envía como `\x1b` al WebSocket.
- Proyecto activo: links a terminal agregan `projectId`; el backend también puede inferir el proyecto desde `agentId` si falta `projectId`.

# Pendientes
- Probar Escape en Chrome/Edge con la pantalla completa interna de la terminal usando `vim`, `nano` y shell normal.
- Validar que desde Clientes -> Consola el sidebar conserve el proyecto activo tanto en render inicial como después de recargar la página.

# Próximos pasos
- Ejecutar `go test ./...`.
- Probar manualmente el menú contextual, fullscreen + vim y navegación desde Clientes de proyecto hacia Terminal.
- Si todo queda bien, hacer un solo commit con toda la implementación de terminal.
