# Completado 78 - Normalizar módulos y corregir gate de tidy

- [x] Aplicar al `go.mod` la normalización reportada por Go 1.26.x en CI.
- [x] Añadir y versionar `go.sum`.
- [x] Corregir el gate para detectar un `go.sum` ausente/no trackeado.
- [x] Mantener `go.mod` como condición estricta e inmutable después de `go mod tidy`.
- [x] Ejecutar `go mod verify` antes de `go vet`, tests y builds.
- [x] Evitar que cambios únicamente de checksums bloqueen falsamente el release.
- [x] Actualizar versión a 0.29 (code 29).
- [x] Mantener la publicación como último paso del workflow.
