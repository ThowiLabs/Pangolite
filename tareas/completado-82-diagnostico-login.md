# Tarea 82 — Diagnóstico de fallos de inicio de sesión

## Objetivo
Evitar que un fallo técnico durante el login quede reducido al mensaje genérico del navegador sin información útil para operación.

## Criterios
- Registrar en el log del servidor el error interno al crear una sesión, incluyendo usuario e IP sin registrar contraseñas.
- Mantener una respuesta JSON segura para errores internos del login.
- Hacer que el frontend distinga errores JSON del backend, respuestas HTTP no JSON y fallos de conectividad.
- No exponer detalles internos de SQLite al navegador.
- Reparar de forma idempotente el esquema crítico de `sessions` aunque el registro de migraciones diga que ya fue aplicado.
- Añadir regresiones para fallos de creación de sesión y metadatos de migración inconsistentes.

## Resultado
Completado: Pangolite repara `sessions.client_ip` al arrancar aunque la metadata de migración esté inconsistente; además los fallos de sesión quedan registrados y el login diferencia errores del backend, respuestas inválidas y problemas de conexión.
