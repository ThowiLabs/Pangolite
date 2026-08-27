# Pangolite

Pangolite es una plataforma de administración de proxys y túneles escrita en Go para servidores Linux. Permite administrar proyectos, dominios, recursos HTTP/HTTPS/TCP/UDP y servidores conectados. Traefik se mantiene como edge HTTP/HTTPS; Pangolite administra directamente los listeners TCP/UDP públicos para poder aplicarlos en caliente.

Repositorio previsto:

```text
github.com/ThowiLabs/Pangolite
```

## Estado del proyecto

La base actual incluye:

- Panel web en Go.
- SQLite embebido para usuarios, sesiones, proyectos, dominios, clientes de sistema y recursos.
- Login con usuario `admin` y contraseña temporal.
- Cambio obligatorio de contraseña en primer acceso.
- Cambio de nombre de usuario desde Perfil y seguridad, confirmado con la contraseña actual.
- Sesiones persistentes con cookie segura.
- CSRF en operaciones administrativas.
- CRUD de proyectos.
- CRUD de dominios administrados.
- Configuracion del dominio publico del dashboard con validacion DNS contra la IP del servidor.
- Clientes de sistema/agentes para servidores NAT/remotos.
- Directorio global de Conexiones SSH para abrir la terminal del servidor Pangolite o de cualquier cliente sin navegar primero por su proyecto; la sección de frecuentes se calcula desde aperturas reales registradas en auditoría y persiste entre dispositivos.
- Transferencia bidireccional de archivos desde la terminal local o remota: subida por arrastrar/selector con progreso y descarga mediante `download ruta`, incluyendo ZIP seguro de directorios.
- Recursos HTTP/HTTPS locales o mediante cliente de sistema.
- Branding de carga Thowilabs opcional por recurso HTTP, desactivado por defecto e inyectado solo en documentos HTML compatibles.
- Recursos TCP/UDP publicados directamente por el plano L4 dinámico de Pangolite, locales o mediante cliente de sistema.
- Validación de puerto público contra recursos existentes y contra puertos ocupados en el sistema.
- Suspension de recursos HTTP/HTTPS con respuesta 403, 404 o HTML personalizado basado en plantillas editables.
- Instalación desde releases con `install.sh`, con detección de sistema/init y configuración de Traefik del sistema.
- Recarga automática de HTTP/HTTPS mediante file provider con `watch=true`.
- Aplicación automática de cambios desde la UI; no se pide al usuario aplicar Traefik manualmente.
- Alta, edición, suspensión y eliminación de listeners TCP/UDP sin reiniciar Traefik ni cortar HTTP/HTTPS u otros puertos activos.

TCP remoto mediante cliente de sistema usa streams/WebSocket persistentes directamente desde el listener público de Pangolite hacia `TunnelHub`; UDP remoto usa jobs de datagrama autenticados. Ya no existe un puerto puente local por recurso ni un entrypoint L4 de Traefik.

## Onboarding inicial

En instalaciones nuevas Pangolite ya no crea un proyecto `default` automáticamente. Después del primer inicio de sesión, el dashboard muestra un onboarding para crear el primer proyecto y recordar el flujo recomendado: proyecto, dominio, cliente de sistema si aplica y recurso publicado.

## Arquitectura

```text
HTTP/HTTPS:
Internet → Traefik → Pangolite o backend HTTP

TCP/UDP:
Internet → listener L4 de Pangolite → servicio local o túnel remoto
```

Para servicios detrás de NAT:

```text
HTTP/HTTPS remoto:
Internet → Traefik → Pangolite → cliente de sistema → servicio remoto

TCP/UDP remoto:
Internet → Pangolite L4 → TunnelHub/job → cliente de sistema → servicio remoto
```

## Requisitos

Servidor Linux con:

- Internet para descargar releases y Traefik si no está instalado.
- `curl` o `wget`.
- `tar` y `gzip`.
- Un gestor de arranque compatible: `systemd`, `OpenRC`, `SysVinit` o `runit`.

`install.sh` descarga binarios precompilados desde GitHub Releases; no necesita Go en el servidor final.

`init.sh` queda como instalador de desarrollo/local porque compila desde el código fuente.

## Instalación rápida

Última versión publicada:

```bash
curl -fsSL https://raw.githubusercontent.com/ThowiLabs/Pangolite/main/install.sh | sudo sh
```

Versión específica:

```bash
curl -fsSL https://raw.githubusercontent.com/ThowiLabs/Pangolite/main/install.sh | sudo sh -s -- --version 0.1
```

Desde un ZIP del código fuente, para desarrollo:

```bash
sudo bash init.sh
```

El panel arranca en la IP detectada por el instalador:

```text
http://<IP_DEL_SERVIDOR>:2424
```

