# 25 - Suspensión avanzada y protección de recursos

Se agregó suspensión avanzada para recursos web: la tabla de acciones usa un solo botón que abre un modal para elegir suspensión simple, plantilla física existente o HTML personalizado.

Las plantillas viven en `PANGOLITE_SUSPENSION_TEMPLATE_DIR` o, por defecto, en `data/templates/suspension`. Las plantillas default se crean automáticamente si no existen y se editan desde el panel.

Variables soportadas en plantillas: `$nombredominio`, `$dominio`, `$nombrerecurso`, `$recurso`, `$proyecto`, `$codigo`, `$motivo` y `$fecha`.

Se agregó validador de HTML para bloquear scripts, iframes, objetos embebidos, SVG, formularios, atributos `on*`, URLs `javascript:` y `data:text/html`.

También se agregó protección por recurso web con tres modos: sin protección, contraseña específica o solo sesión Pangolite. La contraseña puede usar login HTML o Basic Auth para APIs. Cuando hay protección activa, Traefik enruta primero a Pangolite y Pangolite proxyfica al backend tras validar acceso.
