# Contexto 77 - Gate de calidad, tests y CI antes de publicar

## Objetivo

Evitar que Pangolite publique un release si el código, las pruebas, el frontend embebido, los scripts o alguno de los binarios soportados contiene errores detectables por el runner.

## Estado de pruebas

- El proyecto ya contaba con una suite amplia en `internal/app/*_test.go`.
- Se agregaron pruebas unitarias de los entrypoints `cmd/pangolite` y `cmd/pangolite-client` para `version`, `--version` y `-v`.
- `internal/app/version_test.go` ahora valida que `VersionCode` corresponda matemáticamente a la versión `X.Y` usando la misma regla del workflow: `major*100000 + minor`.
- Versión de desarrollo actual: `0.28`, código `28`.

## Gate compartido

`scripts/verify.sh` es la fuente común de controles de calidad para local, CI y releases. Debe fallar ante cualquiera de estos casos:

1. Go sin `gofmt`.
2. `VERSION`/`VERSION_CODE` desincronizados entre `Makefile`, `internal/app/version.go` o la fórmula de código monotónico.
3. `go mod tidy` que deja diferencias en archivos de módulos ya versionados.
4. Fallos de `go vet`.
5. Fallos de tests unitarios.
6. Condiciones de carrera detectadas por `go test -race`.
7. JavaScript con sintaxis inválida.
8. Scripts shell con sintaxis inválida.
9. Un `.zip` trackeado o alcanzable en el historial Git.

La ejecución normal de tests genera `build/quality/coverage.out` y muestra el porcentaje total de cobertura, sin imponer por ahora un porcentaje arbitrario como umbral.

## Build de release compartido

`scripts/build-release.sh VERSION VERSION_CODE` reemplaza la lógica duplicada que antes estaba embebida directamente en `release.yml`.

Compila y valida:

- Pangolite Linux amd64, arm64, 386 y armv7.
- pangolite-client Linux amd64, arm64, 386 y armv7.
- pangolite-client Windows amd64.
- Los cuatro paquetes `.tar.gz` del servidor.
- Smoke test de `pangolite --version` y `pangolite-client --version` cuando el runner es Linux amd64.
- `checksums.txt` para todos los artefactos publicados.

Todos los artefactos requeridos deben existir y tener contenido; los tarballs deben poder listarse correctamente con `tar -tzf`.

## GitHub Actions

### `.github/workflows/ci.yml`

Se ejecuta en:

- push a `main`;
- pull requests;
- ejecución manual.

Corre el gate completo y después un build completo multi-plataforma. Su finalidad es detectar la regresión antes de llegar al botón de release.

### `.github/workflows/release.yml`

El orden obligatorio es:

1. checkout completo;
2. Go + Node;
3. módulos;
4. resolver y validar que la versión solicitada coincida con la versión declarada en el código;
5. `scripts/verify.sh`;
6. `scripts/build-release.sh`;
7. generar notas;
8. `gh release create`.

GitHub Actions no continúa con los pasos posteriores cuando cualquiera de los pasos anteriores falla, por lo que `gh release create` solo se alcanza después de superar todas las validaciones.

## Comandos locales

```bash
make test
make verify
make build
make release-build
```

`make verify` es el comando recomendado antes de commit/push. `make release-build` reproduce el build de artefactos usando la versión de desarrollo configurada en el `Makefile`.

## Regla futura

No agregar un nuevo target/plataforma al release sin añadirlo también a `scripts/build-release.sh` y al conjunto `required_assets`. No duplicar nuevamente lógica de compilación extensa dentro de los workflows.

El release no calcula una versión distinta automáticamente: el tag solo se crea para la versión ya declarada en el código. Si el tag existe, primero se debe incrementar la versión en un commit nuevo.
