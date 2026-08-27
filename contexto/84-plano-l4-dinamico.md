# 84 - Plano L4 público dinámico sin reinicios de Traefik

## Objetivo
Eliminar los reinicios globales de Traefik al crear, editar, suspender, reactivar o eliminar recursos TCP/UDP. Pangolite debe poseer directamente los sockets L4 públicos, preservando HTTP/HTTPS y los demás puertos activos.

## Arquitectura aplicada
- Traefik conserva únicamente los entrypoints `web` (:80) y `websecure` (:443).
- La configuración dinámica `/api/v1/traefik-config` publica exclusivamente routers/servicios HTTP.
- `PublicL4Manager` abre listeners TCP/UDP públicos dentro del proceso Pangolite y los reconcilia con SQLite.
- TCP local conecta directamente al backend y usa proxy dúplex con keepalive/half-close.
- TCP remoto conecta el listener público directamente con `TunnelHub`; deja de existir el salto por `127.0.0.1:<tunnel_port>`.
- UDP local mantiene sesiones por cliente con timeout y límite defensivo; UDP remoto reutiliza jobs autenticados con concurrencia acotada.
- Los valores `tunnel_port` antiguos se conservan en datos solo para compatibilidad/rollback; recursos L4 remotos nuevos no reservan puertos puente.

## Semántica de cambios en caliente
- Cambio solo de backend TCP: el listener se conserva. Conexiones ya aceptadas siguen con el backend anterior; conexiones nuevas usan el nuevo.
- Cambio de puerto/protocolo o reactivación: Pangolite reserva primero el socket nuevo y solo después persiste el cambio. Si el bind falla, SQLite no se modifica.
- Eliminación/suspensión: se cierra únicamente el listener afectado. Conexiones TCP ya aceptadas no dependen del listener y pueden finalizar normalmente.
- UDP cierra sus sesiones cuando se retira el listener; al cambiar backend, sesiones locales existentes conservan su backend hasta expirar y nuevas sesiones usan el nuevo.
- En arranque/reconciliación, un puerto ocupado no impide activar otros listeners independientes.

## Migración desde versiones anteriores
Las versiones antiguas tenían entrypoints `tcp-N`/`udp-N` en `traefik.yml`; Traefik era dueño de esos sockets. El binario nuevo no puede tomarlos hasta liberar esos entrypoints.

`install.sh` e `init.sh` hacen el handoff automáticamente:
1. reemplazan el binario de forma atómica sin matar primero el proceso anterior;
2. renderizan `traefik.yml` sin entrypoints L4;
3. si la configuración estática cambió, reinician Traefik una sola vez para liberar los sockets;
4. reinician Pangolite, que toma los puertos TCP/UDP públicos;
5. en actualizaciones posteriores, si la configuración estática de Traefik no cambió, el instalador no lo reinicia.

Si alguien actualiza manualmente, la secuencia recomendada es `pangolite render-traefik`, reiniciar Traefik y luego reiniciar Pangolite. Si Pangolite nuevo arranca antes de liberar los sockets, el panel sigue operativo y el reconciliador reintenta L4 cada 10 s. `pangolite doctor` avisa si `traefik.yml` conserva entrypoints L4 heredados.

## Seguridad y límites
- `PANGOLITE_L4_TCP_CONCURRENCY` limita conexiones TCP públicas concurrentes (default 512).
- `PANGOLITE_L4_UDP_CONCURRENCY` limita jobs UDP remotos concurrentes (default 256).
- Los streams remotos conservan `PANGOLITE_AGENT_STREAM_CONCURRENCY`.
- UDP local limita sesiones por listener y expira sesiones inactivas.
- Saturación no crea goroutines ilimitadas; TCP se rechaza y UDP remoto se descarta cuando no existe backpressure fiable.

## Validación
- Pruebas aisladas del plano L4 pasan con Go local: continuidad de TCP al cambiar backend, fallo de un puerto sin cortar listeners existentes, UDP local, TCP/UDP remotos directos y reservas de sockets.
- `gofmt`, `git diff --check`, `bash -n init.sh` y `sh -n install.sh` pasan.
- La suite completa `go test ./...` no puede ejecutarse en este entorno porque el repositorio exige Go 1.26 y el host tiene Go 1.23.2 sin acceso DNS para descargar el toolchain; CI mantiene la validación completa con Go 1.26.
