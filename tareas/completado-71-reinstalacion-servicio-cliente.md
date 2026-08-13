# Tarea 71 - Reinstalación segura del servicio pangolite-client

## Objetivo
Hacer que `--install` reemplace instalaciones anteriores y que Windows solicite elevación administrativa automáticamente cuando el comando requiera crear/eliminar el servicio.

## Estado
Completado y documentado.

## Criterios
- Detener y retirar servicios previos antes de reemplazar binarios/configuración.
- Limpiar artefactos de systemd/OpenRC incompatibles o antiguos sin borrar datos fuera de Pangolite.
- Reinstalar el servicio con la configuración nueva de forma idempotente.
- En Windows, `--install` y `--remove` deben relanzarse elevados cuando sea necesario.
- No agregar dependencias nuevas.
