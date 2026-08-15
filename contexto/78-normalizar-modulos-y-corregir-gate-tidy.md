# Contexto 78 - Normalización de módulos y corrección del gate de tidy

## Problema detectado

El primer release con el gate de calidad de `contexto/77` se detuvo correctamente en la fase de módulos porque `go mod tidy`, ejecutado por Go 1.26.x, modificaba `go.mod`.

La causa era doble:

1. `go.mod` conservaba únicamente las cuatro dependencias directas y un orden no normalizado. Go 1.26 añadió el bloque de dependencias indirectas requerido por `modernc.org/sqlite`.
2. `go.sum` no estaba versionado. Además, el gate utilizaba `git diff -- go.mod go.sum`, que no detecta un `go.sum` recién creado y todavía no trackeado.

## Corrección aplicada

- `go.mod` queda normalizado con las dependencias directas ordenadas y el bloque indirecto producido por `go mod tidy` en el runner.
- Se incorpora `go.sum` al repositorio para mantener checksums de módulos y una base reproducible.
- `scripts/verify.sh` separa ahora la validación estructural de `go.mod` de la regeneración de checksums:
  - guarda una copia de `go.mod`;
  - ejecuta `go mod tidy`;
  - falla si `go.mod` cambia;
  - exige que `go.sum` exista, tenga contenido y esté trackeado por Git;
  - ejecuta `go mod verify` después de la normalización.

Los cambios que Go pueda realizar únicamente en `go.sum` durante el runner no bloquean el release por sí solos. El archivo se normaliza antes de `go vet`, tests y builds, y `go mod verify` valida los módulos descargados. Esto evita falsos bloqueos por checksums añadidos por el toolchain sin relajar la coherencia del grafo declarado en `go.mod`.

## Release y CI

El orden del gate sigue siendo estricto: la publicación permanece al final del workflow. Un fallo de módulos, `go vet`, pruebas, race detector, JavaScript, shell, higiene Git o build continúa abortando el job antes de crear el release.

## Versionado

- Versión de desarrollo: `0.29`.
- Código de versión: `29`.
- Servidor y `pangolite-client` comparten ambos valores.

## Regla futura

Siempre que se actualice una dependencia, ejecutar `go mod tidy` con la misma rama de Go utilizada por CI/release y confirmar `go.mod` y `go.sum`. El gate debe exigir un `go.sum` trackeado y verificar módulos con `go mod verify`, pero no debe confundir una actualización legítima de checksums en el workspace efímero con una alteración del grafo declarado.
