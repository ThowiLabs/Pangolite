# Fecha
2026-06-26

# Objetivo
Crear el repositorio limpio de Pangolite como proyecto Go instalable directamente en Linux, sin Docker, con panel web, SQLite embebido, Traefik del sistema y estructura preparada para venderse como producto serio.

# Decisiones tomadas
- Repositorio: `github.com/thowilabs/pangolite`.
- Instalación principal: `/opt/pangolite/`.
- Panel inicial: `0.0.0.0:2424`, sin redirección HTTPS.
- SQLite: `/opt/pangolite/data/pangolite.db`.
- Password temporal: `/opt/pangolite/data/admin-password.txt`.
- Usuario inicial: `admin`.
- Password mínima: 6 caracteres.
- Traefik: instalado y ejecutado en el sistema, no en Docker.
- Config Traefik: `/etc/traefik/traefik.yml`, `/etc/traefik/dynamic/pangolite-dashboard.yml`, `/etc/traefik/acme.json`.
- `init.sh` verifica dependencias, descarga Go temporal si falta, compila, instala, crea systemd y limpia temporales.
- No se entrega `.git` ni commit en este ZIP.

# Arquitectura actual
Internet -> Traefik del sistema -> Pangolite -> servicio local o cliente de sistema remoto.

Pangolite mantiene:
- proyectos/clientes;
- dominios administrados;
- clientes de sistema/agentes;
- recursos HTTP/HTTPS/TCP/UDP;
- sesiones y usuarios;
- render de Traefik.

# Librerías usadas
- Go standard library.
- `golang.org/x/crypto/bcrypt`.
- `modernc.org/sqlite`.

# Archivos importantes modificados
- `README.md`
- `init.sh`
- `go.mod`
- `cmd/pangolite/main.go`
- `internal/app/config.go`
- `internal/app/model.go`
- `internal/app/server.go`
- `internal/app/store.go`
- `internal/app/traefik.go`
- `internal/app/ui.go`
- `internal/app/portcheck.go`

# Problemas encontrados
Traefik falla por completo si un entryPoint TCP/UDP intenta abrir un puerto ocupado. Esto ocurrió con el puerto 2121.

# Soluciones implementadas
- Validación de puerto público contra recursos existentes.
- Validación de puerto público contra el sistema operativo antes de guardar.
- Mensajes claros cuando el puerto ya está ocupado.
- Despliegue directo fuera de Docker.

# Pendientes
- TCP remoto real por cliente de sistema.
- UDP remoto real por cliente de sistema.
- Helper privilegiado para reiniciar Traefik desde UI de forma segura.
- Releases precompilados.

# Próximos pasos
Probar `sudo bash init.sh` en el VPS y continuar con streams TCP remotos.
