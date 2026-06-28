# Fecha
2026-06-28

# Objetivo
Corregir fallo de compilación reportado durante `go test ./...` después de los ajustes finales de terminal.

# Decisiones tomadas
- Se conserva el método nuevo `currentProjectIDFromRequest`, usado por `panelData` para resolver el proyecto activo con contexto de página.
- Se agrega un wrapper compatible `projectIDFromRequest(*http.Request)` para evitar fallos si existen pruebas o código local todavía apuntando al nombre anterior.
- No se cambia el comportamiento de la terminal ni la navegación.

# Arquitectura actual
La resolución de proyecto activo queda centralizada en `currentProjectIDFromRequest`. El wrapper solo delega usando `panelPageForPath`, por lo que no duplica lógica ni crea una abstracción nueva pesada.

# Librerías usadas
Solo librería estándar de Go ya existente en el proyecto.

# Archivos importantes modificados
- `internal/app/ui.go`
- `contexto/36-fix-compilacion-projectid-terminal.md`

# Problemas encontrados
El entorno del usuario reportó que `internal/app/ui_test.go` todavía llamaba a `s.projectIDFromRequest`, pero el método había sido reemplazado por `currentProjectIDFromRequest`.

# Soluciones implementadas
Se restauró compatibilidad binaria/de pruebas con un wrapper pequeño:

```go
func (s *Server) projectIDFromRequest(r *http.Request) string {
    return s.currentProjectIDFromRequest(r, panelPageForPath(r.URL.Path))
}
```

# Pendientes
Ejecutar `go test ./...` en el servidor del usuario con Go temporal 1.26.4.

# Próximos pasos
Si las pruebas pasan, realizar el commit acumulado de la terminal.
