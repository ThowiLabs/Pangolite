# Fecha
2026-08-12

# Objetivo
Auditar Pangolite como aplicación de producción, actualizar las reglas persistentes de trabajo al modo Ponytail vigente y reforzar seguridad/rendimiento del servidor y cliente sin añadir dependencias ni aumentar de forma innecesaria el consumo de RAM/CPU.

# Decisiones tomadas
- Se conserva la arquitectura Go + SQLite + Traefik: ya es apropiada para VPS pequeños y no necesita Redis, colas externas ni otro datastore para esta escala.
- Se conserva SQLite en WAL con una sola conexión porque reduce contención y evita un pool innecesario en equipos de pocos recursos.
- El control de acceso administrativo se implementa por red IP conocida, sin geolocalización externa. El modo por defecto `learn` aprende `/24` para IPv4 o `/64` para IPv6 después del primer login válido y bloquea otras redes.
- Solo se confía en cabeceras `X-Forwarded-*` cuando el peer inmediato pertenece a `PANGOLITE_TRUSTED_PROXY_CIDRS`; el valor por defecto solo confía en loopback.
- Se mantiene el bloqueo existente de 5 fallos de login/10 minutos por usuario+origen y se añade un límite barato previo a bcrypt de 30 intentos/10 minutos por IP.
- Se limita recuperación de contraseña y se requiere un dominio de dashboard efectivo para generar enlaces, evitando depender de un `Host` suministrado por el cliente.
- Se limitan cabeceras HTTP, conexiones ociosas, tamaño de cuerpos del túnel remoto y concurrencia HTTP de agentes.
- Traefik limita el panel a 30 req/s (ráfaga 60) y 64 solicitudes simultáneas. Esto mitiga abuso HTTP pero no pretende sustituir mitigación DDoS volumétrica del proveedor/firewall.
- Se eliminó `InsecureSkipVerify` de los `websocket.Accept` del servidor. Los terminales conservan además su validación manual de origen/autorización.
- No se agregó ninguna dependencia. Los límites y allowlists se implementaron con la librería estándar.
- Se actualizó la rama objetivo del proyecto a Go 1.26 y el toolchain temporal de instalación a Go 1.26.5.
- Las entregas futuras de este flujo deben conservar `.git/`, historial de commits, `contexto/` y `tareas/`; no se debe reinicializar Git.

# Arquitectura actual
- `cmd/pangolite`: servidor/panel.
- `cmd/pangolite-client`: agente remoto Linux/Windows.
- `internal/app/server.go`: HTTP, autenticación, API, gateway y túnel.
- `internal/app/security.go`: resolución de IP confiable, rate limiter acotado y control de redes administrativas.
- `internal/app/store.go`: SQLite/migraciones, incluyendo redes administrativas confiables.
- `internal/app/tunnel.go`: colas de agentes con limpieza al eliminar clientes.
- `internal/app/traefik.go`: render y aplicación de reverse proxy/rate limits.
- SQLite mantiene WAL y `SetMaxOpenConns(1)`.
- Traefik termina HTTP/HTTPS y entrega el panel por loopback cuando corresponde.

# Librerías usadas
No se agregaron dependencias.

Dependencias relevantes existentes:
- `modernc.org/sqlite`: SQLite sin CGO.
- `golang.org/x/crypto`: bcrypt.
- `golang.org/x/sys`: integración de bajo nivel/sistema.
- `nhooyr.io/websocket`: WebSockets; queda pendiente migrarlo al módulo mantenido `github.com/coder/websocket` cuando pueda descargarse y probarse la dependencia.

# Archivos importantes modificados
- `.github/workflows/release.yml`
- `go.mod`
- `README.md`
- `init.sh`
- `install.sh`
- `internal/app/config.go`
- `internal/app/security.go`
- `internal/app/server.go`
- `internal/app/store.go`
- `internal/app/tunnel.go`
- `internal/app/agent_client.go`
- `internal/app/terminal.go`
- `internal/app/traefik.go`
- `internal/app/server_test.go`
- `internal/app/tunnel_test.go`
- `contexto/00-resumen-proyecto.md`
- `contexto/65-hardening-seguridad-rendimiento-ponytail.md`
- `tareas/completado-65-hardening-seguridad-rendimiento.md`

# Problemas encontrados
## Seguridad
- El rate limit de login existente se indexaba con `RemoteAddr`, por lo que detrás de proxy podía ver la IP del proxy y perder precisión.
- `X-Forwarded-Proto` podía influir en cookies sin validar previamente que el peer fuera un proxy confiable.
- Los WebSockets usaban `InsecureSkipVerify` aunque algunos flujos ya hacían validación manual de origen.
- La recuperación podía construir una URL desde el host de la petición cuando no había dominio configurado.
- No existía restricción persistente para impedir usar una sesión/login desde una red administrativa nueva.
- No había límites explícitos de cabeceras/idle en el servidor y Traefik no limitaba concurrencia/rate del panel.
- El túnel HTTP podía leer request/response completos sin tope, permitiendo picos grandes de RAM.

## Rendimiento/recursos
- Las colas asociadas a agentes eliminados podían permanecer en memoria durante la vida del proceso.
- El proxy HTTP por agente no tenía un límite global de concurrencia, por lo que varios cuerpos grandes simultáneos podían elevar RAM.
- El cliente conservaba hasta 32 conexiones HTTP idle globales y 8 por host pese a que el nuevo límite de concurrencia hace innecesario ese margen en instalaciones pequeñas.
- No se detectó necesidad de añadir caches, pools ni workers adicionales. Mantener el diseño simple es mejor para máquinas pequeñas.

