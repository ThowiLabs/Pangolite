# 38 - Limpieza de base después del checkpoint de terminal

# Fecha
2026-06-28

# Objetivo
Dejar el proyecto como base limpia después de hacer commit de la terminal web y de haber mezclado archivos de checkpoints/parches anteriores.

# Decisiones tomadas
- Eliminar archivos locales de herramientas externas que no deben ir en el repositorio.
- Eliminar assets duplicados de SB Admin Pro que ya no son usados por el frontend actual.
- Mantener como assets oficiales los ubicados en `internal/app/assets/app/` porque son los únicos servidos/embebidos por `assets.go`.
- Eliminar documentos de contexto duplicados o inconsistentes con el estado final de la terminal.
- Eliminar código muerto del intento anterior de Backspace Windows, porque Windows quedó con aviso/no confiable y ya no usa ConPTY ni filtro VT.
- Mantener la terminal Linux como backend principal con PTY real.
- Mantener Windows con mensaje claro de plataforma no confiable.
- Corregir la página de Terminal para que, si entra con contexto de proyecto, liste solo clientes de ese proyecto; si entra global, lista todos.

# Arquitectura actual
- Frontend oficial: `internal/app/assets/app/`.
- Templates oficiales: `internal/app/templates/`.
- Contexto vigente de terminal: `30`, `34`, `35`, `36` y `37`.
- Linux/Alpine: terminal por PTY real.
- Windows: terminal interactiva deshabilitada temporalmente con aviso visible.
- Otros sistemas: fallback básico por pipes según build tag correspondiente.

# Librerías usadas
No se agregaron dependencias nuevas.

# Archivos importantes modificados
- `.gitignore`
- `internal/app/ui.go`
- `internal/app/terminal_test.go`
- `contexto/30-terminal-web-xterm.md`
- `contexto/38-limpieza-base-checkpoint-terminal.md`

# Archivos eliminados
- `.commandcode/`
- `internal/app/assets/sb-admin-pro/`
- `internal/app/terminal_vt.go`
- `contexto/31-fix-backspace-terminal-windows.md`
- `contexto/32-fix-compilacion-conpty-windows.md`
- `contexto/33-terminal-modular-linux-windows-backspace.md`
- `contexto/35-terminal-ux-pegado-fullscreen-proyecto.md`
- `contexto/36-fix-pegado-terminal-duplicado.md`

# Problemas encontrados
- Había dos documentos `35-*` y dos documentos `36-*` con versiones diferentes del mismo flujo.
- Algunos documentos describían ConPTY/Backspace Windows, pero el código final ya no usa esa ruta.
- `internal/app/assets/sb-admin-pro/` duplicaba CSS/JS que no estaba referenciado ni embebido.
- `.commandcode/` indicaba explícitamente que no debía incluirse en commits.
- `terminal_vt.go` solo quedaba referenciado por pruebas del intento Windows anterior.

# Soluciones implementadas
- Se dejó una sola versión vigente de cada contexto de terminal.
- Se limpió código muerto.
- Se agregó `.commandcode/` a `.gitignore`.
- Se actualizó el contexto general de terminal para reflejar que Windows muestra aviso.
- Se ajustó `panelData` para no sobrescribir la lista de clientes del proyecto cuando la página Terminal tiene `projectId` activo.

# Pendientes
- Ejecutar `go test ./...` en un entorno con Go 1.24 o superior.
- Ejecutar `bash init.sh` en servidor de prueba.
- Verificar terminal Linux desde proyecto con varios clientes para confirmar que el selector filtra correctamente.

# Próximos pasos
Usar este ZIP como nueva base limpia y continuar los siguientes cambios desde aquí, evitando copiar carpetas completas encima sin revisar eliminaciones inesperadas en Git.
