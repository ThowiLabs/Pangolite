# Tarea 68 — Cambio de nombre de usuario

## Estado
completado

## Objetivo
Permitir que el usuario autenticado cambie su nombre de inicio de sesión desde Perfil y seguridad sin romper sesiones existentes.

## Alcance
- Validar y normalizar el nuevo nombre con las reglas existentes.
- Impedir nombres duplicados.
- Exigir la contraseña actual para confirmar el cambio.
- Actualizar la UI y el usuario mostrado en el panel.
- Registrar auditoría sin exponer credenciales.
- Añadir pruebas mínimas de persistencia y validación.
