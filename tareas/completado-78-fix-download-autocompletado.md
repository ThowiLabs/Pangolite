# Tarea 78 — Corregir download con autocompletado e historial

## Objetivo
Evitar que la pseudo-orden `download ruta` use una ruta parcial o termine llegando a bash cuando la línea fue modificada por autocompletado (`Tab`), historial u otras ediciones administradas por el shell.

## Criterios
- `download li<Tab>` debe usar la línea real renderizada por xterm, no el buffer sombra previo a la expansión del shell.
- No usar una espera fija corta dependiente de latencia.
- Conservar pegado, edición normal, historial, terminal local/remota y alternate buffer.
- Añadir regresiones y actualizar contexto.

## Resultado
Completado y validado con un harness que ejecuta el JavaScript real de `terminal.js`: autocompletado retrasado por 140 ms resolvió `lilith-mcp/`, historial retrasado por 150 ms fue interceptado, comandos normales conservaron Enter y líneas envueltas se reconstruyeron completas.
