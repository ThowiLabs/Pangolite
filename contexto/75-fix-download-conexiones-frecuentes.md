# Fecha
2026-08-17

# Objetivo
Corregir dos regresiones operativas: `download ruta` podía llegar a bash como comando inexistente después de usar historial/cursores o pegar entrada, y la vista SSH no estaba consumiendo el historial real de aperturas para poblar conexiones frecuentes.

# Decisiones tomadas
- `download ruta` continúa siendo una pseudo-orden de Pangolite; no se instala ningún ejecutable ni alias global en el servidor remoto.
- Mantener un editor de línea sombra pequeño en el navegador para escritura, backspace, Home/End, Ctrl+A/E/U/K/W, pegado bracketed-paste y cursores conocidos.
- Cuando el historial del shell hace imposible conocer la línea desde el buffer sombra, usar la línea renderizada de xterm como fallback al presionar Enter antes de enviarlo al PTY.
- No interceptar pseudo-órdenes dentro del alternate buffer (vim/nano/full-screen TUI).
- Las conexiones frecuentes deben salir de la auditoría existente `terminal.open`, no de `localStorage`, para persistir entre navegadores y dispositivos.
- El ranking se calcula por nombre de usuario, cantidad de aperturas y última apertura; destinos eliminados se omiten y destinos offline se muestran sin botón de conexión activo.
- No añadir tablas ni servicios nuevos: se reutiliza `audit_events` y solo se agrega un índice compuesto para la consulta de ranking.

# Arquitectura actual
- Browser/xterm intercepta `download ruta`; el backend continúa preparando tickets autenticados y streaming/ZIP seguro como antes.
- `localTerminalSocket` y `agentTerminalSocket` ya registran `terminal.open` en `audit_events`.
- `/ssh` consulta hasta cinco destinos frecuentes mediante `ListFrequentTerminalTargets` y los resuelve contra el inventario actual de agentes/proyectos.

# Librerías usadas
- Sin dependencias nuevas.
- xterm.js y JavaScript nativo existentes.
- SQLite existente.

# Archivos importantes modificados
- `internal/app/assets/app/terminal.js`
- `internal/app/maintenance.go`
- `internal/app/store.go`
- `internal/app/ui.go`
- `internal/app/templates/pages/ssh_connections.html`
- `internal/app/assets/app/panel.css`
- `internal/app/store_test.go`
- `internal/app/ui_test.go`
- `README.md`
- `contexto/00-resumen-proyecto.md`

# Problemas encontrados
- El detector anterior de `download` invalidaba por completo el seguimiento de línea al recibir determinadas secuencias ANSI. Si el usuario recuperaba un comando con flecha arriba o editaba la línea con cursores, el siguiente Enter podía enviarlo directamente a bash, produciendo `download: command not found`.
- El registro `terminal.open` ya existía y se guardaba correctamente, pero la vista `/ssh` no tenía una consulta backend que transformara ese historial en un ranking persistente de destinos usados.

# Soluciones implementadas
- Seguimiento de línea con cursor y edición básica en `terminal.js`.
- Soporte de bracketed paste sin invalidar el pseudo-comando.
- Fallback sobre la línea visible de xterm para comandos recuperados mediante historial del shell.
- Consulta SQL agrupada por usuario/destino sobre `audit_events` e índice específico de bajo costo.
- Widget compacto de conexiones frecuentes con conteo, última apertura y botón icon-only consistente con la regla global de UI.
- Pruebas de ranking por usuario, resolución de destinos y marcadores de regresión del parser de terminal.

# Pendientes
- Ejecutar la suite Go completa con Go 1.26+ en un entorno con dependencias disponibles.
- Verificar visualmente en el VPS que el historial existente de `terminal.open` rellene el ranking inmediatamente.

# Próximos pasos
- Probar `download` escrito, pegado y recuperado con flecha arriba en terminal local y remota.
- Abrir varias veces dos o tres destinos y confirmar que `/ssh` los ordena por uso y luego por actividad reciente.
