# Fecha
2026-08-13

# Objetivo
Evitar que el historial visual de transferencias modifique el alto de la terminal web y hacer que las cargas masivas sean legibles sin reducir el espacio de xterm.

# Decisiones tomadas
- El progreso de transferencia deja de formar parte del flujo flex de `terminal-body-wrap` y se muestra en una ventana `position: fixed`.
- Solo se representa el archivo activo; un lote reutiliza la misma ventana en vez de crear una fila persistente por archivo.
- La ventana muestra porcentaje, bytes, nombre y posición dentro del lote (`N de M`).
- Cada archivo completado o fallido genera un aviso flotante temporal y no bloqueante.
- Los avisos visibles se limitan a tres para evitar crecimiento del DOM durante lotes grandes.
- Al terminar un lote se muestra un resumen temporal y la ventana de progreso desaparece.
- Las transferencias ya no llaman a `fitTerminal()`/`queueResize()` al crear o eliminar progreso, porque esa UI no debe afectar las dimensiones del PTY.

# Arquitectura actual
La transferencia conserva el protocolo binario y la cola secuencial existentes. `#terminalTransfers` y `#terminalTransferAlerts` viven dentro de la tarjeta de terminal para seguir visibles también en fullscreen, pero ambos usan posicionamiento fijo y no participan en el cálculo de alto de xterm.

# Librerías usadas
- JavaScript nativo.
- CSS nativo.
- Bootstrap Icons ya existente.
- Sin dependencias nuevas.

# Archivos importantes modificados
- `internal/app/templates/pages/terminal.html`
- `internal/app/assets/app/terminal.js`
- `internal/app/assets/app/panel.css`
- `internal/app/ui_test.go`
- `README.md`
- `contexto/00-resumen-proyecto.md`
- `contexto/70-progreso-flotante-transferencias-terminal.md`
- `tareas/completado-70-progreso-transferencias-terminal.md`

# Problemas encontrados
El contenedor `terminalTransfers` era un hijo normal de `terminal-body-wrap`. Cada transferencia añadía una fila con alto propio y disparaba `fitTerminal()`, por lo que xterm se reajustaba repetidamente y la experiencia daba la impresión de que la terminal crecía conforme se subían archivos.

# Soluciones implementadas
- Ventana flotante reutilizable para el archivo activo.
- Indicador de posición del lote.
- Avisos de éxito/error autocerrables y acotados a tres elementos.
- Resumen al finalizar lotes de más de un archivo.
- Eliminación de los reajustes de xterm causados exclusivamente por la UI de progreso.

# Pendientes
- Validar visualmente la posición de avisos y progreso en fullscreen y Android real.

# Próximos pasos
- Ejecutar la suite Go completa con Go 1.26.
- Probar un lote de decenas/cientos de archivos y confirmar que el alto de la terminal permanece constante.