`install.sh` intenta detectar la IP publica real y la imprime al terminar. Si no puede, usa la IP local de salida como respaldo.

No hay redirección HTTPS inicial. La publicación por dominio/HTTPS se configura después desde Traefik y Pangolite.

## Ubicaciones

```text
/opt/pangolite/pangolite                 Binario servidor
/opt/pangolite/pangolite-client          Cliente local compatible con la arquitectura del servidor
/opt/pangolite/public/                   Clientes publicados para descarga desde el panel
/opt/pangolite/data/pangolite.db         SQLite
/opt/pangolite/data/admin-password.txt   Contraseña temporal inicial
/opt/pangolite/pangolite.env             Variables de entorno
/etc/traefik/traefik.yml                 Config estática gestionada
/etc/traefik/dynamic/pangolite-dashboard.yml Config dinámica del dashboard
/etc/traefik/acme.json                   Certificados ACME
```

El servicio se crea según el init detectado:

```text
systemd  -> /etc/systemd/system/pangolite.service
OpenRC   -> /etc/init.d/pangolite + rc-update
SysVinit -> /etc/init.d/pangolite + update-rc.d/chkconfig si existe
runit    -> /etc/sv/pangolite + symlink a /var/service o /service
```

Si `install.sh` encuentra `/etc/traefik/traefik.yml` existente y no fue generado por Pangolite, crea backup antes de escribir.

## Primer acceso

El instalador imprime algo como:

```text
usuario=admin
password=<temporal>
```

También lo guarda en:

```text
/opt/pangolite/data/admin-password.txt
```

Al iniciar sesión por primera vez, Pangolite obliga a cambiar la contraseña. La contraseña nueva debe tener mínimo 6 caracteres.

Cuando se cambia la contraseña temporal, el archivo `admin-password.txt` se elimina automáticamente.

## Releases

El proyecto incluye un workflow manual en `.github/workflows/release.yml`.

- Se ejecuta manualmente desde GitHub Actions.
- Si se indica una versión, publica `vX.Y`.
- Si se deja vacío, toma el último tag `vX.Y` e incrementa el número menor.
- Genera paquetes `pangolite_linux_amd64.tar.gz`, `pangolite_linux_arm64.tar.gz`, `pangolite_linux_386.tar.gz` y `pangolite_linux_armv7.tar.gz`.
- Verifica que el paquete y el cliente Linux ARM64 existan y no estén vacíos antes de publicar.
- Publica `checksums.txt`.

## Configuración

Variables principales en `/opt/pangolite/pangolite.env`:

```env
PANGOLITE_ADDR=0.0.0.0:2424
PANGOLITE_DATA=/opt/pangolite/data/pangolite.db
PANGOLITE_TRAEFIK_DIR=/etc/traefik
PANGOLITE_PUBLIC_IP=<ip-detectada-por-init>
PANGOLITE_INITIAL_ADMIN_USER=admin
PANGOLITE_INITIAL_PASSWORD_FILE=/opt/pangolite/data/admin-password.txt
PANGOLITE_SESSION_DAYS=30
PANGOLITE_TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128
PANGOLITE_ADMIN_ACCESS_MODE=learn
PANGOLITE_AGENT_HTTP_CONCURRENCY=4
PANGOLITE_AGENT_STREAM_CONCURRENCY=16
PANGOLITE_L4_TCP_CONCURRENCY=512
PANGOLITE_L4_UDP_CONCURRENCY=256
# Opcional: tambien puedes definirlos por env, aunque lo recomendado es hacerlo desde Ajustes.
# PANGOLITE_DASHBOARD_DOMAIN=panel.midominio.com
# PANGOLITE_LETSENCRYPT_EMAIL=admin@midominio.com
```

Después de editar variables de entorno, reinicia Pangolite:

```bash
sudo systemctl restart pangolite
```

Los cambios hechos desde la UI aplican Traefik automáticamente cuando es posible.


## Dominio del dashboard

En **Ajustes > Dominio del dashboard** puedes definir el dominio publico del panel, por ejemplo:

```text
panel.midominio.com
```

Antes de guardar, Pangolite valida que el dominio resuelva a la IP detectada del servidor. Esa IP queda registrada en:

```env
PANGOLITE_PUBLIC_IP=<ip-publica>
```

Si el proveedor cambia la IP o el VPS usa una red especial, puedes editar `/opt/pangolite/pangolite.env` y reiniciar:

```bash
sudo systemctl restart pangolite
```

Después de guardar el dominio/correo ACME desde el panel, Pangolite escribe la configuración dinámica en `/etc/traefik/dynamic/`. Traefik la detecta automáticamente mediante `providers.file.watch=true`, sin reiniciar ni cortar recursos HTTP/HTTPS existentes.

