# Pangolite

Pangolite es una plataforma de administración de proxys y túneles escrita en Go para servidores Linux. Permite administrar proyectos/clientes, dominios, recursos HTTP/HTTPS/TCP/UDP y servidores conectados, usando Traefik instalado directamente en el sistema.

Repositorio previsto:

```text
github.com/thowilabs/pangolite
```

## Estado del proyecto

La base actual incluye:

- Panel web en Go.
- SQLite embebido para usuarios, sesiones, proyectos, dominios, clientes de sistema y recursos.
- Login con usuario `admin` y contraseña temporal.
- Cambio obligatorio de contraseña en primer acceso.
- Sesiones persistentes con cookie segura.
- CSRF en operaciones administrativas.
- CRUD de proyectos/clientes.
- CRUD de dominios administrados.
- Configuracion del dominio publico del dashboard con validacion DNS contra la IP del servidor.
- Clientes de sistema/agentes para servidores NAT/remotos.
- Recursos HTTP/HTTPS locales o mediante cliente de sistema.
- Recursos TCP/UDP directos del host Pangolite.
- Validación de puerto público contra recursos existentes y contra puertos ocupados en el sistema.
- Suspension de recursos HTTP/HTTPS con respuesta 403, 404 o HTML personalizado basado en plantillas editables.
- Instalación y configuración de Traefik del sistema desde `init.sh`.
- Recarga automática de HTTP/HTTPS mediante file provider con `watch=true`.
- Aplicación automática de cambios desde la UI; no se pide al usuario aplicar Traefik manualmente.

TCP/UDP mediante cliente de sistema requiere la fase de streams remotos y por ahora está bloqueado para evitar una configuración engañosa.

## Arquitectura

```text
Internet
  ↓
Traefik del sistema
  ↓
Pangolite / recurso local
  ↓
Servicio interno
```

Para servicios detrás de NAT:

```text
Internet
  ↓
Traefik del sistema
  ↓
Pangolite
  ↓ conexión saliente
Cliente de sistema Pangolite
  ↓
Servicio interno remoto
```

## Requisitos

Servidor Linux con:

- `systemd`
- `curl`
- `tar`
- Go >= 1.23, o internet para que `init.sh` descargue Go temporalmente.
- Internet para que `init.sh` descargue Traefik si no está instalado.

El instalador usa Go temporal si no encuentra una versión compatible y borra los archivos temporales al terminar.

## Instalación rápida

```bash
unzip pangolite-system.zip
cd pangolite
sudo bash init.sh
```

El panel arranca en la IP detectada por el instalador:

```text
http://<IP_DEL_SERVIDOR>:2424
```

`init.sh` intenta detectar la IP publica real y la imprime al terminar. Si no puede, usa la IP local de salida como respaldo.

No hay redirección HTTPS inicial. La publicación por dominio/HTTPS se configura después desde Traefik y Pangolite.

## Ubicaciones

```text
/opt/pangolite/pangolite                 Binario
/opt/pangolite/data/pangolite.db         SQLite
/opt/pangolite/data/admin-password.txt   Contraseña temporal inicial
/opt/pangolite/pangolite.env             Variables de entorno
/etc/systemd/system/pangolite.service    Servicio systemd
/etc/traefik/traefik.yml                 Config estática gestionada
/etc/traefik/dynamic/pangolite-dashboard.yml Config dinámica del dashboard
/etc/traefik/acme.json                   Certificados ACME
```

Si `init.sh` encuentra `/etc/traefik/traefik.yml` existente y no fue generado por Pangolite, crea backup antes de escribir.

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
# Opcional: tambien puedes definirlos por env, aunque lo recomendado es hacerlo desde Ajustes.
# PANGOLITE_DASHBOARD_DOMAIN=pangolin.example.com
# PANGOLITE_LETSENCRYPT_EMAIL=admin@example.com
```

Después de editar variables de entorno, reinicia Pangolite:

```bash
sudo systemctl restart pangolite
```

Los cambios hechos desde la UI aplican Traefik automáticamente cuando es posible.


## Dominio del dashboard

En **Ajustes > Dominio del dashboard** puedes definir el dominio publico del panel, por ejemplo:

```text
pangolin.yahirex.us.kg
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

Solo los cambios que agregan o eliminan puertos TCP/UDP públicos requieren tocar entrypoints estáticos. Pangolite lo detecta y ejecuta un reinicio controlado de Traefik automáticamente.

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
- TCP/UDP nuevos: requieren entrypoints estáticos; Pangolite valida puertos, escribe la config y reinicia Traefik de forma automática.

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

Esto permite pausar un dominio de cliente sin perder su configuracion.

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

`-buildvcs=false` es intencional para que los ZIPs limpios sin `.git` compilen de forma reproducible.

## Seguridad

- No se usa token administrativo global.
- Las contraseñas se guardan con bcrypt.
- Las sesiones se guardan hasheadas en SQLite.
- Las cookies son `HttpOnly` y `SameSite=Lax`.
- Las operaciones administrativas requieren CSRF.
- La contraseña temporal se elimina al cambiarla.
- Los puertos públicos se validan antes de persistir recursos TCP/UDP.

## Diferencia entre cliente de sistema y recurso

**Cliente de sistema**: identidad instalada en un servidor remoto o NAT. Tiene ID y token. No publica nada por sí solo.

**Recurso**: servicio que se expone. Puede ser HTTP/HTTPS/TCP/UDP. Decide si el servicio interno vive en este servidor Pangolite o en un servidor remoto conectado.

