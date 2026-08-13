# Fecha
2026-08-13

# Objetivo
Hacer que la instalación de `pangolite-client` sea repetible y segura cuando ya existe una versión anterior, y permitir que Windows solicite privilegios administrativos desde el propio CLI para registrar o eliminar el servicio.

# Decisiones tomadas
- Mantener un único comando `--install`: no crear un flujo separado de actualización.
- Detectar y retirar primero los artefactos del servicio anterior antes de registrar la instalación nueva.
- En Linux conservar soporte systemd/OpenRC sin agregar dependencias.
- En Windows dejar de depender de `sc.exe` para administrar el servicio y usar `golang.org/x/sys/windows/svc/mgr`, dependencia que ya existía en el proyecto.
- La elevación de Windows se realiza con `ShellExecuteExW` y el verbo `runas`; el proceso padre espera al proceso elevado y propaga un fallo si este termina con código distinto de cero.
- Validar la configuración de `--install` antes de solicitar UAC para no pedir privilegios si faltan parámetros obligatorios.

# Arquitectura actual
`cmd/pangolite-client/main.go` decide si la operación requiere privilegios. En Unix la comprobación no relanza nada y `installClient/removeClient` siguen exigiendo root. En Windows `ensureClientPrivileges` comprueba el token actual y, si es necesario, relanza exactamente los argumentos recibidos mediante UAC. Después cada implementación de sistema operativo administra su gestor de servicios nativo.

# Librerías usadas
No se agregaron dependencias. Windows reutiliza `golang.org/x/sys/windows`, `golang.org/x/sys/windows/svc` y `golang.org/x/sys/windows/svc/mgr`, ya declaradas indirecta/directamente por el módulo existente. Linux usa exclusivamente standard library y comandos nativos de systemd/OpenRC.

# Archivos importantes modificados
- `cmd/pangolite-client/main.go`
- `cmd/pangolite-client/install_unix.go`
- `cmd/pangolite-client/install_windows.go`
- `README.md`
- `contexto/00-resumen-proyecto.md`
- `tareas/completado-71-reinstalacion-servicio-cliente.md`

# Problemas encontrados
- La instalación Unix anterior podía escribir encima del cliente sin limpiar de forma explícita servicios previos.
- Windows ejecutaba `sc.exe stop/delete/create/start` ignorando algunos errores de stop/delete y el CLI no era capaz de elevarse por sí mismo.
- Una recreación inmediata de un servicio Windows puede fallar si el Service Control Manager todavía lo conserva como marcado para eliminación.
- El entorno de trabajo disponible no tiene Go 1.26 ni acceso de red para descargar el toolchain/dependencias, por lo que no es posible ejecutar la compilación completa del proyecto aquí.

# Soluciones implementadas
- Linux detecta `/opt/pangolite-client`, unidades systemd y scripts OpenRC previos, detiene/deshabilita lo que corresponda, limpia artefactos y registra nuevamente el servicio.
- Windows consulta el Service Control Manager, solicita stop si procede, espera el estado `Stopped`, elimina el servicio y espera hasta que deje de existir antes de recrearlo.
- El servicio Windows configura recuperación automática escalonada a los 5 s, 15 s y 30 s, con reinicio del contador tras 300 s estables.
- Si el CLI Windows no está elevado, solicita UAC con `runas`, conserva los argumentos originales y espera el código de salida del proceso elevado.
- Si el instalador Windows se ejecuta desde el propio binario instalado, evita intentar reemplazarse a sí mismo; aun así actualiza configuración y recrea el servicio.
- Las copias temporales de binarios eliminan el `.tmp` cuando una copia/rename falla.

# Pendientes
- Ejecutar `go test ./...`, `go vet ./...` y una compilación cruzada Windows con Go >= 1.26 en un entorno con dependencias disponibles.
- Probar manualmente en Windows una actualización desde un binario descargado y una reinstalación ejecutando el binario ya instalado.
- Si se requiere eliminar también el propio ejecutable cuando `--remove` se lanza desde `C:\ProgramData\Pangolite Client`, implementar una limpieza diferida específica de Windows; la administración del servicio ya queda eliminada correctamente.

# Próximos pasos
1. Probar `--install` dos veces consecutivas en Debian/Ubuntu, Alpine y Windows.
2. Confirmar que tras reemplazo solo existe una instancia del cliente y conserva la configuración nueva.
3. En Windows verificar el prompt UAC ejecutando el CLI desde una terminal no elevada.
