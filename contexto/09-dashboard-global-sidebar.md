# Fecha
2026-06-26

# Objetivo
Mejorar la navegación principal del panel y agregar un dashboard global de métricas.

# Decisiones tomadas
- El selector de proyecto se movió a la parte superior del sidebar porque el proyecto define el contexto de recursos y clientes de sistema.
- La vista `/projects` ahora funciona como dashboard global, no solo como tabla.
- Se integró Chart.js por CDN para gráficos de recursos por proyecto y estado global.
- Se integró Animate.css por CDN para microanimaciones visuales, respetando `prefers-reduced-motion`.
- Se mantuvo el frontend sin Node/Vite/SPA pesada.

# Arquitectura actual
- Pangolite sigue sirviendo HTML desde el binario Go.
- El dashboard consume `/api/projects` y calcula métricas globales en el navegador.
- La navegación de proyecto sigue usando rutas reales.

# Librerías usadas
- Bootstrap Icons por CDN.
- Chart.js por CDN.
- Animate.css por CDN.

# Archivos importantes modificados
- `internal/app/ui.go`
- `contexto/09-dashboard-global-sidebar.md`

# Problemas encontrados
- El selector de proyecto estaba debajo del menú de panel y no reflejaba que es el contexto principal del sistema.
- La vista de proyectos no tenía una lectura global útil para operación.

# Soluciones implementadas
- Selector de proyecto arriba del sidebar con búsqueda.
- Dashboard global con cards, gráficos y fallback si el CDN de Chart.js no carga.
- Animaciones suaves y desactivables por configuración del sistema.

# Pendientes
- Empaquetar assets CDN localmente si se requiere instalación offline.
- Agregar métricas históricas reales cuando exista tabla de eventos/auditoría.

# Próximos pasos
- Validar visualmente en VPS.
- Continuar con TCP remoto por cliente de sistema.
