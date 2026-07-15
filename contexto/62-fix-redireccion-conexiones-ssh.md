# 62 - Corrección de redirección en Conexiones SSH

## Problema
La ruta del servidor `/ssh` estaba registrada y renderizaba correctamente la página, pero el router global del frontend no incluía un caso para esa dirección.

Durante la inicialización, `route()` interpretaba `/ssh` como una ruta desconocida y ejecutaba su fallback `go('/projects')`, por lo que el usuario veía una redirección inmediata al listado de proyectos.

## Corrección
- Se agregó el caso explícito `/ssh` antes del fallback de rutas de proyecto.
- La ruta conserva los encabezados `Acceso remoto` y `Conexiones SSH`.
- Se activa correctamente la entrada global `Conexiones SSH` del sidebar.
- La ruta `/terminal` ahora mantiene activa esa misma entrada, ya que ambas vistas pertenecen al mismo flujo de acceso remoto.
- Se agregaron pruebas de regresión sobre el archivo JavaScript embebido para impedir que `/ssh` vuelva a quedar detrás del fallback hacia `/projects`.

## Archivos modificados
- `internal/app/assets/app/agents.js`
- `internal/app/ui_test.go`
- `contexto/62-fix-redireccion-conexiones-ssh.md`

## Pruebas recomendadas
1. Abrir `/ssh` desde el sidebar y confirmar que la URL permanezca en `/ssh`.
2. Recargar directamente `/ssh` con `Ctrl+F5`.
3. Abrir `/ssh?q=cliente&page=2&perPage=18` y confirmar que no se pierdan los parámetros.
4. Entrar a una terminal y comprobar que `Conexiones SSH` permanezca resaltado en el sidebar.
5. Regresar a `/projects` y confirmar que la navegación de proyectos siga funcionando.
