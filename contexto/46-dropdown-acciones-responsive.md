# Fecha
2026-06-29

# Objetivo
Mejorar la experiencia responsive de las tablas principales usando un menú de acciones con botón de tres puntos en lugar de múltiples botones horizontales.

# Decisiones tomadas
- Se creó un dropdown reutilizable de acciones en frontend sin agregar dependencias nuevas.
- El menú usa posición fija calculada por JavaScript para evitar que `table-responsive` lo corte por overflow.
- Se aplica a Clientes de sistema, Recursos y Dominios administrados.
- Se conservan las mismas acciones existentes y sus validaciones.

# Arquitectura actual
- `core.js` contiene helpers globales para crear y controlar dropdowns de acciones.
- Las tablas renderizadas por servidor usan el mismo marcado CSS/JS.
- Las tablas repintadas dinámicamente usan `makeActionDropdown` y `makeActionMenuItem`.

# Librerías usadas
- Sin dependencias nuevas.
- Se reutilizan Bootstrap Icons y CSS propio del panel.

# Archivos importantes modificados
- `internal/app/assets/app/core.js`
- `internal/app/assets/app/resources.js`
- `internal/app/assets/app/projects.js`
- `internal/app/assets/app/panel.css`
- `internal/app/templates/pages/agents.html`
- `internal/app/templates/pages/resources.html`
- `internal/app/templates/pages/settings.html`

# Problemas encontrados
- Las acciones en línea ocupaban demasiado espacio en tablas, especialmente en pantallas pequeñas.
- Los menús dentro de `.table-responsive` pueden quedar recortados si se posicionan solo con `absolute`.

# Soluciones implementadas
- Menú de tres puntos con cierre por clic fuera, Escape, scroll o resize.
- Posicionamiento fijo del menú según el botón activo.
- Estados visuales diferenciados para acciones normales, advertencia y peligro.
- Compatibilidad con botones y enlaces dentro del menú.

# Pendientes
- Evaluar si las tablas de auditoría/backups también requieren menú cuando acumulen más acciones.

# Próximos pasos
- Implementar panel de salud operativo y validación fuerte antes de cambiar dominio principal.
