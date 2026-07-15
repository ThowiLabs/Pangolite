# Arquitectura de Pangolite

Pangolite es un plataforma de administración en Go inspirado en Pangolin. El objetivo es exponer recursos de proyectos usando Traefik como edge proxy y SQLite como almacenamiento local.

## Componentes

- **Pangolite panel:** servidor Go con UI, API, sesiones, proyectos y recursos.
- **SQLite:** base embebida para datos del panel.
- **Traefik del sistema:** proxy público HTTP/HTTPS/TCP/UDP.
- **Cliente de sistema:** agente instalado en servidores NAT/remotos.
- **Recurso:** servicio publicado, por ejemplo una app web, SSH, TCP o UDP.

## Flujos

### Recurso local

```text
Internet -> Traefik -> servicio alcanzable desde el VPS Pangolite
```

### Recurso remoto HTTP

```text
Internet -> Traefik -> Pangolite -> cliente de sistema -> servicio remoto HTTP
```

### Recurso TCP remoto

```text
Internet -> Traefik TCP entrypoint -> Pangolite bridge 127.0.0.1:<tunnel_port> -> WebSocket stream -> cliente de sistema -> servicio TCP remoto
```

### Recurso UDP remoto

```text
Internet -> Traefik UDP entrypoint -> Pangolite bridge 127.0.0.1:<tunnel_port> -> job/datagrama autenticado -> cliente de sistema -> servicio UDP remoto
```

## Seguridad operativa

- Las acciones administrativas críticas se registran en `audit_events`.
- Los respaldos SQLite se generan con `VACUUM INTO` para obtener una copia consistente sin detener el panel. La creación usa el modal reutilizable del panel para pedir un prefijo opcional y no ejecuta nada si se cancela.
- La restauración se mantiene como operación manual segura: detener servicio, reemplazar base y reiniciar.


## Onboarding y proyecto inicial

Las instalaciones nuevas no crean un proyecto por defecto. El panel muestra un onboarding cuando no hay proyectos registrados y guía al administrador para crear el primer proyecto antes de crear dominios, clientes de sistema o recursos. En instalaciones actualizadas, si existen recursos o agentes legados sin proyecto, se conserva un proyecto `default` solo para no perder asociaciones previas.

## Instalación y releases

El instalador principal es `install.sh`. A diferencia de `init.sh`, no compila en el servidor final: detecta el sistema, descarga el paquete adecuado desde GitHub Releases y crea servicios según el gestor de arranque disponible.

Gestores soportados inicialmente:

```text
systemd  -> servicios .service
OpenRC   -> scripts /etc/init.d + rc-update
SysVinit -> scripts /etc/init.d + update-rc.d/chkconfig si existe
runit    -> servicios /etc/sv + symlink a /var/service o /service
```

El workflow manual de releases genera paquetes Linux `amd64`, `arm64`, `386` y `armv7`, además de clientes descargables para Linux amd64 y Windows amd64.

## Suspensión y protección de recursos web

Pangolite soporta suspensión de recursos HTTP por respuesta simple, plantilla HTML física o HTML personalizado. Las plantillas viven en disco bajo `PANGOLITE_SUSPENSION_TEMPLATE_DIR` y se crean con defaults editables si el directorio está vacío.

Para evitar inyección accidental, el panel valida HTML de suspensión antes de guardarlo o aplicarlo a un recurso. Las plantillas admiten variables de reemplazo como `$nombredominio`, `$nombrerecurso`, `$proyecto`, `$codigo`, `$motivo` y `$fecha`.

La protección de recursos HTTP se implementa haciendo que Traefik envíe esos recursos a Pangolite en vez de enviarlos directo al backend. Pangolite valida contraseña específica, sesión activa del panel o prompt básico según configuración, y luego reenvía el tráfico al backend local o al cliente de sistema remoto.

## Operación segura agregada

Pangolite mantiene migraciones SQLite versionadas en `schema_migrations`. Antes de aplicar migraciones pendientes crea un respaldo pre-migración en `data/backups/migrations/` usando `VACUUM INTO`.

El comando `pangolite doctor` revisa la instalación activa: versión, SQLite, migraciones, rutas escribibles, Traefik, archivos clave, puertos 80/443 y estado básico de servicios.

La escritura de configuración de Traefik usa backups temporales y validación. Si el binario soporta `traefik check`, se usa ese comando; si no lo soporta, Pangolite hace un arranque temporal con puertos efímeros para validar la configuración sin ocupar 80/443 ni puertos públicos. Si la validación falla, Pangolite restaura la configuración anterior.

Los respaldos automáticos se controlan con `PANGOLITE_BACKUP_INTERVAL_HOURS` y `PANGOLITE_BACKUP_RETENTION_DAYS`. Por defecto se crea un respaldo automático cada 24 horas y se retienen 14 días.

Los releases publican clientes descargables para Linux amd64, arm64, 386, armv7 y Windows amd64. El comando Linux de cliente de sistema detecta la arquitectura con `uname -m` y descarga el binario correcto.


### Aplicación de cambios por gestor de servicios

Pangolite detecta el gestor de servicios disponible en runtime. En sistemas con systemd usa `systemctl`; en Alpine/OpenRC usa `rc-service`; en SysVinit usa `service` o `/etc/init.d`; y en runit usa `sv`. Los cambios HTTP/HTTPS siguen entrando por configuración dinámica de Traefik sin reinicio. Los cambios que agregan o eliminan entrypoints TCP/UDP requieren reinicio controlado de Traefik porque esos puertos forman parte de la configuración estática.

## Frontend del panel

El panel ya no vive como un string gigante dentro de `internal/app/ui.go`. La interfaz se organiza en templates Go embebidos y assets estáticos:

```txt
internal/app/templates/layouts/
internal/app/templates/components/
internal/app/templates/pages/
internal/app/assets/app/
```

La navegación principal usa rutas normales del servidor (`/projects`, `/projects/{id}/resources`, `/settings`, etc.). No requiere Node, Vite ni bundler; el binario sigue siendo autocontenido mediante `embed.FS`.


## Compatibilidad de terminal Linux y Android

La terminal del cliente resuelve la shell ejecutable desde `SHELL`, rutas Linux comunes, rutas Android y `PATH`. No exige Bash: sistemas con Toybox pueden usar `/system/bin/sh`. Para iniciar la sesión PTY se usa `syscall.ForkExec` directamente, evitando la comprobación de pidfd realizada por `os.StartProcess`, incompatible con algunos kernels Android antiguos que responden con `SIGSYS` al syscall `pidfd_open`.
