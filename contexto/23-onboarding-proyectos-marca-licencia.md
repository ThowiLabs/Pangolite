# 23 - Onboarding, proyectos, marca y licencia

## Objetivo

Eliminar la creación automática de un proyecto por defecto en instalaciones nuevas y evitar la confusión entre proyectos y clientes de sistema.

## Cambios

- Se dejó de crear `default` automáticamente cuando la base está vacía.
- Se agregó onboarding en el dashboard cuando no hay proyectos registrados.
- El onboarding guía el flujo: crear proyecto, configurar dominio, instalar cliente de sistema si aplica y crear recurso.
- Se mantiene compatibilidad de migración: si existen recursos o agentes legados sin proyecto, se crea/conserva `default` solo para no romper asociaciones existentes.
- Se quitó el marcador textual `Pg` y se reemplazó por el logo enviado por el usuario.
- Se agregaron `logo-mark.png` y `favicon.ico` embebidos en assets.
- Se agregó footer con “Creado con amor por ThowiLabs” y enlace a `https://thowilabs.com/opensource/`.
- Se agregó licencia permisiva tipo MIT con solicitud de atribución opcional.
- Se ajustaron textos donde “cliente” se usaba para referirse a proyectos, dejando “cliente de sistema” solo para agentes remotos/NAT.

## Validación esperada

- En una instalación nueva, `/api/projects` debe devolver una lista vacía y el dashboard debe mostrar onboarding.
- Crear un proyecto desde el onboarding debe ocultar la tarjeta de primeros pasos y llevar al proyecto creado.
- El logo debe mostrarse en login, cambio de contraseña y sidebar.
- El navegador debe usar el favicon embebido.
