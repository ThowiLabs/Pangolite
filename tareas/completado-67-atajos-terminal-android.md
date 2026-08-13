# 67 - Atajos táctiles para terminal Android

# Fecha
2026-08-12

# Objetivo
Agregar a la consola web una barra de teclas auxiliares inspirada en Termux para que desde Android se puedan enviar Esc, Ctrl, Alt, Tab, navegación y símbolos sin depender de teclas físicas inexistentes.

# Alcance
- Mostrar el accesorio únicamente en Android/táctil compatible.
- Mantener el teclado virtual abierto al usar los atajos.
- Implementar modificadores Ctrl/Alt de un solo uso para la siguiente tecla/entrada.
- Añadir teclas de navegación ANSI: Home, End, flechas, Page Up/Page Down.
- Añadir macros de símbolos frecuentes sin dependencias nuevas.
- Ajustar la barra sobre el teclado virtual usando Visual Viewport cuando esté disponible.
- Mantener xterm.js como única ruta normal de entrada para evitar duplicaciones.
- Añadir comprobaciones de render/estructura y validar sintaxis JavaScript.

# Estado
Completado y validado con sintaxis JavaScript, harness de atajos, parseo de template y `git diff --check`; pendiente prueba física final en Android real y suite Go completa con Go 1.26.
