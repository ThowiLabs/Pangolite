# 54 - Scrollbar global definitivo

## Objetivo

Unificar el estilo de todas las barras de desplazamiento del panel, no solo las barras verticales principales.

## Cambio

Se agrega una regla CSS global para que cualquier contenedor actual o futuro use el mismo diseño de scrollbar:

- Barras verticales.
- Barras horizontales.
- Esquinas de cruce entre scroll vertical y horizontal.
- Resizers diagonales nativos de `textarea` y otros elementos redimensionables.
- Sidebar, contenido, terminal, modales, tablas, menús, textareas y contenedores nuevos.

## Detalle técnico

Se aplican selectores globales `*::-webkit-scrollbar`, `*::-webkit-scrollbar-thumb`, `*::-webkit-scrollbar-track`, `*::-webkit-scrollbar-corner` y `*::-webkit-resizer` al final de `panel.css` para que funcionen como override definitivo sobre estilos nativos claros del navegador.

También se define `scrollbar-width` y `scrollbar-color` para navegadores compatibles con la especificación estándar.

## Archivos modificados

- `internal/app/assets/app/panel.css`
- `internal/app/assets/app/auth.css`


## Ajuste adicional

También se aplica el mismo estilo en `auth.css` para login, cambio inicial de contraseña y restablecimiento de contraseña.
