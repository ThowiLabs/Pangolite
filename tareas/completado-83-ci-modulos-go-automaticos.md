# Tarea 83 — Dependencias Go automáticas y resistentes en releases

## Objetivo
Mantener `go mod tidy` completamente automatizado en GitHub Actions, generar/persistir `go.sum` sin pasos locales y reducir fallos de release por errores transitorios del proxy de módulos.

## Criterios
- Conservar `go mod tidy` dentro del workflow de release.
- Usar `GOPROXY=https://proxy.golang.org|direct` para permitir fallback también ante errores de red/HTTP distintos de 404/410.
- Reintentar `go mod tidy` y `go mod download` ante fallos transitorios.
- Verificar el cache de módulos con `go mod verify`.
- Exigir que `go.sum` exista después de resolver módulos.
- Committear y subir automáticamente cambios de `go.mod`/`go.sum` desde GitHub Actions antes de crear el release.
- No requerir que el operador ejecute comandos Go localmente.
- Crear el tag del release apuntando explícitamente al `HEAD` que incluye la actualización automática de dependencias.
- Mantener la caché de `actions/setup-go` habilitada para reducir descargas repetidas.

## Resultado
Completado: el release resuelve, reintenta, verifica y persiste automáticamente las dependencias Go; un `go.sum` ausente se genera en CI y queda versionado antes de publicar el tag y los binarios.
