# 39 - Ciclo de vida de dominios y clientes instalados

# Fecha
2026-06-28

# Objetivo
Evitar que un dominio usado para instalar clientes de sistema pueda eliminarse o desaparecer del panel de Traefik, dejando clientes antiguos sin conexión cuando el administrador cambia el dominio principal del dashboard.

# Decisiones tomadas
- Un dominio administrado ya no es solo un string eliminable: ahora tiene estado `active` o `legacy` y puede marcarse como principal.
- El dominio principal se usa para el dashboard y para generar nuevos comandos de instalación de clientes.
- Cada cliente de sistema guarda la `server_url` con la que fue generado y, cuando aplica, el `domain_id` del dominio administrado asociado.
- Si el dominio principal cambia y el dominio anterior tiene clientes asociados, el dominio anterior queda como heredado en vez de eliminarse.
- Los dominios heredados dejan de aparecer para nuevas instalaciones y nuevos recursos administrados, pero se conservan para compatibilidad con clientes existentes.
- La eliminación definitiva queda bloqueada si el dominio tiene clientes o recursos asociados.
- Traefik conserva rutas del panel para el dominio principal y para dominios heredados que todavía tengan clientes asociados.

# Arquitectura actual
- `managed_domains` agrega columnas `status` e `is_primary`.
- `agents` agrega columnas `server_url` y `domain_id`.
- `SaveAppSettings` sincroniza el dominio principal y bloquea dejar el panel sin dominio si hay clientes que dependen del dominio anterior.
- `SetPrimaryManagedDomain` marca el dominio nuevo como principal y mueve el anterior a `legacy` si todavía tiene clientes.
- `PanelDomainsForTraefik` devuelve el dominio principal más dominios heredados que siguen siendo usados por clientes.
- `RenderStaticTraefikWithPanelDomains` y `RenderDynamicTraefikWithPanelDomains` generan reglas `Host(...) || Host(...)` para conservar acceso al panel en dominios heredados.
- La UI de Ajustes muestra estado, uso por clientes/recursos y acciones: Principal, Heredar, Activar o Eliminar cuando sea seguro.

# Librerías usadas
No se agregaron dependencias nuevas. Se usó librería estándar de Go, SQLite existente y JavaScript nativo.

# Archivos importantes modificados
- `internal/app/model.go`
- `internal/app/store.go`
- `internal/app/server.go`
- `internal/app/traefik.go`
- `internal/app/ui.go`
- `internal/app/assets/app/projects.js`
- `internal/app/assets/app/agents.js`
- `internal/app/assets/app/settings.js`
- `internal/app/templates/components/client_templates.html`
- `internal/app/templates/pages/settings.html`
- `internal/app/store_test.go`
- `internal/app/traefik_test.go`
- `contexto/39-ciclo-vida-dominios-clientes.md`

# Problemas encontrados
Antes, si el administrador cambiaba o eliminaba el dominio del panel después de instalar clientes, los clientes antiguos podían quedar apuntando a una URL que ya no estaba publicada por Traefik.

# Soluciones implementadas
- Se captura la URL del servidor al crear el cliente.
- Se vinculan clientes existentes al dominio actual durante la migración si no tenían `server_url` guardada.
- Se bloquea la eliminación de dominios con dependencias.
- Se permite marcar un dominio como heredado para ocultarlo de nuevas configuraciones sin romper clientes antiguos.
- Traefik mantiene publicados dominios heredados mientras existan clientes asociados.

# Pendientes
- Implementar migración asistida de clientes conectados para actualizar su `PANGOLITE_SERVER_URL` al dominio principal nuevo.
- Mostrar listado detallado de clientes vinculados por dominio antes de marcar heredado o eliminar.
- Validar en servidor real que Traefik emite/renueva certificados correctamente para múltiples dominios del panel.

# Próximos pasos
- Ejecutar `go test ./...` en un entorno con Go 1.24 o superior.
- Probar cambio de dominio principal con un cliente conectado.
- Confirmar que el dominio anterior sigue respondiendo al panel mientras existan clientes heredados.
