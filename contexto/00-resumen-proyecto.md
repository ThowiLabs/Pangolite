# 00 - Resumen rápido de Pangolite

## Qué es
Pangolite es un panel ligero en Go inspirado en Pangolin Proxy para administrar proyectos, dominios, recursos web/TCP/UDP, clientes de sistema/NAT, certificados SSL con Traefik, suspensión/protección de recursos, auditoría, respaldos, logs y diagnóstico operativo desde una interfaz web autocontenida.

## Meta del producto
Crear una alternativa simple, premium y mantenible para exponer servicios internos o remotos sin depender de Docker obligatorio, cuidando bajo consumo, instalación sencilla en Linux, soporte Debian/Ubuntu/systemd y Alpine/OpenRC, seguridad razonable y UX clara para usuarios que pagaron por el producto.

## Arquitectura actual
- Backend Go con SQLite, migraciones, auditoría, backups, health checks y `pangolite doctor`.
- Traefik como reverse proxy global para HTTP/HTTPS y entryPoints TCP/UDP.
- Clientes de sistema/NAT conectados al servidor para publicar servicios remotos.
- Frontend autocontenido con `embed.FS`, templates físicas, layouts, componentes, páginas HTML y assets separados.
- El panel usa rutas reales del navegador, render inicial desde servidor y JavaScript solo para hidratación, modales, acciones, health, logs y copiado.
- WebSockets usan el módulo mantenido `github.com/coder/websocket v1.8.15`; no conservar imports nuevos a `nhooyr.io/websocket`.

## Reglas importantes de desarrollo
- No mencionar IA/OpenAI/ChatGPT en código, README, commits ni documentación pública.
- Mantener commits en español y continuar el historial Git existente sin reinicializarlo.
- Mantener `tareas/` numerada con estados `pendiente-`, `en-proceso-` y `completado-`; solo una tarea puede estar en proceso.
- Las entregas de trabajo deben incluir el proyecto completo con `.git/`, historial existente, `contexto/` y `tareas/`, salvo instrucción explícita posterior del usuario.
- Priorizar simplicidad agresiva, seguridad, mantenibilidad, bajo consumo y logs útiles.
- No agregar Node, Vite, React ni dependencias frontend innecesarias.
- Evitar volver a meter HTML/CSS/JS gigante dentro de `ui.go` o `server.go`.
- En `/terminal`, mantener la barra superior compacta: **Destino** y **Tema de color** viven dentro del widget de ajustes; Subir archivo y Desconectar viven dentro del menú de opciones; Pantalla completa permanece como la única acción principal permanente durante una sesión (Conectar solo aparece cuando hace falta iniciar/reintentar). No volver a exponer controles secundarios como una fila de botones/selects que degrade móvil.
- En toda la UI administrativa, priorizar una jerarquía compacta de acciones: **acciones primarias** pueden conservar icono + texto; **acciones secundarias/rutinarias** deben preferir botones de solo icono con `aria-label` y `title`; cuando una zona acumule varias acciones contextuales deben agruparse en dropdown/widget de tres puntos o ajustes. Reservar modales para confirmaciones, formularios que requieren atención o ayuda extensa; no usar un diálogo para navegación o una acción inmediata que cabe en un botón/icono. Esta regla aplica especialmente a móvil y no debe revertirse con filas horizontales de botones.
- Las transferencias de terminal deben ser streaming y de memoria acotada. `download ruta` es una pseudo-orden del navegador/Pangolite, no un binario instalado en el sistema; para directorios debe conservarse la política de ZIP seguro (sin raíces sensibles, symlinks ni archivos especiales, con límites de árbol).

## Estado destacado
- Hardening de producción aplicado: proxies confiables, rate limit de login/recuperación, redes administrativas confiables, límites HTTP/túnel y protección de concurrencia/rate limit en Traefik.
- Corregido el corte periódico de SSH/SFTP: se eliminó el timeout global heredado de 20 s y ahora existe un timeout de 30 s solo para el adjunto inicial del agente, nunca para la vida de la sesión. Los streams TCP usan keepalive y concurrencia global acotada.
- El túnel HTTP de agentes usa `http-stream-v1` en clientes actualizados, con streaming de uploads/downloads y buffers pequeños; agentes anteriores conservan temporalmente el protocolo legado de 16 MiB.
- La terminal web en Android táctil incluye una barra auxiliar tipo Termux con Esc/Ctrl/Alt/Tab, navegación ANSI, símbolos frecuentes y control del teclado virtual; los modificadores son de un solo uso y no duplican la ruta normal de entrada de xterm.js.
- El nombre de usuario del administrador puede cambiarse desde Perfil y seguridad; se normaliza en minúsculas, valida unicidad y exige la contraseña actual antes de modificar la identidad de login.
- La terminal local/remota permite subir archivos por drag & drop o selector móvil al directorio actual del PTY, usando chunks pequeños, una ventana de progreso flotante que no redimensiona xterm, avisos temporales para lotes, temporales limpiables y publicación sin sobrescritura. También permite `download ruta`: archivos por streaming HTTP al navegador y directorios como ZIP seguro, sin cargar el contenido completo en RAM; clientes remotos negocian `terminal-download-v1`.
- `pangolite-client --install` reemplaza instalaciones anteriores de forma idempotente en systemd/OpenRC/Windows; en Windows el propio CLI solicita elevación UAC para instalar o eliminar el servicio cuando no fue iniciado como administrador.
- El acceso administrativo usa por defecto modo `learn`: el primer login correcto registra su red y las sesiones posteriores desde otras redes quedan rechazadas salvo CIDR permitido.
- Go objetivo actualizado a la rama 1.26; instaladores fijan Go 1.26.5 cuando necesitan toolchain temporal.
- `ui.go` ya no contiene el frontend gigante; ahora renderiza templates.
- Existen layouts, componentes, páginas y assets en `internal/app/templates/` e `internal/app/assets/app/`.
- Header, footer, sidebar y botón global son fijos; el scroll es global del navegador.
- Los modales deben quedar siempre por encima del header/sidebar/footer.
- En móvil, el sidebar queda encima del header y se cierra al tocar fuera.
- HTTP/HTTPS se aplica dinámicamente; TCP/UDP con puerto nuevo requiere reinicio controlado de Traefik.
- OpenRC/Alpine y systemd/Debian deben seguir funcionando.

## Prioridades siguientes sugeridas
1. Rediseñar sidebar para navegación por proyecto seleccionado: Resumen, Recursos, Clientes, Dominios, Logs/Actividad y Ajustes.
2. Mejorar widget/listado de proyectos con estado, métricas rápidas y acción primaria clara.
3. Revisar responsive de tablas, modales y formularios largos en móvil real.
4. Reforzar mensajes de error operativos para Traefik, puertos, clientes desconectados, SSL y health.
6. Evaluar MFA y una UI para administrar/revocar redes administrativas confiables si habrá más operadores.
7. Autoalojar dependencias frontend actualmente cargadas desde CDN para reducir dependencia externa y permitir una CSP más estricta.

## Comandos útiles
```bash
go test ./...
bash -n init.sh
bash -n install.sh
sh -n install.sh
git diff --check
```

## Nota para continuar en una ventana nueva
Si se pierde contexto, empezar leyendo este archivo, `README.md`, `docs/arquitectura.md`, `contexto/28-refactor-frontend-templates-rutas.md` y revisar `internal/app/templates/`, `internal/app/assets/app/`, `internal/app/ui.go`, `internal/app/server.go` e `internal/app/service_manager.go`.
