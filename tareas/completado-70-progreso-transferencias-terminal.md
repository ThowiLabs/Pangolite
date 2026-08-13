# Tarea 70 - Progreso flotante de transferencias en terminal

## Objetivo
Evitar que las transferencias cambien el alto de la terminal. Mostrar el archivo activo en una ventana flotante con barra de progreso y emitir avisos temporales al completar o fallar archivos, incluyendo lotes masivos.

## Estado
Completado y validado estáticamente.

## Criterios
- El progreso no participa en el layout de xterm ni dispara `fit()` por transferencias.
- Solo se muestra el archivo que se está transfiriendo actualmente.
- En lotes se indica posición/total y se reutiliza la misma ventana.
- Cada archivo completado/fallido genera un aviso temporal no bloqueante.
- Los avisos quedan acotados para no crecer indefinidamente.
