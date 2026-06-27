# 27 - Gestor de servicios para Traefik en Alpine/OpenRC

Se agregó detección de gestor de servicios en runtime para que Pangolite no dependa exclusivamente de `systemctl` al aplicar cambios que requieren reiniciar Traefik.

## Decisión

- HTTP/HTTPS sigue usando configuración dinámica de Traefik y no requiere reinicio.
- Cambios que agregan o eliminan puertos TCP/UDP públicos modifican entrypoints estáticos y requieren reinicio controlado de Traefik.
- Pangolite detecta el gestor disponible y usa el comando correspondiente:
  - systemd: `systemctl restart traefik`
  - OpenRC: `rc-service traefik restart`
  - SysVinit: `service traefik restart`
  - runit: `sv restart traefik`

## Instaladores

`install.sh` e `init.sh` crean servicios según el gestor detectado. En Alpine/OpenRC se escribe `/etc/init.d/pangolite` y `/etc/init.d/traefik` usando `start-stop-daemon`, pidfile y logs bajo `/opt/pangolite/data/`.

## Motivo

En Alpine no existe `systemctl`. Antes Pangolite escribía la configuración de Traefik pero no podía aplicar puertos TCP/UDP nuevos porque intentaba reiniciar Traefik con systemd. Ahora usa OpenRC cuando corresponde.
