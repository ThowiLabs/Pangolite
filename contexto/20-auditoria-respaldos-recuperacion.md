# 20 - Auditoría, respaldos y recuperación de contexto

## Fecha
2026-06-27

## Objetivo
Retomar el proyecto desde el ZIP `pangolite-system-danger-zone(2).zip`, leer `contexto/` completo y continuar con el siguiente pendiente natural: auditoría de acciones administrativas y respaldos de SQLite.

## Contexto recuperado
- Pangolite es un panel Go ligero, instalable directo en Linux sin Docker.
- Usa SQLite en `/opt/pangolite/data/pangolite.db`.
- Usa Traefik del sistema en `/etc/traefik`.
- El panel corre por defecto en `0.0.0.0:2424`.
- Ya existen clientes NAT con binario `pangolite-client`, soporte Linux/Windows, health checks y streams TCP/UDP remotos.
- La última fase agregada fue Zona de peligro para proyectos y clientes NAT.

## Cambios implementados
- Nueva tabla `audit_events` para registrar acciones administrativas críticas.
- Nuevos endpoints:
  - `GET /api/audit`
  - `GET /api/backups`
  - `POST /api/backups`
  - `GET /api/backups/{name}/download`
- Nueva sección del panel: **Seguridad**.
- UI para ver auditoría y crear/descargar respaldos SQLite.
- Configuración `PANGOLITE_BACKUP_DIR`, por defecto en `data/backups`.
- Respaldos consistentes usando `VACUUM INTO`.
- Actualización de README y arquitectura para reflejar que TCP/UDP remoto por cliente NAT ya no está pendiente.
- Corrección menor: eliminación de un `return` duplicado en la validación de recursos.

## Decisiones tomadas
- No se guardan contraseñas ni tokens en auditoría.
- La restauración de backups queda como operación manual segura desde el panel, con comando visible, para evitar reemplazar la base activa mientras el proceso Go sigue usando SQLite.
- Se evita agregar una restauración automática peligrosa sin helper/flujo transaccional de apagado controlado.

## Archivos modificados
- `internal/app/maintenance.go`
- `internal/app/config.go`
- `internal/app/store.go`
- `internal/app/model.go`
- `internal/app/server.go`
- `internal/app/ui.go`
- `init.sh`
- `README.md`
- `docs/arquitectura.md`
- `contexto/20-auditoria-respaldos-recuperacion.md`

## Validaciones ejecutadas
- `gofmt` OK.
- `sh -n init.sh` OK.
- `node --check` por cada bloque `<script>` embebido OK.
- `go/parser` sobre todos los `.go` OK.

## Validación bloqueada
- `go test ./...` no pudo ejecutarse en el sandbox porque el ZIP no incluye `go.sum` y el entorno no tiene acceso a `proxy.golang.org` para descargar módulos. En el VPS debe ejecutarse `go mod tidy && go test ./...`.

## Próximos pasos
- Ejecutar en VPS `go mod tidy && go test ./...`.
- Probar creación de respaldo desde Seguridad.
- Crear/editar/eliminar un recurso y confirmar que aparece en auditoría.
- Luego implementar descarga/filtrado avanzado de auditoría o restauración automatizada mediante helper seguro si se decide necesario.
