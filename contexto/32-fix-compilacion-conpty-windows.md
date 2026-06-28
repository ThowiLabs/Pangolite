# 32 - Fix de compilación Windows para ConPTY

## Fecha
2026-06-28

## Problema

Al compilar el cliente NAT Windows amd64 con Go moderno apareció este error:

```txt
internal/app/terminal_process_windows.go:218:15: invalid operation: errno != 0 (mismatched types error and untyped int)
internal/app/terminal_process_windows.go:232:15: invalid operation: errno != 0 (mismatched types error and untyped int)
```

## Causa

La implementación de ConPTY usaba `LazyProc.Call` y trataba el tercer valor devuelto (`errno`) como si fuera un entero comparable contra `0`.

En esta versión de Go/`x/sys/windows`, ese valor se tipa como `error`, por lo que `errno != 0` no compila.

## Solución aplicada

Se agregó el helper `windowsCallError(err error) error` para normalizar errores devueltos por llamadas Win32:

- `nil` se considera sin error.
- `syscall.Errno(0)` se considera sin error.
- cualquier otro error se propaga.

Se reemplazaron las comparaciones `errno != 0` por llamadas a ese helper en:

- `InitializeProcThreadAttributeList`
- `UpdateProcThreadAttribute`

## Archivos tocados

- `internal/app/terminal_process_windows.go`
- `contexto/32-fix-compilacion-conpty-windows.md`

## Validación esperada

El cliente Windows debe volver a compilar con:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w' -o pangolite-client-windows-amd64.exe ./cmd/pangolite-client
```

Este cambio no altera la arquitectura ConPTY ni vuelve al workaround incorrecto de Backspace por pipes.
