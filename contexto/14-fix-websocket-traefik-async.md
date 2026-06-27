# 14 - Fix de streams TCP NAT y aplicación asíncrona de Traefik

## Fecha
2026-06-27

## Objetivo
Corregir dos fallos detectados al crear y probar túneles TCP remotos con clientes NAT:

1. El navegador mostraba `Failed to fetch` al crear un recurso TCP porque Pangolite reiniciaba Traefik durante la misma petición HTTP cuando cambiaban entrypoints estáticos.
2. El cliente NAT no lograba adjuntar el stream TCP porque el middleware de logging envolvía el `ResponseWriter` sin exponer `http.Hijacker`, provocando el error `http.ResponseWriter does not implement http.Hijacker`.

## Decisiones tomadas
- Mantener el reinicio controlado de Traefik para cambios TCP/UDP, porque abrir o cerrar entrypoints sigue siendo configuración estática.
- Programar el reinicio en segundo plano con una pequeña demora para permitir que la API responda correctamente antes de que Traefik corte conexiones activas del panel.
- Hacer que el `statusRecorder` conserve compatibilidad con WebSocket delegando `Hijack`, `Flush` y `Unwrap` al writer original.
- Mantener logs explícitos para reinicio programado, éxito y fallo.

## Archivos modificados
- `internal/app/server.go`

## Solución técnica
- `statusRecorder` ahora implementa:
  - `http.Hijacker`
  - `http.Flusher`
  - `Unwrap()`
- `applyTraefikStaticAndRestart()` ya no ejecuta `systemctl restart traefik` de forma síncrona dentro de la petición.
- Se agregó `scheduleTraefikRestart(reason string)` para reiniciar Traefik en segundo plano.

## Resultado esperado
- Crear un recurso TCP/UDP ya no debe mostrar `Failed to fetch` aunque Traefik tenga que aplicar un entrypoint nuevo.
- Los streams TCP remotos deben aceptar WebSocket correctamente.
- El log ya no debe mostrar `http.ResponseWriter does not implement http.Hijacker`.

## Pendientes
- Agregar debounce global para agrupar varios cambios TCP/UDP rápidos en un solo reinicio de Traefik.
- Mostrar en la UI un estado temporal tipo “Traefik aplicando puerto público” para evitar pruebas antes de que el reinicio termine.
