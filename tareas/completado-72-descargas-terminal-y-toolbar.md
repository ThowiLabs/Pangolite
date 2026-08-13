# Tarea 72 — Descargas desde terminal y simplificación del toolbar

Estado: completado.

## Objetivo

- Implementar `download <ruta>` como comando interceptado por la terminal web, sin instalar un binario/comando en el sistema destino.
- Descargar archivos directamente en el navegador mediante streaming HTTP.
- Si el destino es un directorio, generar un ZIP en streaming con protecciones contra rutas de sistema, árboles excesivos y archivos especiales.
- Mantener compatibilidad con terminal local y clientes remotos actualizados mediante capacidad negociada.
- Mover **Servidor local/destino** y el selector de tema a un widget de ajustes.
- Mover Subir archivo y Desconectar a un menú de opciones, manteniendo Pantalla completa como acción visible principal.
- Registrar estas decisiones como reglas persistentes de desarrollo de la terminal.

## Criterios

- Sin cargar archivos completos en RAM.
- Sin base64 para descargas.
- Ticket de descarga autenticado, temporal y de un solo uso.
- Directorios sensibles del sistema no se archivan.
- No seguir symlinks ni incluir sockets/FIFOs/dispositivos.
- Límites de cantidad de entradas y bytes antes de generar ZIP.
- El toolbar no debe volver a crecer con controles secundarios visibles.
