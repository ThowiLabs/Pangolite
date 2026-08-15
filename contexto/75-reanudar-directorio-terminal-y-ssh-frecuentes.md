# 75 - Reanudar directorio de terminal y conexiones SSH frecuentes

# Fecha
2026-08-14

# Objetivo
Conservar el directorio de trabajo real de cada terminal cuando una sesión se desconecta y ofrecer en `/ssh` accesos rápidos a los destinos que más utiliza cada usuario, siguiendo una experiencia similar a las sugerencias de apps frecuentes de un launcher móvil.

# Decisiones de arquitectura
- El historial no se guarda únicamente en `localStorage`. Se añadió la migración SQLite v11 con `terminal_usage`, separada por `user_id` y `target` (`local` o `agent:<id>`).
- Por destino se guarda `connection_count`, `last_connected_at`, `last_dir` y `updated_at`.
- El ranking de `/ssh` usa el contador persistente del usuario y desempata por conexión más reciente. Solo se sugieren destinos disponibles actualmente.
- El navegador no intenta inferir el directorio leyendo comandos `cd`. Solicita `cwd.request` y el proceso dueño del PTY responde `cwd.update` usando `CurrentDir()`, que en Linux ya resuelve `/proc/<pid>/cwd` del foreground process group con fallback al shell.
- El `cwd` se consulta al conectar, cada 2.5 segundos y nuevamente después de enviar Enter. Esto mantiene una ruta reciente incluso si la red se corta sin un cierre limpio.
- Para reanudar, el navegador envía la ruta como query `cwd`. El servidor la pasa como `WorkingDir` al arranque de la PTY. Linux solo la usa si es absoluta, existe y es un directorio utilizable; de lo contrario cae al directorio predeterminado existente.
- En terminales remotas se agregó `WorkingDir` a `AgentStreamJob` y la capability `terminal-cwd-v1`. El servidor solo envía `WorkingDir` y controles `cwd.request` a agentes que anuncian esa capacidad, evitando romper clientes antiguos.

# Experiencia de Terminal
- Cuando una sesión se corta inesperadamente, la vista conserva el último `cwd` confirmado. El overlay de **Reconectar** indica la ruta y la nueva PTY se abre directamente ahí.
- Al entrar nuevamente a `/terminal`, o al elegir otro destino sin conexión activa, si existe un directorio anterior se muestra **¿Reanudar sesión?** con la ruta exacta.
- El usuario puede elegir **Reanudar** o **Iniciar desde carpeta predeterminada**.
- Si se entra desde `/ssh` con `autoconnect=1` y existe una ruta anterior, la pregunta de reanudación tiene prioridad sobre la conexión automática. Sin historial conserva el autoconnect anterior.
- La selección de ruta se mantiene por destino y por usuario; cambiar entre servidor local y agentes no mezcla directorios.

# Experiencia de Conexiones SSH
- `/ssh` incluye un bloque **Más usados** encima del directorio completo.
- Se muestran hasta cuatro destinos disponibles con más conexiones del usuario.
- Las tarjetas son más compactas que las del directorio y priorizan la acción de abrir terminal.
- El bloque se oculta automáticamente hasta que exista historial de uso.
- El directorio completo, búsqueda, paginación y filtros existentes no cambian.

# Compatibilidad
- Clientes Pangolite nuevos anuncian `terminal-cwd-v1` junto con las capacidades de transferencia existentes.
- Un servidor actualizado frente a un agente antiguo no envía controles CWD reservados ni intenta forzar `WorkingDir` remoto.
- Upload/download continúan usando sus protocolos independientes y streaming acotado.
- Windows sigue con terminal interactiva deshabilitada según las decisiones anteriores.

# Persistencia
Migración SQLite v11:

```sql
CREATE TABLE terminal_usage (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target TEXT NOT NULL,
    connection_count INTEGER NOT NULL DEFAULT 0,
    last_connected_at TEXT NOT NULL DEFAULT '',
    last_dir TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL,
    PRIMARY KEY(user_id, target)
);
```

La migración usa el mecanismo existente de backup previo a migraciones.

# Archivos principales modificados
- `README.md`
- `internal/app/store.go`
- `internal/app/terminal_usage.go`
- `internal/app/terminal.go`
- `internal/app/terminal_process_linux.go`
- `internal/app/terminal_process_other.go`
- `internal/app/tunnel.go`
- `internal/app/agent_client.go`
- `internal/app/ui.go`
- `internal/app/templates/pages/terminal.html`
- `internal/app/templates/pages/ssh_connections.html`
- `internal/app/assets/app/terminal.js`
- `internal/app/assets/app/ssh-connections.js`
- `internal/app/assets/app/panel.css`
- pruebas relacionadas.

# Validación realizada
- `node --check internal/app/assets/app/terminal.js` OK.
- `node --check internal/app/assets/app/ssh-connections.js` OK.
- `git diff --check` OK.
- `gofmt` aplicado a todos los archivos Go modificados.
- Se añadieron pruebas para persistencia/ranking, migración v11, arranque en `WorkingDir`, controles CWD y presencia de UI de reanudación/frecuentes.

# Validación Go pendiente en este entorno
`go test ./...` no puede completarse aquí porque `go.mod` requiere Go 1.26.0, el entorno dispone de Go 1.23.2 y el acceso a `proxy.golang.org` está bloqueado, por lo que no puede descargar el toolchain ni dependencias ausentes de `go.sum`.

En el entorno normal del proyecto ejecutar:

```bash
go test ./...
go vet ./...
```

# Pruebas manuales recomendadas
1. Abrir terminal local, ejecutar `cd /ruta/proyecto`, esperar unos segundos y cortar la conexión. Pulsar **Reconectar** y verificar con `pwd` que vuelve a la misma ruta.
2. Repetir con un agente Linux actualizado y confirmar `terminal-cwd-v1` mediante comportamiento de reanudación.
3. Cerrar `/terminal`, volver a entrar y verificar que aparece la pregunta con la ruta exacta y ambas opciones funcionan.
4. Elegir **Iniciar desde carpeta predeterminada** y confirmar que no fuerza el directorio anterior.
5. Conectarse varias veces a distintos destinos, volver a `/ssh` y comprobar que **Más usados** ordena por frecuencia y solo muestra destinos disponibles.
6. Iniciar sesión con otro usuario y confirmar que historial/ranking/directorios no se mezclan.
7. Probar un agente antiguo: la terminal debe seguir abriendo con su directorio tradicional sin recibir controles CWD incompatibles.
8. Regresión: uploads, `download ruta`, pantalla completa, atajos Android, búsqueda y paginación SSH deben conservar su comportamiento.
