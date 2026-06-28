function setTop(crumb,title){setTextIfExists('crumb',crumb);setTextIfExists('topTitle',title)}

function maybeEl(id){return document.getElementById(id)}
function fieldValue(id){const el=maybeEl(id);return el?String(el.value||'').trim():''}
function fieldChecked(id){const el=maybeEl(id);return !!(el&&el.checked)}
function fieldNumber(id){const raw=fieldValue(id);return raw?Number(raw):0}
function setIfExists(id,value){const el=maybeEl(id);if(!el)return;if(el.type==='checkbox')el.checked=(value===true||value==='true'||value==='1');else el.value=value}
function setTextIfExists(id,value){const el=maybeEl(id);if(el)el.textContent=String(value??'')}
function setHrefIfExists(id,value){const el=maybeEl(id);if(el)el.href=value}
function activateView(id){const el=maybeEl(id);if(el)el.classList.add('active')}
function activateNav(name){const el=document.querySelector('[data-nav="'+name+'"]');if(el)el.classList.add('active')}
function classToggleAll(selector,hide){document.querySelectorAll(selector).forEach(el=>el.classList.toggle('d-none',!!hide))}
function buildDomainFromCreateForm(){
  if(fieldValue('domainSelect')==='custom')return fieldValue('customDomain').toLowerCase();
  const base=fieldValue('domainSelect').toLowerCase();
  const sub=fieldValue('subdomain').toLowerCase().replace(/^\.+|\.+$/g,'');
  if(!base)return '';
  return sub?sub+'.'+base:base;
}
function syncDomainMode(){
  const custom=fieldValue('domainSelect')==='custom';
  const customGroup=maybeEl('customDomainGroup');
  const managedGroup=maybeEl('managedDomainGroup');
  if(customGroup)customGroup.classList.toggle('d-none',!custom);
  if(managedGroup)managedGroup.classList.toggle('d-none',custom);
  const preview=maybeEl('domainPreview');
  const domain=buildDomainFromCreateForm();
  if(preview)preview.textContent=domain?((fieldChecked('tls')?'https://':'http://')+domain+(fieldValue('pathPrefix')||'/')):'-';
  paintLocalCertificateHint('certStatusCreate',domain,fieldChecked('tls'));
}
function syncMode(){
  const mode=fieldValue('mode')||'http';
  classToggleAll('.http-only',mode!=='http');
  classToggleAll('.tcpudp-only',!(mode==='tcp'||mode==='udp'));
  syncDomainMode();
  syncOrigin();
}
function syncOrigin(){
  const origin=fieldValue('originType')||'local';
  const group=maybeEl('agentOriginGroup');
  if(group)group.classList.toggle('d-none',origin!=='agent');
  const notice=maybeEl('agentTcpUdpNotice');
  const mode=fieldValue('mode')||'http';
  if(notice)notice.classList.toggle('d-none',!(origin==='agent'&&(mode==='tcp'||mode==='udp')));
}
function syncDisabledMode(){
  const mode=fieldValue('disabledResponseMode')||'403';
  document.querySelectorAll('.html-control').forEach(el=>el.classList.toggle('d-none',mode!=='html'));
}
function syncEditResourceMode(){
  const mode=fieldValue('editMode')||'http';
  document.querySelectorAll('.edit-http-only').forEach(el=>el.classList.toggle('d-none',mode!=='http'));
  document.querySelectorAll('.edit-tcpudp-only').forEach(el=>el.classList.toggle('d-none',!(mode==='tcp'||mode==='udp')));
  syncEditResourceOrigin();
}
function syncEditResourceOrigin(){
  const origin=fieldValue('editOriginType')||'local';
  const group=maybeEl('editAgentOriginGroup');
  if(group)group.classList.toggle('d-none',origin!=='agent');
  const notice=maybeEl('editAgentTcpUdpNotice');
  const mode=fieldValue('editMode')||'http';
  if(notice)notice.classList.toggle('d-none',!(origin==='agent'&&(mode==='tcp'||mode==='udp')));
}
function syncEditDisabledMode(){
  const mode=fieldValue('editDisabledResponseMode')||'403';
  document.querySelectorAll('.edit-html-control').forEach(el=>el.classList.toggle('d-none',mode!=='html'));
}
function fillAgentSelect(){
  ['agentId','editAgentId'].forEach(id=>{const sel=maybeEl(id);if(!sel)return;const current=sel.value;clearNode(sel);const empty=document.createElement('option');empty.value='';empty.textContent='Selecciona un cliente de sistema';sel.appendChild(empty);agents.filter(a=>a.enabled!==false).forEach(a=>{const opt=document.createElement('option');opt.value=a.id;opt.textContent=a.name+' · '+shortID(a.id)+(a.online?' · online':' · offline');sel.appendChild(opt)});if(current)sel.value=current;});
}
async function createProjectFromForm(e){
  e.preventDefault();
  const name=fieldValue('projectName');
  const notes=fieldValue('projectNotes');
  if(!name){msg('Nombre de proyecto requerido',true);return false}
  try{
    showBusy('Creando proyecto','Guardando el proyecto y actualizando el dashboard');
    const project=await api('/api/projects',{method:'POST',body:JSON.stringify({name,notes})});
    closeProjectModal();
    setIfExists('projectName','');setIfExists('projectNotes','');
    await reloadProjects();
    msg('Proyecto creado');
    go('/projects/'+project.id);
  }catch(err){msg(err.message,true)}finally{hideBusy()}
  return false;
}
async function createDomainFromForm(e){
  e.preventDefault();
  try{
    const domain=fieldValue('managedDomainInput').toLowerCase();
    if(!domain)throw new Error('Dominio requerido');
    await api('/api/domains',{method:'POST',body:JSON.stringify({domain})});
    closeDomainModal();setIfExists('managedDomainInput','');
    await reloadDomains();
    msg('Dominio agregado');
  }catch(err){msg(err.message,true)}
  return false;
}
async function createAgent(){
  if(!currentProject){msg('Selecciona un proyecto primero',true);return}
  try{
    const name=fieldValue('agentName');
    if(!name)throw new Error('Nombre del cliente de sistema requerido');
    showBusy('Creando cliente de sistema','Generando ID, token y comandos de instalación');
    const os=fieldValue('agentInstallOS')||'linux';
    const a=await api('/api/agents',{method:'POST',body:JSON.stringify({projectId:currentProject.id,name})});
    $('agentTokenCreate').replaceChildren(renderAgentCredentials(a,os));
    $('agentTokenCreate').classList.remove('d-none');
    showAgentCredentialsDialog(a,{title:'Cliente de sistema creado',meta:'Copia el token y el comando de instalacion ahora. El fallback por IP queda incluido para rescate si cambia el dominio.',os});
    setIfExists('agentName','');
    await reloadProjects();
    await loadProjectData(currentProject.id);
    msg('Cliente de sistema creado. Copia el token ahora.');
  }catch(err){msg(err.message,true)}finally{hideBusy()}
}
function createResourcePayload(prefix=''){
  const isEdit=prefix==='edit';
  const mode=fieldValue(isEdit?'editMode':'mode')||'http';
  const originType=fieldValue(isEdit?'editOriginType':'originType')||'local';
  const payload={
    projectId: currentProject?currentProject.id:fieldValue('editProjectId'),
    name: fieldValue(isEdit?'editResourceName':'resourceName'),
    mode,
    originType,
    agentId: originType==='agent'?fieldValue(isEdit?'editAgentId':'agentId'):'',
    backendHost: fieldValue(isEdit?'editBackendHost':'backendHost'),
    backendPort: fieldNumber(isEdit?'editBackendPort':'backendPort'),
    enabled: isEdit ? fieldValue('editResourceEnabled')!=='false' : true,
    disabledResponseMode: isEdit ? (fieldValue('editDisabledResponseMode')||'403') : '403',
    disabledStatusCode: isEdit ? fieldNumber('editDisabledStatusCode')||403 : 403,
    disabledHtml: isEdit ? fieldValue('editDisabledHtml') : '',
    disabledTemplateId: isEdit ? fieldValue('editDisabledPreset') : '',
    protectionMode: isEdit ? (fieldValue('editProtectionMode')||'none') : (fieldValue('protectionMode')||'none'),
    protectionLoginMode: isEdit ? (fieldValue('editProtectionLoginMode')||'html') : (fieldValue('protectionLoginMode')||'html'),
    protectionPassword: isEdit ? fieldValue('editProtectionPassword') : fieldValue('protectionPassword')
  };
  if(mode==='http'){
    payload.domain=isEdit?fieldValue('editDomain').toLowerCase():buildDomainFromCreateForm();
    payload.pathPrefix=isEdit?fieldValue('editPathPrefix'):(fieldValue('pathPrefix')||'/');
    payload.backendScheme=isEdit?fieldValue('editBackendScheme'):(fieldValue('backendScheme')||'http');
    payload.tls=isEdit?fieldChecked('editTLS'):fieldChecked('tls');
    payload.publicPort=0;
  }else{
    payload.domain='';payload.pathPrefix='';payload.backendScheme='';payload.tls=false;payload.protectionMode='none';payload.protectionLoginMode='html';payload.protectionPassword='';
    payload.publicPort=fieldNumber(isEdit?'editPublicPort':'publicPort');
  }
  return payload;
}
async function createResourceFromForm(e){
  e.preventDefault();
  if(!currentProject){msg('Selecciona un proyecto primero',true);return false}
  let busyClosed=false;
  try{
    const payload=createResourcePayload();
    if(!payload.name)throw new Error('Nombre del recurso requerido');
    if(!await confirmTraefikRestartIfNeeded(payload))return false;
    showBusy('Creando recurso','Validando puerto, cliente de sistema, backend y aplicando Traefik');
    const createResp=await api('/api/resources',{method:'POST',body:JSON.stringify(payload)});
    let cert=null;
    if(payload.mode==='http')cert=await fetchCertificateStatus(payload.domain,payload.tls,'certStatusCreate');
    await reloadProjects();
    await loadProjectData(currentProject.id);
    const tmsg=traefikNotice(createResp);
    const notice='El recurso '+payload.name+' se creó correctamente.'+(cert?' SSL: '+certText(cert)+'.':'')+(tmsg?'\n\n'+tmsg:'');
    hideBusy();busyClosed=true;
    goNotice('/projects/'+currentProject.id+'/resources','Recurso creado',notice);
  }catch(err){msg(err.message,true)}finally{if(!busyClosed)hideBusy()}
  return false;
}
function openEditResource(id){
  const r=resources.find(x=>x.id===id);if(!r){msg('Recurso no encontrado',true);return}
  setIfExists('editResourceId',r.id);setIfExists('editResourceName',r.name);setIfExists('editMode',r.mode||'http');setIfExists('editOriginType',r.originType||'local');
  fillAgentSelect();setIfExists('editAgentId',r.agentId||'');setIfExists('editDomain',r.domain||'');setIfExists('editPathPrefix',r.pathPrefix||'/');setIfExists('editTLS',!!r.tls);setIfExists('editBackendScheme',r.backendScheme||'http');setIfExists('editPublicPort',r.publicPort||'');setIfExists('editBackendHost',r.backendHost||'127.0.0.1');setIfExists('editBackendPort',r.backendPort||'');setIfExists('editResourceEnabled',String(!!r.enabled));setIfExists('editDisabledResponseMode',r.disabledResponseMode||'403');setIfExists('editDisabledStatusCode',r.disabledStatusCode||403);setIfExists('editDisabledHtml',r.disabledHtml||'');refreshTemplateSelects();setIfExists('editDisabledPreset',r.disabledTemplateId||'');setIfExists('editProtectionMode',r.protectionMode||'none');setIfExists('editProtectionLoginMode',r.protectionLoginMode||'html');setIfExists('editProtectionPassword','');
  syncEditResourceMode();syncEditDisabledMode();syncEditProtectionFields();
  if((r.mode||'http')==='http')fetchCertificateStatus(r.domain,!!r.tls,'certStatusEdit').catch(()=>paintLocalCertificateHint('certStatusEdit',r.domain,!!r.tls));
  $('resourceEditModal').classList.add('open');
}
async function saveResourceEdit(e){
  e.preventDefault();
  try{
    const id=fieldValue('editResourceId');if(!id)throw new Error('Recurso no seleccionado');
    const payload=createResourcePayload('edit');
    const current=resources.find(x=>x.id===id)||null;
    if(!await confirmTraefikRestartIfNeeded(payload,current))return false;
    showBusy('Guardando recurso','Validando cambios y aplicando Traefik');
    const editResp=await api('/api/resources/'+id,{method:'PATCH',body:JSON.stringify(payload)});
    let cert=null;
    if(payload.mode==='http')cert=await fetchCertificateStatus(payload.domain,payload.tls,'certStatusEdit');
    closeResourceEditModal();
    await reloadProjects();
    if(currentProject)await loadProjectData(currentProject.id);
    const tmsg=traefikNotice(editResp);
    msg('Recurso actualizado'+(cert?'. SSL: '+certText(cert)+'.':'')+(tmsg?' '+tmsg:''));
  }catch(err){msg(err.message,true)}finally{hideBusy()}
  return false;
}
async function deleteResource(id){
  const r=resources.find(x=>x.id===id);
  const deleteBody='Se eliminara '+(r?r.name:shortID(id))+' y Pangolite aplicara Traefik automaticamente.'+((r&&(r.mode==='tcp'||r.mode==='udp'))?' Este recurso usa puerto TCP/UDP, por lo que Traefik podria reiniciarse para retirar el entryPoint.':'');
  if(!await confirmAction('Eliminar recurso',deleteBody,'Eliminar recurso'))return;
  try{
    const delResp=await api('/api/resources/'+id,{method:'DELETE'});
    removeResourceLocal(id);
    msg('Recurso eliminado'+(traefikNotice(delResp)?'. '+traefikNotice(delResp):''));
    refreshCurrentProjectSoft();
  }catch(err){msg(err.message,true)}
}
function setupForms(){
  const projectForm=maybeEl('projectForm');if(projectForm){projectForm.setAttribute('action','javascript:void(0)');projectForm.addEventListener('submit',createProjectFromForm)}
  const projectSettingsForm=maybeEl('projectSettingsForm');if(projectSettingsForm){projectSettingsForm.setAttribute('action','javascript:void(0)');projectSettingsForm.addEventListener('submit',saveProjectSettings)}
  const domainForm=maybeEl('domainForm');if(domainForm){domainForm.setAttribute('action','javascript:void(0)');domainForm.addEventListener('submit',createDomainFromForm)}
  const resourceForm=maybeEl('resourceForm');if(resourceForm){resourceForm.setAttribute('action','javascript:void(0)');resourceForm.addEventListener('submit',createResourceFromForm)}
  const editForm=maybeEl('resourceEditForm');if(editForm){editForm.setAttribute('action','javascript:void(0)');editForm.addEventListener('submit',saveResourceEdit)}
  const settingsForm=maybeEl('dashboardSettingsForm');if(settingsForm){settingsForm.setAttribute('action','javascript:void(0)');settingsForm.addEventListener('submit',saveSettings)}
  [['mode',syncMode],['originType',syncOrigin],['domainSelect',syncDomainMode],['subdomain',syncDomainMode],['customDomain',syncDomainMode],['tls',syncDomainMode],['disabledResponseMode',syncDisabledMode],['editMode',syncEditResourceMode],['editOriginType',syncEditResourceOrigin],['editTLS',()=>paintLocalCertificateHint('certStatusEdit',fieldValue('editDomain').toLowerCase(),fieldChecked('editTLS'))],['editDomain',()=>paintLocalCertificateHint('certStatusEdit',fieldValue('editDomain').toLowerCase(),fieldChecked('editTLS'))],['editDisabledResponseMode',syncEditDisabledMode],['protectionMode',syncProtectionFields],['editProtectionMode',syncEditProtectionFields],['resourceActionMode',syncResourceActionMode],['templateManagerSelect',()=>loadTemplateIntoEditor().catch(()=>{})]].forEach(([id,fn])=>{const el=maybeEl(id);if(el)el.addEventListener('input',fn);if(el)el.addEventListener('change',fn)});
  const preset=maybeEl('disabledPreset');if(preset)preset.addEventListener('change',()=>{if(preset.value){setIfExists('disabledResponseMode','html');setIfExists('disabledStatusCode','403');setIfExists('disabledHtml','');syncDisabledMode()}});
  const epreset=maybeEl('editDisabledPreset');if(epreset)epreset.addEventListener('change',()=>{if(epreset.value){setIfExists('editDisabledResponseMode','html');setIfExists('editDisabledStatusCode','403');setIfExists('editDisabledHtml','');syncEditDisabledMode()}});
}

