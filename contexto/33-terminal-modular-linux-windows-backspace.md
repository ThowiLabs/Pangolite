# 33 - Terminal modular Linux/Windows y Backspace destructivo

> Nota 2026-06-28: esta decisión quedó supersedida por `34-terminal-ux-linux-root-windows-aviso.md`. Windows se mantiene con aviso claro/no confiable y ya no se intenta abrir ConPTY desde el cliente hasta tener una implementación estable.


# Fecha
2026-06-28

# Objetivo
Corregir dos regresiones de la consola remota: clientes Linux conectados sin mostrar prompt real y clientes Windows donde Backspace movía el cursor pero no borraba visualmente el carácter anterior en xterm.js.

# Decisiones tomadas
- Mantener la arquitectura por backends de sistema operativo mediante build tags.
- Corregir Linux con PTY real en lugar de regresar a pipes.
- Mantener Windows sobre ConPTY porque es la ruta correcta para PowerShell/cmd interactivos.
- Agregar fallback básico por pipes para sistemas no Linux/no Windows, para que el cliente no quede completamente inutilizable en otros Unix o sistemas soportados por Go.
- Normalizar la salida Windows cuando emite Backspace no destructivo (`\b`) hacia secuencia destructiva VT (`\b \b`) antes de escribir en xterm.js.
- Conectar el WebSocket del stream remoto antes de iniciar el shell del cliente, para poder devolver errores visibles al navegador si el PTY/shell falla.

# Arquitectura actual
- Frontend: xterm.js envía entrada por WebSocket como bytes; Backspace normal se fuerza por sistema operativo del destino: Windows usa `BS` (`0x08`) para ConPTY/PowerShell y Unix usa `DEL` (`0x7f`) estilo xterm. Para terminal local se publica `serverOS` en el bootstrap del panel.
- Servidor: `/api/terminal/local` abre terminal local; `/api/terminal/agents/{id}` crea stream remoto hacia el cliente.
- Cliente Linux: usa `/dev/ptmx` con PTY real y shell interactivo.
- Cliente Windows: usa ConPTY con `CreatePseudoConsole` y `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE`.
- Otros sistemas: fallback básico por `stdin/stdout` del shell interactivo; no promete edición avanzada como una PTY real.

# Librerías usadas
- Standard library de Go para procesos, pipes, WebSocket ya existente y control de flujo.
- `golang.org/x/sys/windows` ya existente para llamadas Win32/ConPTY.
- No se agregaron dependencias nuevas.

# Archivos importantes modificados
- `internal/app/agent_client.go`
- `internal/app/assets/app/terminal.js`
- `internal/app/ui.go`
- `internal/app/terminal_process_linux.go`
- `internal/app/terminal_process_windows.go`
- `internal/app/terminal_process_other.go`
- `internal/app/terminal_vt.go`
- `internal/app/terminal_test.go`
- `contexto/30-terminal-web-xterm.md`
- `contexto/33-terminal-modular-linux-windows-backspace.md`

# Problemas encontrados
- En Linux, `SysProcAttr.Ctty` usaba `int(slave.Fd())`. Para `os/exec`, `Ctty` debe referirse al descriptor del proceso hijo; como el slave se conecta a stdin, debe ser `0`. Con el valor anterior el shell podía fallar antes de mostrar prompt.
- En Windows, aunque ConPTY es correcto, algunas salidas de edición de línea pueden emitir solo `BS` (`0x08`). En VT/xterm.js, `BS` solo mueve el cursor; no borra la celda.
- Si el cliente fallaba al iniciar terminal, antes no adjuntaba el WebSocket del stream y el navegador podía quedar conectado sin información útil.

# Soluciones implementadas
- Linux: `Ctty: 0` para que el shell tenga controlling TTY válido.
- Windows: filtro de salida `expandStandaloneBackspaceForVT` para convertir Backspace no destructivo en borrado visual real.
- Frontend: Backspace simple se envía explícitamente como `0x08` para agentes Windows y `0x7f` para agentes Unix/Linux.
- Agente remoto: primero adjunta WebSocket del stream, luego inicia terminal; si falla, escribe error claro al navegador y cierra.
- Otros sistemas: fallback de shell interactivo por pipes para compatibilidad mínima.
- Pruebas unitarias para el filtro VT de Backspace.

# Pendientes
- Probar manualmente en Windows 10/11 y Windows Server 2019+ con PowerShell y cmd.
- Probar manualmente en Alpine/Debian/Ubuntu para validar prompt, Backspace, flechas y resize.
- Si se requiere soporte PTY real en macOS/BSD, evaluar dependencia pequeña y mantenida o implementación específica por plataforma.

# Próximos pasos
- Compilar cliente Windows y Linux.
- Instalar clientes actualizados.
- Validar comandos manuales: escribir `abc`, Backspace, Enter; ejecutar `pwd`/`whoami`; probar flechas e historial.
