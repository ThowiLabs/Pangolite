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

## Reglas importantes de desarrollo
- No mencionar IA/OpenAI/ChatGPT en código, README, commits ni documentación pública.
- Mantener commits en español.
- Entregar ZIP limpio sin `.git` cuando el usuario lo pida.
- Priorizar simplicidad agresiva, seguridad, mantenibilidad, bajo consumo y logs útiles.
- No agregar Node, Vite, React ni dependencias frontend innecesarias.
- Evitar volver a meter HTML/CSS/JS gigante dentro de `ui.go` o `server.go`.

## Estado destacado
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
5. Mantener revisión de seguridad: XSS, CSRF, headers, validación de inputs, sesiones, rate limit y SQL parametrizado.

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
