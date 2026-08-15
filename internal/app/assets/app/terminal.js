(function(){
  let term=null;
  let fit=null;
  let ws=null;
  let connectedTarget='';
  let resizeTimer=null;
  let socketSerial=0;
  let mobileKeysEnabled=false;
  let mobileModifiers={ctrl:false,alt:false};
  let mobileKeyboardFloating=false;
  let mobileLayoutTimer=null;
  let cwdTimer=null;
  let overlayAction='connect';
  let overlayWorkingDir='';
  const terminalLastDirs={};
  const encoder=new TextEncoder();
  const terminalUploadPrefix=encoder.encode('\x00PANGOLITE-TERMINAL-UPLOAD ');
  const terminalUploadChunkSize=24*1024;
  let terminalUploadQueue=[];
  let terminalUploadRunning=false;
  let terminalUploadBatch={total:0,completed:0,failed:0};
  const terminalUploadStates=new Map();
  const terminalDownloadStates=new Map();
  let terminalCommandBuffer='';
  let terminalCommandTracking=true;
  const themes={
    black:{background:'#05070a',foreground:'#f8fafc',cursor:'#f8fafc',selectionBackground:'#475569'},
    dark:{background:'#002b36',foreground:'#fdf6e3',cursor:'#eee8d5',selectionBackground:'#ffffff33',black:'#073642',red:'#dc322f',green:'#859900',yellow:'#b58900',blue:'#268bd2',magenta:'#d33682',cyan:'#2aa198',white:'#eee8d5'},
    light:{background:'#fdf6e3',foreground:'#002b36',cursor:'#073642',selectionBackground:'#00000033',black:'#eee8d5',red:'#dc322f',green:'#859900',yellow:'#b58900',blue:'#268bd2',magenta:'#d33682',cyan:'#2aa198',white:'#073642'}
  };
  const windowsWarning='La consola remota en Windows está deshabilitada temporalmente porque puede fallar demasiado según la versión, el tipo de servicio y la sesión interactiva. Usa Linux para consola estable o entra por RDP, PowerShell Remoting o SSH mientras se implementa soporte Windows confiable.';
  function el(id){return document.getElementById(id)}
  function wsURL(path,query=''){
    const proto=location.protocol==='https:'?'wss:':'ws:';
    return proto+'//'+location.host+path+query;
  }
  function status(text,on=false,bad=false){
    const s=el('terminalStatus');
    if(!s)return;
    s.className='terminal-status '+(on?'on':bad?'bad':'off');
    s.replaceChildren();
    const dot=document.createElement('span');
    dot.className='status-dot '+(on?'ok':'');
    s.appendChild(dot);
    s.appendChild(document.createTextNode(' '+text));
  }
  function setButtons(mode){
    const connect=el('terminalConnectBtn'), disconnect=el('terminalDisconnectBtn'), target=el('terminalTarget'), upload=el('terminalUploadBtn'), options=el('terminalOptionsBtn');
    const busy=mode==='connecting'||mode==='connected';
    if(connect){connect.disabled=busy;connect.classList.toggle('d-none',mode==='connected')}
    if(disconnect)disconnect.disabled=!busy;
    if(target)target.disabled=busy;
    if(upload)upload.disabled=mode!=='connected';
    if(options)options.disabled=mode==='idle';
  }
  function setOverlay(kind,title,text,buttonText,loading,secondaryText){
    const overlay=el('terminalOverlay');
    if(!overlay)return;
    const spinner=el('terminalOverlaySpinner');
    const titleEl=el('terminalOverlayTitle');
    const textEl=el('terminalOverlayText');
    const button=el('terminalOverlayButton');
    const secondary=el('terminalOverlaySecondaryButton');
    const resumePath=el('terminalResumePath');
    overlay.className='terminal-overlay '+(kind||'idle');
    overlay.classList.remove('d-none');
    if(spinner)spinner.classList.toggle('d-none',!loading);
    if(titleEl)titleEl.textContent=title||'';
    if(textEl)textEl.textContent=text||'';
    if(resumePath)resumePath.classList.add('d-none');
    if(button){
      button.classList.toggle('d-none',!buttonText);
      const span=button.querySelector('span');
      if(span)span.textContent=buttonText||'';
      else button.textContent=buttonText||'';
      button.disabled=!!loading;
    }
    if(secondary){
      secondary.classList.toggle('d-none',!secondaryText);
      const span=secondary.querySelector('span');
      if(span)span.textContent=secondaryText||'';
      secondary.disabled=!!loading;
    }
  }
  function hideOverlay(){
    const overlay=el('terminalOverlay');
    if(overlay)overlay.classList.add('d-none');
  }
  function currentTerminalTarget(){return (el('terminalTarget')&&el('terminalTarget').value)||'local'}
  function hydrateTerminalLastDirs(usageOverride){
    let usage=usageOverride||{};
    if(!usageOverride){try{usage=(appBoot&&appBoot.terminalUsage)||{}}catch{}}
    Object.entries(usage||{}).forEach(([target,item])=>{
      const path=String(item&&item.lastDir||'').trim();
      if(path)terminalLastDirs[target]=path;
    });
  }
  async function refreshTerminalLastDirs(){
    try{
      const response=await fetch('/api/terminal/state',{headers:{'Accept':'application/json'},cache:'no-store'});
      if(!response.ok)return;
      const data=await response.json();
      const usage=data&&data.usage||{};
      if(appBoot)appBoot.terminalUsage=usage;
      hydrateTerminalLastDirs(usage);
    }catch{}
  }
  function lastDirForTarget(target){return String(terminalLastDirs[target||currentTerminalTarget()]||'').trim()}
  function rememberTerminalDir(target,path){
    target=String(target||'').trim();path=String(path||'').trim();
    if(target&&path)terminalLastDirs[target]=path;
  }
  function showIdleOverlay(){
    overlayAction='connect';overlayWorkingDir='';
    if(currentTargetOS()==='windows'){
      status('Windows no confiable',false,true);
      setOverlay('warning','Terminal Windows no confiable',windowsWarning,'Cerrar aviso');
      return;
    }
    status('Desconectado',false,false);
    setOverlay('idle','Aún no conectado','Selecciona un destino y presiona Conectar para abrir una consola.','Conectar');
  }
  function showResumeOverlay(dir){
    dir=String(dir||'').trim();
    if(!dir){showIdleOverlay();return false}
    overlayAction='resume';overlayWorkingDir=dir;
    status('Sesión anterior disponible',false,false);
    setOverlay('idle','¿Reanudar sesión?','La última sesión de este destino quedó en esta carpeta:','Reanudar',false,'Iniciar desde carpeta predeterminada');
    const resumePath=el('terminalResumePath');
    if(resumePath){resumePath.textContent=dir;resumePath.classList.remove('d-none')}
    return true;
  }
  function showReconnectOverlay(title,text,dir){
    overlayAction='reconnect';overlayWorkingDir=String(dir||'').trim();
    setOverlay('bad',title||'Conexión cerrada',text||'La sesión de consola se cerró o el cliente dejó de responder.','Reconectar');
    const resumePath=el('terminalResumePath');
    if(resumePath&&overlayWorkingDir){resumePath.textContent='Se reanudará en '+overlayWorkingDir;resumePath.classList.remove('d-none')}
  }
  function requestTerminalConnection(){
    const dir=lastDirForTarget(currentTerminalTarget());
    if(dir){showResumeOverlay(dir);return}
    connectTerminal('');
  }
  function isTerminalFullscreen(){
    const card=el('terminalCard');
    return !!card&&(document.fullscreenElement===card||card.classList.contains('terminal-fullscreen-fallback'));
  }
  function isConnected(){
    return !!(ws&&ws.readyState===WebSocket.OPEN);
  }
  function contextMenuOpen(){
    const menu=el('terminalContextMenu');
    return !!(menu&&menu.classList.contains('open'));
  }
  function sendEscapeToTerminal(){
    if(!isConnected())return false;
    return sendBytes('\x1b');
  }
  function resetTerminalView(){
    fitTerminal();
    queueResize();
    if(term){
      try{term.scrollToBottom()}catch{}
      term.focus();
    }
  }
  async function lockTerminalEscape(){
    try{
      if(navigator.keyboard&&navigator.keyboard.lock){
        await navigator.keyboard.lock(['Escape']);
      }
    }catch{}
  }
  function unlockTerminalEscape(){
    try{
      if(navigator.keyboard&&navigator.keyboard.unlock)navigator.keyboard.unlock();
    }catch{}
  }
  function sendBytes(value){
    if(ws&&ws.readyState===WebSocket.OPEN){
      ws.send(encoder.encode(value));
      return true;
    }
    return false;
  }
  function isAndroidTouchDevice(){
    return /Android/i.test(String(navigator.userAgent||''))&&(Number(navigator.maxTouchPoints)||0)>0;
  }
  function terminalTextarea(){
    const box=el('terminalBox');
    return box&&box.querySelector('.xterm-helper-textarea');
  }
  function updateMobileModifierButtons(){
    const bar=el('terminalMobileKeys');
    if(!bar)return;
    for(const name of ['ctrl','alt']){
      const button=bar.querySelector('[data-terminal-mobile-modifier="'+name+'"]');
      if(!button)continue;
      const active=!!mobileModifiers[name];
      button.classList.toggle('active',active);
      button.setAttribute('aria-pressed',String(active));
    }
  }
  function clearMobileModifiers(){
    if(!mobileModifiers.ctrl&&!mobileModifiers.alt)return;
    mobileModifiers={ctrl:false,alt:false};
    updateMobileModifierButtons();
  }
  function toggleMobileModifier(name){
    if(name!=='ctrl'&&name!=='alt')return;
    mobileModifiers[name]=!mobileModifiers[name];
    updateMobileModifierButtons();
    if(term)term.focus();
  }
  function controlCharacter(value){
    const chars=Array.from(String(value||''));
    if(chars.length!==1)return value;
    const ch=chars[0];
    const code=ch.charCodeAt(0);
    if((code>=65&&code<=90)||(code>=97&&code<=122))return String.fromCharCode(code&31);
    const controls={
      '@':'\x00',' ':'\x00','2':'\x00',
      '[':'\x1b','3':'\x1b',
      '\\':'\x1c','4':'\x1c',
      ']':'\x1d','5':'\x1d',
      '^':'\x1e','6':'\x1e',
      '_':'\x1f','-':'\x1f','7':'\x1f',
      '?':'\x7f','8':'\x7f'
    };
    return Object.prototype.hasOwnProperty.call(controls,ch)?controls[ch]:value;
  }
  function applyMobileModifiersToInput(data){
    if(!mobileKeysEnabled||(!mobileModifiers.ctrl&&!mobileModifiers.alt))return data;
    let output=String(data||'');
    if(mobileModifiers.ctrl)output=controlCharacter(output);
    if(mobileModifiers.alt)output='\x1b'+output;
    clearMobileModifiers();
    return output;
  }
  function mobileModifierCode(){
    if(mobileModifiers.ctrl&&mobileModifiers.alt)return 7;
    if(mobileModifiers.ctrl)return 5;
    if(mobileModifiers.alt)return 3;
    return 0;
  }
  function mobileKeySequence(key){
    const modifier=mobileModifierCode();
    const navigation={up:'A',down:'B',right:'C',left:'D',home:'H',end:'F'};
    let sequence='';
    if(navigation[key])sequence=modifier?'\x1b[1;'+modifier+navigation[key]:'\x1b['+navigation[key];
    else if(key==='pageup')sequence=modifier?'\x1b[5;'+modifier+'~':'\x1b[5~';
    else if(key==='pagedown')sequence=modifier?'\x1b[6;'+modifier+'~':'\x1b[6~';
    else if(key==='escape')sequence='\x1b';
    else if(key==='tab')sequence='\t';
    if(sequence&&mobileModifiers.alt&&(key==='escape'||key==='tab'))sequence='\x1b'+sequence;
    if(sequence)clearMobileModifiers();
    return sequence;
  }
  function toggleMobileKeyboard(){
    const textarea=terminalTextarea();
    if(!textarea||!term)return;
    if(document.activeElement===textarea){
      textarea.blur();
      clearMobileModifiers();
    }else{
      term.focus();
    }
    queueMobileKeyboardLayout();
  }
  function handleMobileTerminalButton(button){
    if(!button)return;
    const modifier=button.dataset.terminalMobileModifier;
    if(modifier){
      if(!isConnected()){status('Conecta la terminal para usar los atajos',false,true);clearMobileModifiers();return}
      toggleMobileModifier(modifier);
      return;
    }
    const key=button.dataset.terminalMobileKey||'';
    if(key==='keyboard'){toggleMobileKeyboard();return}
    if(!isConnected()){
      status('Conecta la terminal para usar los atajos',false,true);
      clearMobileModifiers();
      return;
    }
    invalidateTerminalCommandTracking();
    if(Object.prototype.hasOwnProperty.call(button.dataset,'terminalMobileText')){
      sendBytes(applyMobileModifiersToInput(button.dataset.terminalMobileText||''));
    }else{
      const sequence=mobileKeySequence(key);
      if(sequence)sendBytes(sequence);
    }
    if(term)term.focus();
  }
  function syncMobileKeyboardLayout(){
    const bar=el('terminalMobileKeys');
    if(!bar||!mobileKeysEnabled)return;
    const textarea=terminalTextarea();
    const focused=!!(textarea&&document.activeElement===textarea);
    const keyboardButton=bar.querySelector('[data-terminal-mobile-key="keyboard"]');
    if(keyboardButton)keyboardButton.setAttribute('aria-pressed',String(focused));
    const body=bar.parentElement;
    const shouldFloat=focused&&window.innerWidth<=960;
    bar.classList.toggle('keyboard-open',shouldFloat);
    if(shouldFloat){
      const vv=window.visualViewport;
      const layoutHeight=document.documentElement.clientHeight||window.innerHeight;
      const viewportBottom=vv?vv.offsetTop+vv.height:layoutHeight;
      const keyboardOffset=Math.max(0,layoutHeight-viewportBottom);
      const box=el('terminalBox');
      const rect=box?box.getBoundingClientRect():{left:6,width:window.innerWidth-12};
      const left=Math.max(6,Math.min(rect.left,window.innerWidth-6));
      const width=Math.max(0,Math.min(rect.width,window.innerWidth-left-6));
      bar.style.setProperty('--terminal-mobile-keyboard-offset',keyboardOffset+'px');
      bar.style.left=left+'px';
      bar.style.width=width+'px';
      if(body){
        body.classList.add('mobile-keys-floating');
        body.style.setProperty('--terminal-mobile-keys-reserved',(bar.offsetHeight+8)+'px');
      }
    }else{
      bar.style.removeProperty('--terminal-mobile-keyboard-offset');
      bar.style.removeProperty('left');
      bar.style.removeProperty('width');
      if(body){
        body.classList.remove('mobile-keys-floating');
        body.style.removeProperty('--terminal-mobile-keys-reserved');
      }
    }
    if(mobileKeyboardFloating!==shouldFloat){
      mobileKeyboardFloating=shouldFloat;
      setTimeout(()=>{fitTerminal();queueResize()},20);
    }
  }
  function queueMobileKeyboardLayout(){
    clearTimeout(mobileLayoutTimer);
    mobileLayoutTimer=setTimeout(syncMobileKeyboardLayout,30);
  }
  function installMobileTerminalKeys(){
    const bar=el('terminalMobileKeys');
    if(!bar||!isAndroidTouchDevice())return;
    mobileKeysEnabled=true;
    bar.classList.add('is-enabled');
    bar.setAttribute('aria-hidden','false');
    bar.addEventListener('click',event=>{
      const button=event.target.closest('button[data-terminal-mobile-key],button[data-terminal-mobile-modifier],button[data-terminal-mobile-text]');
      if(!button||!bar.contains(button))return;
      event.preventDefault();
      handleMobileTerminalButton(button);
    });
    const textarea=terminalTextarea();
    if(textarea){
      textarea.addEventListener('focus',queueMobileKeyboardLayout);
      textarea.addEventListener('blur',queueMobileKeyboardLayout);
    }
    if(window.visualViewport){
      window.visualViewport.addEventListener('resize',queueMobileKeyboardLayout);
      window.visualViewport.addEventListener('scroll',queueMobileKeyboardLayout);
    }
    window.addEventListener('orientationchange',queueMobileKeyboardLayout);
    window.addEventListener('resize',queueMobileKeyboardLayout);
    updateMobileModifierButtons();
    queueMobileKeyboardLayout();
  }
  function closeSocket(socket,reason){
    if(!socket)return;
    const close=()=>{try{socket.close(1000,reason||'cerrada')}catch{}};
    if(socket.readyState===WebSocket.CONNECTING){
      socket.addEventListener('open',close,{once:true});
    }
    close();
  }
  function retireCurrentSocket(reason){
    stopCWDTracking();
    const socket=ws;
    if(!socket)return null;
    ws=null;
    socketSerial++;
    closeSocket(socket,reason);
    return socket;
  }
  function sendControl(type,payload){
    if(!ws||ws.readyState!==WebSocket.OPEN)return false;
    ws.send(JSON.stringify(Object.assign({pangoliteTerminal:true,type:type},payload||{})));
    return true;
  }
  function stopCWDTracking(){if(cwdTimer){clearInterval(cwdTimer);cwdTimer=null}}
  function requestTerminalCWD(){if(isConnected())sendControl('cwd.request')}
  function startCWDTracking(){
    stopCWDTracking();
    requestTerminalCWD();
    cwdTimer=setInterval(requestTerminalCWD,2500);
  }
  function refreshCWDAfterCommand(){
    setTimeout(requestTerminalCWD,180);
    setTimeout(requestTerminalCWD,700);
  }
  function terminalUploadID(){
    if(window.crypto&&typeof window.crypto.randomUUID==='function')return 'up_'+window.crypto.randomUUID().replaceAll('-','_');
    return 'up_'+Date.now().toString(36)+'_'+Math.random().toString(36).slice(2,14);
  }
  function formatTerminalBytes(value){
    let n=Math.max(0,Number(value)||0);
    const units=['B','KB','MB','GB','TB'];
    let i=0;
    while(n>=1024&&i<units.length-1){n/=1024;i++}
    return (i===0?Math.round(n):n.toFixed(n>=10?1:2))+' '+units[i];
  }
  function createTransferRow(file,batchPosition){
    const box=el('terminalTransfers');
    if(!box)return null;
    box.replaceChildren();
    box.classList.remove('d-none');
    const row=document.createElement('div');
    row.className='terminal-transfer';
    const eyebrow=document.createElement('div');eyebrow.className='terminal-transfer-eyebrow';
    const title=document.createElement('span');title.textContent='Subiendo archivo';
    const batch=document.createElement('span');batch.className='terminal-transfer-batch';
    eyebrow.append(title,batch);
    const head=document.createElement('div');head.className='terminal-transfer-head';
    const name=document.createElement('span');name.className='terminal-transfer-name';name.textContent=file.name;
    const percent=document.createElement('span');percent.className='terminal-transfer-percent';percent.textContent='0%';
    head.append(name,percent);
    const track=document.createElement('div');track.className='terminal-transfer-track';
    const bar=document.createElement('div');bar.className='terminal-transfer-bar';track.appendChild(bar);
    const meta=document.createElement('div');meta.className='terminal-transfer-meta';meta.textContent='Preparando · '+formatTerminalBytes(file.size);
    row.append(eyebrow,head,track,meta);box.appendChild(row);
    return {row,percent,bar,meta,batch,batchPosition};
  }
  function refreshTransferBatch(state){
    if(!state||!state.ui||!state.ui.batch)return;
    const total=Math.max(1,terminalUploadBatch.total||1);
    state.ui.batch.textContent=total>1?(state.ui.batchPosition+' de '+total):'';
  }
  function paintTransfer(state,doneBytes,statusText,statusClass){
    if(!state||!state.ui)return;
    refreshTransferBatch(state);
    const total=Math.max(0,Number(state.file.size)||0);
    const done=Math.max(0,Math.min(total,Number(doneBytes)||0));
    const pct=total===0?100:Math.min(100,Math.round(done*100/total));
    state.ui.percent.textContent=pct+'%';
    state.ui.bar.style.width=pct+'%';
    state.ui.meta.textContent=statusText||formatTerminalBytes(done)+' / '+formatTerminalBytes(total);
    state.ui.row.classList.toggle('done',statusClass==='done');
    state.ui.row.classList.toggle('error',statusClass==='error');
  }
  function hideTransferWindow(){
    const box=el('terminalTransfers');
    if(!box)return;
    box.replaceChildren();
    box.classList.add('d-none');
  }
  function showTerminalUploadAlert(message,bad=false){
    const box=el('terminalTransferAlerts');
    if(!box)return;
    while(box.children.length>=3)box.firstElementChild.remove();
    const alert=document.createElement('div');
    alert.className='terminal-transfer-alert '+(bad?'error':'success');
    const icon=document.createElement('i');icon.className='bi '+(bad?'bi-exclamation-triangle':'bi-check-circle');
    const text=document.createElement('span');text.textContent=message;
    alert.append(icon,text);box.appendChild(alert);
    requestAnimationFrame(()=>alert.classList.add('visible'));
    setTimeout(()=>{alert.classList.remove('visible');setTimeout(()=>alert.remove(),220)},3200);
  }
  function settleUploadWaiter(state,kind,value,error){
    if(!state)return;
    const waiter=state[kind+'Waiter'];
    if(!waiter)return;
    clearTimeout(waiter.timer);
    state[kind+'Waiter']=null;
    if(error)waiter.reject(error);else waiter.resolve(value);
  }
  function waitUploadSignal(state,kind,timeoutMs){
    return new Promise((resolve,reject)=>{
      const timer=setTimeout(()=>{state[kind+'Waiter']=null;reject(new Error('La terminal no confirmó la transferencia a tiempo'));},timeoutMs||30000);
      state[kind+'Waiter']={resolve,reject,timer};
    });
  }
  function terminalDownloadID(){
    if(window.crypto&&typeof window.crypto.randomUUID==='function')return 'down_'+window.crypto.randomUUID().replaceAll('-','_');
    return 'down_'+Date.now().toString(36)+'_'+Math.random().toString(36).slice(2,14);
  }
  function terminalUsesAlternateBuffer(){
    try{return !!(term&&term.buffer&&term.buffer.active&&term.buffer.active.type==='alternate')}catch{return false}
  }
  function parseTerminalDownloadCommand(line){
    const match=/^\s*download(?:\s+(.*))?\s*$/.exec(String(line||''));
    if(!match)return {matched:false};
    let path=String(match[1]||'').trim();
    if(!path)return {matched:true,error:'Uso: download archivo.ext o download directorio'};
    if((path.startsWith('"')&&path.endsWith('"'))||(path.startsWith("'")&&path.endsWith("'")))path=path.slice(1,-1);
    path=path.replace(/\\ /g,' ').trim();
    if(!path)return {matched:true,error:'Indica el archivo o directorio a descargar'};
    return {matched:true,path:path};
  }
  function resetTerminalCommandTracking(){terminalCommandBuffer='';terminalCommandTracking=true}
  function invalidateTerminalCommandTracking(){terminalCommandBuffer='';terminalCommandTracking=false}
  function requestTerminalDownload(path){
    if(!isConnected()){showTerminalUploadAlert('Conecta la terminal antes de descargar',true);return}
    const id=terminalDownloadID();
    const state={id,path,target:connectedTarget,timer:null};
    state.timer=setTimeout(()=>{
      if(!terminalDownloadStates.has(id))return;
      terminalDownloadStates.delete(id);
      showTerminalUploadAlert('La terminal tardó demasiado en preparar la descarga',true);
    },60000);
    terminalDownloadStates.set(id,state);
    showTerminalUploadAlert('Preparando descarga: '+path);
    if(!sendControl('download.request',{downloadId:id,path:path})){
      clearTimeout(state.timer);terminalDownloadStates.delete(id);
      showTerminalUploadAlert('La terminal se desconectó antes de preparar la descarga',true);
    }
  }
  async function createTerminalDownloadFromOffer(message,state){
    const payload={target:state.target,path:message.path||'',name:message.name||'',kind:message.kind||'',size:Number(message.size)||0};
    let result;
    if(typeof api==='function'){
      result=await api('/api/terminal/downloads',{method:'POST',body:JSON.stringify(payload)});
    }else{
      const headers={'Content-Type':'application/json'};
      try{if(typeof csrf==='string'&&csrf)headers['X-CSRF-Token']=csrf}catch{}
      const response=await fetch('/api/terminal/downloads',{method:'POST',headers,body:JSON.stringify(payload)});
      const text=await response.text();
      try{result=text?JSON.parse(text):{}}catch{result={}}
      if(!response.ok)throw new Error(result.error||'No se pudo preparar la descarga');
    }
    if(!result||!result.url)throw new Error('Pangolite no devolvió una URL de descarga');
    const anchor=document.createElement('a');
    anchor.href=result.url;
    anchor.rel='noopener';
    anchor.style.display='none';
    document.body.appendChild(anchor);
    anchor.click();
    setTimeout(()=>anchor.remove(),1000);
    showTerminalUploadAlert((message.kind==='directory'?'ZIP preparado: ':'Descarga iniciada: ')+(message.name||state.path));
  }
  function cancelTerminalDownloads(message){
    terminalDownloadStates.forEach(state=>{clearTimeout(state.timer)});
    if(terminalDownloadStates.size&&message)showTerminalUploadAlert(message,true);
    terminalDownloadStates.clear();
    resetTerminalCommandTracking();
  }
  function handleTerminalInputData(data){
    data=String(data||'');
    if(!data)return;
    let output='';
    let submitted=false;
    const downloads=[];
    for(const ch of data){
      if(ch==='\r'||ch==='\n'){
        submitted=true;
        const parsed=terminalCommandTracking&&!terminalUsesAlternateBuffer()?parseTerminalDownloadCommand(terminalCommandBuffer):{matched:false};
        if(parsed.matched){
          output+='\x15';
          if(parsed.error)showTerminalUploadAlert(parsed.error,true);else downloads.push(parsed.path);
        }else{
          output+=ch;
        }
        resetTerminalCommandTracking();
        continue;
      }
      output+=ch;
      if(!terminalCommandTracking)continue;
      if(ch==='\x7f'||ch==='\b'){
        terminalCommandBuffer=terminalCommandBuffer.slice(0,-1);
      }else if(ch==='\x15'||ch==='\x03'){
        terminalCommandBuffer='';
      }else if(ch==='\x1b'||(ch<' '&&ch!=='\t')){
        invalidateTerminalCommandTracking();
      }else{
        terminalCommandBuffer+=ch;
        if(terminalCommandBuffer.length>4096)invalidateTerminalCommandTracking();
      }
    }
    if(output)sendBytes(output);
    downloads.forEach(path=>requestTerminalDownload(path));
    if(submitted)refreshCWDAfterCommand();
  }
  function handleTerminalControlMessage(data){
    let message;
    try{message=JSON.parse(data)}catch{return false}
    if(!message||message.pangoliteTerminal!==true)return false;
    const type=String(message.type||'');
    if(type==='cwd.update'){
      const path=String(message.path||'').trim();
      if(path&&connectedTarget)rememberTerminalDir(connectedTarget,path);
      return true;
    }
    if(type.startsWith('download.')){
      const state=terminalDownloadStates.get(String(message.downloadId||''));
      if(!state)return true;
      if(type==='download.offer'){
        clearTimeout(state.timer);
        terminalDownloadStates.delete(state.id);
        createTerminalDownloadFromOffer(message,state).catch(err=>showTerminalUploadAlert(err&&err.message?err.message:'No se pudo iniciar la descarga',true));
      }else if(type==='download.error'){
        clearTimeout(state.timer);
        terminalDownloadStates.delete(state.id);
        showTerminalUploadAlert(message.error||'No se pudo preparar la descarga',true);
      }
      return true;
    }
    if(!type.startsWith('upload.'))return false;
    const state=terminalUploadStates.get(String(message.uploadId||''));
    if(!state)return true;
    if(message.type==='upload.ready'){
      state.path=message.path||'';
      paintTransfer(state,0,'Subiendo a '+(state.path||'directorio actual'));
      settleUploadWaiter(state,'ready',message,false);
    }else if(message.type==='upload.progress'){
      state.confirmed=Math.max(state.confirmed||0,Number(message.written)||0);
    }else if(message.type==='upload.done'){
      state.path=message.path||state.path||'';
      paintTransfer(state,state.file.size,'Completado · '+(state.path||state.file.name),'done');
      settleUploadWaiter(state,'done',message,false);
    }else if(message.type==='upload.error'){
      const error=new Error(message.error||'No se pudo subir el archivo');
      state.error=error;
      paintTransfer(state,state.confirmed||state.sent||0,error.message,'error');
      settleUploadWaiter(state,'ready',null,error);
      settleUploadWaiter(state,'done',null,error);
    }
    return true;
  }
  function encodeTerminalUploadChunk(uploadId,data){
    const header=encoder.encode(uploadId+' '+data.byteLength+'\n');
    const out=new Uint8Array(terminalUploadPrefix.length+header.length+data.byteLength);
    out.set(terminalUploadPrefix,0);
    out.set(header,terminalUploadPrefix.length);
    out.set(data,terminalUploadPrefix.length+header.length);
    return out;
  }
  function sleepTerminal(ms){return new Promise(resolve=>setTimeout(resolve,ms))}
  async function uploadTerminalFile(file){
    if(!isConnected())throw new Error('Conecta la terminal antes de subir archivos');
    const id=terminalUploadID();
    const batchPosition=terminalUploadBatch.completed+terminalUploadBatch.failed+1;
    const state={id,file,ui:createTransferRow(file,batchPosition),sent:0,confirmed:0,path:'',error:null};
    terminalUploadStates.set(id,state);
    try{
      const ready=waitUploadSignal(state,'ready',30000);
      if(!sendControl('upload.start',{uploadId:id,name:file.name,size:file.size}))throw new Error('La terminal se desconectó');
      await ready;
      let offset=0;
      while(offset<file.size){
        if(state.error)throw state.error;
        if(!isConnected())throw new Error('La terminal se desconectó durante la subida');
        while(ws&&ws.bufferedAmount>512*1024){await sleepTerminal(20);if(state.error)throw state.error;if(!isConnected())throw new Error('La terminal se desconectó durante la subida')}
        const end=Math.min(file.size,offset+terminalUploadChunkSize);
        const chunk=new Uint8Array(await file.slice(offset,end).arrayBuffer());
        if(state.error)throw state.error;
        ws.send(encodeTerminalUploadChunk(id,chunk));
        offset=end;
        state.sent=offset;
        paintTransfer(state,offset,formatTerminalBytes(offset)+' / '+formatTerminalBytes(file.size));
      }
      if(state.error)throw state.error;
      const done=waitUploadSignal(state,'done',60000);
      if(!sendControl('upload.finish',{uploadId:id}))throw new Error('La terminal se desconectó');
      await done;
      showTerminalUploadAlert('Subido: '+file.name+(state.path?' · '+state.path:''));
    }catch(err){
      if(isConnected())sendControl('upload.cancel',{uploadId:id});
      paintTransfer(state,state.sent||0,err.message||'Transferencia fallida','error');
      throw err;
    }finally{
      terminalUploadStates.delete(id);
    }
  }
  async function runTerminalUploadQueue(){
    if(terminalUploadRunning)return;
    terminalUploadRunning=true;
    try{
      while(terminalUploadQueue.length){
        const file=terminalUploadQueue.shift();
        try{
          await uploadTerminalFile(file);
          terminalUploadBatch.completed++;
        }catch(err){
          terminalUploadBatch.failed++;
          showTerminalUploadAlert('Error al subir '+file.name+': '+(err&&err.message?err.message:'transferencia fallida'),true);
          if(isConnected())status(connectedTarget==='local'?'Local conectado':'Cliente conectado',true,false);else status('Error al subir archivo',false,true);
        }
      }
    }finally{
      hideTransferWindow();
      if(terminalUploadBatch.total>1){
        const ok=terminalUploadBatch.completed,failed=terminalUploadBatch.failed;
        showTerminalUploadAlert(failed?('Lote terminado: '+ok+' subidos, '+failed+' con error'):('Lote terminado: '+ok+' archivos subidos'),failed>0);
      }
      terminalUploadBatch={total:0,completed:0,failed:0};
      terminalUploadRunning=false;
      if(term)term.focus();
    }
  }
  function queueTerminalFiles(files){
    const list=Array.from(files||[]).filter(file=>file&&typeof file.name==='string');
    if(!list.length)return;
    if(!isConnected()){status('Conecta la terminal antes de subir archivos',false,true);return}
    terminalUploadBatch.total+=list.length;
    terminalUploadQueue.push(...list);
    runTerminalUploadQueue();
  }
  function cancelTerminalUploads(message){
    terminalUploadQueue=[];
    terminalUploadBatch.total=terminalUploadBatch.completed+terminalUploadBatch.failed+terminalUploadStates.size;
    const error=new Error(message||'Transferencia cancelada');
    terminalUploadStates.forEach(state=>{
      settleUploadWaiter(state,'ready',null,error);
      settleUploadWaiter(state,'done',null,error);
      paintTransfer(state,state.sent||0,error.message,'error');
    });
  }
  function installTerminalFileTransfer(box){
    const input=el('terminalFileInput'),button=el('terminalUploadBtn'),overlay=el('terminalDropOverlay');
    if(button&&input)button.addEventListener('click',()=>{if(isConnected())input.click();else status('Conecta la terminal antes de subir archivos',false,true)});
    if(input)input.addEventListener('change',()=>{queueTerminalFiles(input.files);input.value=''});
    if(!box)return;
    let dragDepth=0;
    const hasFiles=event=>Array.from((event.dataTransfer&&event.dataTransfer.types)||[]).includes('Files');
    box.addEventListener('dragenter',event=>{if(!hasFiles(event))return;event.preventDefault();dragDepth++;if(overlay&&isConnected())overlay.classList.add('visible')});
    box.addEventListener('dragover',event=>{if(!hasFiles(event))return;event.preventDefault();if(event.dataTransfer)event.dataTransfer.dropEffect=isConnected()?'copy':'none'});
    box.addEventListener('dragleave',event=>{if(!hasFiles(event))return;dragDepth=Math.max(0,dragDepth-1);if(!dragDepth&&overlay)overlay.classList.remove('visible')});
    box.addEventListener('drop',event=>{if(!hasFiles(event))return;event.preventDefault();dragDepth=0;if(overlay)overlay.classList.remove('visible');queueTerminalFiles(event.dataTransfer&&event.dataTransfer.files)});
  }
  function ensureTerminal(){
    const box=el('terminalBox');
    if(!box)return false;
    if(!window.Terminal){
      box.classList.add('d-none');
      const fb=el('terminalFallback');
      if(fb)fb.classList.remove('d-none');
      return false;
    }
    if(term)return true;
    term=new Terminal({
      cursorBlink:true,
      fontSize:14,
      fontFamily:'Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      convertEol:false,
      scrollback:5000,
      rightClickSelectsWord:true,
      macOptionIsMeta:true,
      theme:themes[localStorage.getItem('pangolite.terminal.theme')||'black']||themes.black
    });
    if(window.FitAddon&&window.FitAddon.FitAddon){
      fit=new FitAddon.FitAddon();
      term.loadAddon(fit);
    }
    term.open(box);
    installTerminalContextMenu(box);
    installTerminalFileTransfer(box);
    installMobileTerminalKeys();
    term.attachCustomKeyEventHandler(handleTerminalKey);
    term.onData(data=>handleTerminalInputData(applyMobileModifiersToInput(data)));
    term.onResize(()=>queueResize());
    fitTerminal();
    window.addEventListener('resize',()=>{fitTerminal();queueResize()});
    document.addEventListener('fullscreenchange',()=>{
      updateFullscreenButton();
      if(isTerminalFullscreen())lockTerminalEscape();else unlockTerminalEscape();
      setTimeout(()=>{fitTerminal();sendResize();if(term)term.focus()},80);
    });
    document.addEventListener('keydown',handleDocumentTerminalKey,true);
    return true;
  }
  function fitTerminal(){
    try{if(fit)fit.fit()}catch{}
  }
  function queueResize(){
    clearTimeout(resizeTimer);
    resizeTimer=setTimeout(()=>sendResize(),90);
  }
  function sendResize(){
    if(!ws||ws.readyState!==WebSocket.OPEN||!term)return;
    sendControl('resize',{cols:term.cols||80,rows:term.rows||24});
  }
  function selectedTheme(){return (el('terminalTheme')&&el('terminalTheme').value)||'black'}
  function applyTheme(){
    if(!term)return;
    const name=selectedTheme();
    localStorage.setItem('pangolite.terminal.theme',name);
    term.options.theme=themes[name]||themes.black;
  }
  function targetSocketPath(target){
    if(target==='local')return '/api/terminal/local';
    if(target.startsWith('agent:'))return '/api/terminal/agents/'+encodeURIComponent(target.slice(6));
    return '';
  }
  function buildSizeQuery(workingDir){
    if(!term)return '';
    const params=new URLSearchParams();
    params.set('cols',String(term.cols||80));
    params.set('rows',String(term.rows||24));
    workingDir=String(workingDir||'').trim();
    if(workingDir)params.set('cwd',workingDir);
    return '?'+params.toString();
  }
  function currentTargetOS(){
    const target=currentTerminalTarget();
    if(target==='local'){
      let boot={};
      try{boot=appBoot||{}}catch{}
      return String(boot.serverOS||'').toLowerCase();
    }
    if(!target.startsWith('agent:'))return '';
    const id=target.slice(6);
    let list=[];
    try{list=Array.isArray(agents)?agents:[]}catch{}
    const agent=list.find(a=>a&&a.id===id);
    return String((agent&&agent.os)||'').toLowerCase();
  }
  function writeTerminalNotice(message,bad){
    if(!term)return;
    term.clear();
    term.writeln((bad?'\x1b[31m':'\x1b[90m')+message+'\x1b[0m');
  }
  function connectTerminal(workingDir=''){
    if(!ensureTerminal())return;
    clearMobileModifiers();
    resetTerminalCommandTracking();
    retireCurrentSocket('reemplazada');
    applyTheme();
    const target=(el('terminalTarget')&&el('terminalTarget').value)||'local';
    const targetOS=currentTargetOS();
    if(targetOS==='windows'){
      connectedTarget='';
      setButtons('idle');
      status('Windows no confiable',false,true);
      writeTerminalNotice(windowsWarning,true);
      setOverlay('warning','Terminal Windows no confiable',windowsWarning,'Cerrar aviso');
      return;
    }
    const path=targetSocketPath(target);
    if(!path){status('Destino inválido',false,true);showReconnectOverlay('Destino inválido','Selecciona un destino válido para la consola.');return}
    connectedTarget=target;
    if(term){
      term.clear();
      term.writeln('\x1b[90mIniciando conexión de consola...\x1b[0m');
    }
    status('Conectando...',false,false);
    setButtons('connecting');
    setOverlay('connecting','Conectando consola','Preparando sesión remota. Esto puede tardar unos segundos.',null,true);
    const serial=++socketSerial;
    const socket=new WebSocket(wsURL(path,buildSizeQuery(workingDir)));
    const decoder=new TextDecoder();
    ws=socket;
    socket.binaryType='arraybuffer';
    socket.onopen=()=>{
      if(ws!==socket||serial!==socketSerial){closeSocket(socket,'reemplazada');return}
      status(target==='local'?'Local conectado':'Cliente conectado',true,false);
      setButtons('connected');
      hideOverlay();
      term.focus();
      fitTerminal();
      sendResize();
      startCWDTracking();
    };
    socket.onmessage=(event)=>{
      if(ws!==socket||serial!==socketSerial||!term)return;
      if(typeof event.data==='string'){if(!handleTerminalControlMessage(event.data))term.write(event.data);}
      else term.write(decoder.decode(new Uint8Array(event.data),{stream:true}));
    };
    socket.onerror=()=>{
      if(ws===socket&&serial===socketSerial)status('Error de conexión',false,true);
    };
    socket.onclose=()=>{
      if(ws!==socket||serial!==socketSerial)return;
      const decoderTail=decoder.decode();
      if(decoderTail&&term)term.write(decoderTail);
      const closedTarget=connectedTarget||target;
      const reconnectDir=lastDirForTarget(closedTarget);
      stopCWDTracking();
      ws=null;
      cancelTerminalUploads('La terminal se desconectó durante la transferencia');
      cancelTerminalDownloads('La terminal se desconectó mientras preparaba una descarga');
      clearMobileModifiers();
      setButtons('idle');
      connectedTarget='';
      status('Sesión cerrada',false,true);
      if(term)term.writeln('\r\n\x1b[31mSesión cerrada.\x1b[0m');
      showReconnectOverlay('Conexión cerrada','La consola se cerró o el cliente se desconectó. Al reconectar se conservará tu directorio de trabajo.',reconnectDir);
    };
  }
  function disconnectTerminal(writeMessage=true){
    const previousTarget=connectedTarget||currentTerminalTarget();
    requestTerminalCWD();
    cancelTerminalUploads('Transferencia cancelada al desconectar la terminal');
    cancelTerminalDownloads('Descarga cancelada al desconectar la terminal');
    clearMobileModifiers();
    const socket=retireCurrentSocket('usuario');
    connectedTarget='';
    setButtons('idle');
    status('Desconectado',false,false);
    if(!showResumeOverlay(lastDirForTarget(previousTarget)))setOverlay('idle','Aún no conectado','La consola está cerrada. Puedes volver a conectar cuando lo necesites.','Conectar');
    if(socket&&writeMessage&&term)term.writeln('\r\n\x1b[90mDesconectado por el usuario.\x1b[0m');
  }
  async function copySelection(){
    if(!term)return false;
    const text=term.getSelection();
    if(!text){term.focus();return false}
    try{
      if(typeof copyText==='function')await copyText(text);else await navigator.clipboard.writeText(text);
      status('Selección copiada',true,false);
      setTimeout(()=>{if(ws)status(connectedTarget==='local'?'Local conectado':'Cliente conectado',true,false)},900);
      return true;
    }catch(err){
      status('No se pudo copiar',false,true);
      return false;
    }finally{
      term.focus();
    }
  }
  async function pasteFromClipboard(){
    if(!term)return false;
    try{
      const text=await navigator.clipboard.readText();
      if(!text)return true;
      clearMobileModifiers();
      if(!isConnected()){
        status('Conecta la terminal antes de pegar',false,true);
        return false;
      }
      term.paste(text);
      return true;
    }catch(err){
      status('Pega con Ctrl+V o Shift+Insert',false,true);
      return false;
    }finally{
      term.focus();
    }
  }
  function handleTerminalKey(event){
    const key=(event.key||'').toLowerCase();
    if(event.key==='Escape'&&isTerminalFullscreen()&&isConnected()){
      event.preventDefault();
      event.stopPropagation();
      sendEscapeToTerminal();
      return false;
    }
    if((event.ctrlKey||event.metaKey)&&!event.altKey&&key==='v'){
      return true;
    }
    if(event.shiftKey&&event.key==='Insert'){
      event.preventDefault();
      pasteFromClipboard();
      return false;
    }
    if((event.ctrlKey||event.metaKey)&&event.shiftKey&&key==='c'){
      event.preventDefault();
      copySelection();
      return false;
    }
    if(event.ctrlKey&&event.key==='Insert'){
      event.preventDefault();
      copySelection();
      return false;
    }
    return true;
  }
  function handleDocumentTerminalKey(event){
    if(event.key!=='Escape'||!isTerminalFullscreen()||contextMenuOpen()||!isConnected())return;
    const card=el('terminalCard');
    if(card&&!card.contains(event.target))return;
    event.preventDefault();
    event.stopPropagation();
    sendEscapeToTerminal();
  }
  function installTerminalContextMenu(box){
    const menu=el('terminalContextMenu');
    if(!menu)return;
    const close=()=>{menu.classList.remove('open');menu.setAttribute('aria-hidden','true')};
    box.addEventListener('contextmenu',event=>{
      event.preventDefault();
      const maxX=window.innerWidth-230;
      const maxY=window.innerHeight-150;
      menu.style.left=Math.max(8,Math.min(event.clientX,maxX))+'px';
      menu.style.top=Math.max(8,Math.min(event.clientY,maxY))+'px';
      menu.classList.add('open');
      menu.setAttribute('aria-hidden','false');
    });
    menu.addEventListener('click',event=>{
      const btn=event.target.closest('button[data-terminal-action]');
      if(!btn)return;
      const action=btn.dataset.terminalAction;
      close();
      if(action==='copy')copySelection();
      if(action==='paste')pasteFromClipboard();
      if(action==='reset')resetTerminalView();
    });
    document.addEventListener('click',event=>{if(!menu.contains(event.target))close()});
    document.addEventListener('keydown',event=>{if(event.key==='Escape'&&menu.classList.contains('open')){event.stopPropagation();close()}});
  }
  function updateFullscreenButton(){
    const card=el('terminalCard');
    const btn=el('terminalFullscreenBtn');
    if(!card||!btn)return;
    const active=document.fullscreenElement===card||card.classList.contains('terminal-fullscreen-fallback');
    btn.setAttribute('aria-pressed',String(active));
    btn.innerHTML=active?'<i class="bi bi-fullscreen-exit"></i> <span>Salir</span>':'<i class="bi bi-fullscreen"></i> <span>Pantalla completa</span>';
  }
  async function toggleFullscreen(){
    const card=el('terminalCard');
    if(!card)return;
    if(document.fullscreenElement===card){
      unlockTerminalEscape();
      try{await document.exitFullscreen()}catch{}
    }
    card.classList.toggle('terminal-fullscreen-fallback');
    updateFullscreenButton();
    setTimeout(()=>{fitTerminal();sendResize();if(term)term.focus()},100);
  }
  function positionTerminalSettings(){
    const button=el('terminalSettingsBtn'),popover=el('terminalSettingsPopover');
    if(!button||!popover||!popover.classList.contains('open'))return;
    const rect=button.getBoundingClientRect();
    const width=Math.min(280,window.innerWidth-16);
    popover.style.width=width+'px';
    popover.style.visibility='hidden';
    let left=Math.max(8,Math.min(rect.right-width,window.innerWidth-width-8));
    let top=rect.bottom+8;
    const height=popover.offsetHeight||130;
    if(top+height>window.innerHeight-8)top=Math.max(8,rect.top-height-8);
    popover.style.left=left+'px';
    popover.style.top=top+'px';
    popover.style.visibility='';
  }
  function closeTerminalSettings(){
    const button=el('terminalSettingsBtn'),popover=el('terminalSettingsPopover');
    if(!popover)return;
    popover.classList.remove('open');popover.setAttribute('aria-hidden','true');
    if(button)button.setAttribute('aria-expanded','false');
  }
  function toggleTerminalSettings(event){
    if(event){event.preventDefault();event.stopPropagation()}
    const button=el('terminalSettingsBtn'),popover=el('terminalSettingsPopover');
    if(!button||!popover)return;
    const open=!popover.classList.contains('open');
    closeTerminalSettings();
    try{if(typeof closeActionDropdowns==='function')closeActionDropdowns()}catch{}
    if(open){popover.classList.add('open');popover.setAttribute('aria-hidden','false');button.setAttribute('aria-expanded','true');positionTerminalSettings()}
  }
  function installTerminalSettings(){
    const widget=el('terminalSettingsWidget'),button=el('terminalSettingsBtn'),popover=el('terminalSettingsPopover');
    if(!widget||!button||!popover)return;
    button.addEventListener('click',toggleTerminalSettings);
    document.addEventListener('click',event=>{if(!widget.contains(event.target))closeTerminalSettings()});
    document.addEventListener('keydown',event=>{if(event.key==='Escape'&&popover.classList.contains('open')){event.stopPropagation();closeTerminalSettings()}});
    window.addEventListener('resize',()=>{if(popover.classList.contains('open'))positionTerminalSettings()});
    window.addEventListener('scroll',()=>closeTerminalSettings(),true);
  }
  async function initTerminal(){
    if(!el('terminalBox'))return;
    let requestedTargetAvailable=true;
    const target=el('terminalTarget');
    if(target){
      const params=new URLSearchParams(location.search);
      const agentId=params.get('agentId');
      if(agentId){
        const value='agent:'+agentId;
        const option=Array.from(target.options).find(o=>o.value===value&&!o.disabled);
        if(option)target.value=value;
        else requestedTargetAvailable=false;
      }
      target.addEventListener('change',()=>{if(!ws&&!showResumeOverlay(lastDirForTarget(currentTerminalTarget())))showIdleOverlay()});
    }
    const theme=el('terminalTheme');
    if(theme){
      theme.value=localStorage.getItem('pangolite.terminal.theme')||'black';
      theme.addEventListener('change',applyTheme);
    }
    installTerminalSettings();
    const connect=el('terminalConnectBtn');
    const disconnect=el('terminalDisconnectBtn');
    const fullscreen=el('terminalFullscreenBtn');
    const overlayButton=el('terminalOverlayButton');
    if(connect)connect.addEventListener('click',requestTerminalConnection);
    if(disconnect)disconnect.addEventListener('click',()=>disconnectTerminal(true));
    if(fullscreen)fullscreen.addEventListener('click',toggleFullscreen);
    const overlaySecondary=el('terminalOverlaySecondaryButton');
    if(overlayButton)overlayButton.addEventListener('click',()=>{
      if(currentTargetOS()==='windows'){hideOverlay();if(term)term.focus();return;}
      if(overlayAction==='resume'||overlayAction==='reconnect'){
        const dir=overlayWorkingDir;
        overlayAction='connect';overlayWorkingDir='';
        connectTerminal(dir);
        return;
      }
      requestTerminalConnection();
    });
    if(overlaySecondary)overlaySecondary.addEventListener('click',()=>{
      overlayAction='connect';overlayWorkingDir='';
      connectTerminal('');
    });
    hydrateTerminalLastDirs();
    const terminalReady=ensureTerminal();
    setButtons('idle');
    showIdleOverlay();
    await refreshTerminalLastDirs();
    const params=new URLSearchParams(location.search);
    if(!requestedTargetAvailable){
      status('Cliente no disponible',false,true);
      setOverlay('warning','Cliente no disponible','El cliente solicitado está offline, inactivo o usa un sistema no compatible. Regresa a Conexiones SSH y selecciona otro destino.',null,false);
      return;
    }
    const previousDir=lastDirForTarget(currentTerminalTarget());
    if(previousDir){
      showResumeOverlay(previousDir);
      return;
    }
    showIdleOverlay();
    if(terminalReady&&params.get('autoconnect')==='1'){
      setTimeout(()=>connectTerminal(''),80);
    }
  }
  document.addEventListener('DOMContentLoaded',()=>{initTerminal().catch(err=>console.warn('terminal init',err))});
})();
