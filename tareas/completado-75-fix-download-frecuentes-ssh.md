# Tarea 75 — Corregir download y conexiones frecuentes SSH

Estado: completado y verificado con validaciones estáticas/harness disponibles.

## Objetivo
- Corregir la pseudo-orden `download ruta` para que vuelva a interceptarse antes de llegar a bash, incluyendo entrada pegada y edición normal de línea.
- Hacer que la sección de conexiones frecuentes SSH use actividad real registrada por Pangolite y persista entre navegadores/dispositivos.
- Mantener compatibilidad, bajo consumo y sin dependencias nuevas.

## Criterios de aceptación
- `download archivo`, rutas absolutas, relativas y entre comillas no llegan al shell cuando coinciden con la pseudo-orden.
- Pegar `download ...` también funciona.
- Una terminal abierta registra uso y el directorio SSH muestra los destinos más usados del usuario.
- Destinos eliminados o no disponibles no rompen la vista.
- Tests de parser/uso frecuente y validaciones estáticas pasan cuando el entorno lo permite.
