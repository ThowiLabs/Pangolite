# Pangolite

Pangolite es un panel ligero de proxy y túneles inspirado en Pangolin, escrito en Go para servidores Linux. Está pensado para administrar proyectos/clientes, dominios, recursos HTTP/HTTPS/TCP/UDP y clientes de sistema para redes NAT/remotas, usando Traefik instalado directamente en el sistema.

Repositorio previsto:

```text
github.com/thowilabs/pangolite
```

## Estado del proyecto

Pangolite está en fase inicial de producto. La base actual incluye:

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
- Render de configuración Traefik del sistema.

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
- Traefik instalado si se quiere publicar HTTP/HTTPS/TCP/UDP por puertos públicos.
- Go >= 1.23, o internet para que `init.sh` descargue Go temporalmente.

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
/etc/traefik/traefik.yml                 Config estática renderizada
/etc/traefik/pangolite-dynamic-base.yml  Config base dinámica
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

Después de editar:

```bash
sudo systemctl restart pangolite
sudo /opt/pangolite/pangolite render-traefik
sudo systemctl restart traefik
```


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

Despues de guardar el dominio/correo ACME desde el panel:

```bash
sudo /opt/pangolite/pangolite render-traefik
sudo systemctl restart traefik
```

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
sudo systemctl restart traefik
```

Healthcheck:

```bash
curl http://127.0.0.1:2424/healthz
```

## Desarrollo local

```bash
go mod tidy
go test ./...
go run ./cmd/pangolite serve --addr 127.0.0.1:2424 --data ./data/pangolite.db
```

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
