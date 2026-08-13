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
  const encoder=new TextEncoder();
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
    const connect=el('terminalConnectBtn'), disconnect=el('terminalDisconnectBtn'), target=el('terminalTarget');
    const busy=mode==='connecting'||mode==='connected';
    if(connect)connect.disabled=busy;
    if(disconnect)disconnect.disabled=!busy;
    if(target)target.disabled=busy;
  }
  function setOverlay(kind,title,text,buttonText,loading){
    const overlay=el('terminalOverlay');
    if(!overlay)return;
    const spinner=el('terminalOverlaySpinner');
    const titleEl=el('terminalOverlayTitle');
    const textEl=el('terminalOverlayText');
    const button=el('terminalOverlayButton');
    overlay.className='terminal-overlay '+(kind||'idle');
    overlay.classList.remove('d-none');
    if(spinner)spinner.classList.toggle('d-none',!loading);
    if(titleEl)titleEl.textContent=title||'';
    if(textEl)textEl.textContent=text||'';
    if(button){
      button.classList.toggle('d-none',!buttonText);
      const span=button.querySelector('span');
      if(span)span.textContent=buttonText||'';
      else button.textContent=buttonText||'';
      button.disabled=!!loading;
    }
  }
  function hideOverlay(){
    const overlay=el('terminalOverlay');
    if(overlay)overlay.classList.add('d-none');
  }
  function showIdleOverlay(){
    if(currentTargetOS()==='windows'){
      status('Windows no confiable',false,true);
      setOverlay('warning','Terminal Windows no confiable',windowsWarning,'Cerrar aviso');
      return;
    }
    status('Desconectado',false,false);
    setOverlay('idle','Aún no conectado','Selecciona un destino y presiona Conectar para abrir una consola.','Conectar');
  }
  function showReconnectOverlay(title,text){
    setOverlay('bad',title||'Conexión cerrada',text||'La sesión de consola se cerró o el cliente dejó de responder.','Reconectar');
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
    const socket=ws;
    if(!socket)return null;
    ws=null;
    socketSerial++;
    closeSocket(socket,reason);
    return socket;
  }
  function sendControl(type,payload){
    if(!ws||ws.readyState!==WebSocket.OPEN)return;
    ws.send(JSON.stringify(Object.assign({pangoliteTerminal:true,type:type},payload||{})));
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
    installMobileTerminalKeys();
    term.attachCustomKeyEventHandler(handleTerminalKey);
    term.onData(data=>sendBytes(applyMobileModifiersToInput(data)));
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
  function buildSizeQuery(){
    if(!term)return '';
    return '?cols='+encodeURIComponent(term.cols||80)+'&rows='+encodeURIComponent(term.rows||24);
  }
  function currentTargetOS(){
    const target=(el('terminalTarget')&&el('terminalTarget').value)||'local';
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
  function connectTerminal(){
    if(!ensureTerminal())return;
    clearMobileModifiers();
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
    const socket=new WebSocket(wsURL(path,buildSizeQuery()));
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
    };
    socket.onmessage=(event)=>{
      if(ws!==socket||serial!==socketSerial||!term)return;
      if(typeof event.data==='string')term.write(event.data);
      else term.write(decoder.decode(new Uint8Array(event.data),{stream:true}));
    };
    socket.onerror=()=>{
      if(ws===socket&&serial===socketSerial)status('Error de conexión',false,true);
    };
    socket.onclose=()=>{
      if(ws!==socket||serial!==socketSerial)return;
      const decoderTail=decoder.decode();
      if(decoderTail&&term)term.write(decoderTail);
      ws=null;
      clearMobileModifiers();
      setButtons('idle');
      connectedTarget='';
      status('Sesión cerrada',false,true);
      if(term)term.writeln('\r\n\x1b[31mSesión cerrada.\x1b[0m');
      showReconnectOverlay('Conexión cerrada','La consola se cerró o el cliente se desconectó. Presiona Reconectar para iniciar otra sesión.');
    };
  }
  function disconnectTerminal(writeMessage=true){
    clearMobileModifiers();
    const socket=retireCurrentSocket('usuario');
    connectedTarget='';
    setButtons('idle');
    status('Desconectado',false,false);
    setOverlay('idle','Aún no conectado','La consola está cerrada. Puedes volver a conectar cuando lo necesites.','Conectar');
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
  function initTerminal(){
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
      target.addEventListener('change',()=>{if(!ws)showIdleOverlay()});
    }
    const theme=el('terminalTheme');
    if(theme){
      theme.value=localStorage.getItem('pangolite.terminal.theme')||'black';
      theme.addEventListener('change',applyTheme);
    }
    const connect=el('terminalConnectBtn');
    const disconnect=el('terminalDisconnectBtn');
    const fullscreen=el('terminalFullscreenBtn');
    const overlayButton=el('terminalOverlayButton');
    if(connect)connect.addEventListener('click',connectTerminal);
    if(disconnect)disconnect.addEventListener('click',()=>disconnectTerminal(true));
    if(fullscreen)fullscreen.addEventListener('click',toggleFullscreen);
    if(overlayButton)overlayButton.addEventListener('click',()=>{
      if(currentTargetOS()==='windows'){hideOverlay();if(term)term.focus();return;}
      connectTerminal();
    });
    const terminalReady=ensureTerminal();
    setButtons('idle');
    showIdleOverlay();
    const params=new URLSearchParams(location.search);
    if(!requestedTargetAvailable){
      status('Cliente no disponible',false,true);
      setOverlay('warning','Cliente no disponible','El cliente solicitado está offline, inactivo o usa un sistema no compatible. Regresa a Conexiones SSH y selecciona otro destino.',null,false);
      return;
    }
    if(terminalReady&&params.get('autoconnect')==='1'){
      setTimeout(connectTerminal,80);
    }
  }
  document.addEventListener('DOMContentLoaded',initTerminal);
})();
