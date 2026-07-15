# 60 - Corrección de Backspace y endurecimiento de la terminal remota

## Objetivo
Corregir que una sola pulsación de Backspace eliminara dos caracteres en la terminal web y auditar el flujo completo de teclado, WebSocket, PTY y streams remotos para evitar duplicaciones, sesiones mezcladas, pérdidas de datos y recursos bloqueados.

## Causa principal del doble borrado
`terminal.js` interceptaba Backspace mediante `attachCustomKeyEventHandler`, enviaba manualmente `\x7f` al WebSocket y, al mismo tiempo, xterm.js conservaba su propia ruta normal de entrada mediante `term.onData`.

La terminal ahora deja Backspace exclusivamente a cargo de xterm.js. Toda entrada normal viaja por una sola ruta:

```text
teclado -> xterm.js -> term.onData -> WebSocket -> PTY
```

## Cambios implementados

### Entrada de teclado y portapapeles
- Se eliminó el envío manual de Backspace.
- Ctrl+V continúa usando el manejo nativo de xterm.js.
- Shift+Insert y el menú contextual ahora usan `term.paste()` en vez de escribir directamente al WebSocket, por lo que respetan el modo de pegado entre corchetes de la terminal.
- Se evita pegar cuando no existe una conexión activa.
- El resize de xterm se agrupa con debounce para evitar mensajes repetidos durante reajustes rápidos.

### Reconexión y WebSocket
- Cada conexión tiene un identificador de generación.
- Los eventos tardíos de una conexión anterior ya no pueden cambiar el estado ni escribir datos sobre una sesión nueva.
- Desconectar durante el estado `CONNECTING` deja la conexión invalidada y la cierra al abrirse si el navegador no permitió cerrarla antes.
- Se eliminó el mensaje duplicado de “Desconectado por el usuario”.
- Cada WebSocket usa su propio `TextDecoder` con modo streaming para no corromper caracteres UTF-8 cuando una secuencia multibyte llega dividida entre frames.

### Protocolo de control de terminal
- Los mensajes JSON de control ahora requieren explícitamente `pangoliteTerminal: true` y un tipo conocido.
- La terminal local solo interpreta controles enviados como frames de texto.
- La terminal del agente solo interpreta controles internos enmarcados dentro del stream.
- Un comando normal con apariencia JSON ya no se confunde con un resize.
- Los frames reservados inválidos se preservan como datos normales en vez de desaparecer.
- Cuando un prefijo de control llega dividido, los bytes normales anteriores se entregan inmediatamente y solo se retiene el fragmento necesario.
- Las escrituras hacia PTY/net.Conn completan todo el payload aunque el writer realice escrituras parciales.

### PTY y entorno Linux
- El cierre de `terminalProcess` es idempotente para evitar dobles cierres concurrentes.
- El goroutine que espera al shell Linux reutiliza el mismo cierre idempotente.
- Las variables `TERM`, `COLORTERM`, `HOME`, `USER`, `LOGNAME`, `LANG` y `LC_ALL` sustituyen valores previos en vez de quedar duplicadas en el entorno del proceso.

### Ciclo de vida de streams remotos
- Un stream ya adjuntado no acepta una segunda conexión simultánea.
- `SubmitStream` deja de esperar si su contexto se cancela después de adjuntarse.
- Si el WebSocket del agente falla durante el handshake después de adjuntar el stream, `CompleteStream` se ejecuta igualmente.
- Esto evita sesiones huérfanas y bloqueos al desconectar/reconectar rápidamente.

## Archivos modificados
- `internal/app/assets/app/terminal.js`
- `internal/app/terminal.go`
- `internal/app/terminal_process_linux.go`
- `internal/app/terminal_process_other.go`
- `internal/app/agent_client.go`
- `internal/app/tunnel.go`
- `internal/app/server.go`
- `internal/app/terminal_test.go`
- `internal/app/tunnel_test.go`
- `contexto/60-fix-backspace-hardening-terminal.md`

## Validaciones realizadas
- `node --check internal/app/assets/app/terminal.js`.
- Prueba aislada del frontend con WebSocket/xterm simulados: una pulsación de Backspace produce exactamente un frame `0x7f` mediante `term.onData`.
- Ocho pruebas unitarias de filtros de control, escrituras parciales, entorno y cierre idempotente ejecutadas correctamente en un harness aislado.
- Dos pruebas unitarias de ciclo de vida de streams ejecutadas correctamente.
- `gofmt` aplicado a todos los archivos Go modificados.

## Validación pendiente en el entorno real
El entorno de edición no pudo ejecutar `go test ./...` sobre el proyecto completo porque solo dispone de Go 1.23.2, mientras `go.mod` exige Go 1.24.0, y no tuvo acceso de red para descargar el toolchain y módulos. Debe ejecutarse en el servidor/equipo del proyecto:

```bash
go test ./...
```

## Pruebas manuales recomendadas
1. Escribir `Hola mundo` y pulsar Backspace una vez: debe quedar `Hola mund`.
2. Mantener Backspace presionado: debe borrar progresivamente sin saltos dobles.
3. Probar Ctrl+Backspace, flechas, Delete, Home, End, Tab y Enter.
4. Pegar con Ctrl+V, Shift+Insert y el menú contextual; el texto debe aparecer una sola vez.
5. Pegar texto con acentos, `ñ` y emojis para verificar UTF-8.
6. Abrir `nano` o `vim`, redimensionar la ventana y comprobar que la vista se reajusta.
7. Desconectar mientras está conectando y volver a conectar; la sesión anterior no debe cambiar el estado ni escribir salida.
8. Abrir la terminal remota, cerrar la pestaña abruptamente y reconectar; no debe quedar un stream bloqueado.
9. Intentar abrir dos conexiones simultáneas al mismo stream; la segunda debe rechazarse.
