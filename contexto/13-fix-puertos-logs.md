# 13 - Validacion de puertos TCP/UDP y logs operativos

## Fecha
2026-06-27

## Objetivo
Corregir falsos positivos al crear recursos TCP/UDP con clientes NAT y agregar un archivo de logs persistente para depurar errores del panel, Traefik y validaciones.

## Decisiones tomadas
- La validacion de puerto publico ahora busca el recurso conflictivo exacto por `mode + public_port`.
- Se ignoran valores nulos o cero para evitar coincidencias falsas.
- El mensaje de error incluye nombre e ID del recurso que ocupa el puerto.
- Se reemplazo el indice unico parcial de puertos por un indice de busqueda no unico; la integridad se controla desde la capa de aplicacion para poder dar errores explicitos.
- Se agrego `PANGOLITE_LOG_FILE` con valor por defecto en el directorio de datos.
- El archivo de log se mantiene con maximo 1000 entradas.
- Se agrego vista `/logs` y API `/api/system/logs` para inspeccion desde el panel.

## Arquitectura actual
- `pangolite` escribe logs estructurados a stdout y al archivo configurado.
- El handler HTTP registra metodo, ruta, host, status y duracion.
- Los fallos de validacion/creacion/edicion de recursos se registran como `WARN`.
- Las panics de request se capturan, se registran con stack trace y devuelven JSON 500.

## Archivos importantes modificados
- `cmd/pangolite/main.go`
- `internal/app/config.go`
- `internal/app/eventlog.go`
- `internal/app/server.go`
- `internal/app/store.go`
- `internal/app/ui.go`
- `init.sh`

## Problemas detectados
El flujo de recursos TCP remotos podia devolver mensajes ambiguos como puerto ocupado sin indicar cual recurso lo causaba. Tambien no habia una fuente facil de diagnostico desde el panel cuando el navegador mostraba `Failed to fetch`.

## Soluciones
- Nueva funcion `ResourcePublicPortConflictExcept`.
- Logs persistentes en `/opt/pangolite/data/pangolite.log`.
- Vista operativa de logs con copiar/actualizar.
- Rotacion simple por numero de entradas.

## Pendientes
- Agregar filtros por nivel (`INFO`, `WARN`, `ERROR`).
- Agregar descarga del archivo de logs.
- Agregar correlacion de request ID en errores de API.