El correo ACME también puede quedar configurado aunque todavía no publiques el panel con dominio propio. Esto permite crear recursos web con el switch **Usar SSL** activado. Si desactivas el switch en un recurso, Pangolite publica solo HTTP y no elimina certificados que Traefik ya haya generado.

Los recursos TCP/UDP no modifican la configuración estática de Traefik. Pangolite abre y cierra sus sockets públicos directamente; crear un puerto nuevo, cambiar el backend, suspender o eliminar un recurso L4 no reinicia Traefik.

Los recursos HTTP pueden activar individualmente **Mostrar loader Thowilabs**. Al activarlo, Traefik enruta únicamente ese recurso a Pangolite para transformar respuestas `text/html`; JSON, APIs, descargas, HEAD y contenido no HTML no se modifican. Los assets del loader se sirven desde una ruta reservada dentro del mismo prefijo del recurso (`/.pangolite/branding/`). Si el CSP del backend no permite estilos del propio origen, Pangolite omite la inyección en vez de relajar la política de seguridad.

### Compatibilidad de servicios en Linux

Pangolite conserva soporte para systemd, OpenRC, SysVinit y runit. El gestor de servicios solo se necesita para reinicios explícitos del propio servicio o para cambios estáticos de Traefik relacionados con el panel/ACME, no para CRUD de TCP/UDP.

En una actualización desde una versión anterior al plano L4 dinámico, el instalador realiza una migración coordinada una sola vez: renderiza `traefik.yml` sin entrypoints TCP/UDP, reinicia Traefik para liberar esos sockets y después reinicia Pangolite para tomarlos. Ese handoff puede interrumpir brevemente conexiones L4 durante la actualización, pero no vuelve a repetirse al administrar recursos. En actualizaciones posteriores el instalador compara `traefik.yml`: si la configuración estática no cambió, no reinicia Traefik.

## Recarga automática de Traefik

Pangolite instala o detecta Traefik del sistema y genera una configuración estática con:

```yaml
providers:
  http:
    endpoint: http://127.0.0.1:2424/api/v1/traefik-config
    pollInterval: 5s
  file:
    directory: /etc/traefik/dynamic
    watch: true
```

Esto significa:

- Recursos HTTP/HTTPS: Traefik consulta a Pangolite y se actualiza automáticamente.
- Dominio del dashboard: se escribe como archivo dinámico y Traefik lo recarga automáticamente.
- Suspensión 403/404/HTML: se aplica sin reiniciar.
- TCP/UDP: no pasan por Traefik; Pangolite mantiene listeners públicos dinámicos y aplica cambios en caliente.

El usuario no debe ejecutar `render-traefik` para el flujo normal del panel. Ese comando queda como herramienta de reparación/diagnóstico.

## Validación de puertos

Al crear recursos TCP/UDP, Pangolite valida:

1. Que no exista otro recurso con el mismo puerto público y protocolo.
2. Que el puerto no esté reservado para HTTP/HTTPS o el panel.
3. Que el puerto pueda abrirse en el sistema operativo.

Si un proceso externo usa el puerto, el panel responde con error antes de guardar el recurso.


## Suspension de recursos

Los recursos HTTP/HTTPS se pueden suspender sin borrarlos. Pangolite conserva la ruta en Traefik y responde desde el panel con una de estas opciones:

- `403`: acceso prohibido.
- `404`: no encontrado.
- HTML personalizado: pagina editable, usando presets como pago pendiente, mantenimiento o servicio suspendido.

Esto permite pausar un dominio del proyecto sin perder su configuracion.


## Auditoría y respaldos

La sección **Seguridad** del panel centraliza dos tareas operativas:

- **Auditoría:** registra cambios administrativos como crear/editar/eliminar proyectos, recursos, dominios, clientes NAT, rotar tokens, aplicar Traefik y crear respaldos. No guarda contraseñas ni tokens.
- **Respaldos SQLite:** crea copias consistentes de la base con `VACUUM INTO` en `PANGOLITE_BACKUP_DIR` o, por defecto, `/opt/pangolite/data/backups`. Cada copia puede verificarse desde la UI sobre una base temporal antes de restaurarla.
- **Copia remota opcional:** replica respaldos manual o automáticamente hacia WebDAV o cualquier endpoint S3-compatible mediante streaming. La configuración admite endpoints personalizados, prefijos, credenciales conservadas en servidor y prueba de conectividad. Hugging Face Storage Buckets se configura como un endpoint S3-compatible.
- **Acciones masivas por proyecto:** Recursos permite activar, suspender y ejecutar health checks sobre una selección; Clientes permite mantenimiento, reactivación y reconexión en lote. La pertenencia al proyecto se valida en servidor y Traefik se aplica una sola vez por operación cuando corresponde.

