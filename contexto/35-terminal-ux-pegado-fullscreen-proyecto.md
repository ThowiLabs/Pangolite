# 35 - Terminal: pegado, fullscreen y contexto de proyecto

# Fecha
2026-06-28

# Objetivo
Corregir detalles pendientes de la consola web antes de hacer commit: pegado desde clic derecho, acción de reiniciar vista, tecla Esc en modo pantalla completa y preservación del proyecto activo al entrar a la terminal desde Clientes.

# Decisiones tomadas
- No usar la API nativa Fullscreen del navegador para la terminal. En navegadores, Esc puede sacar al usuario del fullscreen nativo por seguridad y no es confiable bloquearlo para enviarlo a programas como vim. Se usa fullscreen visual por CSS (`terminal-fullscreen-fallback`) para que Esc pueda enviarse al terminal.
- El menú contextual mantiene Copiar/Pegar, pero Pegar ahora tiene fallback visible si el navegador bloquea `navigator.clipboard.readText()` por permisos o contexto no seguro.
- La acción "Reiniciar vista" se reemplaza por "Reajustar vista". Ya no ejecuta `term.reset()` porque eso borra estado visual de xterm y puede dejar solo el cursor parpadeando aunque la sesión siga viva.
- La ruta `/terminal` conserva contexto de proyecto con `projectId` en query. Si no viene `projectId`, intenta resolverlo desde `agentId`.
- Desde `/projects/{id}/agents`, el botón Consola abre `/terminal?projectId={id}&agentId={agentId}` para no deseleccionar el proyecto.

# Arquitectura actual
- `terminal.js` controla overlay inicial, conexión, reconexión, menú contextual, fullscreen CSS y fallback de pegado.
- El backend determina el proyecto actual desde path, query `projectId` o query `agentId` mediante `projectIDFromRequest`.
- La plantilla del layout conserva el proyecto activo en el enlace global de Terminal si existe `.HasProject`.
- La página de terminal filtra agentes por proyecto cuando existe contexto de proyecto; si se abre terminal global sin proyecto, lista todos los agentes.

# Librerías usadas
- JavaScript nativo.
- Go standard library.
- No se agregaron dependencias nuevas.

# Archivos importantes modificados
- `internal/app/assets/app/terminal.js`
- `internal/app/assets/app/panel.css`
- `internal/app/assets/app/agents.js`
- `internal/app/templates/pages/terminal.html`
- `internal/app/templates/pages/agents.html`
- `internal/app/templates/layouts/panel.html`
- `internal/app/ui.go`
- `internal/app/ui_test.go`
- `contexto/35-terminal-ux-pegado-fullscreen-proyecto.md`

# Problemas encontrados
- El botón Pegar del menú contextual dependía solo de `navigator.clipboard.readText()`, que puede fallar sin permiso, sin HTTPS o por política del navegador.
- `term.reset()` reiniciaba el estado de xterm y podía dejar la terminal sin pantalla útil, solo con cursor.
- La API fullscreen nativa del navegador captura Esc para salir del modo pantalla completa, lo cual choca con programas interactivos como vim.
- `/terminal?agentId=...` no contenía `projectId`; el backend calculaba `CurrentID` solo desde el path, así que el sidebar perdía el proyecto activo.

# Soluciones implementadas
- Pegar intenta primero Clipboard API; si falla, muestra un panel interno para pegar con Ctrl+V y enviar el texto al WebSocket.
- Reajustar vista solo recalcula tamaño (`fit`) y envía resize; no borra la sesión ni el buffer.
- Pantalla completa usa clase CSS en vez de fullscreen nativo; Esc en ese modo se captura y se envía como `\x1b` a la terminal si la sesión está conectada.
- `projectIDFromRequest` preserva proyecto desde path, query `projectId` o el `projectId` del agente indicado por `agentId`.
- En la UI de Clientes, el enlace a consola incluye `projectId`.
- Se agregó prueba unitaria para validar que `/terminal` conserva proyecto por `projectId` y por `agentId`.

# Pendientes
- Probar manualmente en navegador real: clic derecho → Pegar con permiso concedido, y fallback pegando con Ctrl+V dentro del panel.
- Probar vim/nano en pantalla completa CSS: Esc debe llegar al programa y no debe cerrar la vista.
- Probar abrir consola desde Clientes de proyecto y confirmar que el sidebar mantiene el proyecto activo.

# Próximos pasos
- Ejecutar `go test ./...` con Go 1.24+ o el Go temporal del instalador.
- Compilar servidor y cliente Linux.
- Validar flujo completo antes de hacer commit del bloque terminal.
