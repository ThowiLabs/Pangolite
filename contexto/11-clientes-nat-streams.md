# 11 - Clientes NAT y streams remotos

## Fecha
2026-06-26

## Objetivo
Implementar clientes NAT reales y túneles remotos HTTP/TCP/UDP para publicar servicios que viven detrás de NAT sin abrir puertos en el servidor remoto.

## Decisiones tomadas
- Se agregó un binario independiente `pangolite-client`.
- `init.sh` compila el panel y el cliente con `CGO_ENABLED=0` y publica el cliente en `/opt/pangolite/public/pangolite-client-linux-amd64`.
- Al crear o rotar un cliente, el panel genera un comando instalador listo para copiar.
- El cliente soporta `--install` y `--remove`.
- `--install` detecta systemd u OpenRC, copia el binario a `/opt/pangolite-client/`, crea el servicio y lo inicia.
- `--remove` detiene el servicio, lo deshabilita y elimina archivos del cliente.
- TCP remoto usa streams persistentes sobre WebSocket autenticado.
- UDP remoto usa datagramas request/response a través de la cola autenticada.
- Traefik sigue publicando el puerto público; Pangolite escucha un puerto interno de puente para recursos remotos.

## Arquitectura actual
- Panel: `/opt/pangolite/pangolite`.
- Cliente NAT: `/opt/pangolite/pangolite-client`.
- Descarga del cliente: `/download/pangolite-client-linux-amd64`.
- Servicio cliente remoto: `pangolite-client.service` o script OpenRC.
- Recursos HTTP remotos: cola HTTP existente.
- Recursos TCP remotos: bridge local + stream WebSocket + dial TCP en el cliente.
- Recursos UDP remotos: bridge UDP local + datagrama hacia cliente.

## Archivos modificados
- `cmd/pangolite-client/main.go`
- `internal/app/agent_client.go`
- `internal/app/tunnel.go`
- `internal/app/stream_bridge.go`
- `internal/app/model.go`
- `internal/app/store.go`
- `internal/app/server.go`
- `internal/app/traefik.go`
- `internal/app/ui.go`
- `init.sh`
- `Makefile`
- `README.md`

## Pendientes
- Cliente NAT para arm64 publicado desde init o pipeline de release.
- Métricas detalladas de streams activos.
- Reconexión con backoff configurable.
- Logs descargables por cliente.
- Health checks por recurso remoto.
- Soporte UDP avanzado orientado a sesiones largas.
