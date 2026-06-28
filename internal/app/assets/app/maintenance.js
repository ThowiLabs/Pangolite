async function loadAudit(){try{const data=await api('/api/audit?limit=200');auditEvents=data.events||[];paintAudit()}catch(err){const box=$('auditRows');if(box){clearNode(box);box.appendChild(makeEmpty('No se pudo cargar auditoría: '+err.message))}}}
function auditMeta(ev){let meta=ev.metadata||'';try{const obj=JSON.parse(meta);meta=Object.entries(obj).map(([k,v])=>k+': '+String(v)).join(' · ')}catch{}return meta||ev.remoteIp||'-'}
function paintAudit(){const box=$('auditRows');if(!box)return;clearNode(box);if(!auditEvents.length){box.appendChild(makeEmpty('Sin eventos de auditoría todavía.'));return}const table=tplNode('tpl-audit-table');const body=slot(table,'body');auditEvents.forEach(ev=>{const row=tplNode('tpl-audit-row');setSlot(row,'date',fmt(ev.createdAt));setSlot(row,'action',ev.action);setSlot(row,'entityType',ev.entityType);setSlot(row,'entityId',ev.entityId||'-');setSlot(row,'username',ev.username||'-');setSlot(row,'detail',auditMeta(ev));body.appendChild(row)});box.appendChild(table)}
async function loadBackups(){try{const data=await api('/api/backups');backups=data.backups||[];if($('backupDirHint'))$('backupDirHint').textContent='Ruta: '+(data.backupDir||'-');paintBackups()}catch(err){const box=$('backupRows');if(box){clearNode(box);box.appendChild(makeEmpty('No se pudieron cargar respaldos: '+err.message))}}}
function paintBackups(){const box=$('backupRows');if(!box)return;clearNode(box);if(!backups.length){box.appendChild(makeEmpty('Aún no hay respaldos. Crea uno antes de cambios destructivos.'));return}const table=tplNode('tpl-backup-table');const body=slot(table,'body');backups.forEach(b=>{const row=tplNode('tpl-backup-row');setSlot(row,'name',b.name);setSlot(row,'size',formatBytes(b.sizeBytes||0));setSlot(row,'created',fmt(b.createdAt));appendSlot(row,'actions',makeDownload('Descargar','/api/backups/'+encodeURIComponent(b.name)+'/download'));body.appendChild(row)});box.appendChild(table)}
function formatBytes(n){n=Number(n||0);if(n<1024)return n+' B';if(n<1024*1024)return (n/1024).toFixed(1)+' KB';return (n/1024/1024).toFixed(2)+' MB'}
async function createBackup(button=null){
  const label=await confirmInputAction({title:'Crear respaldo SQLite',body:'Escribe un prefijo opcional para identificar el respaldo. Dejalo vacio para usar el nombre automatico.',label:'Prefijo del respaldo',placeholder:'antes-eliminar-proyecto',confirmText:'Crear respaldo',icon:'bi-database-add',danger:false});
  if(label===null)return;
  await withActionLoading(button,'Creando',async()=>{
    try{
      showBusy('Creando respaldo','SQLite genera una copia consistente sin detener el panel');
      const backup=await api('/api/backups',{method:'POST',body:JSON.stringify({label})});
      msg('Respaldo creado: '+backup.name);
      await loadBackups();
      await loadAudit();
    }catch(err){msg(err.message,true)}finally{hideBusy()}
  })
}
async function loadMaintenance(){await Promise.all([loadBackups(),loadAudit()])}
