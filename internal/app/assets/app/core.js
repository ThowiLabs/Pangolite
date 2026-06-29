function loadBootstrap(){const el=document.getElementById('appBootstrap');if(!el||!el.textContent.trim())return null;try{const b=JSON.parse(el.textContent);appBoot=b;csrf=b.csrfToken||csrf;projects=b.projects||[];stats=b.stats||{};resources=b.resources||[];agents=b.agents||[];domains=b.domains||[];panelSettings=b.settings||{};networkInfo=b.network||{};currentProject=b.hasProject?b.currentProject:null;auditEvents=b.auditEvents||[];backups=b.backups||[];logsLines=b.logLines||[];suspensionTemplates=b.suspensionTemplates||[];return b}catch(err){console.warn('bootstrap invalido',err);return null}}
let appBoot=null;let csrf='';let projects=[];let stats={};let resources=[];let agents=[];let domains=[];let panelSettings={};let networkInfo={};let currentProject=null;let charts={};let logsLines=[];let auditEvents=[];let backups=[];let resourceHealth={};let suspensionTemplates=[];let deletingResources=new Set();
const $=id=>document.getElementById(id);
const domainRe=/^([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/i;
const labelIcon={projects:'bi-building',settings:'bi-sliders'};
function esc(s){return String(s??'').replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]))}
function tplNode(id){const tpl=$(id);if(!tpl||!tpl.content||!tpl.content.firstElementChild)return null;return tpl.content.firstElementChild.cloneNode(true)}
function cloneTemplateHTML(id,mutate){const node=tplNode(id);if(!node)return '';if(mutate)mutate(node);return node.outerHTML}
function slot(root,name){return root?root.querySelector('[data-slot="'+name+'"]'):null}
function setSlot(root,name,value){const el=slot(root,name);if(el)el.textContent=String(value??'');return el}
function appendSlot(root,name,child){const el=slot(root,name);if(el&&child)el.appendChild(child);return el}
function clearNode(node){if(node)node.replaceChildren()}
function appendText(parent,text){if(parent)parent.appendChild(document.createTextNode(String(text??'')))}
function makeEmpty(message,tag='div',colspan=0){let node;if(tag==='tr'){node=tplNode('tpl-empty-row')||document.createElement('tr');const cell=slot(node,'message')||node.firstElementChild;if(cell){cell.textContent=message;if(colspan)cell.colSpan=colspan}}else{node=tplNode('tpl-empty')||document.createElement(tag);node.textContent=message;node.classList.add('empty')}return node}
function makeIcon(icon){const i=document.createElement('i');i.className='bi '+icon;i.setAttribute('aria-hidden','true');return i}
function setButtonContent(btn,icon,label){btn.replaceChildren();if(icon){btn.appendChild(makeIcon(icon));btn.appendChild(document.createTextNode(' '))}const span=btn.querySelector('[data-slot="label"]')||document.createElement('span');span.textContent=label||'';if(!span.parentNode)btn.appendChild(span)}
function makeButton(label,icon,classes='btn btn-sm btn-outline-secondary',onClick=null){const btn=tplNode('tpl-action-button')||document.createElement('button');btn.type='button';btn.className=classes;setButtonContent(btn,icon,label);if(onClick)btn.addEventListener('click',onClick);return btn}