Para restaurar un respaldo, detén Pangolite, copia el archivo `.db` elegido sobre la base activa y vuelve a iniciar el servicio:

```bash
sudo systemctl stop pangolite
sudo cp /opt/pangolite/data/backups/ARCHIVO.db /opt/pangolite/data/pangolite.db
sudo systemctl start pangolite
```

## Comandos útiles

```bash
sudo systemctl status pangolite --no-pager
sudo journalctl -u pangolite -f
sudo /opt/pangolite/pangolite render-traefik
```

Healthcheck:

```bash
curl http://127.0.0.1:2424/healthz
```

## Desarrollo local

```bash
go mod tidy
go test ./...
go build -buildvcs=false -trimpath -ldflags='-s -w' -o bin/pangolite ./cmd/pangolite
go run ./cmd/pangolite serve --addr 127.0.0.1:2424 --data ./data/pangolite.db
```

`-buildvcs=false` evita que el resultado del binario dependa de metadatos VCS durante builds y releases.

## Seguridad

- No se usa token administrativo global.
- Las contraseñas se guardan con bcrypt y las sesiones se guardan hasheadas en SQLite.
- Las cookies son `HttpOnly`, `SameSite=Lax` y solo confían en `X-Forwarded-Proto` cuando la conexión llega desde un proxy configurado como confiable.
- Las operaciones administrativas requieren CSRF y las respuestas de autenticación/API usan `Cache-Control: no-store`.
- El login conserva el bloqueo por usuario y origen (5 fallos durante 10 minutos) y añade un límite por IP de 30 intentos durante 10 minutos antes de ejecutar bcrypt.
- La recuperación de contraseña tiene límites independientes y solo genera enlaces cuando existe un dominio de dashboard configurado; no se construyen enlaces de recuperación desde un `Host` arbitrario.
- `PANGOLITE_TRUSTED_PROXY_CIDRS` define qué proxies pueden aportar `X-Forwarded-For`, `X-Real-IP` y `X-Forwarded-Proto`. El valor por defecto solo confía en loopback.
- `PANGOLITE_ADMIN_ACCESS_MODE=learn` liga cada sesión a la IP exacta con la que se inició. Si la IP cambia, la sesión deja de autorizar el panel y se solicita la contraseña otra vez. Un login correcto desde cualquier IP nueva está permitido y crea una sesión nueva ligada a esa IP.
- `PANGOLITE_ADMIN_ACCESS_MODE=off` desactiva el enlace de la sesión a la IP. No existe una lista administrativa de CIDR: cambiar de ISP, VPN o red no requiere editar el servidor, solo volver a autenticarse.
- Los WebSockets administrativos usan validación de origen y ya no desactivan la comprobación automática del paquete WebSocket.
- El servidor limita cabeceras y conexiones ociosas. Los clientes actuales anuncian `http-stream-v1`: uploads y downloads HTTP se transmiten por streaming con buffers pequeños y ya no tienen un límite fijo de 16 MiB. El servidor conserva el protocolo legado de 16 MiB solo para agentes antiguos hasta que se actualicen. `PANGOLITE_AGENT_HTTP_CONCURRENCY` limita las solicitudes HTTP remotas simultáneas (4 por defecto).
- Los recursos TCP/SSH/SFTP no tienen un timeout de duración fijo: los 30 segundos se usan únicamente para esperar que el agente adjunte el WebSocket. Una vez conectado, la sesión dura mientras los extremos sigan vivos. Pangolite aplica keepalive TCP/WebSocket y `TCP_NODELAY`. `PANGOLITE_AGENT_STREAM_CONCURRENCY` limita los streams TCP remotos (16 por defecto), mientras `PANGOLITE_L4_TCP_CONCURRENCY` limita conexiones TCP públicas globales (512 por defecto) y `PANGOLITE_L4_UDP_CONCURRENCY` limita trabajos UDP remotos simultáneos (256 por defecto).
- Traefik aplica al panel un rate limit de 30 solicitudes/s con ráfaga 60 y un máximo de 64 solicitudes simultáneas. Esto reduce abuso HTTP, pero no sustituye protección DDoS volumétrica del proveedor, firewall o red perimetral.
- La contraseña temporal se elimina al cambiarla, los puertos públicos se validan antes de persistir recursos TCP/UDP y las acciones administrativas críticas quedan registradas en auditoría.
- Los respaldos SQLite se crean con `VACUUM INTO` desde el panel de Seguridad.

### Recomendaciones para producción

