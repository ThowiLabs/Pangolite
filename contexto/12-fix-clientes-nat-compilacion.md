# 12 - Corrección de compilación en clientes NAT

## Fecha
2026-06-27

## Objetivo
Corregir la compilación de la fase de clientes NAT antes del commit de integración.

## Problema detectado
El código de túneles remotos empezó a usar `TunnelPort` para crear puentes TCP/UDP internos, pero el campo no quedó agregado al modelo `Resource`. También faltaba el canal `Attached` en `StreamSession`, usado para confirmar que el cliente NAT ya se conectó al stream WebSocket antes de iniciar el puente.

## Decisiones tomadas
- Agregar `TunnelPort` al struct `Resource` con serialización JSON interna.
- Mantener la migración SQLite `tunnel_port` ya incluida para bases existentes.
- Agregar `Attached` a `StreamSession`.
- Actualizar la prueba de validación para que TCP remoto mediante cliente NAT sea válido.
- Corregir un duplicado accidental de `CreatedAt` en el modelo `User`.

## Arquitectura actual
Los recursos TCP/UDP remotos usan:

1. `public_port`: puerto público expuesto por Traefik.
2. `tunnel_port`: puerto interno local en `127.0.0.1` donde Pangolite abre el puente.
3. `backend_host/backend_port`: servicio real dentro del servidor remoto donde corre el cliente NAT.
4. `agent_id`: cliente NAT que recibe el stream o datagrama.

## Validación
Se validó compilación estática con módulos simulados para detectar errores internos de tipos y referencias, debido a que el entorno local no tiene acceso al proxy de módulos Go. En VPS, `init.sh` ejecuta `go mod tidy` y `go test ./...` con dependencias reales.

## Pendientes
- Probar TCP remoto real con SSH detrás de NAT.
- Agregar estado de salud por cliente NAT en UI.
- Agregar logs de conexión por stream.
