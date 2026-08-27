# Tarea 84 - Plano L4 dinámico

Estado: completado

- [x] Mover listeners TCP/UDP públicos desde Traefik a Pangolite.
- [x] Mantener Traefik exclusivamente para HTTP/HTTPS.
- [x] Preservar conexiones TCP existentes durante cambios no destructivos.
- [x] Añadir reservas de socket antes de persistir altas/reactivaciones.
- [x] Aislar fallos de puertos durante reconciliación.
- [x] Mantener límites de concurrencia y sesiones UDP acotadas.
- [x] Migrar instalaciones existentes mediante handoff automático de Traefik a Pangolite.
- [x] Evitar reinicios innecesarios de Traefik en actualizaciones posteriores.
- [x] Actualizar UI, README, arquitectura, doctor y contexto.
- [x] Añadir pruebas de regresión del plano L4.
