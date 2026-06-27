async function init(){
  const boot=loadBootstrap();
  if(!boot){
    const s=await fetch('/api/session').then(r=>r.json());
    if(!s.authenticated){location.href='/login';return}
    if(s.user.forcePasswordChange){location.href='/password';return}
    csrf=s.csrfToken;
    if($('userLabel'))$('userLabel').textContent=s.user.username;
    await reloadDomains();
    await reloadProjects();
    await loadSettings();
  }else{
    if(boot.user&&$('userLabel'))$('userLabel').textContent=boot.user.username||'';
    syncOnboarding();
    renderGlobalDashboard();
    fillDomainSelect();
    paintNetworkInfo(boot.certificate);
  }
  if(suspensionTemplates.length){refreshTemplateSelects()}else{await reloadSuspensionTemplates()}
  await route({initial:!!boot});
  showStoredNotice();
}
async function reloadProjects(){const data=await api('/api/projects');projects=data.projects||[];stats=data.stats||{};paintProjectNav();paintProjectTable();let tr=0,ta=0,active=0;Object.values(stats).forEach(x=>{tr+=x.resources||0;ta+=x.agents||0;active+=x.activeResources||0});setTextIfExists('statProjects',projects.length);setTextIfExists('statResources',tr);setTextIfExists('statAgents',ta);setTextIfExists('statActiveResources',active);syncOnboarding();renderGlobalDashboard();}
function syncOnboarding(){const card=maybeEl('onboardingCard');if(card)card.classList.toggle('d-none',projects.length>0)}
function renderGlobalDashboard(){
  const canvasResources=$('resourcesByProjectChart');
  const canvasStatus=$('resourceStatusChart');
  if(!canvasResources||!canvasStatus)return;
  if(typeof Chart==='undefined'){
    ['resourcesChartFallback','statusChartFallback'].forEach(id=>{const el=$(id);if(el)el.style.display='block'});
    canvasResources.style.display='none';canvasStatus.style.display='none';return;
  }
  ['resourcesChartFallback','statusChartFallback'].forEach(id=>{const el=$(id);if(el)el.style.display='none'});
  canvasResources.style.display='block';canvasStatus.style.display='block';
  const ordered=projects.slice().sort((a,b)=>((stats[b.id]||{}).resources||0)-((stats[a.id]||{}).resources||0)).slice(0,8);
  const labels=ordered.map(p=>p.name.length>22?p.name.slice(0,21)+'…':p.name);
  const values=ordered.map(p=>(stats[p.id]||{}).resources||0);
  const active=Object.values(stats).reduce((n,x)=>n+(x.activeResources||0),0);
  const total=Object.values(stats).reduce((n,x)=>n+(x.resources||0),0);
  const inactive=Math.max(total-active,0);
  if(charts.resourcesByProject)charts.resourcesByProject.destroy();
  if(charts.resourceStatus)charts.resourceStatus.destroy();
  const common={responsive:true,maintainAspectRatio:false,plugins:{legend:{labels:{color:'#cdcdcd',boxWidth:10}},tooltip:{backgroundColor:'#08080c',borderColor:'rgba(255,255,255,.18)',borderWidth:1,titleColor:'#fff',bodyColor:'#cdcdcd'}},scales:{x:{ticks:{color:'#afafaf'},grid:{color:'rgba(255,255,255,.08)'}},y:{beginAtZero:true,ticks:{color:'#afafaf',precision:0},grid:{color:'rgba(255,255,255,.08)'}}}};
  charts.resourcesByProject=new Chart(canvasResources,{type:'bar',data:{labels:labels.length?labels:['Sin proyectos'],datasets:[{label:'Recursos',data:values.length?values:[0],borderRadius:12,borderSkipped:false}]},options:common});
  charts.resourceStatus=new Chart(canvasStatus,{type:'doughnut',data:{labels:['Activos','Suspendidos / inactivos'],datasets:[{label:'Recursos',data:[active,inactive]}]},options:{responsive:true,maintainAspectRatio:false,cutout:'68%',plugins:{legend:{position:'bottom',labels:{color:'#cdcdcd',boxWidth:10}},tooltip:{backgroundColor:'#08080c',borderColor:'rgba(255,255,255,.18)',borderWidth:1,titleColor:'#fff',bodyColor:'#cdcdcd'}}}});
}
async function loadSettings(){const data=await api('/api/settings');panelSettings=data.settings||{};networkInfo=data.network||{};if($('dashboardDomain'))$('dashboardDomain').value=panelSettings.dashboardDomain||'';if($('letsEncryptEmail'))$('letsEncryptEmail').value=panelSettings.letsEncryptEmail||'';if($('backupIntervalHours'))$('backupIntervalHours').value=panelSettings.backupIntervalHours??24;if($('backupRetentionDays'))$('backupRetentionDays').value=panelSettings.backupRetentionDays??14;paintNetworkInfo(data.certificate)}
function certText(status){if(!status)return 'Sin revisar';if(status.status==='issued')return 'Generado';if(status.status==='pending')return 'En proceso';if(status.status==='disabled')return 'Desactivado';if(status.status==='acme_disabled')return 'Falta correo ACME';if(status.status==='missing_domain')return 'Sin dominio';return 'No disponible'}
function certClass(status){if(!status)return 'warn';if(status.status==='issued')return 'good';if(status.status==='disabled')return 'warn';if(status.status==='pending')return 'warn';return 'bad'}
function certificateMessage(status){if(!status)return 'Estado SSL sin revisar.';let msg=status.message||certText(status);if(status.expiresAt)msg+=' Expira: '+fmt(status.expiresAt);return msg}
function paintCertificateStatus(targetId,status){const el=maybeEl(targetId);if(!el)return;el.classList.remove('alert-success','alert-warning','alert-danger','pl-success','pl-warning','pl-danger','pl-surface');const cls=certClass(status);el.classList.add(cls==='good'?'pl-success':(cls==='bad'?'pl-danger':'pl-warning'));el.replaceChildren(makeIcon('bi-shield-lock'), document.createTextNode(' '+certificateMessage(status)))}
function paintLocalCertificateHint(targetId,domain,sslEnabled){const el=maybeEl(targetId);if(!el)return;const status=!domain?{status:'missing_domain',message:'Selecciona o escribe un dominio para revisar SSL.'}:(!sslEnabled?{status:'disabled',message:'SSL desactivado. El recurso quedará disponible solo por HTTP.'}:{status:'pending',message:'SSL activado. Se revisará el certificado al guardar o crear el recurso.'});paintCertificateStatus(targetId,status)}
async function fetchCertificateStatus(domain,sslEnabled,targetId){const data=await api('/api/certificates/status?domain='+encodeURIComponent(domain||'')+'&ssl='+(sslEnabled?'1':'0'));const cert=data.certificate||data;paintCertificateStatus(targetId,cert);return cert}
function paintNetworkInfo(certificate){const ip=networkInfo.publicIp||'No detectada';const domain=panelSettings.dashboardDomain||'';const ips=(networkInfo.dashboardDomainIps||[]).join(', ');const dnsText=ips?(ips+(networkInfo.dnsMatchesServer?' ✓':' ✗')):'Sin DNS consultado';if($('serverIpHint'))$('serverIpHint').textContent=ip;if($('dashboardDnsHint'))$('dashboardDnsHint').textContent=dnsText;if($('dashboardCertHint')){$('dashboardCertHint').textContent=certText(certificate);$('dashboardCertHint').className='font-mono text-white '+certClass(certificate)}if($('dashboardCertDetail'))$('dashboardCertDetail').textContent=certificateMessage(certificate);if($('dashPanelDomain'))$('dashPanelDomain').textContent=domain||'Sin configurar';if($('dashPublicIp'))$('dashPublicIp').textContent=ip;if($('dashDnsState')){const el=$('dashDnsState');el.classList.remove('good','warn','bad');if(!domain){el.textContent='Sin dominio';el.classList.add('warn')}else if(networkInfo.dnsMatchesServer){el.textContent='Correcto';el.classList.add('good')}else if(ips){el.textContent='No coincide';el.classList.add('bad')}else{el.textContent='Pendiente';el.classList.add('warn')}}}
async function saveSettings(e){e.preventDefault();try{const payload={dashboardDomain:$('dashboardDomain').value.trim().toLowerCase(),letsEncryptEmail:$('letsEncryptEmail').value.trim().toLowerCase(),backupIntervalHours:fieldNumber('backupIntervalHours'),backupRetentionDays:fieldNumber('backupRetentionDays')};const data=await api('/api/settings',{method:'PATCH',body:JSON.stringify(payload)});panelSettings=data.settings||{};networkInfo=data.network||{};paintNetworkInfo(data.certificate);const t=data.traefik||{};const cert=data.certificate?(' SSL: '+certText(data.certificate)+'.'):'';msg((t.message||'Ajustes guardados. Traefik se actualizo automaticamente.')+cert)}catch(err){msg(err.message,true)}}
async function reloadDomains(){const data=await api('/api/domains');domains=data.domains||[];paintDomains();fillDomainSelect();}
