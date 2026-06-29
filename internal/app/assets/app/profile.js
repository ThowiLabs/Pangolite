function setupProfilePage(){
  const emailForm=$('profileEmailForm');
  const passForm=$('profilePasswordForm');
  if(!emailForm&&!passForm)return;

  const boot=appBoot||loadBootstrap()||{};
  const user=boot.user||{};
  const emailInput=$('profileEmail');
  const currentInput=$('profileCurrentPassword');
  const nextInput=$('profileNewPassword');
  const confirmInput=$('profileConfirmPassword');

  if(emailInput)emailInput.value=user.email||'';
  paintProfileEmailState(user.email||'');

  if(emailForm){
    bindAsyncSubmit(emailForm,async()=>{
      const email=(emailInput&&emailInput.value||'').trim();
      const data=await api('/api/profile',{method:'PATCH',body:JSON.stringify({email})});
      if(appBoot)appBoot.user=data.user||appBoot.user;
      const updatedUser=(data&&data.user)||{};
      if($('userLabel'))$('userLabel').textContent=updatedUser.username||user.username||'Usuario';
      paintProfileEmailState(updatedUser.email||email);
      msg('Correo de recuperación actualizado');
    },'Guardando');
  }

  if(passForm){
    bindAsyncSubmit(passForm,async()=>{
      const current=currentInput?currentInput.value:'';
      const next=nextInput?nextInput.value:'';
      const confirm=confirmInput?confirmInput.value:'';
      if(next!==confirm)throw new Error('Las contraseñas no coinciden');
      await api('/api/password',{method:'POST',body:JSON.stringify({currentPassword:current,newPassword:next})});
      passForm.reset();
      if(emailInput)emailInput.value=(appBoot&&appBoot.user&&appBoot.user.email)||user.email||'';
      msg('Contraseña actualizada');
    },'Cambiando');
  }
}
function paintProfileEmailState(email){
  const state=$('profileEmailState');
  if(!state)return;
  const has=String(email||'').trim()!=='';
  state.classList.toggle('on',has);
  state.classList.toggle('off',!has);
  const dot=state.querySelector('.status-dot');
  if(dot)dot.classList.toggle('ok',has);
  const text=state.querySelector('[data-slot="text"]');
  if(text)text.textContent=has?'Correo configurado':'Sin correo';
}
