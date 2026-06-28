function resourceLabel(r){return r.mode==='http'?((r.tls?'https://':'http://')+r.domain+(r.pathPrefix||'/')):(r.mode.toUpperCase()+' :'+r.publicPort)}
function label(r){return resourceLabel(r)}
function stateNode(enabled,activeText='Activo',inactiveText='Suspendido'){return enabled?makeStatePill(activeText,true,false):makeStatePill(inactiveText,false,true)}
function state(r){return stateNode(!!r.enabled).outerHTML}
function paintProjectNav(filter=''){const box=maybeEl('projectNav');if(!box)return;clearNode(box);const q=String(filter||'').trim().toLowerCase();const list=projects.filter(p=>!q||p.name.toLowerCase().includes(q)||p.slug.toLowerCase().includes(q)||p.id.toLowerCase().includes(q));if(!list.length){box.appendChild(makeEmpty('Sin proyectos todavía'));return}list.forEach(p=>{const st=stats[p.id]||{};const btn=tplNode('tpl-project-nav-item');btn.classList.toggle('active',!!(currentProject&&currentProject.id===p.id));btn.dataset.projectNav=p.id;setSlot(btn,'name',p.name);setSlot(btn,'meta',p.slug+' · '+(st.resources||0)+' recursos · '+(st.agents||0)+' clientes');btn.addEventListener('click',()=>{closeProjectSwitcher();go('/projects/'+p.id)});box.appendChild(btn)});updateProjectSwitcherLabel()}
function updateProjectSwitcherLabel(){const label=$('currentProjectLabel');if(label)label.textContent=currentProject?currentProject.name:'Selecciona proyecto';updateSidebarProjectContext()}
function updateSidebarProjectContext(){
  if(!currentProject)return;
  const st=stats[currentProject.id]||{};
  setTextIfExists('sidebarProjectName',currentProject.name||'Proyecto');
  setTextIfExists('sidebarProjectMeta','/'+(currentProject.slug||currentProject.id||''));
  setTextIfExists('sidebarProjectResources',st.resources||resources.length||0);
  setTextIfExists('sidebarProjectAgents',st.agents||agents.length||0);
  setTextIfExists('sidebarProjectActive',st.activeResources||resources.filter(r=>r.enabled).length||0);
  setTextIfExists('sidebarResourceCount',st.resources||resources.length||0);
  setTextIfExists('sidebarAgentCount',st.agents||agents.length||0);
  setHrefIfExists('sidebarNewResource','/projects/'+currentProject.id+'/resources/create');
  setHrefIfExists('sidebarNewAgent','/projects/'+currentProject.id+'/agents/create');
  setHrefIfExists('terminalNavLink','/terminal?projectId='+encodeURIComponent(currentProject.id));
  document.querySelectorAll('[data-project-link]').forEach(a=>{
    const key=a.getAttribute('data-project-link');
    if(key==='overview')a.href='/projects/'+currentProject.id;
    if(key==='resources')a.href='/projects/'+currentProject.id+'/resources';
    if(key==='agents')a.href='/projects/'+currentProject.id+'/agents';
  });
}
function activateProjectNav(name){document.querySelectorAll('.project-nav-link').forEach(a=>a.classList.toggle('active',a.getAttribute('data-project-link')===name))}
function projectStateBadge(p){const wrap=document.createElement('span');wrap.className='project-card-eyebrow';wrap.appendChild(stateNode(!!p.enabled,'Activo','Inactivo'));return wrap}
function paintProjectTable(){const rows=maybeEl('projectRows');if(!rows)return;clearNode(rows);if(!projects.length){rows.appendChild(makeEmpty('No hay proyectos. Usa + Crear para iniciar el onboarding y agregar el primer proyecto.'));return}projects.forEach(p=>{const st=stats[p.id]||{};const card=tplNode('tpl-project-card');card.classList.toggle('project-card-disabled',!p.enabled);appendSlot(card,'state',stateNode(!!p.enabled,'Activo','Inactivo'));setSlot(card,'name',p.name);setSlot(card,'slug','/'+p.slug);setSlot(card,'resources',st.resources||0);setSlot(card,'agents',st.agents||0);setSlot(card,'active',st.activeResources||0);const open=slot(card,'open');if(open){open.href='/projects/'+p.id;open.setAttribute('aria-label','Abrir '+p.name)}const actions=slot(card,'actions');actions.appendChild(makeLink('Resumen','/projects/'+p.id,'bi-command','btn btn-sm btn-primary'));actions.appendChild(makeLink('Recursos','/projects/'+p.id+'/resources','bi-diagram-3','btn btn-sm btn-outline-secondary'));actions.appendChild(makeLink('Clientes','/projects/'+p.id+'/agents','bi-hdd-network','btn btn-sm btn-outline-secondary'));rows.appendChild(card)})}
function domainStatusNode(d){
  const label=d.primary?'Principal':(d.status==='legacy'?'Heredado':(d.enabled?'Activo':'Inactivo'));
  const ok=d.primary||d.enabled;
  const warn=d.status==='legacy';
  const pill=makeStatePill(label,ok,false);
  if(warn) pill.classList.add('warn');
  return pill;
}
function domainUsageText(d){const a=d.agentCount||0;const r=d.resourceCount||0;const u=d.unsafeAgentCount||0;let text=a+' cliente(s) · '+r+' recurso(s)';if(u>0)text+=' · '+u+' sin fallback confirmado';return text}
function domainActionButton(label,icon,classes,onClick,title=''){const btn=makeButton(label,icon,classes,onClick);if(title)btn.title=title;return btn}
function paintDomains(){
  const rows=$('domainRows');if(!rows)return;clearNode(rows);
  if(!domains.length){rows.appendChild(makeEmpty('Sin dominios administrados. Agrega domain.tld antes de crear proxys HTTP.','tr',5));return}
  domains.forEach(d=>{
    const tr=tplNode('tpl-domain-row');
    setSlot(tr,'domain',d.domain);
    appendSlot(tr,'state',domainStatusNode(d));
    setSlot(tr,'usage',domainUsageText(d));
    setSlot(tr,'created',fmt(d.createdAt));
    const actions=slot(tr,'actions');
    if(!actions)return rows.appendChild(tr);
    if(!d.primary&&d.status!=='legacy')actions.appendChild(domainActionButton('Heredar','bi-archive','btn btn-sm btn-outline-secondary',event=>markDomainLegacy(d.id,event.currentTarget),'Oculta el dominio en nuevas instalaciones, pero conserva compatibilidad.'));
    if(!d.primary)actions.appendChild(domainActionButton('Hacer principal','bi-star','btn btn-sm btn-outline-secondary',event=>makeDomainPrimary(d.id,event.currentTarget),'Usar como dominio del panel y nuevos clientes.'));
    if(d.status==='legacy')actions.appendChild(domainActionButton('Activar','bi-toggle-on','btn btn-sm btn-outline-secondary',event=>activateDomain(d.id,event.currentTarget),'Volver a mostrar como dominio disponible.'));
    const locked=!!d.deleteLocked||d.primary||(d.resourceCount||0)>0;
    const deleteHint=locked?(d.deleteReason||'No se puede eliminar mientras sea principal, tenga recursos o clientes sin fallback confirmado.'):((d.agentCount||0)>0?'Eliminar: clientes con fallback por IP confirmado.':'Eliminar definitivamente el dominio.');
    const del=domainActionButton('Eliminar','bi-trash','btn btn-sm btn-outline-danger',event=>deleteDomain(d.id,event.currentTarget),deleteHint);
    del.disabled=locked;
    actions.appendChild(del);
    rows.appendChild(tr)
  })
}
function fillDomainSelect(){const sel=$('domainSelect');if(!sel)return;clearNode(sel);domains.filter(d=>d.enabled&&d.status!=='legacy').forEach(d=>{const opt=document.createElement('option');opt.value=d.domain;opt.textContent=d.primary?d.domain+' (principal)':d.domain;sel.appendChild(opt)});const custom=document.createElement('option');custom.value='custom';custom.textContent='Custom';sel.appendChild(custom);if(!domains.filter(d=>d.enabled&&d.status!=='legacy').length)sel.value='custom';syncDomainMode()}
function paintProjectOverview(){if(!maybeEl('projectHeroTitle'))return;$('projectHeroTitle').textContent=currentProject.name;$('projectHeroText').textContent=currentProject.notes||'Recursos y agentes separados por proyecto.';$('projectResourceCount').textContent=resources.length;$('projectAgentCount').textContent=agents.length;$('projectActiveCount').textContent=resources.filter(r=>r.enabled).length;setIfExists('projectEditName',currentProject.name||'');setIfExists('projectEditNotes',currentProject.notes||'');const delBtn=maybeEl('deleteProjectBtn');const hint=maybeEl('projectDangerHint');const blocked=resources.length>0||agents.length>0;if(delBtn){delBtn.disabled=blocked;delBtn.title=blocked?'Elimina primero recursos y clientes de sistema vinculados.':''}if(hint){hint.textContent=blocked?'Para eliminar este proyecto primero debe quedar sin recursos ni clientes de sistema. Actual: '+resources.length+' recurso(s), '+agents.length+' cliente(s) de sistema.':'El proyecto no tiene recursos ni clientes de sistema. Puedes eliminarlo si ya no se usa.'}const rr=$('projectResourceRows');clearNode(rr);resources.slice(0,6).forEach(r=>{const row=tplNode('tpl-project-resource-row');appendSlot(row,'state',stateNode(!!r.enabled));setSlot(row,'type',(r.mode||'').toUpperCase());setSlot(row,'name',r.name);setSlot(row,'entry',label(r));rr.appendChild(row)});if(!resources.length)rr.appendChild(makeEmpty('Sin recursos.','tr',4));const ar=$('projectAgentRows');clearNode(ar);agents.slice(0,6).forEach(a=>{const row=tplNode('tpl-project-agent-row');appendSlot(row,'state',agentStateNode(a));setSlot(row,'name',a.name);setSlot(row,'lastSeen',fmt(a.lastSeen));ar.appendChild(row)});if(!agents.length)ar.appendChild(makeEmpty('Sin agentes.','tr',3))}
async function loadProjectData(id){resources=(await api('/api/resources?projectId='+encodeURIComponent(id))).resources||[];agents=(await api('/api/agents?projectId='+encodeURIComponent(id))).agents||[];paintResources();paintAgents();fillAgentSelect();updateSidebarProjectContext();}
function removeResourceLocal(id){resources=resources.filter(x=>x.id!==id);paintResources();const ctl=maybeEl('resourceControlSelect');if(ctl&&ctl.value===id)ctl.value='';}
async function refreshCurrentProjectSoft(){try{await reloadProjects();if(currentProject)await loadProjectData(currentProject.id);return true}catch(err){msg('Cambios guardados. Reintentando actualizar la tabla en unos segundos...');setTimeout(()=>{reloadProjects().then(()=>currentProject&&loadProjectData(currentProject.id)).catch(()=>{})},3500);return false}}
