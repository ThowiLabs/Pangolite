# 76 - Fix de reanudación de terminal, frecuentes SSH, versionado y purga ZIP

# Fecha
2026-08-15

# Problema observado
Las funciones documentadas en el contexto 75 podían parecer inexistentes porque el bloque `Más usados` permanecía oculto hasta tener historial y Terminal dependía exclusivamente de `terminalUsage` incluido en el bootstrap HTML de la página. Además, `Pangolite.zip` seguía trackeado en Git y permanecía en commits históricos.

# Correcciones
- `/ssh` mantiene visible el bloque `Más usados` desde la primera carga. Sin historial muestra un estado vacío explicando cómo se poblará.
- Se añadió `GET /api/terminal/state`, autenticado y sin caché desde el navegador, para leer el historial/directorio persistido del usuario de forma autoritativa.
- Terminal mezcla el bootstrap inicial con el estado obtenido por API antes de decidir si debe mostrar `¿Reanudar sesión?`.
- El bridge de terminal publica y persiste el `cwd` inicial inmediatamente, sin esperar al primer `cwd.request` del navegador. También intenta persistir una última vez al finalizar la PTY.
- `/ssh` refresca el ranking desde `/api/terminal/state`, por lo que una recarga muestra el uso registrado aunque el bootstrap HTML haya quedado desfasado.

# Versionado
- Versión actual de desarrollo: `0.27`.
- Código de versión: `27`.
- Ambos valores viven en `internal/app/version.go` y son compartidos por servidor y cliente.
- `pangolite version`, `pangolite --version`, `pangolite -v` y `pangolite-client --version` muestran versión y código.
- El `User-Agent` del cliente ya no está fijado a `pangolite-client/0.5`; usa la versión real.
- GitHub Actions inyecta `Version` y `VersionCode` con `ldflags` y genera `VERSION_CODE` en paquetes.
- `Makefile` acepta `VERSION` y `VERSION_CODE`.
- El panel muestra versión y código en el footer.

# Limpieza Git
Se reescribió todo el historial con `git filter-branch` para eliminar `Pangolite.zip`. Después se eliminaron refs de respaldo, reflogs y objetos inalcanzables con `git gc --prune=now --aggressive`. La verificación `git rev-list --objects --all | grep -Ei '\.(zip)$'` no devuelve resultados.

Se añadieron `*.zip` y `*.ZIP` a `.gitignore` para impedir que paquetes de entrega vuelvan a entrar al repositorio.

# Importante al sincronizar remoto
Como se reescribió el historial, los hashes desde el punto donde entró el ZIP cambiaron. Para actualizar un remoto existente debe usarse un push forzado seguro (`git push --force-with-lease --all` y tags si corresponde) desde una copia revisada.

# Pruebas manuales
1. Abrir `/ssh` antes de conectarse: debe verse `Más usados` con estado vacío.
2. Conectarse a un destino, trabajar en otra carpeta, volver a `/ssh`: debe aparecer en frecuentes.
3. En Terminal ejecutar `cd /ruta`, esperar al menos un cambio de prompt, cortar el WebSocket y pulsar Reconectar: `pwd` debe conservar `/ruta`.
4. Salir de `/terminal` y volver: debe aparecer `¿Reanudar sesión?` con la ruta exacta.
5. Ejecutar `pangolite --version` y `pangolite-client --version`: ambos deben reportar `0.27 (code 27)` en builds locales actuales.
6. Confirmar que `git rev-list --objects --all` no contiene rutas `.zip`.
