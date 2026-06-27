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
