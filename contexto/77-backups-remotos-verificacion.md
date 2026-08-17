# Fecha
2026-08-17
# Objetivo
Hacer verificables y replicables fuera del VPS los respaldos SQLite de Pangolite.
# Decisiones tomadas
- Verificación real sobre una copia temporal: apertura con el Store actual, migraciones y `PRAGMA integrity_check` sin tocar la base activa.
- Destinos remotos estándar: WebDAV y S3-compatible con endpoint/region/bucket configurables.
- S3 Signature V4 implementada con librería estándar para no incorporar un SDK pesado al VPS de 500 MB.
- WebDAV usa MKCOL/PUT y transmisión directa desde archivo.
- Secretos configurados no se devuelven al navegador; dejar el campo secreto vacío conserva el valor existente.
- HTTP sin TLS está bloqueado por defecto y requiere habilitación explícita.
- Subida automática opcional para respaldos nuevos; un fallo remoto nunca elimina el backup local.
# Arquitectura actual
La configuración remota se persiste como JSON en `app_settings`. Los archivos locales siguen siendo la fuente primaria y se transmiten al proveedor sin cargarlos completos en RAM.
# Librerías usadas
Solo standard library (`net/http`, `crypto/hmac`, `crypto/sha256`). No se agregaron dependencias.
# Archivos importantes modificados
`internal/app/backup_remote.go`, `internal/app/maintenance.go`, rutas en `server.go`, página y JS de Seguridad.
# Problemas encontrados
- Las APIs S3 difieren en endpoint; path-style y endpoint configurable cubren servicios S3-compatible que acepten Signature V4, incluyendo Storage Buckets de Hugging Face.
- El entorno de trabajo disponible tiene Go 1.23.2 y no tiene salida de red/DNS, mientras el proyecto exige Go 1.26. Por ello `go test ./...` y `go vet ./...` no pueden descargar el toolchain ni los módulos reales en este entorno.
# Soluciones implementadas
- Carga manual, carga automática opcional, prueba de destino con limpieza del objeto temporal y verificación de restauración local.
- Se ejecutaron pruebas aisladas con `-race` sobre el código real de WebDAV/S3 y acciones masivas usando stubs solo para las dependencias externas; también se verificó la firma SigV4 contra una referencia independiente de botocore.
- Se realizó compilación de comprobación de todos los paquetes y tests con stubs de dependencias en una copia temporal; la suite real completa queda pendiente de un entorno Go 1.26 con módulos disponibles.
# Pendientes
No hay borrado remoto por retención: la retención local no elimina objetos externos para evitar pérdida de copias off-site por una configuración incorrecta.
# Próximos pasos
Probar cada proveedor real desde la UI antes de habilitar auto-upload y conservar credenciales de alcance mínimo.
