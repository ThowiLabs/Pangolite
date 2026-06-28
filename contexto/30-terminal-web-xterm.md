# 30 - Terminal web xterm

## Objetivo

Agregar una consola web premium al panel Pangolite para administrar el servidor local y clientes de sistema conectados desde el navegador, con una experiencia similar a Cockpit: xterm.js en frontend, WebSocket en el panel y shell interactiva en backend.

## Alcance implementado

- Ruta `/terminal` dentro del panel.
- Terminal local del servidor Pangolite por WebSocket en `/api/terminal/local`.
- Terminal remota de clientes de sistema por WebSocket en `/api/terminal/agents/{id}`.
- En Linux/Alpine se usa PTY real sobre `/dev/ptmx`, por lo que `/bin/sh`, `/bin/ash` o `/bin/bash` se comportan como terminal interactiva.
- En Windows la consola interactiva remota queda deshabilitada temporalmente y muestra un aviso claro, porque el soporte por servicio/sesión puede fallar demasiado hasta tener una implementación estable.
- El cliente de sistema reutiliza el canal de streams persistentes ya usado para TCP remoto, agregando el modo `terminal`.
- La terminal corre con los permisos del proceso Pangolite o del cliente de sistema. Si el servicio corre como root, la terminal local tiene privilegios root.

## Seguridad aplicada

- Requiere sesión administrativa del panel.
- Verifica `Origin` en WebSocket para bloquear uso cruzado desde otros sitios.
- Bloquea terminal si el usuario aún debe cambiar contraseña temporal.
- Registra auditoría `terminal.open` para terminal local y remota.
- Clientes remotos deben estar habilitados y online para abrir consola.

## Limitaciones actuales

- xterm.js se carga desde CDN jsDelivr, igual que otros assets externos existentes del panel. Si se quiere operación 100% offline, se debe empaquetar xterm en `internal/app/assets/app/vendor/`.
- Windows no intenta abrir terminal interactiva por ahora. Debe mostrar advertencia y recomendar RDP, PowerShell Remoting o SSH mientras se implementa soporte confiable.
- La terminal remota reenvía resize mediante mensajes de control internos con prefijo reservado para evitar que JSON de control llegue al shell.
