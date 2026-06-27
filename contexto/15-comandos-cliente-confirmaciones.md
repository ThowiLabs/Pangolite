# 15 - Comandos de cliente y confirmaciones del panel

## Fecha
2026-06-27

## Objetivo
Mejorar la experiencia operativa del panel para clientes NAT y recursos que tardan unos segundos en aplicarse.

## Decisiones tomadas
- Los comandos de instalacion y eliminacion del cliente NAT se muestran como bloques `code` dentro de tarjetas copiables.
- Cada comando tiene boton Copiar y confirmacion visual al copiar al portapapeles.
- Se agrego un modal reutilizable de confirmacion para acciones destructivas o sensibles.
- Se reemplazaron confirmaciones nativas del navegador para eliminar recursos, eliminar dominios, deshabilitar clientes, rotar tokens y suspender/activar recursos rapidamente.
- Crear y editar recursos muestra un modal de proceso mientras Pangolite valida puertos, guarda datos y aplica Traefik.

## Arquitectura actual
- La UI sigue siendo HTML/CSS/JS embebido en Go, sin frontend pesado ni build con Node.
- Los componentes nuevos son funciones JS reutilizables: `confirmAction`, `showBusy`, `hideBusy`, `renderAgentCredentials` y `copyCommand`.
- El flujo de creacion de recursos no cambia a nivel backend; solo se muestra estado de progreso durante la espera.

## Archivos importantes modificados
- `internal/app/ui.go`

## Problemas detectados
- Los comandos del cliente NAT se mostraban como texto plano, incomodos de copiar.
- Las acciones sensibles usaban `confirm()` nativo, inconsistente con la interfaz del producto.
- Al crear recursos TCP/UDP remotos, la UI parecia congelada durante validaciones internas y aplicacion de Traefik.

## Soluciones aplicadas
- Comandos encapsulados en tarjetas con `<code>` y boton de copiado.
- Modal de confirmacion reutilizable con titulo, descripcion y accion configurable.
- Modal de espera con animacion para creacion/edicion de recursos.

## Pendientes
- Convertir el aviso de copia en toast flotante para evitar mover el scroll superior.
- Agregar historial visual de operaciones en progreso si se implementan colas largas.
