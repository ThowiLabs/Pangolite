function addTemplateOption(select,value,label,selected=false){const opt=document.createElement('option');opt.value=value;opt.textContent=label;if(selected)opt.selected=true;select.appendChild(opt)}
function refreshTemplateSelects(){['disabledPreset','editDisabledPreset','resourceActionTemplate','templateManagerSelect'].forEach(id=>{const el=maybeEl(id);if(!el)return;const current=el.value;clearNode(el);addTemplateOption(el,'','Sin plantilla / HTML personalizado',current==='');suspensionTemplates.forEach(t=>addTemplateOption(el,t.id,(t.name||t.id)+' ('+t.id+')',current===t.id));if(current)el.value=current;if(typeof refreshWidgetSelect==='function')refreshWidgetSelect(id);});}
async function reloadSuspensionTemplates(){try{const data=await api('/api/suspension-templates');suspensionTemplates=data.templates||[];refreshTemplateSelects()}catch(err){console.warn(err)}}
async function templateHTML(id){if(!id)return '';const data=await api('/api/suspension-templates/'+encodeURIComponent(id));return data.html||''}
function closeResourceActionModal(){$('resourceActionModal').classList.remove('open')}
function selectedMaintenanceScopes(){return{web:fieldChecked('maintenanceScopeWeb'),tcp:fieldChecked('maintenanceScopeTCP'),udp:fieldChecked('maintenanceScopeUDP')}} 
function hasSelectedWebScope(){const scope=fieldValue('resourceActionScope')||'resource';if(scope==='resource'){const r=resources.find(x=>x.id===fieldValue('resourceActionId'));return !r||r.mode==='http'}return fieldChecked('maintenanceScopeWeb')}
function syncResourceActionMode(){
  const mode=fieldValue('resourceActionMode')||'simple';
  const scope=fieldValue('resourceActionScope')||'resource';
  const op=fieldValue('maintenanceOperation')||'suspend';
  const scopes=selectedMaintenanceScopes();
  const webActive=op==='suspend'&&hasSelectedWebScope();
  const tcpudpActive=scope==='agent'&&op==='suspend'&&(scopes.tcp||scopes.udp)&&!scopes.web;
  maybeEl('maintenanceWebOptions')?.classList.toggle('d-none',!webActive);
  maybeEl('maintenanceTcpUdpOnlyNotice')?.classList.toggle('d-none',!tcpudpActive);
  maybeEl('maintenanceTcpUdpWarning')?.classList.toggle('d-none',!(scope==='agent'&&(scopes.tcp||scopes.udp)));
  document.querySelectorAll('.resource-action-html-only').forEach(el=>el.classList.toggle('d-none',mode==='simple'||!webActive));
  document.querySelectorAll('.resource-action-template-only').forEach(el=>el.classList.toggle('d-none',mode!=='template'||!webActive));
  document.querySelectorAll('.resource-action-custom-only').forEach(el=>el.classList.toggle('d-none',mode!=='custom'||!webActive));
  const btn=maybeEl('resourceActionApplyButton');
  if(btn){const suspend=op==='suspend';setButtonContent(btn,suspend?'bi-pause-circle':'bi-play-circle',suspend?'Aplicar suspensión':'Reactivar seleccionados');btn.className=suspend?'btn btn-outline-danger':'btn btn-outline-success'}
}
function defaultMaintenanceMode(){return 'simple'}
function fillWebActionDefaults(){setIfExists('resourceActionMode',defaultMaintenanceMode());setIfExists('resourceActionStatus','403');setIfExists('resourceActionHTML','');refreshTemplateSelects();setIfExists('resourceActionTemplate',suspensionTemplates[0]?suspensionTemplates[0].id:'');syncResourceActionMode()}
function setSlotText(id,value){const el=maybeEl(id);if(el)el.textContent=String(value??'')}
function setScopeMeta(a){
  setSlotText('maintenanceScopeWebMeta',(a.webEnabledCount||0)+' activos / '+(a.webResourceCount||0)+' total · '+(a.webSuspendedCount||0)+' suspendidos');
  setSlotText('maintenanceScopeTCPMeta',(a.tcpEnabledCount||0)+' activos / '+(a.tcpResourceCount||0)+' total · '+(a.tcpSuspendedCount||0)+' suspendidos');
  setSlotText('maintenanceScopeUDPMeta',(a.udpEnabledCount||0)+' activos / '+(a.udpResourceCount||0)+' total · '+(a.udpSuspendedCount||0)+' suspendidos');
}
function openResourceAction(id){openMaintenanceResource(id)}
function openMaintenanceResource(id){
  const r=resources.find(x=>x.id===id);if(!r)return;if(!r.enabled){quickActivate(id);return}
  setIfExists('resourceActionScope','resource');setIfExists('resourceActionId',id);setIfExists('resourceActionAgentId','');
  maybeEl('maintenanceOperationBox')?.classList.add('d-none');maybeEl('maintenanceScopeBox')?.classList.add('d-none');
  $('resourceActionTitle').textContent='Suspender '+r.name;
  $('resourceActionMeta').textContent=label(r);
  if(r.mode==='http'){$('maintenanceTcpUdpOnlyNotice').classList.add('d-none');fillWebActionDefaults()}else{maybeEl('maintenanceWebOptions')?.classList.add('d-none');maybeEl('maintenanceTcpUdpOnlyNotice')?.classList.remove('d-none')}
  $('resourceActionModal').classList.add('open')
}
function openAgentMaintenance(id){
  const a=agents.find(x=>x.id===id);if(!a)return;
  setIfExists('resourceActionScope','agent');setIfExists('resourceActionId','');setIfExists('resourceActionAgentId',id);
  maybeEl('maintenanceOperationBox')?.classList.remove('d-none');maybeEl('maintenanceScopeBox')?.classList.remove('d-none');
  const active=!!a.maintenanceActive;
  setIfExists('maintenanceOperation',active?'resume':'suspend');
  setIfExists('maintenanceScopeWeb',active?(a.webSuspendedCount||0)>0:(a.webEnabledCount||0)>0);
  setIfExists('maintenanceScopeTCP',active?(a.tcpSuspendedCount||0)>0:false);
  setIfExists('maintenanceScopeUDP',active?(a.udpSuspendedCount||0)>0:false);
  ['maintenanceScopeWeb','maintenanceScopeTCP','maintenanceScopeUDP'].forEach(k=>{const el=maybeEl(k);if(el){el.checked=fieldValue(k)==='true'||el.checked}});
  setScopeMeta(a);
  $('resourceActionTitle').textContent=(active?'Reactivar mantenimiento de ':'Aplicar mantenimiento a ')+a.name;
  $('resourceActionMeta').textContent='Selecciona Web, TCP/UDP o ambos. TCP/UDP puede cortar SSH u otros túneles privados.';
  fillWebActionDefaults();
  $('resourceActionModal').classList.add('open')
}
function maintenanceWebPayload(){
  const mode=fieldValue('resourceActionMode')||'simple';
  if(mode==='hidden')return{disabledResponseMode:'hidden',disabledStatusCode:404,disabledHtml:'',disabledTemplateId:''};
  if(mode==='template'){const templateId=fieldValue('resourceActionTemplate');if(!templateId)throw new Error('Selecciona una plantilla');return{disabledResponseMode:'html',disabledStatusCode:fieldNumber('resourceActionStatus')||403,disabledHtml:'',disabledTemplateId:templateId}}
  if(mode==='custom'){const html=fieldValue('resourceActionHTML');if(!html)throw new Error('Escribe el HTML personalizado');return{disabledResponseMode:'html',disabledStatusCode:fieldNumber('resourceActionStatus')||403,disabledHtml:html,disabledTemplateId:''}}
  return{disabledResponseMode:'403',disabledStatusCode:403,disabledHtml:'',disabledTemplateId:''}
}
async function applyResourceAction(button=null){await withActionLoading(button,'Aplicando',async()=>{try{const scope=fieldValue('resourceActionScope')||'resource';if(scope==='agent'){await applyAgentMaintenanceAction();return}const id=fieldValue('resourceActionId');if(!id)throw new Error('Recurso no seleccionado');const r=resources.find(x=>x.id===id);if(!r)throw new Error('Recurso no encontrado');let payload={enabled:false,disabledResponseMode:'403',disabledStatusCode:403,disabledHtml:'',disabledTemplateId:''};if(r.mode==='http')payload=Object.assign({enabled:false},maintenanceWebPayload());await api('/api/resources/'+id,{method:'PATCH',body:JSON.stringify(payload)});closeResourceActionModal();msg('Suspensión aplicada');await reloadProjects();if(currentProject)await loadProjectData(currentProject.id)}catch(err){msg(err.message,true)}})}
async function applyAgentMaintenanceAction(){
  const id=fieldValue('resourceActionAgentId');if(!id)throw new Error('Cliente no seleccionado');
  const op=fieldValue('maintenanceOperation')||'suspend';const scopes=selectedMaintenanceScopes();
  if(!scopes.web&&!scopes.tcp&&!scopes.udp)throw new Error('Selecciona web, tcp o udp');
  let payload={suspended:op==='suspend',web:scopes.web,tcp:scopes.tcp,udp:scopes.udp};
  if(op==='suspend'&&scopes.web)payload=Object.assign(payload,maintenanceWebPayload());
  if(op==='suspend'&&(scopes.tcp||scopes.udp)){const a=agents.find(x=>x.id===id);const risky=(resources||[]).filter(r=>r.agentId===id&&(r.mode==='tcp'||r.mode==='udp')&&[22,2222,3306,5432,6379].includes(Number(r.publicPort||r.backendPort||0)));if(risky.length){const ok=await confirmAction('TCP/UDP sensible','Este cliente tiene túneles que podrían ser SSH, bases de datos o servicios críticos. Si los suspendes podrías perder acceso remoto.','Entiendo, suspender',false);if(!ok)return}}
  showBusy(op==='suspend'?'Aplicando mantenimiento':'Reactivando servicios','Actualizando recursos del cliente y Traefik');
  try{await api('/api/agents/'+id+'/maintenance',{method:'POST',body:JSON.stringify(payload)});closeResourceActionModal();msg(op==='suspend'?'Mantenimiento aplicado al cliente':'Servicios reactivados');await reloadProjects();if(currentProject)await loadProjectData(currentProject.id)}finally{hideBusy()}
}
function openTemplateManager(){refreshTemplateSelects();$('templateManagerModal').classList.add('open');loadTemplateIntoEditor().catch(()=>{})}
function closeTemplateManager(){$('templateManagerModal').classList.remove('open')}
async function loadTemplateIntoEditor(){const id=fieldValue('templateManagerSelect')||(suspensionTemplates[0]&&suspensionTemplates[0].id)||'';if(!id){setIfExists('templateManagerHTML','');return}setIfExists('templateManagerSelect',id);setIfExists('templateManagerHTML',await templateHTML(id))}
function newTemplateDraft(){const id=fieldValue('templateNewId').toLowerCase().trim();if(!id){msg('Escribe un id de plantilla',true);return}const draft=maybeEl('tpl-suspension-draft');setIfExists('templateManagerSelect',id);setIfExists('templateManagerHTML',draft?draft.textContent.trim():'')}
async function saveTemplateFromEditor(button=null){await withActionLoading(button,'Guardando',async()=>{try{let id=fieldValue('templateManagerSelect');const newId=fieldValue('templateNewId').toLowerCase().trim();if(newId)id=newId;if(!id)throw new Error('Selecciona o escribe un id de plantilla');await api('/api/suspension-templates/'+encodeURIComponent(id),{method:'PUT',body:JSON.stringify({html:fieldValue('templateManagerHTML')})});setIfExists('templateNewId','');await reloadSuspensionTemplates();setIfExists('templateManagerSelect',id);msg('Plantilla guardada')}catch(err){msg(err.message,true)}})}
function syncProtectionFields(){const mode=fieldValue('protectionMode')||'none';document.querySelectorAll('.protection-password-only').forEach(el=>el.classList.toggle('d-none',mode!=='password'))}
function syncEditProtectionFields(){const mode=fieldValue('editProtectionMode')||'none';document.querySelectorAll('.edit-protection-password-only').forEach(el=>el.classList.toggle('d-none',mode!=='password'))}
function agentOSLabel(os){return os==='windows'?'Windows':'Linux'}
function selectedAgentOS(a,fallback='linux'){const os=String((a&&a.os)||'').toLowerCase();if(os.includes('windows'))return 'windows';if(os.includes('linux')||os.includes('ubuntu')||os.includes('debian')||os.includes('alpine'))return 'linux';return fallback||'linux'}
function agentOSNote(os){return os==='windows'?'Ejecuta estos comandos en PowerShell como administrador. Si rotaste el token, el anterior ya no conectara; reinstala/actualiza para sobrescribirlo o elimina la instalacion vieja.':'Ejecuta estos comandos en la shell del servidor Linux con permisos sudo/root. Si rotaste el token, el anterior ya no conectara; reinstala/actualiza para sobrescribirlo o elimina la instalacion vieja.'}
function syncAgentCommandOS(sel){const box=sel.closest('.agent-credentials');if(!box)return;const os=sel.value||'linux';box.querySelectorAll('[data-agent-os]').forEach(el=>el.classList.toggle('d-none',el.getAttribute('data-agent-os')!==os));const note=box.querySelector('[data-agent-os-note]');if(note)note.textContent=agentOSNote(os)}
function agentOSSelectNode(selected){selected=selected==='windows'?'windows':'linux';const node=tplNode('tpl-agent-os-select');const select=node.querySelector('select');select.value=selected;select.addEventListener('change',()=>syncAgentCommandOS(select));const note=node.querySelector('[data-agent-os-note]');if(note)note.textContent=agentOSNote(selected);return node}
function agentCommandGroupNode(os,selected,nodes=[]){const node=tplNode('tpl-agent-command-group');node.dataset.agentOs=os;node.classList.toggle('d-none',selected!==os);const content=slot(node,'content');nodes.forEach(child=>content.appendChild(child));return node}
function renderAgentCredentials(a,preferredOS){const selected=selectedAgentOS(a,preferredOS||'linux');const remove=a.removeCommand||'sudo /opt/pangolite-client/pangolite-client --remove';const wremove=a.windowsRemoveCommand||"Start-Process -Verb RunAs 'C:\\ProgramData\\Pangolite Client\\pangolite-client.exe' -ArgumentList '--remove'";const root=tplNode('tpl-agent-credentials');const secrets=slot(root,'secrets');secrets.appendChild(secretBlockNode('ID del cliente de sistema',a.id||''));secrets.appendChild(secretBlockNode('Token nuevo',a.token||''));secrets.appendChild(secretBlockNode('URL del panel',a.serverUrl||''));if(a.fallbackUrl)secrets.appendChild(secretBlockNode('URL fallback por IP',a.fallbackUrl));appendSlot(root,'osSelect',agentOSSelectNode(selected));appendSlot(root,'linux',agentCommandGroupNode('linux',selected,[commandBlockNode('Instalar/actualizar Linux systemd/OpenRC',a.installCommand||''),commandBlockNode('Eliminar token anterior o instalacion Linux',remove)]));appendSlot(root,'windows',agentCommandGroupNode('windows',selected,[commandBlockNode('Instalar/actualizar Windows como servicio',a.windowsInstallCommand||''),commandBlockNode('Eliminar token anterior o instalacion Windows',wremove)]));return root}
function closeAgentCredentials(){$('agentCredentialsModal').classList.remove('open')}
function showAgentCredentialsDialog(a,opts={}){const modal=$('agentCredentialsModal');$('agentCredentialsTitle').textContent=opts.title||'Credenciales del cliente';$('agentCredentialsMeta').textContent=opts.meta||'Copia el token y los comandos ahora. El token solo se muestra una vez.';const body=$('agentCredentialsBody');clearNode(body);body.appendChild(renderAgentCredentials(a,opts.os||selectedAgentOS(a,'linux')));modal.classList.add('open')}