## Dependencias/toolchain
- La dependencia `nhooyr.io/websocket` está deprecada/movida a Coder; no se migró en esta entrega porque el entorno actual no puede descargar módulos ni ejecutar la suite con la nueva dependencia.
- Por compatibilidad histórica, los agentes aún aceptan fallback `http://IP:2424`. Ese modo envía autenticación del agente sin TLS y no debe cruzar redes no confiables; endurecerlo requiere una migración compatible (TLS/pinning o retirar HTTP) que debe probarse contra clientes instalados.
- El ZIP recibido no incluye `go.sum`; esto reduce la reproducibilidad exacta de dependencias. Debe generarse/verificarse con acceso de red mediante `go mod tidy` y conservarse en Git.

# Soluciones implementadas
- Resolución de IP real con cadena de proxies confiables y rechazo de cabeceras forwarded provenientes de peers no autorizados.
- Rate limiter en memoria con mapa acotado para evitar que el propio mecanismo de defensa sea usado para consumir RAM indefinidamente.
- Aprendizaje/persistencia de redes administrativas en SQLite y validación de red también para sesiones existentes. La lista aprendida se carga en memoria al iniciar y solo se refresca al aprender una red, evitando consultar SQLite en cada request.
- Validación estricta de las variables CIDR al arrancar.
- Rate limit separado para login y recuperación de contraseña.
- Cookies `Secure` basadas únicamente en TLS real o forwarded HTTPS de proxy confiable.
- `Cache-Control: no-store` en API y páginas sensibles.
- Recuperación por correo condicionada a dominio de dashboard configurado.
- `ReadHeaderTimeout=5s`, `IdleTimeout=60s`, `MaxHeaderBytes=32 KiB`.
- Límite de 16 MiB por cuerpo del túnel HTTP remoto y máximo de 4 solicitudes concurrentes por defecto.
- Pool idle del cliente reducido a 8 conexiones globales/4 por host para bajar el estado ocioso sin impedir la concurrencia normal.
- Límite del body de respuestas publicadas por agentes antes de decodificarlas en el servidor.
- Limpieza de colas/streams de `TunnelHub` cuando se elimina un agente.
- Rate limit + in-flight limit en el router de panel de Traefik.
- WebSockets sin `InsecureSkipVerify`.
- Go objetivo actualizado y documentación/instaladores sincronizados.

# Auditoría Ponytail
- `stdlib`: rate limit, CIDR, timeouts y semáforo se resolvieron con `net`, `net/http`, `sync`, `time` y canales; reemplazo: ninguna dependencia nueva.
- `native`: el límite perimetral HTTP se delega a middlewares nativos de Traefik en lugar de duplicar toda esa lógica dentro del panel.
- `shrink`: no se introdujo una capa de servicios/ACL nueva; la política pequeña vive en `security.go` y persistencia en `Store`.
- `yagni`: no se añadió geolocalización IP, Redis, captcha externo, WAF embebido ni sistema de reputación; requerirían dependencias/estado y no garantizan protección volumétrica.
- `delete`: al eliminar un agente ahora se elimina también su estado efímero del hub; reemplazo: nada.

No se hizo un refactor masivo porque la auditoría no justificó romper módulos estables solo para reducir líneas.

# Pruebas realizadas
- `gofmt` sobre archivos Go modificados.
- `bash -n init.sh`.
- `bash -n install.sh`.
- `sh -n install.sh`.
- `git diff --check`.
- Revisión estática dirigida de ejecuciones de comandos, SQL concatenado, lecturas de body, `websocket.Accept` y operaciones de archivos.

La suite `go test ./...` no pudo ejecutarse en este entorno: el contenedor dispone de Go 1.23.2 y no tiene resolución/salida para descargar Go 1.26 ni los módulos faltantes. No se rebajó el `go.mod` para ocultar esa limitación.

# Pendientes
- Ejecutar en un equipo con Internet: `go mod tidy`, revisar/añadir `go.sum`, `go test ./...`, `go vet ./...` y, si está disponible, `govulncheck ./...`.
- Migrar `nhooyr.io/websocket` a `github.com/coder/websocket` y validar terminal/streams en Linux y Windows.
- Diseñar migración del fallback legado `http://IP:2424` hacia transporte autenticado con TLS/pinning o exigir HTTPS sin romper clientes ya desplegados.
- Considerar MFA para administradores y una UI de alta/baja de redes confiables.
- Autoalojar Chart.js, xterm, iconos/animaciones actualmente servidos desde CDN y endurecer CSP eliminando `unsafe-inline` gradualmente.
- Para DDoS volumétrico, activar mitigación de red/proveedor y firewall; los límites de Pangolite/Traefik solo protegen cuando el tráfico ya alcanzó el host.

# Próximos pasos
1. Ejecutar la suite completa con Go 1.26.5 y dependencias disponibles.
2. Probar login desde la red habitual, luego desde una red/VPN distinta y confirmar `403`.
3. Verificar recuperación de contraseña con dominio de panel configurado.
4. Probar HTTP/HTTPS, TCP/UDP, terminal y túneles de agentes para detectar regresiones.
5. Medir RAM/CPU con 1, 4 y más solicitudes de túnel simultáneas antes de aumentar `PANGOLITE_AGENT_HTTP_CONCURRENCY`.
6. Configurar firewall/upstream DDoS según el proveedor del VPS.
