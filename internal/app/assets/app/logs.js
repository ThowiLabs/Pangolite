function logLevelOf(line){
  const value=String(line||'');
  if(value.includes('level=ERROR')||value.includes(' level=error')||value.includes('level":"error'))return 'error';
  if(value.includes('level=WARN')||value.includes(' level=warn')||value.includes('level":"warn'))return 'warn';
  if(value.includes('level=INFO')||value.includes(' level=info')||value.includes('level":"info'))return 'info';
  return 'other';
}
function paintLogs(){
  const box=maybeEl('logsBox');
  if(!box)return;
  const q=fieldValue('logsSearch').toLowerCase();
  const level=fieldValue('logsLevel')||'all';
  const filtered=(logsLines||[]).filter(line=>{
    const text=String(line||'');
    if(level!=='all'&&logLevelOf(text)!==level)return false;
    if(q&&text.toLowerCase().indexOf(q)<0)return false;
    return true;
  });
  clearNode(box);
  if(!filtered.length){box.textContent='Sin eventos para el filtro actual.';return}
  filtered.forEach(line=>{
    const node=tplNode('tpl-log-line')||document.createElement('span');
    node.textContent=line;
    const levelName=logLevelOf(line);
    if(levelName==='error')node.classList.add('error');
    if(levelName==='warn')node.classList.add('warn');
    box.appendChild(node);
  });
  box.scrollTop=box.scrollHeight;
}
async function loadLogs(){
  try{
    const data=await api('/api/system/logs?limit=300');
    logsLines=data.lines||[];
    if($('logsPath'))$('logsPath').textContent='Archivo: '+(data.path||'no configurado')+' · maximo '+(data.maxEntries||1000)+' entradas';
    paintLogs();
  }catch(err){if($('logsBox'))$('logsBox').textContent='No se pudieron cargar logs: '+err.message}
}
async function copyLogs(btn){
  try{await copyText((logsLines||[]).join('\n'));copyFeedback(btn,true)}catch(err){copyFeedback(btn,false);msg('No se pudieron copiar logs: '+err.message,true)}
}
function clearLogsView(){logsLines=[];paintLogs();msg('Vista de logs limpiada. El archivo original no se borro.')}
function downloadLogs(){location.href='/api/system/logs/download'}
function setupLogsControls(){
  const search=maybeEl('logsSearch');if(search)search.addEventListener('input',paintLogs);
  const level=maybeEl('logsLevel');if(level)level.addEventListener('change',paintLogs);
}
