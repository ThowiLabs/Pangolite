# 67 - Atajos táctiles de terminal para Android

# Fecha
2026-08-12

# Objetivo
Hacer cómoda la consola web de Pangolite desde Android sin teclado físico, tomando como referencia la fila de teclas auxiliares de Termux que aparece encima del teclado virtual.

# Decisiones tomadas
- Implementar el accesorio con HTML/CSS/JavaScript nativo; no agregar librerías ni componentes externos.
- Mostrarlo únicamente cuando el navegador es Android y reporta capacidad táctil.
- Mantener la entrada normal de xterm.js por una sola ruta (`term.onData -> WebSocket`) para no reintroducir el bug histórico de teclas duplicadas.
- Enviar solo los atajos explícitos directamente al WebSocket.
- Tratar `CTRL` y `ALT` como modificadores de un solo uso: se activan visualmente y se consumen con la siguiente tecla o carácter.
- Convertir `Ctrl+A..Z` y combinaciones de control ASCII habituales a sus bytes reales para que funcionen con el teclado virtual de Android.
- Usar secuencias ANSI para flechas, Home, End, Page Up y Page Down, incluyendo variantes con Ctrl/Alt.
- Reutilizar el textarea oculto de xterm únicamente para saber si el teclado virtual está enfocado y para mostrarlo/ocultarlo desde el botón dedicado.
- Usar `VisualViewport` cuando existe para fijar la barra en el borde visible superior del teclado y recalcular el tamaño de xterm sin tapar líneas útiles.

# Arquitectura actual
La página `/terminal` contiene `#terminalMobileKeys` justo debajo de la superficie de xterm. El componente permanece oculto por defecto y `terminal.js` agrega `is-enabled` solo en Android táctil.

La barra tiene dos filas horizontales desplazables:

- `ESC`, `CTRL`, `ALT`, `TAB`, teclado, `HOME`, `↑`, `END`.
- `PGUP`, `{}`, `()`, `[]`, `\\`, `←`, `↓`, `→`, `PGDN`.

Cuando el textarea de xterm tiene foco, la barra cambia a modo flotante y usa `window.visualViewport` para colocarse sobre el teclado virtual. El contenedor de terminal reserva el alto equivalente para que el accesorio no cubra la última línea visible.

# Librerías usadas
- JavaScript nativo.
- CSS nativo.
- xterm.js ya existente.
- Bootstrap Icons ya existente para el icono del teclado.
- No se agregaron dependencias.

# Archivos importantes modificados
- `internal/app/templates/pages/terminal.html`
- `internal/app/assets/app/terminal.js`
- `internal/app/assets/app/panel.css`
- `internal/app/ui_test.go`
- `README.md`
- `contexto/00-resumen-proyecto.md`
- `contexto/67-atajos-terminal-android.md`
- `tareas/completado-67-atajos-terminal-android.md`

# Problemas encontrados
- Los teclados virtuales Android no exponen cómodamente Esc, Ctrl, Alt, Home, End ni navegación de terminal.
- Un botón HTML puede quitar temporalmente el foco al textarea de xterm y provocar que el teclado virtual se repliegue.
- Reenviar manualmente teclas que xterm ya entrega normalmente puede producir duplicados, como ocurrió anteriormente con Backspace.

# Soluciones implementadas
- Después de usar un atajo se devuelve el foco a xterm, salvo cuando el usuario pulsa explícitamente el botón de mostrar/ocultar teclado.
- Los eventos de foco/blur y `VisualViewport.resize/scroll` reajustan la barra con debounce corto.
- `Ctrl+C`, `Ctrl+D` y el resto de controles ASCII se construyen desde el siguiente carácter recibido por `term.onData`, sin generar una segunda entrada normal.
- Los atajos de navegación generan directamente secuencias ANSI y consumen Ctrl/Alt después del envío.
- Al desconectar, reconectar, cerrar la sesión o pegar desde portapapeles se limpian modificadores pendientes para evitar combinaciones accidentales.

# Validaciones realizadas
- `node --check internal/app/assets/app/terminal.js`.
- Harness JavaScript aislado para comprobar `Ctrl+C`, `Alt+X`, `Ctrl+Alt+D`, `Ctrl+Left`, `Alt+Up` y `PageUp`.
- `gofmt` sobre `internal/app/ui_test.go`.
- Parseo aislado de `terminal.html` con `html/template` y `agentSystem` simulado.
- `git diff --check`.
- Se añadió `TestTerminalAndroidShortcutAccessoryIsEmbedded` para validar que template, JS y CSS conservan las piezas críticas del accesorio.

# Limitaciones de validación
El entorno disponible usa Go 1.23.2 mientras el proyecto requiere Go 1.26, por lo que la suite Go completa debe ejecutarse en un entorno con el toolchain correcto. La comprobación visual final del posicionamiento sobre Gboard/teclado del fabricante debe hacerse en un Android real porque el comportamiento del teclado virtual depende del navegador/IME.

# Pendientes
- Validar físicamente en Chrome Android y, si se usa, navegador WebView del dispositivo.
- Confirmar que Gboard y otros IME mantienen la barra inmediatamente encima del teclado en orientación vertical y horizontal.
- Si después se desea, agregar configuración del orden/contenido de teclas sin convertirlo en un sistema complejo.

# Próximos pasos
- Ejecutar `go test ./...` y `go vet ./...` con Go 1.26.
- Probar `Ctrl+C`, `Ctrl+D`, `Esc` en `vim`/`nano`, flechas, Home/End y ocultar/mostrar teclado desde Android.