function actionTargetFromEvent(event){
  if(!event)return null;
  if(event.submitter)return event.submitter;
  const target=event.currentTarget||event.target;
  if(!target)return null;
  if(target.matches&&target.matches('button,a'))return target;
  if(target.querySelector)return target.querySelector('button[type="submit"],button:not([type]),.btn');
  return null;
}
const actionLoadingStates=new WeakMap();
function setActionLoading(target,label='Procesando'){
  const el=target&&target.closest?target.closest('button,a'):target;
  if(!el||actionLoadingStates.has(el))return()=>{};
  actionLoadingStates.set(el,{html:el.innerHTML,disabled:el.disabled,aria:el.getAttribute('aria-busy')});
  if('disabled' in el)el.disabled=true;
  el.setAttribute('aria-busy','true');
  el.classList.add('btn-loading');
  el.innerHTML='<span class="btn-loading-spinner" aria-hidden="true"></span><span>'+esc(label||'Procesando')+'</span>';
  return()=>{
    const st=actionLoadingStates.get(el);if(!st)return;
    el.innerHTML=st.html;
    if('disabled' in el)el.disabled=!!st.disabled;
    if(st.aria===null)el.removeAttribute('aria-busy');else el.setAttribute('aria-busy',st.aria);
    el.classList.remove('btn-loading');
    actionLoadingStates.delete(el);
  }
}
async function withActionLoading(target,label,work){
  const done=setActionLoading(target,label);
  try{return await work()}finally{done()}
}
function bindAsyncSubmit(form,handler,label='Procesando'){
  if(!form)return;
  form.setAttribute('action','javascript:void(0)');
  form.addEventListener('submit',event=>{
    event.preventDefault();
    const target=actionTargetFromEvent(event)||form;
    withActionLoading(target,label,()=>handler(event)).catch(err=>msg(err.message||String(err),true));
  });
}

function closeActionDropdowns(except=null){document.querySelectorAll('.action-dropdown.open').forEach(w=>{if(except&&w===except)return;w.classList.remove('open');const b=w.querySelector('.action-menu-toggle');if(b)b.setAttribute('aria-expanded','false')})}
function positionActionDropdownMenu(wrap){if(!wrap)return;const btn=wrap.querySelector('.action-menu-toggle');const menu=wrap.querySelector('.action-menu');if(!btn||!menu)return;const rect=btn.getBoundingClientRect();menu.style.visibility='hidden';menu.style.display='grid';const mw=menu.offsetWidth||220;const mh=menu.offsetHeight||180;let left=rect.right-mw;let top=rect.bottom+8;if(left<8)left=8;if(left+mw>window.innerWidth-8)left=Math.max(8,window.innerWidth-mw-8);if(top+mh>window.innerHeight-8)top=Math.max(8,rect.top-mh-8);menu.style.left=left+'px';menu.style.top=top+'px';menu.style.visibility='';menu.style.display=''}
function toggleActionDropdown(event){if(event){event.preventDefault();event.stopPropagation()}const wrap=event&&event.currentTarget?event.currentTarget.closest('.action-dropdown'):null;if(!wrap)return;const willOpen=!wrap.classList.contains('open');closeActionDropdowns(wrap);wrap.classList.toggle('open',willOpen);const b=wrap.querySelector('.action-menu-toggle');if(b)b.setAttribute('aria-expanded',willOpen?'true':'false');if(willOpen)positionActionDropdownMenu(wrap)}
function makeActionMenuItem(label,icon,options={}){const disabled=!!options.disabled;let item;if(options.href){item=document.createElement('a');item.href=disabled?'#':options.href;if(disabled){item.setAttribute('aria-disabled','true');item.tabIndex=-1}}else{item=document.createElement('button');item.type='button';if(disabled)item.disabled=true}item.className='action-menu-item'+(options.danger?' danger':'')+(options.warning?' warning':'')+(disabled?' disabled':'');item.setAttribute('role','menuitem');if(options.title)item.title=options.title;if(icon)item.appendChild(makeIcon(icon));const span=document.createElement('span');span.textContent=label||'';item.appendChild(span);if(options.onClick&&!disabled){item.addEventListener('click',event=>{event.stopPropagation();options.onClick(event);setTimeout(()=>closeActionDropdowns(),80)})}else if(options.href&&!disabled){item.addEventListener('click',()=>setTimeout(()=>closeActionDropdowns(),80))}return item}
function makeActionDropdown(items=[],label='Acciones'){const wrap=document.createElement('div');wrap.className='action-dropdown';wrap.setAttribute('data-action-dropdown','');const btn=document.createElement('button');btn.type='button';btn.className='btn btn-sm btn-outline-secondary action-menu-toggle';btn.setAttribute('aria-haspopup','menu');btn.setAttribute('aria-expanded','false');btn.setAttribute('aria-label',label);btn.addEventListener('click',toggleActionDropdown);btn.appendChild(makeIcon('bi-three-dots-vertical'));const menu=document.createElement('div');menu.className='action-menu';menu.setAttribute('role','menu');items.filter(Boolean).forEach(item=>menu.appendChild(item));wrap.appendChild(btn);wrap.appendChild(menu);return wrap}
document.addEventListener('click',event=>{if(!event.target.closest)return;const item=event.target.closest('.action-menu-item');if(item)setTimeout(()=>closeActionDropdowns(),80);if(!event.target.closest('.action-dropdown'))closeActionDropdowns()});document.addEventListener('keydown',event=>{if(event.key==='Escape')closeActionDropdowns()});window.addEventListener('resize',()=>closeActionDropdowns());window.addEventListener('scroll',()=>closeActionDropdowns(),true);