**Servicio interno**: host y puerto reales del servicio, por ejemplo `127.0.0.1:22` o `127.0.0.1:8080`.

## Licencia

Pendiente de definir.

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

Pangolite aplica automáticamente cambios HTTP/HTTPS. Si cambias el correo ACME o activas ACME por primera vez, Pangolite escribe la configuración estática y reinicia Traefik de forma controlada.


## Edicion de recursos y selector de proyectos

Cada recurso tiene boton **Editar** desde la tabla del proyecto. Los cambios HTTP/HTTPS se aplican por configuracion dinamica de Traefik sin reiniciar. Si cambia un puerto publico TCP/UDP, Pangolite reinicia Traefik de forma controlada porque cambia un entrypoint estatico. El sidebar ahora usa selector desplegable con busqueda y puede ocultarse desde el topbar.

### Dashboard global y selector de proyecto

La ruta `/projects` funciona como dashboard global. Muestra métricas agregadas, gráficos de recursos por proyecto y estado de recursos usando Chart.js por CDN. El selector de proyecto vive arriba del sidebar porque define el contexto de todo el flujo: recursos, clientes de sistema y acciones rápidas.

Si el navegador administrador no tiene acceso al CDN, el panel sigue funcionando y muestra un fallback textual para los gráficos.

### Pulido de producto

El panel evita textos internos de desarrollo en la interfaz visible. El sidebar muestra marca de producto, selector de proyecto y estado operativo. El dashboard global incluye un bloque de operación para revisar dominio del panel, IP pública detectada y validación DNS sin entrar a la configuración.

## Clientes NAT y túneles remotos

Pangolite incluye un binario de cliente NAT independiente. El instalador principal compila y publica el cliente en:

```text
/opt/pangolite/pangolite-client
/opt/pangolite/public/pangolite-client-linux-amd64
```

Al crear o rotar un cliente desde el panel, Pangolite muestra un comando listo para copiar en el servidor remoto:

```bash
curl -fsSL https://panel.example.com/download/pangolite-client-linux-amd64 -o /tmp/pangolite-client \
  && chmod +x /tmp/pangolite-client \
  && sudo /tmp/pangolite-client --install --server-url https://panel.example.com --agent-id ID --token TOKEN
```

El cliente detecta systemd u OpenRC, se copia a `/opt/pangolite-client/`, guarda sus credenciales en un archivo privado y arranca como servicio. Para eliminarlo completamente del servidor remoto:

```bash
sudo /opt/pangolite-client/pangolite-client --remove
```

Capacidades iniciales del cliente NAT:

- HTTP/HTTPS remoto detrás de NAT mediante cola saliente.
- TCP remoto detrás de NAT mediante stream persistente sobre WebSocket.
- UDP remoto mediante intercambio de datagramas de solicitud/respuesta.
- Heartbeat mediante polling autenticado.
- Rotación de token desde el panel.
- Instalación y eliminación automática del servicio del cliente.

Para recursos TCP/UDP remotos, Pangolite crea un puerto interno local de puente, Traefik publica el puerto público y el cliente NAT abre una conexión saliente hacia el panel. No se requiere abrir puertos en el servidor remoto.

## Logs operativos

Pangolite escribe eventos del panel en stdout y en un archivo persistente configurado por `PANGOLITE_LOG_FILE`.

Por defecto, `init.sh` usa:

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

Los recursos TCP/UDP requieren entrypoints estáticos en Traefik. Pangolite escribe la configuración y programa el reinicio controlado de Traefik en segundo plano para que la API responda antes de aplicar el cambio. Los cambios HTTP/HTTPS continúan aplicándose por configuración dinámica sin reinicio.

Los túneles TCP de clientes NAT usan WebSocket autenticado entre el cliente y Pangolite para transportar el stream bidireccional.

## Experiencia del panel

- Los comandos de instalación y eliminación del cliente NAT se muestran en bloques de código con botón para copiar al portapapeles.
- Las acciones sensibles del panel usan un modal de confirmación propio: eliminar recursos, eliminar dominios, deshabilitar clientes, rotar tokens y suspensión/activación rápida.
- Al crear o editar recursos, el panel muestra un modal de progreso mientras valida puertos, guarda cambios y aplica Traefik automáticamente.

### Eliminación de recursos y Traefik

La eliminación de recursos es idempotente para evitar errores por doble clic, reintentos del navegador o confirmaciones repetidas. Cuando se elimina un recurso desde el panel, la tabla se actualiza localmente de inmediato y luego se sincroniza con la API.

Para recursos TCP/UDP, Pangolite agrupa los reinicios de Traefik durante unos segundos. Esto evita reinicios repetidos al eliminar varios recursos seguidos y reduce cortes temporales si el panel se usa detrás del mismo Traefik.


## Clientes NAT

Pangolite compila y publica clientes NAT para Linux amd64 y Windows amd64. Al crear o rotar un cliente desde el panel se generan comandos listos para copiar.

- Linux: instala en `/opt/pangolite-client` y registra el servicio en systemd u OpenRC.
- Windows: instala en `C:\ProgramData\Pangolite Client` y registra el servicio `PangoliteClient`.
- Ambos modos soportan eliminación completa con `--remove`.

El panel muestra estado online/offline, última conexión, sistema operativo, arquitectura, hostname/IP y recursos asociados al cliente.

## Health checks

La vista de recursos incluye una acción para probar disponibilidad básica de recursos HTTP/TCP y confirmar si el cliente NAT requerido está conectado.


## Nota de UI

Los formularios principales del panel se manejan por JavaScript y llamadas JSON a la API para evitar submits HTML accidentales.