- Expón públicamente solo los puertos necesarios. Para el panel normal, publica 80/443 por Traefik; evita exponer `2424/tcp` a Internet si no necesitas acceso directo/fallback de agentes.
- Activa firewall del host/proveedor y limita SSH a redes administrativas conocidas. La aplicación no puede absorber por sí sola un ataque que sature el enlace antes de llegar al proceso.
- Mantén Go y Traefik actualizados, revisa logs/auditoría y conserva respaldos fuera del host.
- En un VPS cercano a 500 MiB/1 vCPU, conserva inicialmente `PANGOLITE_AGENT_HTTP_CONCURRENCY=4` y `PANGOLITE_AGENT_STREAM_CONCURRENCY=16`; reduce streams a 8 si publicas muchos puertos hostiles o detectas presión sostenida de CPU/RAM.
- Prefiere `https://` para `PANGOLITE_SERVER_URL` y para cualquier fallback atravesando redes no confiables. El modo legado `http://IP:2424` mantiene compatibilidad, pero transporta las credenciales del agente sin cifrado TLS y debe limitarse por firewall/red confiable.
- Si el panel queda accesible a más administradores o redes, el siguiente endurecimiento recomendado es MFA y gestión explícita de redes confiables desde la UI.

## Diferencia entre cliente de sistema y recurso

**Cliente de sistema**: identidad instalada en un servidor remoto o NAT. Tiene ID y token. No publica nada por sí solo.

**Recurso**: servicio que se expone. Puede ser HTTP/HTTPS/TCP/UDP. HTTP/HTTPS se publica mediante Traefik; TCP/UDP usa listeners dinámicos del propio Pangolite. El backend puede vivir en este servidor o en un servidor remoto conectado.

**Servicio interno**: host y puerto reales del servicio, por ejemplo `127.0.0.1:22` o `127.0.0.1:8080`.

## Licencia

Pangolite usa una licencia permisiva tipo MIT. Puedes usar, modificar y distribuir el proyecto siguiendo los terminos del archivo `LICENSE`.

## Certificados del dashboard

Si el navegador muestra `TRAEFIK DEFAULT CERT`, Traefik todavía no tiene un certificado ACME válido para ese dominio o no pudo usarlo.

Revisión rápida:

```bash
sudo ls -l /etc/traefik/acme.json
sudo grep -n "TU_DOMINIO" /etc/traefik/acme.json || true
sudo journalctl -u traefik -n 200 --no-pager | grep -iE 'acme|certificate|error|TU_DOMINIO'
```

Requisitos:

- El dominio del dashboard debe resolver a la IP pública del servidor.
- Los puertos 80 y 443 deben llegar a este servidor.
- `/etc/traefik/acme.json` debe existir y tener permisos `0600`.
- El correo ACME debe ser real, no `example.com`.

Pangolite aplica automáticamente cambios HTTP/HTTPS. Si cambias el correo ACME o activas ACME por primera vez, Pangolite escribe la configuración estática y reinicia Traefik de forma controlada. El panel muestra si el certificado está generado, pendiente/en proceso, desactivado o si falta configurar ACME.


## Edicion de recursos y selector de proyectos

Cada recurso tiene boton **Editar** desde la tabla del proyecto. Los cambios HTTP/HTTPS se aplican por configuracion dinamica de Traefik. Los cambios TCP/UDP se aplican directamente sobre los listeners de Pangolite; no requieren reiniciar Traefik. El sidebar ahora usa selector desplegable con busqueda y puede ocultarse desde el topbar.

### Dashboard global y selector de proyecto

La ruta `/projects` funciona como dashboard global. Muestra métricas agregadas, gráficos de recursos por proyecto y estado de recursos usando Chart.js por CDN. El selector de proyecto vive arriba del sidebar porque define el contexto de todo el flujo: recursos, clientes de sistema y acciones rápidas.

Si el navegador administrador no tiene acceso al CDN, el panel sigue funcionando y muestra un fallback textual para los gráficos.

### Pulido de producto

El panel evita textos internos de desarrollo en la interfaz visible. El sidebar muestra marca de producto, selector de proyecto y estado operativo. El dashboard global incluye un bloque de operación para revisar dominio del panel, IP pública detectada y validación DNS sin entrar a la configuración.

## Clientes NAT y túneles remotos

Pangolite incluye un binario de cliente NAT independiente. El instalador principal descarga y publica el cliente en:

```text
/opt/pangolite/pangolite-client
/opt/pangolite/public/pangolite-client-linux-amd64
```

Al crear o rotar un cliente desde el panel, Pangolite muestra un comando listo para copiar en el servidor remoto:

```bash
curl -fsSL https://panel.midominio.com/download/pangolite-client-linux-amd64 -o /tmp/pangolite-client \
  && chmod +x /tmp/pangolite-client \
  && sudo /tmp/pangolite-client --install --server-url https://panel.midominio.com --agent-id ID --token TOKEN
```

