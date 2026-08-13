# Tarea 69 — Transferencia de archivos desde terminal

## Estado
pendiente

## Objetivo
Permitir subir archivos al directorio actual de una terminal local o remota mediante arrastrar y soltar o selector de archivos, con progreso y uso de memoria acotado.

## Alcance
- Streaming por bloques pequeños, sin leer el archivo completo en RAM ni base64.
- Resolver el directorio actual real del proceso de terminal en Linux.
- Soportar terminal local y terminal de agentes mediante el mismo canal seguro existente.
- Sanitizar nombres y evitar sobrescrituras accidentales.
- Mostrar progreso, éxito y errores en la UI.
- Añadir selector de archivos usable en Android.
- Limpiar archivos parciales al cancelar o cerrar la sesión.
- Añadir pruebas del framing, rutas y ciclo de vida de uploads.
