# Tarea 79 — Reautenticar al cambiar la IP administrativa

## Objetivo
Hacer que una sesión administrativa deje de ser válida cuando cambia la IP del cliente y permita iniciar sesión otra vez con contraseña desde la IP nueva en modo `learn`.

## Criterios
- La sesión debe recordar la IP exacta con la que fue creada.
- Un cambio de IP debe llevar de vuelta a `/login`, incluso dentro del mismo `/24` o `/64`.
- `learn` no debe bloquear `/api/login` por ser una red nueva: una contraseña válida debe permitir crear una sesión nueva y aprender la red.
- `allowlist` debe seguir bloqueando redes no autorizadas.
- Mantener resolución de IP mediante proxies confiables, rate limits y CSRF existentes.
- Añadir migración y regresiones.

## Resultado
Completado: migración SQLite v11, sesión ligada a IP exacta, reautenticación funcional desde IP/red nueva en `learn` y `allowlist` sin cambios de política.