El cliente detecta systemd u OpenRC, se copia a `/opt/pangolite-client/`, guarda sus credenciales en un archivo privado y arranca como servicio. `--install` es idempotente: si encuentra una instalación anterior, detiene y retira el servicio previo, reemplaza binario/configuración y vuelve a registrarlo con el init detectado. Esto permite ejecutar de nuevo el comando generado por el panel para actualizar el cliente sin limpiar manualmente una instalación anterior.

Para eliminarlo completamente del servidor remoto:

```bash
sudo /opt/pangolite-client/pangolite-client --remove
```

En Windows, `pangolite-client.exe --install ...` y `pangolite-client.exe --remove` comprueban si el proceso está elevado. Si no lo está, el propio CLI solicita UAC, relanza la misma operación con privilegios administrativos y espera su resultado. La instalación usa el Service Control Manager de Windows para detener/eliminar una instancia previa y crear de nuevo `PangoliteClient`; no depende de interpretar la salida de `sc.exe`. El servicio queda además con recuperación automática escalonada (5 s, 15 s y 30 s) ante fallos.

Capacidades iniciales del cliente NAT:

- HTTP/HTTPS remoto detrás de NAT mediante cola saliente.
- TCP remoto detrás de NAT mediante stream persistente sobre WebSocket.
- UDP remoto mediante intercambio de datagramas de solicitud/respuesta.
- Heartbeat mediante polling autenticado.
- Rotación de token desde el panel.
- Instalación y eliminación automática del servicio del cliente.

Para recursos TCP/UDP remotos, Pangolite escucha directamente en el puerto público. TCP se conecta a `TunnelHub` y viaja como stream WebSocket autenticado hacia el cliente; UDP se transporta como jobs de datagrama autenticados. No se crea un puerto puente interno y no se requiere abrir puertos en el servidor remoto.

## Logs operativos

Pangolite escribe eventos del panel en stdout y en un archivo persistente configurado por `PANGOLITE_LOG_FILE`.

Por defecto, `install.sh` usa binarios de release. Para desarrollo, `init.sh` usa:

```env
PANGOLITE_LOG_FILE=/opt/pangolite/data/pangolite.log
```

El archivo se mantiene en maximo 1000 entradas para evitar crecimiento indefinido. Desde el panel se pueden revisar en:

```text
/logs
```

Tambien se pueden consultar por API autenticada:

```text
GET /api/system/logs?limit=300
```

Los errores de validacion de puertos TCP/UDP registran modo, puerto publico, origen, cliente NAT y usuario que ejecuto la accion.

### Nota operativa sobre TCP/UDP

Los recursos TCP/UDP son gestionados por `PublicL4Manager` dentro de Pangolite. La sincronización reserva primero todos los sockets nuevos; si alguno no puede abrirse, los listeners ya activos permanecen intactos y el reconciliador vuelve a intentar la aplicación periódicamente. Cambiar solamente el backend no cierra el socket público: las conexiones TCP ya aceptadas pueden terminar con el backend anterior y las nuevas usan el backend actualizado.

TCP local usa proxy dúplex con keepalive, `TCP_NODELAY` y half-close. TCP de clientes NAT usa WebSocket autenticado directamente desde el listener público hacia `TunnelHub`. UDP local mantiene pseudo-sesiones por origen con expiración por inactividad; UDP remoto conserva el intercambio autenticado de datagramas con concurrencia acotada.

## Experiencia del panel

- Los comandos de instalación y eliminación del cliente NAT se muestran en bloques de código con botón para copiar al portapapeles.
- Las acciones sensibles del panel usan un modal de confirmación propio: eliminar recursos, eliminar dominios, deshabilitar clientes, rotar tokens y suspensión/activación rápida.
- Al crear o editar recursos, el panel muestra un modal de progreso mientras valida puertos, guarda cambios y aplica Traefik automáticamente.

### Eliminación de recursos y aplicación en caliente

La eliminación de recursos es idempotente para evitar errores por doble clic, reintentos del navegador o confirmaciones repetidas. Cuando se elimina un recurso desde el panel, la tabla se actualiza localmente de inmediato y luego se sincroniza con la API.

Para TCP/UDP, eliminar o suspender un recurso cierra únicamente su listener público. No se reinicia Traefik ni Pangolite y los demás puertos continúan atendiendo tráfico.


## Clientes NAT

Pangolite compila y publica clientes NAT para Linux amd64, arm64, 386 y armv7, además de Windows amd64. Al crear o rotar un cliente desde el panel se generan comandos listos para copiar.

