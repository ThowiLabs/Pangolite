async function showAgentDetail(id){try{const data=await api('/api/agents/'+id);const a=data.agent||{};const rs=data.resources||[];$('agentDetailTitle').textContent=a.name||'Cliente de sistema';$('agentDetailMeta').textContent=(a.online?'Online':'Offline')+' · '+(a.os||'sistema desconocido')+'/'+(a.arch||'-')+' · '+(a.hostname||a.publicIp||'sin hostname');const body=$('agentDetailBody');clearNode(body);const info=tplNode('tpl-agent-detail-info');setSlot(info,'publicIp',a.publicIp||'-');setSlot(info,'privateIp',a.privateIp||'-');setSlot(info,'version',a.version||'-');setSlot(info,'lastSeen',fmt(a.lastSeen));body.appendChild(info);const card=tplNode('tpl-agent-detail-resources');const tbody=slot(card,'body');if(rs.length){rs.forEach(r=>{const row=tplNode('tpl-agent-detail-resource-row');appendSlot(row,'state',stateNode(!!r.enabled));setSlot(row,'type',r.mode);setSlot(row,'name',r.name);setSlot(row,'entry',label(r));setSlot(row,'backend',(r.backendHost||'-')+':'+(r.backendPort||'-'));tbody.appendChild(row)})}else{tbody.appendChild(makeEmpty('Este cliente de sistema no tiene recursos asociados.','tr',5))}body.appendChild(card);$('agentDetailModal').classList.add('open')}catch(err){msg(err.message,true)}}
function closeAgentDetail(){$('agentDetailModal').classList.remove('open')}
function selectResource(id){openEditResource(id)}
async function saveResourceControl(){msg('La suspensión ahora se gestiona desde el botón Suspender de cada recurso.',true)}
async function activateSelectedResource(){msg('La activación ahora se gestiona desde el botón Activar de cada recurso.',true)}
async function quickSuspend(id,mode){const r=resources.find(x=>x.id===id);if(!await confirmAction('Suspender recurso','El recurso '+(r?r.name:shortID(id))+' dejara de responder hacia su backend y mostrara la respuesta configurada.','Suspender',false))return;try{await api('/api/resources/'+id,{method:'PATCH',body:JSON.stringify({enabled:false,disabledResponseMode:mode,disabledStatusCode:mode==='404'?404:403,disabledHtml:''})});msg('Recurso suspendido');await reloadProjects();await loadProjectData(currentProject.id)}catch(err){msg(err.message,true)}}
async function quickActivate(id){const r=resources.find(x=>x.id===id);if(!await confirmAction('Activar recurso','El recurso '+(r?r.name:shortID(id))+' volvera a enviar trafico al servicio interno.','Activar',false))return;try{await api('/api/resources/'+id,{method:'PATCH',body:JSON.stringify({enabled:true,disabledResponseMode:(r&&r.disabledResponseMode)||'403',disabledStatusCode:(r&&r.disabledStatusCode)||403,disabledHtml:(r&&r.disabledHtml)||''})});msg('Recurso activado');await reloadProjects();await loadProjectData(currentProject.id)}catch(err){msg(err.message,true)}}
async function saveProjectSettings(e){e.preventDefault();if(!currentProject){msg('Selecciona un proyecto',true);return false}try{const name=fieldValue('projectEditName');const notes=fieldValue('projectEditNotes');if(!name)throw new Error('Nombre de proyecto requerido');showBusy('Guardando proyecto','Actualizando nombre y descripción');const updated=await api('/api/projects/'+currentProject.id,{method:'PATCH',body:JSON.stringify({name,notes})});currentProject=updated;await reloadProjects();await loadProjectData(updated.id);paintProjectOverview();msg('Proyecto actualizado')}catch(err){msg(err.message,true)}finally{hideBusy()}return false}
async function deleteCurrentProject(){if(!currentProject)return;const st=stats[currentProject.id]||{};if((st.resources||resources.length)>0||(st.agents||agents.length)>0){msg('Primero elimina todos los recursos y clientes de sistema de este proyecto',true);return}if(!await confirmAction('Eliminar proyecto','Se eliminara el proyecto '+currentProject.name+'. Esta accion no se puede deshacer.','Eliminar proyecto'))return;try{await api('/api/projects/'+currentProject.id,{method:'DELETE'});msg('Proyecto eliminado');currentProject=null;await reloadProjects();go('/projects')}catch(err){msg(err.message,true)}}
async function deleteAgent(id){const a=agents.find(x=>x.id===id);const count=a?(a.resourceCount||0):0;const body='Eliminar el cliente de sistema '+(a?a.name:shortID(id))+' también eliminará '+count+' recurso(s) vinculado(s). Escribe tu contraseña para confirmar.';const password=await confirmPasswordAction('Eliminar cliente de sistema',body,'Eliminar cliente de sistema y recursos');if(!password)return;try{showBusy('Eliminando cliente de sistema','Eliminando recursos vinculados y aplicando Traefik');const res=await api('/api/agents/'+id,{method:'DELETE',body:JSON.stringify({password})});msg('Cliente de sistema eliminado. Recursos eliminados: '+(res.deletedResources||0));await reloadProjects();if(currentProject)await loadProjectData(currentProject.id)}catch(err){msg(err.message,true)}finally{hideBusy()}}
async function rotateAgentToken(id){const a0=agents.find(x=>x.id===id);if(!await confirmAction('Rotar token del cliente de sistema','El token actual dejara de servir. Deberas reinstalar o actualizar el cliente de sistema '+(a0?a0.name:shortID(id))+' con el nuevo comando. Tambien se incluirá fallback por IP si el servidor lo conoce.','Rotar token'))return;try{const a=await api('/api/agents/'+id+'/token',{method:'POST',body:'{}'});const os=selectedAgentOS(a0,'linux');$('agentTokenBox').replaceChildren(renderAgentCredentials(a,os));$('agentTokenBox').classList.remove('d-none');showAgentCredentialsDialog(a,{title:'Token rotado',meta:'El token anterior ya no conecta. Usa Instalar/actualizar para sobrescribir la configuracion o Eliminar para borrar una instalacion vieja. El fallback por IP queda incluido si esta disponible.',os});msg('Token rotado. Copia el nuevo comando ahora.');if(currentProject)await loadProjectData(currentProject.id)}catch(err){msg(err.message,true)}}
async function refreshDomainsFromResponse(data){if(data&&data.domains)domains=data.domains;else await reloadDomains();paintDomains();fillDomainSelect();if(data&&data.settings){panelSettings=data.settings;if(data.network)networkInfo=data.network;if($('dashboardDomain'))$('dashboardDomain').value=panelSettings.dashboardDomain||'';if(data.certificate)paintNetworkInfo(data.certificate)}}
async function markDomainLegacy(id){const d=domains.find(x=>x.id===id);if(!await confirmAction('Marcar dominio heredado','El dominio '+(d?d.domain:shortID(id))+' dejará de aparecer en nuevas instalaciones, pero se conservará para clientes antiguos.','Marcar heredado',false))return;try{const data=await api('/api/domains/'+id,{method:'PATCH',body:JSON.stringify({status:'legacy'})});await refreshDomainsFromResponse(data);msg('Dominio marcado como heredado')}catch(err){msg(err.message,true)}}
async function activateDomain(id){try{const data=await api('/api/domains/'+id,{method:'PATCH',body:JSON.stringify({status:'active'})});await refreshDomainsFromResponse(data);msg('Dominio activado para nuevas configuraciones')}catch(err){msg(err.message,true)}}
async function makeDomainPrimary(id){const d=domains.find(x=>x.id===id);const name=d?d.domain:shortID(id);if(!await confirmAction('Cambiar dominio principal','El dominio '+name+' será el dominio del panel y de los nuevos clientes. El dominio principal anterior pasará a Heredado si tiene uso. No se elimina nada en este paso.','Hacer principal',false))return;try{const data=await api('/api/domains/'+id,{method:'PATCH',body:JSON.stringify({primary:true})});await refreshDomainsFromResponse(data);msg('Dominio principal actualizado. Los clientes nuevos usarán '+name)}catch(err){msg(err.message,true)}}
async function deleteDomain(id){const d=domains.find(x=>x.id===id);if(d&&d.deleteLocked){msg(d.deleteReason||'No se puede eliminar este dominio todavía.',true);return}if(d&&d.primary){msg('No se puede eliminar el dominio principal. Primero establece otro dominio como principal.',true);return}if(d&&(d.resourceCount||0)>0){msg('No se puede eliminar: el dominio todavía tiene recursos asociados.',true);return}const name=d?d.domain:shortID(id);let body='Se eliminará definitivamente '+name+'.';if(d&&(d.agentCount||0)>0){body+=' Hay '+(d.agentCount||0)+' cliente(s) vinculados, pero el servidor los considera seguros porque ya confirmaron fallback por IP. Si esos clientes fueron reinstalados manualmente sin fallback, no elimines este dominio todavía.'}else{body+=' Esta acción solo debería usarse cuando ya no tiene clientes ni recursos asociados.'}if(!await confirmAction('Eliminar dominio administrado',body,'Eliminar dominio'))return;try{const data=await api('/api/domains/'+id,{method:'DELETE'});await refreshDomainsFromResponse(data);msg('Dominio eliminado')}catch(err){msg(err.message,true)}}
async function renderTraefik(){try{const r=await api('/api/render-traefik',{method:'POST',body:'{}'});msg(r.message||'Traefik actualizado')}catch(err){msg(err.message,true)}}
async function loadConfig(){const cfg=maybeEl('config');if(!cfg)return;cfg.textContent=await fetch('/api/v1/traefik-config').then(r=>r.text())}
async function route(options={}){
  const initial=!!options.initial;
  document.querySelectorAll('.route-view').forEach(v=>v.classList.add('active'));
  document.querySelectorAll('.nav-link').forEach(a=>a.classList.remove('active'));
  document.querySelectorAll('.project-nav-link').forEach(a=>a.classList.remove('active'));
  const path=location.pathname;
  if(path==='/'||path==='/projects'){
    setTop('Dashboard','Operación global');
    activateNav('projects');
    renderGlobalDashboard();
    return;
  }
  if(path==='/logs'){
    setTop('Logs','Diagnostico del sistema');
    activateNav('logs');
    if(!(initial&&appBoot&&appBoot.pageKey==='logs'))await loadLogs();
    return;
  }
  if(path==='/terminal'){
    setTop('Terminal','Consola web');
    activateNav('terminal');
    if(currentProject){
      document.querySelectorAll('[data-project-nav="'+currentProject.id+'"]').forEach(a=>a.classList.add('active'));
      updateProjectSwitcherLabel();
    }
    return;
  }
  if(path==='/maintenance'){
    setTop('Seguridad','Auditoría y respaldos');
    activateNav('maintenance');
    if(!(initial&&appBoot&&appBoot.pageKey==='maintenance'))await loadMaintenance();
    return;
  }
  if(path==='/settings'){
    setTop('Ajustes','Configuración del sistema');
    activateNav('settings');
    if(initial){paintDomains();fillDomainSelect();}
    if(!initial){await loadSettings();await reloadDomains();}
    if(!initial&&maybeEl('config'))await loadConfig();
    return;
  }
  const m=path.match(/^\/projects\/([^/]+)(?:\/(resources|agents)(?:\/(create))?|settings)?$/);
  if(!m){go('/projects');return}
  const id=m[1];
  const section=m[2]||'overview';
  const action=m[3]||'';
  currentProject=projects.find(p=>p.id===id||p.slug===id)||currentProject;
  if(!currentProject||currentProject.id!==id){
    await reloadProjects();
    currentProject=projects.find(p=>p.id===id||p.slug===id);
  }
  if(!currentProject){msg('Proyecto no encontrado',true);go('/projects');return}
  const projectID=currentProject.id;
  document.querySelectorAll('[data-project-nav="'+projectID+'"]').forEach(a=>a.classList.add('active'));
  updateProjectSwitcherLabel();
  const hasBootProject=initial&&appBoot&&appBoot.hasProject&&currentProject&&(currentProject.id===id||currentProject.slug===id);
  if(!hasBootProject){
    await loadProjectData(projectID);
  }else{
    fillAgentSelect();
  }
  setHrefIfExists('goCreateResource','/projects/'+projectID+'/resources/create');
  setHrefIfExists('goCreateAgent','/projects/'+projectID+'/agents/create');
  setHrefIfExists('goResources','/projects/'+projectID+'/resources');
  setHrefIfExists('goAgents','/projects/'+projectID+'/agents');
  setHrefIfExists('goCreateResourceFromList','/projects/'+projectID+'/resources/create');
  setHrefIfExists('goCreateAgentFromList','/projects/'+projectID+'/agents/create');
  if(section==='resources'&&action==='create'){
    setTop(currentProject.name,'Crear recurso');
    activateProjectNav('resources');
    fillDomainSelect();
    fillAgentSelect();
    return;
  }
  if(section==='agents'&&action==='create'){
    setTop(currentProject.name,'Crear cliente de sistema');
    activateProjectNav('agents');
    return;
  }
  if(section==='resources'){
    setTop(currentProject.name,'Recursos');
    activateProjectNav('resources');
    if(resources.length)checkResourceHealth(true);
    return;
  }
  if(section==='agents'){
    setTop(currentProject.name,'Clientes de sistema');
    activateProjectNav('agents');
    return;
  }
  setTop(currentProject.name,'Resumen');
  activateProjectNav('overview');
  updateSidebarProjectContext();
  if(!initial)paintProjectOverview();
}

