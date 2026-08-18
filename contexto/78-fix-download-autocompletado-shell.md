# Fecha
2026-08-17

# Objetivo
Corregir la pseudo-orden `download ruta` cuando Bash/readline modifica la línea mediante autocompletado (`Tab`), historial u otras ediciones administradas por el shell.

# Decisiones tomadas
- `download` continúa siendo una pseudo-orden exclusiva de Pangolite; no se instala ningún comando, alias ni función permanente en el servidor.
- El buffer sombra del navegador solo es autoridad mientras la línea depende exclusivamente de entrada que Pangolite puede modelar de forma determinista.
- `Tab` invalida inmediatamente ese buffer porque readline puede completar, expandir o reescribir el texto dentro del shell.
- Después de una edición administrada por el shell, Pangolite usa como fuente de verdad la línea lógica completa renderizada por xterm.
- No usar una espera fija dependiente de latencia. La línea renderizada debe quedar estable antes de decidir si interceptar `download` o entregar Enter al PTY, con un tiempo máximo acotado para no bloquear la terminal.
- Alternate buffer (vim, nano y otras TUI) sigue excluido de la interceptación.

# Arquitectura actual
- Entrada normal: xterm -> buffer sombra -> detección de `download` -> PTY si no coincide.
- Entrada modificada por shell: xterm -> readline/Bash -> línea renderizada estable -> detección de `download` -> PTY si no coincide.
- `terminalRenderedCommandLine` reconstruye toda la línea lógica incluyendo filas envueltas, no solamente el texto a la izquierda del cursor.
- El backend de tickets/streaming/ZIP no cambia.

# Librerías usadas
- Sin dependencias nuevas.
- JavaScript nativo y xterm.js existentes.

# Archivos importantes modificados
- `internal/app/assets/app/terminal.js`
- `internal/app/ui_test.go`
- `README.md`
- `contexto/00-resumen-proyecto.md`

# Problemas encontrados
- `Tab` se trataba como texto rastreable. Si el usuario escribía `download li` y Bash lo completaba visualmente a `download lilith-mcp/`, Pangolite conservaba `download li` en su buffer sombra y solicitaba `/root/li`.
- Para historial/ediciones no rastreables se utilizaba una espera fija de 35 ms antes de consultar xterm. Con latencia suficiente, Enter podía enviarse antes de que la línea renderizada reflejara `download`, haciendo que Bash respondiera `download: command not found`.
- La lectura de xterm terminaba en la posición del cursor; después de ciertas ediciones podía omitir texto válido situado a la derecha o en una fila envuelta.

# Soluciones implementadas
- `Tab` invalida el buffer sombra y delega la resolución posterior a la línea real del shell.
- Se registra el momento de la última edición administrada por shell y se espera estabilidad visual de xterm en lugar de dormir 35 ms de forma fija.
- La sincronización tiene límite máximo de 900 ms; si no hay pseudo-orden se envía Enter normalmente y la terminal no queda bloqueada.
- La reconstrucción de línea recorre todas las filas pertenecientes a la línea lógica envuelta.
- Se añadieron marcadores de regresión en `ui_test.go` y un harness de ejecución del JavaScript real reprodujo autocompletado con latencia, historial y líneas envueltas.

# Pendientes
- Ejecutar `go test ./...` en un entorno con Go 1.26+ y acceso a los módulos. El entorno de trabajo actual intenta descargar Go 1.26 pero su DNS de salida está bloqueado.

# Próximos pasos
- Probar en navegador real: `download li<Tab>`, `download` recuperado con flecha arriba, ruta con espacios y comando normal después de usar historial.