- Linux: instala en `/opt/pangolite-client` y registra el servicio en systemd u OpenRC.
- Windows: instala en `C:\ProgramData\Pangolite Client` y registra el servicio `PangoliteClient`.
- Ambos modos soportan eliminación completa con `--remove`.

El panel muestra estado online/offline, última conexión, sistema operativo, arquitectura, hostname/IP y recursos asociados al cliente.

La terminal remota Linux detecta la shell disponible en lugar de asumir `/bin/sh`. Resuelve `SHELL`, shells Linux comunes, `sh` mediante `PATH` y rutas habituales de Android como `/system/bin/sh`. El proceso de la terminal se inicia mediante una ruta compatible con kernels Linux antiguos para evitar que `pidfd_open` cierre el cliente completo en dispositivos Android heredados.

En Android táctil, la consola web muestra una barra auxiliar inspirada en Termux encima del teclado virtual con `ESC`, `CTRL`, `ALT`, `TAB`, `HOME`, `END`, flechas, `PGUP`/`PGDN`, símbolos frecuentes y un botón para mostrar u ocultar el teclado. `CTRL` y `ALT` funcionan como modificadores de un solo uso para la siguiente tecla o carácter, lo que permite enviar combinaciones como `Ctrl+C`, `Ctrl+D`, `Alt+X` o `Ctrl+←` sin teclado físico.

La terminal también permite subir archivos al directorio actual del shell. En escritorio se pueden arrastrar uno o varios archivos sobre la consola; en Android y escritorio **Subir archivo** vive dentro del menú de opciones. La transferencia usa bloques pequeños por el WebSocket ya autenticado de la terminal, muestra el archivo activo en una ventana de progreso flotante que no cambia el alto de xterm, emite avisos temporales al completar cada elemento/lote y no carga el archivo completo en memoria. El cliente remoto resuelve el directorio actual del PTY en Linux, por lo que después de `cd /ruta/destino` la subida se guarda en esa ruta. Los nombres se sanean, las subidas incompletas permanecen como temporales ocultos y se eliminan al cancelar/cerrar la sesión, y un archivo existente nunca se sobrescribe: se genera un nombre como `archivo (1).zip`.

Para descargar desde la consola se usa `download ruta`. No se instala ningún ejecutable `download` en el servidor o cliente: el navegador intercepta esa línea antes de que el shell la ejecute, incluso cuando se pega el comando, se recupera desde el historial o la ruta se completa con `Tab`; cuando el shell modifica la línea, Pangolite toma como autoridad la línea real ya renderizada por xterm antes de preparar la descarga. Pangolite resuelve la ruta contra el directorio actual del PTY y entrega un ticket HTTP autenticado, temporal y de un solo uso. Los archivos regulares se transmiten directamente al navegador sin cargarlos completos en RAM. Si la ruta es un directorio, se genera un ZIP en streaming con `archive/zip`: no se siguen symlinks ni se incluyen sockets/FIFOs/dispositivos, se bloquean raíces/directorios sensibles del sistema (incluido `/var` completo, permitiendo rutas de datos concretas como `/var/www`) y se limita el árbol a 20.000 entradas y 8 GiB sin comprimir. Los formatos ya comprimidos y los archivos de 128 MiB o más se almacenan sin recomprimir; los archivos menores potencialmente compresibles usan Deflate de velocidad alta. Además, solo puede existir un ZIP activo por proceso y se permiten como máximo dos descargas de terminal simultáneas, evitando que un VPS pequeño quede monopolizado por compresión concurrente.

En terminales remotas las funciones se negocian mediante `terminal-upload-v1` y `terminal-download-v1`. Un servidor nuevo no envía protocolos de archivo a clientes antiguos: muestra un error solicitando actualizar `pangolite-client`, evitando que controles o bytes de transferencia terminen accidentalmente como entrada del shell.