function makeLink(label,href,icon,classes='btn btn-sm btn-outline-secondary'){const a=tplNode('tpl-icon-link')||document.createElement('a');a.href=href||'#';a.className=classes;setButtonContent(a,icon,label);return a}
function makeDownload(label,href){const a=tplNode('tpl-download-link')||document.createElement('a');a.href=href||'#';const s=slot(a,'label');if(s)s.textContent=label||'Descargar';return a}
function makeStatePill(text,on=false,off=false){const pill=tplNode('tpl-state-pill')||document.createElement('span');pill.classList.toggle('on',!!on);pill.classList.toggle('off',!!off);const dot=pill.querySelector('.status-dot');if(dot)dot.classList.toggle('ok',!!on);setSlot(pill,'text',text);return pill}
function makeResourceTag(text,cls='tag-muted',title=''){const tag=tplNode('tpl-resource-tag')||document.createElement('span');tag.className='resource-tag '+cls;tag.textContent=String(text??'');if(title)tag.title=title;return tag}
function msg(t,bad=false){const m=$('msg');m.className='alert '+(bad?'alert-danger':'alert-success');m.textContent=t;m.classList.remove('d-none');setTimeout(()=>m.classList.add('d-none'),7000);window.scrollTo({top:0,behavior:'smooth'})}

