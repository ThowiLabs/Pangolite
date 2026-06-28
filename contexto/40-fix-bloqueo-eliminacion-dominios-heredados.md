# Fecha
2026-06-28

# Objetivo
Corregir el bloqueo de eliminacion de dominios administrados cuando existen clientes creados antes de que el sistema guardara `server_url`/`domain_id` de forma confiable.

# Decisiones tomadas
- Un dominio principal nunca se elimina directamente.
- Un dominio usado por clientes o recursos se bloquea.
- Un dominio administrado se bloquea mientras exista al menos un cliente de sistema, aunque el conteo directo sea cero. La única eliminación física permitida es cuando no existen clientes ni recursos que puedan depender de dominios históricos.
- Al cambiar el dominio principal, el dominio anterior pasa a heredado si existen clientes en el sistema o si tiene uso directo.
- No se agregan dependencias.

# Arquitectura actual
La proteccion vive en Store mediante `hydrateManagedDomainUsageAndLock`, que calcula `DeleteLocked` y `DeleteReason` para la UI y para la validacion backend de `DeleteManagedDomain`.

# Librerias usadas
Solo libreria estandar y SQLite existente.

# Archivos importantes modificados
- `internal/app/model.go`
- `internal/app/store.go`
- `internal/app/store_test.go`
- `internal/app/assets/app/projects.js`
- `internal/app/assets/app/agents.js`

# Problemas encontrados
El primer bloqueo dependia de poder asociar cada cliente al dominio mediante `server_url` o `domain_id`. En bases existentes o clientes creados antes del ciclo de vida de dominios, esos campos pueden estar vacios. En ese caso el uso directo salia como cero y permitia eliminar un dominio heredado potencialmente usado por clientes antiguos.

# Soluciones implementadas
Se agrego un bloqueo conservador: si existe cualquier cliente de sistema, no se permite eliminar dominios administrados porque puede haber clientes antiguos que dependan de ellos aunque no haya metadatos suficientes para probarlo. En ese caso la accion correcta es marcar como heredado.

# Pendientes
Implementar migracion remota de clientes entre dominios cuando ambos dominios esten activos y el cliente este conectado.

# Proximos pasos
Probar en una base existente con dominio anterior y cliente creado antes del cambio. El boton Eliminar debe quedar deshabilitado o el backend debe rechazar la eliminacion con razon clara mientras exista cualquier cliente de sistema.
