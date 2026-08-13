# Fecha
2026-08-13

# Objetivo
Implementar descargas iniciadas desde la terminal mediante `download ruta`, soportar directorios como ZIP seguro y simplificar la barra superior de la terminal para escritorio/móvil sin perder funciones.

# Decisiones tomadas
- `download` no se instala como ejecutable ni modifica el sistema remoto. Es una pseudo-orden interceptada por `terminal.js` antes de enviar Enter al shell.
- El texto escrito sí llega al PTY mientras se compone; al detectar Enter sobre una línea `download ...`, el navegador envía `Ctrl+U` para limpiar la línea y manda `download.request` por el canal de control de Pangolite en lugar de ejecutarla.
- La interceptación se desactiva si el usuario entra al alternate buffer (vim/nano/tmux con pantalla alterna) o utiliza secuencias complejas de edición, para no secuestrar entradas de aplicaciones interactivas.
- El proceso dueño del PTY resuelve rutas relativas contra su cwd real. En Linux esto conserva el seguimiento existente mediante `/proc/<pid>/cwd`.
- La preparación devuelve `download.offer` con ruta canónica, tipo y nombre. El navegador registra un ticket HTTP autenticado, de un solo uso y con vencimiento corto; después dispara una descarga normal del navegador.
- Los archivos se transmiten por `io.CopyBuffer`; no se guardan completos en RAM, no se usa base64 y no pasan por Blob de JavaScript.
- Los clientes remotos anuncian `terminal-download-v1`. La descarga remota usa un `AgentStreamJob` dedicado (`terminal-download`) y transmite metadata + payload por el túnel ya autenticado.
- Solo se permiten dos descargas de terminal simultáneas en el servidor para reducir competencia de CPU/disco/red en VPS pequeños.
- Los directorios se archivan con `archive/zip`. Se usa Deflate `BestSpeed` para archivos que se benefician de compresión y `Store` para formatos ya comprimidos.
- Antes de emitir cabeceras de un ZIP se recorre el árbol con límites: máximo 20.000 entradas y 8 GiB de datos regulares sin comprimir.
- No se siguen symlinks ni se agregan sockets, FIFOs, dispositivos u otros archivos especiales.
- En Linux se rechazan `/` y `/var` cuando se solicitan como raíces completas. También se bloquean recursivamente `/proc`, `/sys`, `/dev`, `/run`, `/boot`, `/etc`, `/usr`, `/bin`, `/sbin`, `/lib`, `/lib64`, `/var/lib`, `/var/cache`, `/var/log`, `/var/spool`, `/var/backups`, `/var/lock`, `/snap` y `/lost+found`; `/var/www` y otros directorios de datos concretos siguen permitidos. Un archivo regular concreto dentro de esas ubicaciones puede descargarse si el usuario de la terminal ya tiene permiso; la prohibición se aplica al archivado recursivo de directorios.
- En Windows se bloquea la raíz completa de una unidad, Windows, Program Files y ProgramData para archivado recursivo. La terminal Windows continúa deshabilitada por la limitación ya documentada, pero la política queda preparada.
- El toolbar deja **Pantalla completa** como la única acción principal permanente durante una sesión; `Conectar` solo aparece cuando hace falta iniciar/reintentar la sesión.
- El selector de destino y el selector de tema se movieron al widget flotante de ajustes (`gear`).
- Subir archivo y Desconectar se movieron al menú de opciones de tres puntos reutilizando el patrón global `action-dropdown`.

# Arquitectura actual
La terminal usa dos rutas separadas para archivos:

1. **Subida**: navegador -> WebSocket de terminal -> PTY owner, con framing binario `terminal-upload-v1`.
2. **Descarga**: pseudo-orden -> control `download.request` -> resolución del cwd/target -> ticket HTTP -> respuesta streaming. En agentes remotos el GET crea un stream `terminal-download` hacia `pangolite-client`, que abre/archiva el target y transmite el payload hacia el servidor y navegador.

Los tickets viven solo en memoria, están ligados al `user_id`, vencen a los dos minutos y se consumen en el primer GET válido. No se persisten rutas de descarga ni archivos temporales de ZIP.

# Librerías usadas
- Standard library Go: `archive/zip`, `compress/flate`, `filepath`, `io`, `mime`, `net/http`, `bufio`.
- WebSocket ya existente (`nhooyr.io/websocket`) para el stream remoto.
- JavaScript/CSS nativos y componentes existentes del panel.
- No se agregaron dependencias.

# Archivos importantes modificados
- `internal/app/terminal_download.go`
- `internal/app/terminal_download_test.go`
- `internal/app/terminal.go`
- `internal/app/terminal_upload.go`
- `internal/app/tunnel.go`
- `internal/app/agent_client.go`
- `internal/app/server.go`
- `internal/app/templates/pages/terminal.html`
- `internal/app/assets/app/terminal.js`
- `internal/app/assets/app/panel.css`
- `internal/app/tunnel_test.go`
- `internal/app/ui_test.go`
- `README.md`
- `contexto/00-resumen-proyecto.md`

# Problemas encontrados
- Descargar a través del mismo WebSocket y reconstruir un Blob en JavaScript haría que archivos grandes consumieran memoria del navegador; se descartó.
- Crear un ZIP temporal completo en disco duplicaría uso de almacenamiento y complicaría limpieza; se descartó.
- Un `download` instalado como comando real requeriría tocar PATH/shell del host y sería menos portable; se descartó.
- Comprimir `/`, `/proc`, `/sys` o árboles enormes podría consumir CPU, E/S o recorrer pseudo-filesystems indefinidamente; se implementó una política previa al streaming.
- El toolbar anterior todavía exponía tema, subir y desconectar como controles permanentes, ocupando demasiado espacio en móvil.

# Soluciones implementadas
- Descarga HTTP nativa del navegador mediante ticket temporal y respuesta streaming.
- Nuevo modo de stream remoto y capability `terminal-download-v1`.
- ZIP incremental sin archivo temporal, con compresión selectiva, un solo ZIP activo por proceso y límites de seguridad. Los archivos de 128 MiB o más se almacenan sin Deflate para no monopolizar un único CPU.
- Política de directorios de sistema protegidos y exclusión de symlinks/especiales.
- Máximo de dos descargas concurrentes.
- Interceptación conservadora de la pseudo-orden para no afectar programas en alternate buffer.
- Widget flotante de ajustes y menú compacto de opciones reutilizando patrones existentes.

# Pendientes
- Ejecutar `go test ./...`, `go vet ./...` y `govulncheck ./...` con Go 1.26 en un entorno con dependencias disponibles.
- Validar descargas de varios GiB en navegador real y comportamiento si el archivo cambia mientras se transmite.
- Validar el toolbar en Android real tanto vertical como horizontal.

# Próximos pasos
- Probar `download archivo.ext`, `download "archivo con espacios.ext"`, `download .` dentro de un directorio de proyecto y un intento de `download /etc`.
- Probar las mismas operaciones contra un `pangolite-client` actualizado.
- Confirmar que clientes antiguos muestran el mensaje de actualización y no reciben controles desconocidos.
