# Tarea 85 - Branding de carga opcional por recurso HTTP

Estado: completado

- [x] Añadir bandera de branding por recurso HTTP, desactivada por defecto.
- [x] Migrar SQLite a v13 con backup pre-migración y reparación defensiva de columna.
- [x] Enrutar por Pangolite únicamente los recursos HTTP que tengan branding activo.
- [x] Inyectar loader Thowilabs solo en documentos HTML compatibles.
- [x] Servir CSS/logo desde el mismo origen sin JavaScript inyectado.
- [x] Respetar CSP estricto y omitir la inyección si no es compatible.
- [x] Evitar respuestas comprimidas/304 y limpiar validadores inválidos tras transformar HTML.
- [x] Forzar revalidación del HTML transformado para que desactivar branding no deje copias frescas obsoletas.
- [x] Añadir switches de crear/editar y etiqueta visual en la lista de recursos.
- [x] Normalizar branding a apagado en redirects, TCP y UDP.
- [x] Añadir pruebas de persistencia, routing, edición, CSP, parser HTML, assets y no-documentos.
