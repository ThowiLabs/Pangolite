# Tarea 73 - Migración WebSocket mantenida

## Objetivo
Migrar el transporte WebSocket de `nhooyr.io/websocket` a `github.com/coder/websocket` sin alterar protocolos, timeouts, framing ni comportamiento observable.

## Alcance
- Actualizar `go.mod` a `github.com/coder/websocket v1.8.15`.
- Cambiar imports en servidor, agente, terminal y bridges.
- No refactorizar la lógica WebSocket durante esta tarea.
- Actualizar contexto actual y documentar la migración.
- Validar formato, referencias residuales y sintaxis disponible en el entorno.

## Estado
Completado y revisado estáticamente.
