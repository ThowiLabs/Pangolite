# 53 - Scrollbar del sidebar unificado

## Motivo

La terminal web ya usaba un scrollbar interno oscuro, pero el sidebar seguía mostrando una barra nativa clara del navegador en Windows/Chrome cuando su contenido era más alto que la ventana.

## Cambio

Se centralizó el estilo de scrollbars para contenedores internos relevantes:

- `html` / `body`
- `.sidebar`
- `.main-content`
- `.modal-card`
- `#agentCredentialsBody`
- `.terminal-box .xterm-viewport`
- `.table-responsive`
- menús desplegables y áreas de texto/preformateadas

El sidebar ahora usa el mismo thumb oscuro, track translúcido, bordes redondeados y hover que el resto del panel.

## Notas

No se modifica la estructura del layout ni el comportamiento de scroll. Solo se normaliza el estilo visual para evitar scrollbars blancos nativos dentro del tema oscuro.
