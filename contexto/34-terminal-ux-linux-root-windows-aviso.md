# 34 - Terminal UX, Linux root y aviso Windows

# Fecha
2026-06-28

# Objetivo
Corregir el flujo de la consola remota antes de hacer commit: evitar falsas expectativas en Windows, mejorar el arranque de shell Linux y hacer que la UI no abra sesiones automáticamente ni quede vacía sin estado.

# Decisiones tomadas
- Windows queda temporalmente como plataforma no confiable para consola remota interactiva. El cliente/servidor devuelven un mensaje claro en vez de intentar ConPTY roto.
- Linux se mantiene como backend principal con PTY real.
- La consola ya no debe parecer conectada al entrar. Debe mostrar estado "Aún no conectado" y botón para conectar.
- Al iniciar conexión se muestra overlay con animación de carga.
- Si la conexión se cierra o el cliente se desconecta, se muestra mensaje visible y botón de reconectar.
- En Linux se fuerza shell interactivo (`-i`) para que builtins como `cd` funcionen como consola real.
- Si el proceso tiene root, la terminal arranca con identidad root (`HOME=/root`, `USER=root`, `LOGNAME=root`) y directorio `/root` si está disponible.
- Si el proceso no es root pero tiene `sudo -n` sin contraseña, se intenta abrir `sudo -n -i`; si no, cae al shell normal del usuario.
- En el servicio Linux del cliente se elimina `ProtectHome=true` para no romper el acceso de la consola root a `/root` cuando el cliente se reinstale.

# Arquitectura actual
- Frontend `terminal.js`: controla estados `idle`, `connecting`, `connected`, `bad/warning` con overlay dentro del área de terminal.
- Backend local/remoto: sigue usando WebSocket y stream persistente.
- Linux: `/dev/ptmx` + shell interactivo por PTY real.
- Windows: `startTerminalProcess` devuelve error explícito de plataforma no confiable; el frontend también detecta `windows` y muestra aviso antes de abrir socket.
- Otros sistemas: fallback básico por pipes permanece para compatibilidad mínima.

# Librerías usadas
- Go standard library.
- JavaScript nativo.
- No se agregaron dependencias nuevas.

# Archivos importantes modificados
- `internal/app/assets/app/terminal.js`
- `internal/app/assets/app/panel.css`
- `internal/app/templates/pages/terminal.html`
- `internal/app/terminal_process_linux.go`
- `internal/app/terminal_process_windows.go`
- `internal/app/terminal_process_linux_test.go`
- `cmd/pangolite-client/install_unix.go`
- `contexto/31-fix-backspace-terminal-windows.md`
- `contexto/33-terminal-modular-linux-windows-backspace.md`
- `contexto/34-terminal-ux-linux-root-windows-aviso.md`

# Problemas encontrados
- Windows seguía fallando con edición interactiva; intentar parches de Backspace no daba confiabilidad real.
- La UI podía quedar con cursor parpadeando sin explicar si estaba desconectada, conectando o cerrada.
- Linux podía abrir un shell no forzado a modo interactivo; eso aumenta riesgo de que builtins o comportamiento de línea no funcionen como consola real.
- El servicio Linux del cliente protegía `/root` con `ProtectHome=true`, lo cual contradice abrir una consola root funcional desde el panel.

# Soluciones implementadas
- Mensaje claro para Windows en frontend y backend.
- Overlay inicial "Aún no conectado" con botón interno.
- Overlay de carga durante conexión.
- Overlay de reconexión al cierre no manual.
- Shell Linux interactivo por defecto.
- Preferencia root cuando el proceso ya corre como root o cuando existe `sudo -n` disponible.
- Servicio Linux del cliente sin `ProtectHome=true` para futuras instalaciones/reinstalaciones.
- Pruebas unitarias mínimas para selección de shell Linux.

# Pendientes
- Reinstalar clientes Linux para que la unidad systemd pierda `ProtectHome=true` si ya estaban instalados con versiones anteriores.
- Probar manualmente `whoami`, `pwd`, `cd /`, `cd`, flechas, Backspace y desconexión/reconexión.
- Diseñar soporte Windows real más adelante, posiblemente con un host interactivo dedicado por usuario/sesión, no desde un servicio Windows aislado.

# Próximos pasos
- Ejecutar `go test ./...` con Go 1.24+.
- Compilar servidor y clientes.
- Reinstalar cliente Linux en máquinas de prueba.
- Validar UX completa desde el navegador.
