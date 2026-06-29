const msg=document.getElementById('msg');
const loginForm=document.getElementById('loginForm');
const username=document.getElementById('username');
const password=document.getElementById('password');
const resetEntry=document.getElementById('resetEntry');
const showResetForm=document.getElementById('showResetForm');
const resetRequestForm=document.getElementById('resetRequestForm');
const resetEmail=document.getElementById('resetEmail');
const cancelResetForm=document.getElementById('cancelResetForm');
function show(t,bad=true){msg.className='alert '+(bad?'alert-danger':'alert-success');msg.textContent=t;msg.classList.remove('d-none')}
function escAuth(s){return String(s??'').replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]))}
function setAuthLoading(btn,label){if(!btn)return()=>{};const html=btn.innerHTML;btn.disabled=true;btn.setAttribute('aria-busy','true');btn.classList.add('btn-loading');btn.innerHTML='<span class="btn-loading-spinner" aria-hidden="true"></span><span>'+escAuth(label)+'</span>';return()=>{btn.innerHTML=html;btn.disabled=false;btn.removeAttribute('aria-busy');btn.classList.remove('btn-loading')}}
function hideMessage(){msg.classList.add('d-none')}
loginForm?.addEventListener('submit',async e=>{e.preventDefault();hideMessage();const done=setAuthLoading(e.submitter,'Entrando');try{const res=await fetch('/api/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:username.value,password:password.value})});const data=await res.json();if(!res.ok)throw new Error(data.error||'No se pudo iniciar sesion');location.href=(data.user&&data.user.forcePasswordChange)?'/password':'/'}catch(err){show(err.message);done()}})
async function initResetAvailability(){if(!resetEntry)return;try{const res=await fetch('/api/password-reset/status',{headers:{'Accept':'application/json'},cache:'no-store'});const data=await res.json();if(data.enabled){resetEntry.classList.remove('d-none')}else{resetEntry.classList.add('d-none')}}catch{resetEntry.classList.add('d-none')}}
showResetForm?.addEventListener('click',()=>{hideMessage();resetRequestForm?.classList.remove('d-none');resetEntry?.classList.add('d-none');resetEmail?.focus()})
cancelResetForm?.addEventListener('click',()=>{hideMessage();resetRequestForm?.classList.add('d-none');resetEntry?.classList.remove('d-none')})
resetRequestForm?.addEventListener('submit',async e=>{e.preventDefault();hideMessage();const done=setAuthLoading(e.submitter,'Enviando');try{const res=await fetch('/api/password-reset/request',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({email:resetEmail.value})});const data=await res.json();if(!res.ok)throw new Error(data.error||'No se pudo solicitar recuperacion');show(data.message||'Si la cuenta existe, enviaremos instrucciones.',false);resetRequestForm.classList.add('d-none');resetEntry?.classList.remove('d-none')}catch(err){show(err.message)}finally{done()}})
initResetAvailability();
