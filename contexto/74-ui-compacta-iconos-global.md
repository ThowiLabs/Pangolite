# Fecha
2026-08-13

# Objetivo
Extender al panel completo el lenguaje visual compacto definido primero para `/terminal`: menos filas de botones, acciones secundarias por iconos accesibles y configuración contextual dentro de widgets/dropdowns.

# Decisiones tomadas
- Una acción primaria que crea, guarda o inicia un flujo importante puede conservar icono + texto.
- Acciones secundarias y rutinarias deben preferir botones de solo icono con `aria-label` y `title` para conservar claridad sin ocupar espacio innecesario.
- Cuando existen varias acciones contextuales sobre una misma entidad se mantiene/reutiliza `action-dropdown` con tres puntos.
- Ajustes de visualización o configuración secundaria deben vivir en widgets/dropdowns compactos en vez de ocupar permanentemente el toolbar.
- Los modales quedan reservados para confirmaciones, formularios que realmente requieren foco o ayuda extensa. No se debe abrir un modal para navegación o una acción inmediata que pueda representarse con un botón/icono.
- Los controles dinámicos y server-rendered deben compartir las mismas clases para que la UI no cambie después de hidratarse.
- Los botones icon-only siempre deben conservar nombre accesible mediante `aria-label`; `title` funciona como ayuda adicional en escritorio.
- Los estados de carga no deben ensanchar un botón icon-only: durante loading se muestra únicamente spinner y se restauran label/title al terminar.

# Arquitectura actual
La UI continúa siendo HTML/CSS/JavaScript nativo embebido, usando Bootstrap Icons y los componentes existentes. `panel.css` incorpora `pl-icon-action`/`pl-icon-actions` como primitivas ligeras; no se añadió framework ni dependencia frontend.

# Librerías usadas
- CSS/JavaScript nativos existentes.
- Bootstrap Icons ya presente en el proyecto.
- Sin dependencias nuevas.

# Archivos importantes modificados
- `internal/app/templates/pages/ssh_connections.html`
- `internal/app/templates/pages/projects.html`
- `internal/app/templates/pages/resources.html`
- `internal/app/templates/pages/logs.html`
- `internal/app/templates/pages/maintenance.html`
- `internal/app/templates/pages/login.html`
- `internal/app/templates/pages/password.html`
- `internal/app/templates/pages/reset.html`
- `internal/app/templates/components/*.html` relevantes
- `internal/app/templates/components/client_templates.html`
- `internal/app/assets/app/panel.css`
- `internal/app/assets/app/core.js`
- `internal/app/assets/app/projects.js`
- `internal/app/assets/app/maintenance.js`
- `internal/app/assets/app/ssh-connections.js`
- `internal/app/ui_test.go`
- `contexto/00-resumen-proyecto.md`

# Problemas encontrados
- `/ssh` usaba dos botones anchos por tarjeta y un selector de cantidad visible permanentemente, ocupando demasiado espacio, especialmente en móvil.
- Dashboard/proyectos, logs, recursos y mantenimiento mantenían varias acciones secundarias con texto aunque su iconografía ya era inequívoca.
- Los helpers de loading/copiado asumían botones con texto; al convertir una acción a icon-only podían ensancharla temporalmente o restaurarla con un label visible.
- La UI server-rendered de proyectos y la generada por JavaScript debían mantenerse sincronizadas para evitar cambios visuales al hidratar.

# Soluciones implementadas
- `/ssh`: conectar/ajustes/proyecto pasan a iconos de 42 px; “conexiones por página” se mueve a un widget de ajustes con engrane y el buscador permanece como control principal del directorio.
- Proyectos: navegación secundaria por tarjeta se compacta en iconos; el CTA global de crear proyecto conserva texto.
- Recursos: Plantillas y Health pasan a acciones icon-only; Crear recurso conserva texto como acción primaria.
- Logs: actualizar/copiar/descargar/limpiar forman una barra compacta de iconos.
- Mantenimiento: actualización de auditoría y descarga individual de backup se compactan; Crear respaldo conserva texto por ser acción primaria y porque abre un flujo con datos opcionales.
- Autenticación y modales conservados reciben iconografía coherente; no se eliminan confirmaciones de seguridad.
- `setActionLoading` y `copyFeedback` preservan tamaño, markup y accesibilidad de botones compactos.

# Pendientes
- Validar visualmente el panel en Android real y escritorio tras despliegue.
- Ejecutar la suite completa Go con Go 1.26 y dependencias disponibles.

# Próximos pasos
- Probar `/ssh` vertical/horizontal en Android, especialmente búsqueda + engrane + tarjetas.
- Revisar Dashboard, Recursos, Logs y Mantenimiento en anchos 360/430/768/desktop.
- Mantener esta regla para futuras pantallas y no volver a introducir toolbars con muchas acciones de texto.
