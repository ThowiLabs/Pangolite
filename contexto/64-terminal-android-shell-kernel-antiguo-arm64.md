# 64 - Terminal Android, shell universal y validación ARM64

## Problema confirmado

Un cliente ARMv7 ejecutado como root en una TV Box Android podía conectarse al panel y a los streams proxy, pero la terminal remota se desconectaba al abrirla.

El diagnóstico confirmó:

- Android solo dispone de `/system/bin/sh` mediante Toybox.
- La variable `SHELL` no estaba definida dentro del proceso del cliente.
- No existen `/bin/bash`, `/bin/ash` ni `/bin/sh`.
- La implementación anterior terminaba usando el fallback fijo `/bin/sh`, una ruta inexistente.
- El kernel Android 3.14 registraba el syscall 434 (`pidfd_open`) al intentar lanzar procesos desde binarios Go modernos.
- PTY, `/dev/ptmx`, `/dev/pts`, permisos root y SELinux permisivo estaban disponibles, por lo que no eran la causa principal.

## Cambios

- La detección de shell ya no devuelve una ruta inexistente como fallback.
- Se valida que cada candidato exista, sea archivo y tenga permiso de ejecución.
- Se resuelve `sh` mediante `PATH` usando `exec.LookPath`.
- Se admiten shells Linux comunes y rutas Android:
  - `/system/bin/sh`
  - `/system/xbin/bash`
  - `/vendor/bin/sh`
- La shell elegida se propaga en la variable `SHELL` de la sesión.
- Para root se reutiliza `HOME` cuando `/root` no existe, habitual en Android.
- La PTY Linux inicia el proceso con `syscall.ForkExec` y espera con `syscall.Wait4`, sin pasar por `os.StartProcess`/`os/exec`.
- La cancelación cierra la PTY y termina todo el grupo de procesos de la sesión.
- La comprobación de sudo sin contraseña usa la misma ruta de ejecución compatible.
- Se añadieron pruebas de resolución por `PATH`, candidatos Android, rechazo de archivos no ejecutables y una prueba real de ida y vuelta por PTY.

## Workflow

El workflow de release ya compilaba Linux ARM64. Se conserva esa compilación y se añade una validación obligatoria para impedir que se publique un release si faltan o están vacíos:

- `pangolite_linux_arm64.tar.gz`
- `pangolite-client_linux_arm64`

ARM64 se mantiene adicional a ARMv7; no lo reemplaza, porque algunos Android con CPU ARMv8 ejecutan únicamente ABI `armeabi-v7a` de 32 bits.

## Compatibilidad

El cambio es general para Linux. No crea un cliente separado para Android y no exige instalar Bash. En distribuciones Linux normales seguirá prefiriendo Bash/Zsh/Ash/Dash según disponibilidad; en Android podrá utilizar Toybox `sh`.
