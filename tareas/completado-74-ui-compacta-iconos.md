# Tarea 74 - Lenguaje visual compacto por iconos

## Objetivo
Revisar toda la interfaz administrativa y aplicar una regla consistente: acciones principales con texto cuando aportan claridad, acciones secundarias/rutinarias con botones de icono y `aria-label`/`title`, y múltiples acciones contextuales dentro de dropdowns/widgets en lugar de filas de botones o diálogos innecesarios.

## Alcance
- Pulir especialmente `/ssh` para reducir botones anchos y mover la paginación/configuración visual a un widget compacto.
- Compactar acciones secundarias en Dashboard, Proyectos, Recursos, Logs y Mantenimiento.
- Añadir iconografía a acciones de autenticación y diálogos existentes que deban conservarse.
- No cambiar contratos API ni flujos críticos; es un ajuste de UI/UX.
- Registrar la regla como norma global persistente en `contexto/`.

## Estado
Completado y validado estáticamente.
