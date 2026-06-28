# 31 - Fix real de Backspace en terminal Windows con ConPTY

> Nota 2026-06-28: esta decisión quedó supersedida por `34-terminal-ux-linux-root-windows-aviso.md`. Windows se mantiene con aviso claro/no confiable y ya no se intenta abrir ConPTY desde el cliente hasta tener una implementación estable.


## Fecha
2026-06-27

## Problema

La terminal remota Windows funcionaba con PowerShell conectado por `stdin/stdout` de proceso. Ese modo no crea una consola interactiva real.

Síntomas observados:

- Backspace solo movía el cursor visualmente.
- El carácter anterior seguía existiendo en el buffer de PowerShell.
- Después de algunas entradas la sesión podía cerrarse.
- El workaround de convertir `DEL` (`0x7f`) a `Ctrl+H` (`0x08`) no era suficiente porque seguíamos usando pipes crudos.

## Causa raíz

PowerShell por pipes no equivale a una TTY/PTY. Las teclas interactivas como Backspace, flechas, historial, edición de línea y resize necesitan una consola real o una pseudoconsola.

En Windows moderno la solución correcta es ConPTY, usando `CreatePseudoConsole` y asociando el proceso hijo con `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE`.

## Solución aplicada

Se reemplazó la implementación Windows de terminal por una pseudoconsola ConPTY:

- Se crean pipes síncronos para entrada/salida de la pseudoconsola.
- Se crea `HPCON` con `CreatePseudoConsole`.
- Se prepara `STARTUPINFOEX` con `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE`.
- Se inicia `powershell.exe` dentro de esa pseudoconsola.
- El navegador sigue hablando con `xterm.js`, pero ahora Windows recibe una sesión interactiva real.
- Se eliminó el workaround `DEL -> Ctrl+H` porque era propio del enfoque incorrecto por pipes.
- Se habilitó el reenvío de mensajes de resize también para agentes Windows.

## Archivos tocados

- `internal/app/terminal_process_windows.go`
- `internal/app/terminal.go`
- `internal/app/terminal_test.go`
- `contexto/30-terminal-web-xterm.md`
- `contexto/31-fix-backspace-terminal-windows.md`

## Requisito de sistema

ConPTY requiere Windows 10 1809 / Windows Server 2019 o superior. En sistemas más viejos la terminal Windows devolverá un error claro indicando que ConPTY no está disponible.

## Validación esperada en Windows

Al abrir la terminal remota Windows:

1. Escribir `abc`.
2. Presionar Backspace.
3. Debe quedar `ab` realmente, no solo mover el cursor.
4. Presionar Enter.
5. PowerShell debe ejecutar `ab`, no `abc` ni cerrar sesión.

También deberían mejorar flechas, historial, edición de línea y comportamiento de resize.

## Nota de implementación

Para `UpdateProcThreadAttribute`, el valor de `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE` debe recibir el valor `HPCON` y el tamaño de `HPCON`. No debe enviarse el texto de control al PowerShell ni traducirse Backspace manualmente.

## Actualización 2026-06-28

Después de probar en cliente Windows, ConPTY siguió requiriendo dos ajustes complementarios:

- Entrada: el frontend envía Backspace como `0x08` cuando el destino es un agente Windows, manteniendo `0x7f` para agentes Unix/Linux.
- Salida: si ConPTY/PowerShell emite `BS` no destructivo (`\b`), el cliente lo normaliza a `\b \b` antes de mandarlo a xterm.js, porque en VT `BS` solo mueve el cursor y no limpia la celda.

Esto no regresa al modo de pipes crudos; Windows sigue usando ConPTY.
