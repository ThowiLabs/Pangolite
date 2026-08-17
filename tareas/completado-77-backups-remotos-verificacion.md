# Fecha
2026-08-17
# Objetivo
Ampliar respaldos SQLite con verificación real de restauración y réplica remota WebDAV/S3-compatible, incluyendo Hugging Face Storage Buckets.
# Estado
completado
# Alcance
- Verificar una copia temporal del backup antes de declararlo restaurable.
- Configuración remota desde UI con secretos no devueltos al navegador.
- WebDAV mediante MKCOL/PUT.
- S3-compatible mediante Signature V4, path-style y endpoint configurable.
- Subida manual y automática opcional.