function modalReady(ids){return ids.every(id=>!!$(id))}
function resetConfirmModalSafe(){const modal=$('confirmModal');if(!modal)return;const icon=modal.querySelector('.confirm-icon');if(icon){icon.className='confirm-icon';icon.replaceChildren(makeIcon('bi-exclamation-triangle'))}const cancel=$('confirmCancelBtn');if(cancel)cancel.classList.remove('d-none')}
function confirmAction(title,body,confirmText='Confirmar',danger=true){return new Promise(resolve=>{if(!modalReady(['confirmModal','confirmTitle','confirmBody','confirmCancelBtn','confirmAcceptBtn'])){resolve(window.confirm(String(title||'Confirmar')+'\n\n'+String(body||'')));return}const modal=$('confirmModal');resetConfirmModalSafe();$('confirmTitle').textContent=title;$('confirmBody').textContent=body;setButtonContent($('confirmAcceptBtn'),'bi-check2',confirmText);$('confirmAcceptBtn').className=danger?'btn btn-outline-danger':'btn btn-primary';const finish=v=>{modal.classList.remove('open');resetConfirmModalSafe();resolve(v)};$('confirmCancelBtn').onclick=()=>finish(false);$('confirmAcceptBtn').onclick=()=>finish(true);modal.classList.add('open')})}
function showNotice(title,body,confirmText='Entendido'){return new Promise(resolve=>{if(!modalReady(['confirmModal','confirmTitle','confirmBody','confirmCancelBtn','confirmAcceptBtn'])){window.alert(String(title||'Aviso')+'\n\n'+String(body||''));resolve(true);return}const modal=$('confirmModal');resetConfirmModalSafe();const icon=modal.querySelector('.confirm-icon');if(icon){icon.className='confirm-icon notice-icon';icon.replaceChildren(makeIcon('bi-check2-circle'))}$('confirmTitle').textContent=title;$('confirmBody').textContent=body;$('confirmCancelBtn').classList.add('d-none');setButtonContent($('confirmAcceptBtn'),'bi-check2',confirmText);$('confirmAcceptBtn').className='btn btn-primary';const finish=()=>{modal.classList.remove('open');resetConfirmModalSafe();resolve(true)};$('confirmAcceptBtn').onclick=finish;modal.classList.add('open')})}
function showBusy(title,body){if(!modalReady(['busyModal','busyTitle','busyBody']))return;$('busyTitle').textContent=title||'Procesando';$('busyBody').textContent=body||'Validando y aplicando cambios';$('busyModal').classList.add('open')}
function hideBusy(){const modal=$('busyModal');if(modal)modal.classList.remove('open')}
function confirmInputAction(opts={}){return new Promise(resolve=>{if(!modalReady(['passwordConfirmModal','passwordConfirmInput','passwordConfirmTitle','passwordConfirmBody','passwordConfirmInputLabel','passwordConfirmCancelBtn','passwordConfirmAcceptBtn'])){const value=window.prompt(String(opts.title||'Confirmar accion')+'\n\n'+String(opts.body||''),opts.value||'');resolve(value);return}const modal=$('passwordConfirmModal');const input=$('passwordConfirmInput');$('passwordConfirmTitle').textContent=opts.title||'Confirmar accion';$('passwordConfirmBody').textContent=opts.body||'Confirma para continuar.';$('passwordConfirmInputLabel').textContent=opts.label||'Valor';input.type=opts.type||'text';input.autocomplete=opts.autocomplete||'off';input.placeholder=opts.placeholder||'';input.className='form-control '+(opts.inputClass||'');input.value=opts.value||'';setButtonContent($('passwordConfirmAcceptBtn'),opts.icon||'bi-check2',opts.confirmText||'Confirmar');$('passwordConfirmAcceptBtn').className=opts.danger?'btn btn-outline-danger':'btn btn-primary';const finish=accepted=>{const value=input.value;modal.classList.remove('open');resolve(accepted?value:null)};$('passwordConfirmCancelBtn').onclick=()=>finish(false);$('passwordConfirmAcceptBtn').onclick=()=>finish(true);input.onkeydown=e=>{if(e.key==='Enter'){e.preventDefault();finish(true)}else if(e.key==='Escape'){e.preventDefault();finish(false)}};modal.classList.add('open');setTimeout(()=>input.focus(),50)})}
function confirmPasswordAction(title,body,confirmText='Confirmar eliminación'){return confirmInputAction({title,body,label:'Contraseña del administrador',type:'password',autocomplete:'current-password',confirmText,icon:'bi-trash3',danger:true,inputClass:'password-confirm-input'})}
function showHelp(t,b){$('helpTitle').textContent=t;$('helpBody').textContent=b;$('helpModal').classList.add('open')}function closeHelp(){$('helpModal').classList.remove('open')}
let commandCopies={};
function rememberCopy(value){const id='copy_'+Math.random().toString(36).slice(2);commandCopies[id]=String(value||'');return id}
function copyButtonNode(id,label='Copiar'){const btn=tplNode('tpl-copy-button');btn.dataset.copyLabel=label;const slotEl=btn.querySelector('[data-slot="label"]');if(slotEl)slotEl.textContent=label;btn.addEventListener('click',()=>copyCommand(id,btn));return btn}
function copyButton(id,label='Copiar'){return copyButtonNode(id,label).outerHTML}
function commandBlockNode(title,cmd){const id=rememberCopy(cmd);const node=tplNode('tpl-command-card');setSlot(node,'title',title);appendSlot(node,'button',copyButtonNode(id));setSlot(node,'command',String(cmd||''));return node}
function commandBlock(title,cmd){return commandBlockNode(title,cmd).outerHTML}
function secretBlockNode(label,value){const id=rememberCopy(value);const node=tplNode('tpl-secret-row');setSlot(node,'label',label);setSlot(node,'value',String(value||''));appendSlot(node,'button',copyButtonNode(id));return node}
function secretBlock(label,value){return secretBlockNode(label,value).outerHTML}
function copyFeedback(btn,ok=true){if(!btn)return;const original=btn.dataset.copyOriginal||btn.textContent||'Copiar';btn.dataset.copyOriginal=original;btn.disabled=true;setButtonContent(btn,ok?'bi-check2':'bi-exclamation-triangle',ok?'Copiado':'Error');setTimeout(()=>{setButtonContent(btn,'bi-clipboard',btn.dataset.copyOriginal);btn.disabled=false},1300)}
async function copyText(value){const sx=window.scrollX,sy=window.scrollY;const active=document.activeElement;let ta=null;try{if(navigator.clipboard&&window.isSecureContext){await navigator.clipboard.writeText(String(value||''))}else{ta=document.createElement('textarea');ta.value=String(value||'');ta.setAttribute('readonly','');ta.style.position='fixed';ta.style.top='0';ta.style.left='0';ta.style.width='1px';ta.style.height='1px';ta.style.opacity='0';ta.style.pointerEvents='none';document.body.appendChild(ta);ta.focus({preventScroll:true});ta.select();if(!document.execCommand('copy'))throw new Error('copiado no disponible')}return true}catch(err){throw err}finally{if(ta)ta.remove();if(active&&active.focus)active.focus({preventScroll:true});window.scrollTo(sx,sy)}}
async function copyCommand(id,btn){try{await copyText(commandCopies[id]||'');copyFeedback(btn,true)}catch(err){copyFeedback(btn,false)}}
function shortID(id){id=String(id||'');return id.length>10?id.slice(0,6)+'...'+id.slice(-4):id}
function fmt(v){if(!v||v==='0001-01-01T00:00:00Z')return '-';try{return new Date(v).toLocaleString()}catch{return v}}
async function api(url,opt={}){opt.headers=Object.assign({'Content-Type':'application/json'},opt.headers||{});if(opt.method&&opt.method!=='GET')opt.headers['X-CSRF-Token']=csrf;let res;try{res=await fetch(url,opt)}catch(err){throw new Error('No se pudo conectar con Pangolite. Si acabas de cambiar puertos TCP/UDP, espera unos segundos y vuelve a intentar.')}const text=await res.text();let data={};try{data=text?JSON.parse(text):{}}catch{data={raw:text}}if(!res.ok){if(res.status===401)location.href='/login';let detail=data.error||data.message||res.statusText||'Error inesperado';if(res.status===400)detail='Solicitud invalida: '+detail;if(res.status===403)detail='Accion bloqueada por seguridad: '+detail;if(res.status===409)detail='Conflicto: '+detail;if(res.status===429)detail='Demasiados intentos: '+detail;throw new Error(detail)}return data}
function requiresStaticTraefikRestart(payload,current=null){if(!payload)return false;const mode=(payload.mode||'').toLowerCase();if(mode!=='tcp'&&mode!=='udp')return false;if(!current)return true;return (current.mode||'').toLowerCase()!==mode||Number(current.publicPort||0)!==Number(payload.publicPort||0)}
async function confirmTraefikRestartIfNeeded(payload,current=null){if(!requiresStaticTraefikRestart(payload,current))return true;const port=payload&&payload.publicPort?(' puerto '+payload.publicPort):'';return confirmAction('Reinicio global requerido','Este tunnel '+String(payload.mode||'TCP/UDP').toUpperCase()+port+' necesita actualizar entryPoints de Traefik. Para aplicarlo, Pangolite reiniciara el servicio global de Traefik y las conexiones activas podrian cortarse unos segundos.','Entendido, continuar',false)}
function traefikNotice(data){const t=data&&(data.traefik||data);if(!t||!t.message)return '';return (t.warning? t.warning+' ' : '')+t.message}
function go(path){location.href=path}
function goNotice(path,title,body){try{sessionStorage.setItem('pangolite.notice',JSON.stringify({title,body}))}catch{}location.href=path}
function showStoredNotice(){let raw='';try{raw=sessionStorage.getItem('pangolite.notice');sessionStorage.removeItem('pangolite.notice')}catch{}if(!raw)return;try{const n=JSON.parse(raw);if(n&&n.title)showNotice(n.title,n.body||'')}catch{}}
