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
function createTextNode(tag,className,text){const el=document.createElement(tag);if(className)el.className=className;el.textContent=text||'';return el}
function widgetSelectText(opt){return opt?String(opt.textContent||'').replace(/\s+/g,' ').trim():''}
function closeWidgetSelects(except=null){document.querySelectorAll('.pl-widget-select-wrap.open').forEach(w=>{if(w!==except)w.classList.remove('open')})}
function optionAgentMeta(opt){return {name:opt.dataset.agentName||widgetSelectText(opt).split(' · ')[0]||'Cliente',id:opt.dataset.agentId||opt.value||'',online:opt.dataset.agentOnline==='true'}}
function appendWidgetOptionContent(target,select,opt,selected=false){
  const kind=select.dataset.widgetKind||'';
  const icon=select.dataset.widgetIcon||'bi-chevron-down';
  if(kind==='agent'&&opt&&opt.value){
    const meta=optionAgentMeta(opt);
    const dot=document.createElement('span');dot.className='pl-widget-dot '+(meta.online?'on':'off');target.appendChild(dot);
    const body=document.createElement('span');body.className='pl-widget-body';
    body.appendChild(createTextNode('span','pl-widget-title',meta.name));
    body.appendChild(createTextNode('span','pl-widget-meta',meta.id+(meta.online?' · online':' · offline')));
    target.appendChild(body);
  }else{
    const ico=document.createElement('i');ico.className='bi '+icon;target.appendChild(ico);
    const body=document.createElement('span');body.className='pl-widget-body';
    body.appendChild(createTextNode('span','pl-widget-title',widgetSelectText(opt)||select.dataset.placeholder||'Selecciona una opción'));
    if(opt&&opt.dataset&&opt.dataset.meta)body.appendChild(createTextNode('span','pl-widget-meta',opt.dataset.meta));
    target.appendChild(body);
  }
  if(selected){const check=document.createElement('i');check.className='bi bi-check2 pl-widget-check';target.appendChild(check)}
}
function enhanceWidgetSelect(select){
  if(!select||select.dataset.widgetEnhanced==='1')return refreshWidgetSelect(select&&select.id);
  if(!select.id)select.id='widgetSelect'+Math.random().toString(36).slice(2);
  select.dataset.widgetEnhanced='1';
  select.classList.add('pl-widget-native');
  const wrap=document.createElement('div');wrap.className='pl-widget-select-wrap';wrap.dataset.selectId=select.id;
  const btn=document.createElement('button');btn.type='button';btn.className='pl-widget-select-button';btn.setAttribute('aria-haspopup','listbox');btn.setAttribute('aria-expanded','false');
  const menu=document.createElement('div');menu.className='pl-widget-select-menu';menu.setAttribute('role','listbox');
  wrap.appendChild(btn);wrap.appendChild(menu);select.insertAdjacentElement('afterend',wrap);
  btn.addEventListener('click',e=>{e.preventDefault();e.stopPropagation();const open=!wrap.classList.contains('open');closeWidgetSelects(wrap);wrap.classList.toggle('open',open);btn.setAttribute('aria-expanded',open?'true':'false')});
  select.addEventListener('change',()=>refreshWidgetSelect(select.id));
  refreshWidgetSelect(select.id);
}
function refreshWidgetSelect(id){
  const select=maybeEl(id);if(!select)return;
  if(select.dataset.widgetEnhanced!=='1')return enhanceWidgetSelect(select);
  const wrap=Array.from(document.querySelectorAll('.pl-widget-select-wrap')).find(w=>w.dataset.selectId===select.id);if(!wrap)return;
  const btn=wrap.querySelector('.pl-widget-select-button');const menu=wrap.querySelector('.pl-widget-select-menu');if(!btn||!menu)return;
  clearNode(btn);clearNode(menu);
  const selected=select.selectedOptions&&select.selectedOptions[0]?select.selectedOptions[0]:Array.from(select.options).find(o=>o.value===select.value)||select.options[0];
  appendWidgetOptionContent(btn,select,selected,false);
  const chevron=document.createElement('i');chevron.className='bi bi-chevron-down pl-widget-chevron';btn.appendChild(chevron);
  Array.from(select.options).forEach(opt=>{
    const item=document.createElement('button');item.type='button';item.className='pl-widget-option';item.setAttribute('role','option');item.disabled=opt.disabled;item.setAttribute('aria-selected',opt.value===select.value?'true':'false');
    appendWidgetOptionContent(item,select,opt,opt.value===select.value);
    item.addEventListener('click',e=>{e.preventDefault();e.stopPropagation();if(opt.disabled)return;select.value=opt.value;select.dispatchEvent(new Event('input',{bubbles:true}));select.dispatchEvent(new Event('change',{bubbles:true}));closeWidgetSelects()});
    menu.appendChild(item);
  });
}
function refreshAllWidgetSelects(root=document){root.querySelectorAll('select.pl-widget-select').forEach(sel=>refreshWidgetSelect(sel.id))}
function initWidgetSelects(root=document){root.querySelectorAll('select.pl-widget-select').forEach(enhanceWidgetSelect);if(!window.__plWidgetSelectDocBound){window.__plWidgetSelectDocBound=true;document.addEventListener('click',e=>{if(!e.target.closest('.pl-widget-select-wrap'))closeWidgetSelects()})}}
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
function syncCreateAgentSelects(sourceId=''){
  const a=maybeEl('agentId');const b=maybeEl('agentIdTcpUdp');
  if(!a||!b)return;
  if(sourceId==='agentId'&&a.value!==b.value){b.value=a.value;refreshWidgetSelect('agentIdTcpUdp')}
  if(sourceId==='agentIdTcpUdp'&&b.value!==a.value){a.value=b.value;refreshWidgetSelect('agentId')}
}
function syncEditAgentSelects(sourceId=''){
  const a=maybeEl('editAgentId');const b=maybeEl('editAgentIdTcpUdp');
  if(!a||!b)return;
  if(sourceId==='editAgentId'&&a.value!==b.value){b.value=a.value;refreshWidgetSelect('editAgentIdTcpUdp')}
  if(sourceId==='editAgentIdTcpUdp'&&b.value!==a.value){a.value=b.value;refreshWidgetSelect('editAgentId')}
}
function syncHTTPResourceKind(){
  const mode=fieldValue('mode')||'http';
  const kind=fieldValue('httpResourceKind')||'app-local';
  const isHTTP=mode==='http';
  const isRedirect=isHTTP&&kind==='redirect';
  const isAgent=isHTTP&&kind==='app-agent';
  setIfExists('redirectEnabled',isRedirect);
  if(isHTTP)setIfExists('originType',isAgent?'agent':'local');
  document.querySelectorAll('.resource-agent-only').forEach(el=>el.classList.toggle('d-none',!isAgent));
  document.querySelectorAll('.resource-app-only').forEach(el=>el.classList.toggle('d-none',!isHTTP||isRedirect));
  document.querySelectorAll('.resource-backend-only').forEach(el=>el.classList.toggle('d-none',isHTTP&&isRedirect));
  document.querySelectorAll('.redirect-only').forEach(el=>el.classList.toggle('d-none',!isRedirect));
  if(isRedirect){setIfExists('hideWhenUnavailable',false);setIfExists('protectionMode','none');setIfExists('backendScheme','http');syncProtectionFields();refreshWidgetSelect('protectionMode');refreshWidgetSelect('backendScheme')}
  syncDomainMode();
}
function syncMode(){
  const mode=fieldValue('mode')||'http';
  classToggleAll('.http-only',mode!=='http');
  classToggleAll('.tcpudp-only',!(mode==='tcp'||mode==='udp'));
  if(mode!=='http')setIfExists('redirectEnabled',false);
  syncHTTPResourceKind();
  syncOrigin();
}
function syncOrigin(){
  const origin=fieldValue('originType')||'local';
  const mode=fieldValue('mode')||'http';
  const tcpAgent=origin==='agent'&&(mode==='tcp'||mode==='udp');
  const group=maybeEl('agentOriginTcpUdpGroup');
  if(group)group.classList.toggle('d-none',!tcpAgent);
  document.querySelectorAll('.resource-agent-tcpudp-only').forEach(el=>el.classList.toggle('d-none',!tcpAgent));
  const notice=maybeEl('agentTcpUdpNotice');
  if(notice)notice.classList.toggle('d-none',!tcpAgent);
}
function syncDisabledMode(){
  const mode=fieldValue('disabledResponseMode')||'403';
  document.querySelectorAll('.html-control').forEach(el=>el.classList.toggle('d-none',mode!=='html'));
}
function syncRedirectFields(){syncHTTPResourceKind()}
function syncEditResourceKind(){
  const mode=fieldValue('editMode')||'http';
  const kind=fieldValue('editHttpResourceKind')||'app-local';
  const isHTTP=mode==='http';
  const isRedirect=isHTTP&&kind==='redirect';
  const isAgent=isHTTP&&kind==='app-agent';
  setIfExists('editRedirectEnabled',isRedirect);
  if(isHTTP)setIfExists('editOriginType',isAgent?'agent':'local');
  document.querySelectorAll('.edit-resource-agent-only').forEach(el=>el.classList.toggle('d-none',!isAgent));
  document.querySelectorAll('.edit-resource-app-only').forEach(el=>el.classList.toggle('d-none',!isHTTP||isRedirect));
  document.querySelectorAll('.edit-resource-backend-only').forEach(el=>el.classList.toggle('d-none',isHTTP&&isRedirect));
  document.querySelectorAll('.edit-redirect-only').forEach(el=>el.classList.toggle('d-none',!isRedirect));
  if(isRedirect){setIfExists('editHideWhenUnavailable',false);setIfExists('editProtectionMode','none');setIfExists('editBackendScheme','http');syncEditProtectionFields();refreshWidgetSelect('editProtectionMode');refreshWidgetSelect('editBackendScheme')}
}
function syncEditRedirectFields(){syncEditResourceKind()}
function syncEditResourceMode(){
  const mode=fieldValue('editMode')||'http';
  document.querySelectorAll('.edit-http-only').forEach(el=>el.classList.toggle('d-none',mode!=='http'));
  document.querySelectorAll('.edit-tcpudp-only').forEach(el=>el.classList.toggle('d-none',!(mode==='tcp'||mode==='udp')));
  if(mode!=='http')setIfExists('editRedirectEnabled',false);
  syncEditResourceKind();
  syncEditResourceOrigin();
}
function syncEditResourceOrigin(){
  const origin=fieldValue('editOriginType')||'local';
  const mode=fieldValue('editMode')||'http';
  const tcpAgent=origin==='agent'&&(mode==='tcp'||mode==='udp');
  const group=maybeEl('editAgentOriginTcpUdpGroup');
  if(group)group.classList.toggle('d-none',!tcpAgent);
  document.querySelectorAll('.edit-resource-agent-tcpudp-only').forEach(el=>el.classList.toggle('d-none',!tcpAgent));
  const notice=maybeEl('editAgentTcpUdpNotice');
  if(notice)notice.classList.toggle('d-none',!tcpAgent);
}
function syncEditDisabledMode(){
  const mode=fieldValue('editDisabledResponseMode')||'403';
  document.querySelectorAll('.edit-html-control').forEach(el=>el.classList.toggle('d-none',mode!=='html'));
}
function fillAgentSelect(){
  ['agentId','agentIdTcpUdp','editAgentId','editAgentIdTcpUdp'].forEach(id=>{const sel=maybeEl(id);if(!sel)return;const current=sel.value;clearNode(sel);const empty=document.createElement('option');empty.value='';empty.textContent='Selecciona un cliente de sistema';sel.appendChild(empty);agents.filter(a=>a.enabled!==false).forEach(a=>{const opt=document.createElement('option');opt.value=a.id;opt.textContent=a.name+' · '+a.id+(a.online?' · online':' · offline');opt.dataset.agentId=a.id;opt.dataset.agentName=a.name||shortID(a.id);opt.dataset.agentOnline=a.online?'true':'false';sel.appendChild(opt)});if(current)sel.value=current;refreshWidgetSelect(id);});
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
async function createAgent(button=null){
  if(!currentProject){msg('Selecciona un proyecto primero',true);return}
  await withActionLoading(button,'Creando',async()=>{
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
  })
}
function currentResourceKind(isEdit=false){return isEdit?(fieldValue('editHttpResourceKind')||'app-local'):(fieldValue('httpResourceKind')||'app-local')}
function validateResourcePayload(payload){
  if(!payload.name)throw new Error('Nombre del recurso requerido');
  if(payload.mode==='http'){
    if(!payload.domain)throw new Error('Dominio requerido');
    if(payload.redirectEnabled){if(!payload.redirectTarget)throw new Error('Destino de redirección requerido');return}
  }
  if(payload.originType==='agent'&&!payload.agentId)throw new Error('Selecciona un cliente de sistema');
  if(!payload.backendHost)throw new Error('Host interno requerido');
  if(!payload.backendPort)throw new Error('Puerto interno requerido');
}
function createResourcePayload(prefix=''){
  const isEdit=prefix==='edit';
  const mode=fieldValue(isEdit?'editMode':'mode')||'http';
  const kind=currentResourceKind(isEdit);
  const isHTTP=mode==='http';
  const isRedirect=isHTTP&&kind==='redirect';
  let originType=fieldValue(isEdit?'editOriginType':'originType')||'local';
  if(isHTTP)originType=kind==='app-agent'?'agent':'local';
  const agentField=isEdit?(isHTTP?'editAgentId':'editAgentIdTcpUdp'):(isHTTP?'agentId':'agentIdTcpUdp');
  const backendHostField=isEdit?'editBackendHost':'backendHost';
  const backendPortField=isEdit?'editBackendPort':'backendPort';
  const payload={
    projectId: currentProject?currentProject.id:fieldValue('editProjectId'),
    name: fieldValue(isEdit?'editResourceName':'resourceName'),
    mode,
    originType,
    agentId: originType==='agent'?fieldValue(agentField):'',
    backendHost: fieldValue(backendHostField),
    backendPort: fieldNumber(backendPortField),
    enabled: isEdit ? fieldValue('editResourceEnabled')!=='false' : true,
    disabledResponseMode: isEdit ? (fieldValue('editDisabledResponseMode')||'403') : '403',
    disabledStatusCode: isEdit ? fieldNumber('editDisabledStatusCode')||403 : 403,
    disabledHtml: isEdit ? fieldValue('editDisabledHtml') : '',
    disabledTemplateId: isEdit ? fieldValue('editDisabledPreset') : '',
    protectionMode: isEdit ? (fieldValue('editProtectionMode')||'none') : (fieldValue('protectionMode')||'none'),
    protectionLoginMode: isEdit ? (fieldValue('editProtectionLoginMode')||'html') : (fieldValue('protectionLoginMode')||'html'),
    protectionPassword: isEdit ? fieldValue('editProtectionPassword') : fieldValue('protectionPassword'),
    redirectEnabled: isRedirect,
    redirectTarget: isRedirect ? (isEdit ? fieldValue('editRedirectTarget') : fieldValue('redirectTarget')) : '',
    redirectStatusCode: isRedirect ? (isEdit ? fieldNumber('editRedirectStatusCode')||308 : fieldNumber('redirectStatusCode')||308) : 308,
    hideWhenUnavailable: isRedirect ? false : (isEdit ? fieldChecked('editHideWhenUnavailable') : fieldChecked('hideWhenUnavailable'))
  };
  if(mode==='http'){
    payload.domain=isEdit?fieldValue('editDomain').toLowerCase():buildDomainFromCreateForm();
    payload.pathPrefix=isEdit?fieldValue('editPathPrefix'):(fieldValue('pathPrefix')||'/');
    payload.backendScheme=isRedirect?'http':(isEdit?fieldValue('editBackendScheme'):(fieldValue('backendScheme')||'http'));
    payload.tls=isEdit?fieldChecked('editTLS'):fieldChecked('tls');
    payload.publicPort=0;
    if(isRedirect){
      payload.backendHost=payload.backendHost||'127.0.0.1';
      payload.backendPort=payload.backendPort||80;
      payload.agentId='';
      payload.protectionMode='none';
      payload.protectionLoginMode='html';
      payload.protectionPassword='';
    }
  }else{
    payload.domain='';payload.pathPrefix='';payload.backendScheme='';payload.tls=false;payload.protectionMode='none';payload.protectionLoginMode='html';payload.protectionPassword='';payload.redirectEnabled=false;payload.redirectTarget='';payload.redirectStatusCode=308;payload.hideWhenUnavailable=false;
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
    validateResourcePayload(payload);
    if(!await confirmTraefikRestartIfNeeded(payload))return false;
    showBusy('Creando recurso','Validando puerto, cliente de sistema, backend y aplicando Traefik');
    const createResp=await api('/api/resources',{method:'POST',body:JSON.stringify(payload)});
    let cert=null;
    if(payload.mode==='http')cert=await fetchCertificateStatus(payload.domain,!!((createResp.resource||{}).tls),'certStatusCreate');
    await reloadProjects();
    await loadProjectData(currentProject.id);
    const tmsg=traefikNotice(createResp);
    const warn=createResp.warning?('\n\n'+createResp.warning):'';
    const kind=payload.redirectEnabled?'Redirección permanente':(payload.originType==='agent'?'Aplicación por cliente':'Aplicación local');
    const notice='El recurso '+payload.name+' se creó correctamente. Tipo: '+kind+'.'+(cert?' SSL: '+certText(cert)+'.':'')+(tmsg?'\n\n'+tmsg:'')+warn;
    hideBusy();busyClosed=true;
    goNotice('/projects/'+currentProject.id+'/resources','Recurso creado',notice);
  }catch(err){msg(err.message,true)}finally{if(!busyClosed)hideBusy()}
  return false;
}
function openEditResource(id){
  const r=resources.find(x=>x.id===id);if(!r){msg('Recurso no encontrado',true);return}
  setIfExists('editResourceId',r.id);setIfExists('editResourceName',r.name);setIfExists('editMode',r.mode||'http');setIfExists('editOriginType',r.originType||'local');
  fillAgentSelect();setIfExists('editAgentId',r.agentId||'');setIfExists('editAgentIdTcpUdp',r.agentId||'');refreshWidgetSelect('editAgentId');refreshWidgetSelect('editAgentIdTcpUdp');
  const kind=(r.mode||'http')==='http'?(r.redirectEnabled?'redirect':((r.originType||'local')==='agent'?'app-agent':'app-local')):'app-local';
  setIfExists('editHttpResourceKind',kind);
  setIfExists('editDomain',r.domain||'');setIfExists('editPathPrefix',r.pathPrefix||'/');setIfExists('editTLS',!!r.tls);setIfExists('editBackendScheme',r.backendScheme||'http');setIfExists('editPublicPort',r.publicPort||'');setIfExists('editBackendHost',r.backendHost||'127.0.0.1');setIfExists('editBackendPort',r.backendPort||'');setIfExists('editResourceEnabled',String(!!r.enabled));setIfExists('editDisabledResponseMode',r.disabledResponseMode||'403');setIfExists('editDisabledStatusCode',r.disabledStatusCode||403);setIfExists('editDisabledHtml',r.disabledHtml||'');refreshTemplateSelects();setIfExists('editDisabledPreset',r.disabledTemplateId||'');setIfExists('editProtectionMode',r.protectionMode||'none');setIfExists('editProtectionLoginMode',r.protectionLoginMode||'html');setIfExists('editProtectionPassword','');setIfExists('editRedirectEnabled',!!r.redirectEnabled);setIfExists('editRedirectTarget',r.redirectTarget||'');setIfExists('editRedirectStatusCode',r.redirectStatusCode||308);setIfExists('editHideWhenUnavailable',!!r.hideWhenUnavailable);
  refreshAllWidgetSelects();syncEditResourceMode();syncEditDisabledMode();syncEditProtectionFields();syncEditRedirectFields();
  if((r.mode||'http')==='http')fetchCertificateStatus(r.domain,!!r.tls,'certStatusEdit').catch(()=>paintLocalCertificateHint('certStatusEdit',r.domain,!!r.tls));
  $('resourceEditModal').classList.add('open');
}
async function saveResourceEdit(e){
  e.preventDefault();
  try{
    const id=fieldValue('editResourceId');if(!id)throw new Error('Recurso no seleccionado');
    const payload=createResourcePayload('edit');
    validateResourcePayload(payload);
    const current=resources.find(x=>x.id===id)||null;
    if(!await confirmTraefikRestartIfNeeded(payload,current))return false;
    showBusy('Guardando recurso','Validando cambios y aplicando Traefik');
    const editResp=await api('/api/resources/'+id,{method:'PATCH',body:JSON.stringify(payload)});
    let cert=null;
    if(payload.mode==='http')cert=await fetchCertificateStatus(payload.domain,!!((editResp.resource||{}).tls),'certStatusEdit');
    closeResourceEditModal();
    await reloadProjects();
    if(currentProject)await loadProjectData(currentProject.id);
    const tmsg=traefikNotice(editResp);
    msg('Recurso actualizado'+(cert?'. SSL: '+certText(cert)+'.':'')+(tmsg?' '+tmsg:'')+(editResp.warning?' '+editResp.warning:''));
  }catch(err){msg(err.message,true)}finally{hideBusy()}
  return false;
}
async function deleteResource(id,button=null){
  const r=resources.find(x=>x.id===id);
  const deleteBody='Se eliminara '+(r?r.name:shortID(id))+' y Pangolite aplicara Traefik automaticamente.'+((r&&(r.mode==='tcp'||r.mode==='udp'))?' Este recurso usa puerto TCP/UDP, por lo que Traefik podria reiniciarse para retirar el entryPoint.':'');
  if(!await confirmAction('Eliminar recurso',deleteBody,'Eliminar recurso'))return;
  await withActionLoading(button,'Eliminando',async()=>{
    try{
      const delResp=await api('/api/resources/'+id,{method:'DELETE'});
      removeResourceLocal(id);
      msg('Recurso eliminado'+(traefikNotice(delResp)?'. '+traefikNotice(delResp):''));
      refreshCurrentProjectSoft();
    }catch(err){msg(err.message,true)}
  })
}
function setupForms(){
  bindAsyncSubmit(maybeEl('projectForm'),createProjectFromForm,'Creando');
  bindAsyncSubmit(maybeEl('projectSettingsForm'),saveProjectSettings,'Guardando');
  bindAsyncSubmit(maybeEl('domainForm'),createDomainFromForm,'Agregando');
  bindAsyncSubmit(maybeEl('resourceForm'),createResourceFromForm,'Creando');
  bindAsyncSubmit(maybeEl('resourceEditForm'),saveResourceEdit,'Guardando');
  bindAsyncSubmit(maybeEl('dashboardSettingsForm'),saveSettings,'Guardando');
  bindAsyncSubmit(maybeEl('smtpSettingsForm'),saveSettings,'Validando');
  const bindings=[
    ['mode',syncMode],['httpResourceKind',syncHTTPResourceKind],['originType',syncOrigin],['agentId',()=>syncCreateAgentSelects('agentId')],['agentIdTcpUdp',()=>syncCreateAgentSelects('agentIdTcpUdp')],
    ['domainSelect',syncDomainMode],['subdomain',syncDomainMode],['customDomain',syncDomainMode],['pathPrefix',syncDomainMode],['tls',syncDomainMode],['disabledResponseMode',syncDisabledMode],['redirectEnabled',syncRedirectFields],
    ['editMode',syncEditResourceMode],['editHttpResourceKind',syncEditResourceKind],['editOriginType',syncEditResourceOrigin],['editAgentId',()=>syncEditAgentSelects('editAgentId')],['editAgentIdTcpUdp',()=>syncEditAgentSelects('editAgentIdTcpUdp')],
    ['editTLS',()=>paintLocalCertificateHint('certStatusEdit',fieldValue('editDomain').toLowerCase(),fieldChecked('editTLS'))],['editDomain',()=>paintLocalCertificateHint('certStatusEdit',fieldValue('editDomain').toLowerCase(),fieldChecked('editTLS'))],
    ['editDisabledResponseMode',syncEditDisabledMode],['editRedirectEnabled',syncEditRedirectFields],['protectionMode',syncProtectionFields],['editProtectionMode',syncEditProtectionFields],
    ['resourceActionMode',syncResourceActionMode],['maintenanceOperation',syncResourceActionMode],['maintenanceScopeWeb',syncResourceActionMode],['maintenanceScopeTCP',syncResourceActionMode],['maintenanceScopeUDP',syncResourceActionMode],['templateManagerSelect',()=>loadTemplateIntoEditor().catch(()=>{})]
  ];
  bindings.forEach(([id,fn])=>{const el=maybeEl(id);if(el)el.addEventListener('input',fn);if(el)el.addEventListener('change',fn)});
  const preset=maybeEl('disabledPreset');if(preset)preset.addEventListener('change',()=>{if(preset.value){setIfExists('disabledResponseMode','html');setIfExists('disabledStatusCode','403');setIfExists('disabledHtml','');syncDisabledMode()}});
  const epreset=maybeEl('editDisabledPreset');if(epreset)epreset.addEventListener('change',()=>{if(epreset.value){setIfExists('editDisabledResponseMode','html');setIfExists('editDisabledStatusCode','403');setIfExists('editDisabledHtml','');syncEditDisabledMode();refreshWidgetSelect('editDisabledResponseMode');refreshWidgetSelect('editDisabledStatusCode')}});
  initWidgetSelects();syncMode();syncOrigin();syncHTTPResourceKind();syncEditResourceMode();syncEditResourceOrigin();syncEditResourceKind();syncRedirectFields();syncEditRedirectFields();refreshAllWidgetSelects();
}
