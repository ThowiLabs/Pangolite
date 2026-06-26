# Arquitectura de Pangolite

Pangolite es un plataforma de administración en Go inspirado en Pangolin. El objetivo es exponer recursos de proyectos/clientes usando Traefik como edge proxy y SQLite como almacenamiento local.

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

### Recurso TCP/UDP remoto

Pendiente. Requiere streams persistentes entre Pangolite y el cliente de sistema.