El toolbar de terminal mantiene **Pantalla completa** como única acción principal permanente durante la sesión; **Destino** y **Tema** viven dentro del widget de ajustes, **Subir archivo**/**Desconectar** dentro del menú de opciones y **Conectar** solo aparece cuando hace falta iniciar o reintentar la sesión. Esta jerarquía debe conservarse en cambios futuros para no volver a saturar la consola, especialmente en móvil.

## Health checks

La vista de recursos incluye una acción para probar disponibilidad básica de recursos HTTP/TCP y confirmar si el cliente NAT requerido está conectado.


## Nota de UI

Los formularios principales del panel se manejan por JavaScript y llamadas JSON a la API para evitar submits HTML accidentales.

## Operaciones destructivas seguras

Pangolite incluye una Zona de peligro por proyecto. Desde ahí se puede renombrar el proyecto, cambiar su descripción y eliminarlo solo cuando no tenga recursos ni clientes de sistema vinculados.

La eliminación de clientes de sistema es una acción fuerte: elimina el cliente de sistema y todos los recursos vinculados a ese cliente de sistema. Para evitar errores, el panel solicita la contraseña del administrador actual antes de ejecutar la eliminación.

Los cambios que eliminan recursos HTTP se reflejan por la configuración dinámica de Traefik. Para TCP/UDP, Pangolite retira directamente el listener correspondiente sin reiniciar Traefik ni los demás puertos.

### Suspensión avanzada y protección de recursos

Los recursos web pueden suspenderse desde la tabla de acciones con un único botón. Al suspender, Pangolite permite elegir entre una suspensión simple, una plantilla HTML física existente o HTML personalizado validado.

Las plantillas de suspensión se guardan como archivos `.html` en `PANGOLITE_SUSPENSION_TEMPLATE_DIR`; si no se define, se usa `/opt/pangolite/data/templates/suspension`. Las plantillas pueden usar variables seguras como `$nombredominio`, `$nombrerecurso`, `$proyecto`, `$codigo`, `$motivo` y `$fecha`.

El validador rechaza etiquetas y atributos peligrosos como `script`, `iframe`, `object`, `embed`, `svg`, formularios, atributos `on*`, URLs `javascript:` y `data:text/html`.

También se puede proteger un recurso web antes de enviarlo al backend:

- **Contraseña específica:** muestra una pantalla HTML de login o, si se elige, un prompt básico útil para APIs.
- **Sesión Pangolite:** solo permite pasar a usuarios con sesión iniciada en Pangolite desde ese dominio.

Cuando un recurso web tiene protección activa, Traefik lo enruta primero por Pangolite para validar el acceso y después Pangolite lo proxyfica hacia el backend local o remoto.

## Diagnóstico y operación

Ejecuta un diagnóstico rápido de la instalación:

```bash
sudo /opt/pangolite/pangolite doctor
```

El diagnóstico revisa SQLite, migraciones, rutas escribibles, Traefik, puertos 80/443 y servicios.

### Restablecer contraseña de un usuario desde CLI

El mismo binario del servidor incluye un subcomando de recuperación; no hace falta instalar ni mantener un segundo ejecutable. Para restablecer la contraseña del usuario `admin`:

```bash
sudo /opt/pangolite/pangolite user reset-password admin
```

La contraseña se solicita dos veces directamente desde la terminal y no se muestra en pantalla. El comando usa `PANGOLITE_DATA` para localizar SQLite; si necesitas operar sobre otra base puedes indicar `--data /ruta/pangolite.db`.

Para automatización se puede leer la contraseña desde stdin sin exponerla como argumento del proceso:

```bash
printf '%s\n' "$NUEVA_PASSWORD" | sudo /opt/pangolite/pangolite user reset-password --password-stdin admin
```

No se admite la nueva contraseña como argumento posicional porque terminaría expuesta con facilidad en el historial de la shell o en herramientas que inspeccionan la línea de comandos. Después del reset, Pangolite invalida todas las sesiones y enlaces de recuperación existentes de ese usuario. `--require-change` permite marcar la contraseña como temporal y exigir otro cambio al iniciar sesión.

La jerarquía `pangolite user ...` queda reservada para futuras operaciones multiusuario como listar, crear, deshabilitar, eliminar o revocar sesiones sin cambiar el modo `serve` ni crear otro binario.

Los respaldos automáticos se configuran con:

```env
PANGOLITE_BACKUP_INTERVAL_HOURS=24
PANGOLITE_BACKUP_RETENTION_DAYS=14
```

Usa `0` en `PANGOLITE_BACKUP_INTERVAL_HOURS` para desactivar respaldos automáticos. Los respaldos automáticos antiguos se limpian según la retención configurada.

El instalador verifica `checksums.txt` del release antes de instalar el paquete descargado. Si el checksum no coincide, la instalación se cancela.

Los releases incluyen clientes para:

- Linux amd64
- Linux arm64
- Linux 386
- Linux armv7
- Windows amd64

## Frontend del panel

El panel ya no vive como un string gigante dentro de `internal/app/ui.go`. La interfaz se organiza en templates Go embebidos y assets estáticos:

```txt
internal/app/templates/layouts/
internal/app/templates/components/
internal/app/templates/pages/
internal/app/assets/app/
```

La navegación principal usa rutas explícitas administradas por Go (`/projects`, `/projects/{id}/resources`, `/ssh`, `/terminal`, `/settings`, etc.). El servidor valida autenticación, proyecto y parámetros antes de renderizar; las rutas desconocidas responden `404`. JavaScript únicamente hidrata la vista indicada por el `PageKey` emitido por Go y no funciona como router. No requiere Node, Vite ni bundler; el binario sigue siendo autocontenido mediante `embed.FS`.

