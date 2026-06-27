package app

const loginHTML = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pangolite - Iniciar sesion</title>
  <link rel="stylesheet" href="/assets/app/styles.css">
  <style>
    :root{--pg-bg:#000;--pg-card:rgba(255,255,255,.08);--pg-border:rgba(255,255,255,.18);--pg-text:oklab(0.867818 0.0000395482 0.0000174274 / 0.967216);--pg-muted:#afafaf;--pg-accent:#8b5cf6;--pg-accent-2:#22d3ee;--pg-radius:16px}
    body.auth-shell{min-height:100vh;background:radial-gradient(circle at 12% 0%,rgba(139,92,246,.32),transparent 34%),radial-gradient(circle at 88% 12%,rgba(34,211,238,.18),transparent 30%),var(--pg-bg);color:var(--pg-text);font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,Arial,sans-serif}
    .auth-card{max-width:440px;margin:auto;border:1px solid var(--pg-border);border-radius:var(--pg-radius);box-shadow:0 24px 70px rgba(0,0,0,.38);background:var(--pg-card);backdrop-filter:blur(20px);color:var(--pg-text)}
    .brand-mark{width:48px;height:48px;border-radius:16px;display:grid;place-items:center;background:linear-gradient(135deg,var(--pg-accent),var(--pg-accent-2));color:#fff;font-weight:800;letter-spacing:-.05em}
    .auth-wrap{min-height:100vh;display:flex;align-items:center;padding:2rem 1rem}
    .text-muted{color:var(--pg-muted)!important}.card-footer{border-top:1px solid var(--pg-border)!important;color:var(--pg-muted)!important}.form-floating>.form-control{height:3.35rem}.form-control{background:rgba(0,0,0,.48);border:1px solid var(--pg-border);color:#fff;border-radius:10px}.form-control:focus{background:#050505;color:#fff;border-color:var(--pg-accent-2);box-shadow:0 0 0 3px rgba(34,211,238,.22)}.form-floating>label{color:var(--pg-muted)}.btn-primary{border:0;background:linear-gradient(135deg,var(--pg-accent),var(--pg-accent-2));border-radius:10px;font-weight:700}.btn:focus-visible,.form-control:focus-visible{outline:3px solid rgba(34,211,238,.55);outline-offset:2px}
  </style>
</head>
<body class="auth-shell">
  <main class="auth-wrap">
    <div class="card auth-card w-100">
      <div class="card-body p-4 p-sm-5">
        <div class="d-flex align-items-center gap-3 mb-4">
          <div class="brand-mark">Pg</div>
          <div>
            <h1 class="h4 mb-1">Pangolite</h1>
            <div class="small text-muted">Panel seguro de proxys y agentes</div>
          </div>
        </div>
        <div id="msg" class="alert d-none" role="alert"></div>
        <form id="loginForm" autocomplete="on">
          <div class="form-floating mb-3">
            <input class="form-control" id="username" name="username" type="text" autocomplete="username" placeholder="admin" required>
            <label for="username">Usuario</label>
          </div>
          <div class="form-floating mb-3">
            <input class="form-control" id="password" name="password" type="password" autocomplete="current-password" placeholder="Contraseña" required>
            <label for="password">Contraseña</label>
          </div>
          <button class="btn btn-primary w-100 py-2" type="submit">Entrar</button>
        </form>
      </div>
      <div class="card-footer bg-transparent text-center small text-muted py-3">La contraseña temporal inicial vive en <code>/opt/pangolite/data/admin-password.txt</code>.</div>
    </div>
  </main>
<script>
const msg=document.getElementById('msg');
function show(t,bad=true){msg.className='alert '+(bad?'alert-danger':'alert-success');msg.textContent=t;msg.classList.remove('d-none')}
document.getElementById('loginForm').addEventListener('submit',async e=>{e.preventDefault();msg.classList.add('d-none');try{const res=await fetch('/api/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:username.value,password:password.value})});const data=await res.json();if(!res.ok)throw new Error(data.error||'No se pudo iniciar sesion');location.href=(data.user&&data.user.forcePasswordChange)?'/password':'/'}catch(err){show(err.message)}})
</script>
</body>
</html>`

const passwordHTML = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pangolite - Cambiar contraseña</title>
  <link rel="stylesheet" href="/assets/app/styles.css">
  <style>
    :root{--pg-bg:#000;--pg-card:rgba(255,255,255,.08);--pg-border:rgba(255,255,255,.18);--pg-text:oklab(0.867818 0.0000395482 0.0000174274 / 0.967216);--pg-muted:#afafaf;--pg-accent:#8b5cf6;--pg-accent-2:#22d3ee;--pg-radius:16px}
    body.auth-shell{min-height:100vh;background:radial-gradient(circle at 12% 0%,rgba(139,92,246,.32),transparent 34%),radial-gradient(circle at 88% 12%,rgba(34,211,238,.18),transparent 30%),var(--pg-bg);color:var(--pg-text);font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,Arial,sans-serif}
    .auth-card{max-width:480px;margin:auto;border:1px solid var(--pg-border);border-radius:var(--pg-radius);box-shadow:0 24px 70px rgba(0,0,0,.38);background:var(--pg-card);backdrop-filter:blur(20px);color:var(--pg-text)}
    .auth-wrap{min-height:100vh;display:flex;align-items:center;padding:2rem 1rem}
    .brand-mark{width:48px;height:48px;border-radius:16px;display:grid;place-items:center;background:linear-gradient(135deg,var(--pg-accent),var(--pg-accent-2));color:#fff;font-weight:800;letter-spacing:-.05em}
    .text-muted{color:var(--pg-muted)!important}.form-control{background:rgba(0,0,0,.48);border:1px solid var(--pg-border);color:#fff;border-radius:10px}.form-control:focus{background:#050505;color:#fff;border-color:var(--pg-accent-2);box-shadow:0 0 0 3px rgba(34,211,238,.22)}.form-floating>label{color:var(--pg-muted)}.btn-primary{border:0;background:linear-gradient(135deg,var(--pg-accent),var(--pg-accent-2));border-radius:10px;font-weight:700}.btn:focus-visible,.form-control:focus-visible{outline:3px solid rgba(34,211,238,.55);outline-offset:2px}
  </style>
</head>
<body class="auth-shell">
  <main class="auth-wrap">
    <div class="card auth-card w-100">
      <div class="card-body p-4 p-sm-5">
        <div class="d-flex align-items-center gap-3 mb-4">
          <div class="brand-mark">Pg</div>
          <div>
            <h1 class="h4 mb-1">Cambiar contraseña</h1>
            <div class="small text-muted">Reemplaza la contraseña temporal antes de administrar el panel.</div>
          </div>
        </div>
        <div id="msg" class="alert d-none" role="alert"></div>
        <form id="passForm">
          <div id="currentGroup" class="form-floating mb-3 d-none">
            <input class="form-control" id="currentPassword" type="password" autocomplete="current-password" placeholder="Actual">
            <label for="currentPassword">Contraseña actual</label>
          </div>
          <div class="form-floating mb-3">
            <input class="form-control" id="newPassword" type="password" autocomplete="new-password" placeholder="Nueva" minlength="6" required>
            <label for="newPassword">Nueva contraseña</label>
          </div>
          <div class="form-floating mb-4">
            <input class="form-control" id="confirmPassword" type="password" autocomplete="new-password" placeholder="Confirmar" minlength="6" required>
            <label for="confirmPassword">Confirmar contraseña</label>
          </div>
          <button class="btn btn-primary w-100 py-2" type="submit">Guardar contraseña</button>
        </form>
      </div>
    </div>
  </main>
<script>
let csrf='';let force=true;const msg=document.getElementById('msg');
function show(t,bad=true){msg.className='alert '+(bad?'alert-danger':'alert-success');msg.textContent=t;msg.classList.remove('d-none')}
async function init(){const res=await fetch('/api/session');const data=await res.json();if(!data.authenticated){location.href='/login';return}csrf=data.csrfToken;force=!!data.user.forcePasswordChange;if(!force){currentGroup.classList.remove('d-none');currentPassword.required=true}}
passForm.addEventListener('submit',async e=>{e.preventDefault();msg.classList.add('d-none');if(newPassword.value!==confirmPassword.value){show('Las contraseñas no coinciden');return}try{const res=await fetch('/api/password',{method:'POST',headers:{'Content-Type':'application/json','X-CSRF-Token':csrf},body:JSON.stringify({currentPassword:currentPassword.value,newPassword:newPassword.value})});const data=await res.json();if(!res.ok)throw new Error(data.error||'No se pudo cambiar la contraseña');show('Contraseña actualizada',false);setTimeout(()=>location.href='/',700)}catch(err){show(err.message)}})
init();
</script>
</body>
</html>`

const appHTML = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pangolite Platform</title>
  <link rel="stylesheet" href="/assets/app/styles.css">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.13.1/font/bootstrap-icons.min.css">
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/animate.css@4.1.1/animate.min.css">
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4.5.1/dist/chart.umd.min.js"></script>
  <style>
    :root{
      --pg-sidebar:292px;--pg-font:-apple-system-body,ui-sans-serif,-apple-system,system-ui,"Segoe UI",Helvetica,"Apple Color Emoji",Arial,sans-serif,"Segoe UI Emoji","Segoe UI Symbol";
      --pg-bg:#000;--pg-surface:#050505;--pg-surface-2:#08080c;--pg-raised:rgba(255,255,255,.075);--pg-raised-2:rgba(255,255,255,.105);--pg-border:rgba(255,255,255,.16);--pg-border-strong:rgba(255,255,255,.30);
      --pg-text:oklab(0.867818 0.0000395482 0.0000174274 / 0.967216);--pg-white:#fff;--pg-muted:#afafaf;--pg-tertiary:#cdcdcd;--pg-accent:#8b5cf6;--pg-accent-2:#22d3ee;--pg-success:#34d399;--pg-danger:#fb7185;--pg-warning:#fbbf24;
      --pg-radius-xs:6px;--pg-radius-sm:8px;--pg-radius-md:10px;--pg-radius-lg:12px;--pg-radius-xl:16px;--pg-motion:150ms;
    }
    *{box-sizing:border-box}html{background:var(--pg-bg)}body{min-height:100vh;margin:0;background:radial-gradient(circle at 12% 0%,rgba(139,92,246,.20),transparent 34%),radial-gradient(circle at 92% 8%,rgba(34,211,238,.13),transparent 28%),var(--pg-bg);color:var(--pg-text);font-family:var(--pg-font);font-size:14px;line-height:20px}.app{display:flex;min-height:100vh}.sidebar{position:fixed;inset:0 auto 0 0;width:var(--pg-sidebar);padding:18px;border-right:1px solid var(--pg-border);background:rgba(0,0,0,.76);backdrop-filter:blur(18px);overflow-y:auto}.brand{display:flex;align-items:center;gap:12px;padding:10px 10px 18px}.brand-mark{width:42px;height:42px;border-radius:14px;display:grid;place-items:center;background:linear-gradient(135deg,var(--pg-accent),var(--pg-accent-2));font-weight:900;color:#fff;letter-spacing:-.06em}.brand-title{color:var(--pg-white);font-weight:850;letter-spacing:-.03em}.brand-sub{font-size:12px;color:var(--pg-muted)}.nav-section{margin-top:18px}.nav-title{padding:0 10px 7px;color:var(--pg-muted);font-size:12px;text-transform:uppercase;letter-spacing:.08em}.nav-link{display:flex;align-items:center;gap:10px;width:100%;padding:10px 12px;border:1px solid transparent;border-radius:12px;color:var(--pg-tertiary);text-decoration:none;background:transparent;transition:all var(--pg-motion)}.nav-link i{width:18px;text-align:center}.nav-link:hover,.nav-link.active{border-color:var(--pg-border);background:var(--pg-raised);color:#fff}.project-link{justify-content:space-between}.project-dot{width:8px;height:8px;border-radius:999px;background:var(--pg-success);box-shadow:0 0 0 3px rgba(52,211,153,.15)}.main{margin-left:var(--pg-sidebar);width:calc(100% - var(--pg-sidebar));min-height:100vh;padding:24px}.topbar{height:64px;border:1px solid var(--pg-border);border-radius:18px;background:rgba(255,255,255,.055);display:flex;align-items:center;justify-content:space-between;padding:0 16px;margin-bottom:18px}.crumb{color:var(--pg-muted);font-size:13px}.top-user{display:flex;gap:10px;align-items:center}.hero{border:1px solid var(--pg-border);border-radius:22px;padding:24px;background:linear-gradient(135deg,rgba(139,92,246,.30),rgba(34,211,238,.12) 48%,rgba(255,255,255,.05));box-shadow:0 22px 70px rgba(0,0,0,.32);margin-bottom:18px}.hero h1,.page-title{margin:0;color:#fff!important;font-size:24px;line-height:30px;font-weight:850;letter-spacing:-.04em}.hero p{margin:8px 0 0;color:var(--pg-tertiary);max-width:820px}.toolbar{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:14px}.grid{display:grid;gap:16px}.grid-2{grid-template-columns:repeat(2,minmax(0,1fr))}.grid-3{grid-template-columns:repeat(3,minmax(0,1fr))}.card{border:1px solid var(--pg-border)!important;border-radius:18px!important;background:var(--pg-raised)!important;color:var(--pg-text)!important;box-shadow:0 16px 48px rgba(0,0,0,.22)}.card-header{border-bottom:1px solid var(--pg-border)!important;background:transparent!important}.card-title{color:#fff;font-weight:800}.stat{padding:18px;border:1px solid var(--pg-border);border-radius:18px;background:rgba(255,255,255,.055)}.stat-label{color:var(--pg-muted);font-size:13px}.stat-value{font-size:26px;line-height:32px;color:#fff;font-weight:850}.table{color:var(--pg-text)!important;margin-bottom:0}.table thead th{color:var(--pg-muted);font-size:12px;text-transform:uppercase;letter-spacing:.05em;border-color:var(--pg-border)!important}.table td{border-color:var(--pg-border)!important;vertical-align:middle}.form-control,.form-select{background:rgba(0,0,0,.42)!important;border:1px solid var(--pg-border)!important;color:#fff!important;border-radius:12px!important}.form-control:focus,.form-select:focus{border-color:var(--pg-accent-2)!important;box-shadow:0 0 0 3px rgba(34,211,238,.20)!important}.form-label{color:#fff;font-weight:700}.form-text,.text-muted{color:var(--pg-muted)!important}.btn{border-radius:12px!important;font-weight:750;display:inline-flex;align-items:center;gap:8px}.btn-primary{border:0!important;background:linear-gradient(135deg,var(--pg-accent),var(--pg-accent-2))!important}.btn-outline-secondary{color:#fff!important;border-color:var(--pg-border)!important}.btn-outline-secondary:hover{background:var(--pg-raised-2)!important}.btn-outline-danger{color:#fecdd3!important;border-color:rgba(251,113,133,.42)!important}.btn-help{width:30px;height:30px;border-radius:999px!important;padding:0!important;display:inline-grid!important;place-items:center!important;border:1px solid var(--pg-border)!important;color:#fff!important;background:rgba(255,255,255,.07)!important}.badge-mode{border-radius:999px;padding:.35rem .55rem}.state-pill{display:inline-flex;align-items:center;gap:6px;border:1px solid var(--pg-border);border-radius:999px;padding:4px 8px;font-size:12px}.state-pill.on{color:#bbf7d0}.state-pill.off{color:#fecdd3}.status-dot{width:8px;height:8px;border-radius:999px;background:var(--pg-muted);display:inline-block}.status-dot.ok{background:var(--pg-success)}.empty{padding:28px;text-align:center;color:var(--pg-muted)}.pl-primary{border:1px solid rgba(139,92,246,.36)!important;background:linear-gradient(135deg,rgba(139,92,246,.18),rgba(34,211,238,.08))!important;color:var(--pg-text)!important}.pl-secondary,.pl-secundary{border:1px solid var(--pg-border)!important;background:rgba(255,255,255,.055)!important;color:var(--pg-text)!important}.pl-surface{border:1px solid var(--pg-border)!important;background:rgba(0,0,0,.34)!important;color:var(--pg-text)!important}.pl-success{border:1px solid rgba(52,211,153,.36)!important;background:rgba(52,211,153,.10)!important;color:#d1fae5!important}.pl-warning{border:1px solid rgba(251,191,36,.36)!important;background:rgba(251,191,36,.10)!important;color:#fef3c7!important}.pl-danger{border:1px solid rgba(251,113,133,.38)!important;background:rgba(251,113,133,.10)!important;color:#fecdd3!important}.pl-callout{border-radius:16px;padding:14px;box-shadow:0 14px 38px rgba(0,0,0,.18)}.pl-callout-primary{border:1px solid rgba(139,92,246,.36);background:linear-gradient(135deg,rgba(139,92,246,.18),rgba(34,211,238,.08));color:var(--pg-text)}.token-box{white-space:pre-wrap;word-break:break-all}.command-stack{display:grid;gap:12px}.command-card{border:1px solid var(--pg-border);border-radius:16px;background:rgba(0,0,0,.34);overflow:hidden}.command-head{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:10px 12px;border-bottom:1px solid var(--pg-border);color:#fff;font-weight:800}.command-card pre{margin:0;padding:13px 14px;white-space:pre-wrap;word-break:break-all;background:#020204;color:#dbeafe}.command-card code{font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,"Liberation Mono",monospace;font-size:12px;color:#dbeafe}.secret-grid{display:grid;grid-template-columns:1fr;gap:8px}.secret-row{display:flex;align-items:center;justify-content:space-between;gap:10px;border:1px solid var(--pg-border);border-radius:12px;padding:9px 11px;background:rgba(255,255,255,.04)}.secret-row code{color:#fff;word-break:break-all}.busy-card{width:min(460px,100%);text-align:center}.busy-spinner{width:58px;height:58px;border-radius:999px;border:4px solid rgba(255,255,255,.12);border-top-color:var(--pg-accent-2);margin:4px auto 18px;animation:pgspin .9s linear infinite}.busy-dots::after{content:"";animation:pgdots 1.2s infinite}.confirm-card{width:min(520px,100%)}.confirm-icon{width:48px;height:48px;border-radius:16px;display:grid;place-items:center;background:rgba(251,113,133,.13);color:#fecdd3;border:1px solid rgba(251,113,133,.28);font-size:22px}.danger-zone{border-color:rgba(251,113,133,.34)!important;background:linear-gradient(135deg,rgba(127,29,29,.28),rgba(255,255,255,.045))!important}.danger-zone .card-title{color:#fecdd3}.danger-note{color:#fecdd3;font-size:13px}.password-confirm-input{letter-spacing:.08em}@keyframes pgspin{to{transform:rotate(360deg)}}@keyframes pgdots{0%{content:""}33%{content:"."}66%{content:".."}100%{content:"..."}}.font-mono{font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,"Liberation Mono",monospace;font-size:12px}.route-view{display:none}.route-view.active{display:block}.modal-lite{position:fixed;inset:0;background:rgba(0,0,0,.72);display:none;align-items:center;justify-content:center;z-index:50;padding:18px}.modal-lite.open{display:flex}.modal-card{width:min(720px,100%);border:1px solid var(--pg-border);border-radius:20px;background:#08080c;color:var(--pg-text);padding:20px;box-shadow:0 24px 90px rgba(0,0,0,.55)}.alert{border-radius:14px}.d-none{display:none!important}a{color:inherit}button:focus-visible,a:focus-visible,input:focus-visible,select:focus-visible,textarea:focus-visible{outline:3px solid rgba(34,211,238,.55)!important;outline-offset:2px!important}.sidebar{transition:transform var(--pg-motion),width var(--pg-motion)}.sidebar-collapsed .sidebar{transform:translateX(calc(-1 * var(--pg-sidebar)))}.sidebar-collapsed .main{margin-left:0;width:100%}.sidebar-toggle{width:38px;height:38px;display:inline-grid!important;place-items:center!important;padding:0!important}.project-primary{margin-top:10px;padding:10px;border:1px solid var(--pg-border);border-radius:18px;background:rgba(255,255,255,.04)}.project-switcher{position:relative}.project-switcher-button{width:100%;justify-content:space-between!important;text-align:left;background:rgba(255,255,255,.055)!important;border:1px solid var(--pg-border)!important;color:#fff!important}.project-switcher-menu{display:none;margin-top:8px;border:1px solid var(--pg-border);border-radius:16px;background:#08080c;padding:10px;box-shadow:0 22px 60px rgba(0,0,0,.48)}.project-switcher.open .project-switcher-menu{display:block}.project-results{max-height:260px;overflow:auto;margin-top:8px;display:grid;gap:6px}.project-result{width:100%;border:1px solid transparent;border-radius:12px;background:transparent;color:var(--pg-tertiary);display:flex;align-items:center;justify-content:space-between;gap:10px;padding:9px 10px;text-align:left}.project-result:hover,.project-result.active{border-color:var(--pg-border);background:var(--pg-raised);color:#fff}.project-meta{font-size:12px;color:var(--pg-muted)}.side-footer{margin-top:auto;padding:14px 10px;color:var(--pg-muted);font-size:12px;border-top:1px solid var(--pg-border)}.product-mini{display:flex;align-items:center;gap:10px}.product-status-dot{width:9px;height:9px;border-radius:999px;background:var(--pg-success);box-shadow:0 0 0 4px rgba(52,211,153,.14)}.ops-list{display:grid;gap:10px}.ops-row{display:flex;align-items:center;justify-content:space-between;gap:12px;border:1px solid var(--pg-border);background:rgba(255,255,255,.045);border-radius:14px;padding:12px 14px}.ops-label{color:var(--pg-muted);font-size:12px}.ops-value{color:#fff;font-weight:750;text-align:right;word-break:break-all}.ops-value.good{color:#bbf7d0}.ops-value.warn{color:#fde68a}.ops-value.bad{color:#fecdd3}.main-layout-note{font-size:12px;color:var(--pg-muted)}.resource-edit-grid{max-height:70vh;overflow:auto;padding-right:2px}.dashboard-panel{margin-bottom:16px}.metric-card{position:relative;overflow:hidden;min-height:150px}.metric-card::after{content:"";position:absolute;inset:auto -20% -55% 35%;height:130px;background:radial-gradient(circle,rgba(34,211,238,.18),transparent 62%);pointer-events:none}.metric-icon{width:42px;height:42px;border-radius:14px;display:grid;place-items:center;background:rgba(255,255,255,.08);border:1px solid var(--pg-border);color:#fff}.stat-trend{font-size:12px;color:var(--pg-muted);margin-top:4px}.chart-card canvas{max-height:260px}.chart-card .card-body{min-height:300px}.chart-fallback{display:none;color:var(--pg-muted);font-size:13px;padding:14px;border:1px dashed var(--pg-border);border-radius:14px}.log-view{background:rgba(0,0,0,.55);border:1px solid var(--pg-border);border-radius:16px;padding:14px;max-height:62vh;overflow:auto;white-space:pre-wrap;word-break:break-word;color:#d6d6d6}.log-line{display:block;border-bottom:1px solid rgba(255,255,255,.06);padding:4px 0}.log-line.warn{color:#fde68a}.log-line.error{color:#fecdd3}.log-path{font-size:12px;color:var(--pg-muted);word-break:break-all}.soft-enter{animation:pgFadeUp .42s ease both}.soft-enter-delay-1{animation-delay:.04s}.soft-enter-delay-2{animation-delay:.08s}.soft-enter-delay-3{animation-delay:.12s}@keyframes pgFadeUp{from{opacity:0;transform:translateY(10px) scale(.99)}to{opacity:1;transform:none}}@media(prefers-reduced-motion:reduce){.soft-enter,.soft-enter-delay-1,.soft-enter-delay-2,.soft-enter-delay-3{animation:none!important}.animate__animated{animation:none!important;transition:none!important}}@media(max-width:960px){.sidebar{position:fixed;z-index:40;transform:translateX(-100%);width:min(292px,86vw)}body.sidebar-open .sidebar{transform:translateX(0)}.sidebar-collapsed .sidebar{transform:translateX(-100%)}.app{display:block}.main,.sidebar-collapsed .main{margin-left:0;width:100%;padding:14px}.grid-2,.grid-3{grid-template-columns:1fr}.topbar,.toolbar{height:auto;align-items:flex-start;gap:10px;flex-direction:column}}
  </style>
</head>
<body>
<div class="app">
  <aside class="sidebar" aria-label="Navegacion principal">
    <div class="brand"><div class="brand-mark">Pg</div><div><div class="brand-title">Pangolite</div><div class="brand-sub">Edge Platform</div></div></div>
    <nav class="nav-section project-primary">
      <div class="nav-title">Proyecto activo</div>
      <div class="project-switcher" id="projectSwitcher">
        <button class="btn project-switcher-button" type="button" id="projectSwitcherButton" onclick="toggleProjectSwitcher()"><span><i class="bi bi-folder2-open"></i> <span id="currentProjectLabel">Selecciona proyecto</span></span><i class="bi bi-chevron-down"></i></button>
        <div class="project-switcher-menu" id="projectSwitcherMenu">
          <input id="projectSearch" class="form-control form-control-sm" placeholder="Buscar proyecto o cliente..." autocomplete="off">
          <div id="projectNav" class="project-results" role="listbox" aria-label="Resultados de proyectos"></div>
        </div>
      </div>
      <div class="main-layout-note mt-2 px-2">Selecciona primero el cliente. Todo lo demás depende del proyecto activo.</div>
    </nav>
    <nav class="nav-section">
      <div class="nav-title">Panel</div>
      <a class="nav-link" href="/projects" data-nav="projects"><i class="bi bi-speedometer2"></i><span>Dashboard</span></a>
      <a class="nav-link" href="/logs" data-nav="logs"><i class="bi bi-journal-text"></i><span>Logs</span></a>
      <a class="nav-link" href="/maintenance" data-nav="maintenance"><i class="bi bi-shield-check"></i><span>Seguridad</span></a>
      <a class="nav-link" href="/settings" data-nav="settings"><i class="bi bi-sliders"></i><span>Ajustes</span></a>
    </nav>
    <div class="side-footer"><div class="product-mini"><span class="product-status-dot"></span><div><div class="fw-semibold text-white">Sistema operativo</div><div>Dominios, túneles y recursos bajo control.</div></div></div></div>
  </aside>
  <main class="main">
    <div class="topbar">
      <div class="d-flex align-items-center gap-3"><button class="btn btn-outline-secondary sidebar-toggle" type="button" id="sidebarToggle" aria-label="Ocultar o mostrar menu"><i class="bi bi-list"></i></button><div><div class="crumb" id="crumb">Proyectos</div><div class="page-title" id="topTitle">Operación global</div></div></div>
      <div class="top-user"><span id="userLabel" class="text-muted"></span><button class="btn btn-sm btn-outline-secondary" id="logoutBtn"><i class="bi bi-box-arrow-right"></i> Cerrar sesion</button></div>
    </div>
    <div id="msg" class="alert d-none" role="alert"></div>

    <section id="view-projects" class="route-view">
      <div class="hero animate__animated animate__fadeIn"><h1>Dashboard global</h1><p>Vista ejecutiva de clientes, recursos publicados y servidores conectados. Selecciona un proyecto para operar sobre su entorno.</p></div>
      <div class="grid grid-3 mb-3 dashboard-panel">
        <div class="stat metric-card soft-enter"><div class="d-flex align-items-center justify-content-between"><div><div class="stat-label">Proyectos</div><div class="stat-value" id="statProjects">0</div><div class="stat-trend">Clientes y entornos registrados</div></div><div class="metric-icon"><i class="bi bi-building"></i></div></div></div>
        <div class="stat metric-card soft-enter soft-enter-delay-1"><div class="d-flex align-items-center justify-content-between"><div><div class="stat-label">Recursos</div><div class="stat-value" id="statResources">0</div><div class="stat-trend"><span id="statActiveResources">0</span> activos</div></div><div class="metric-icon"><i class="bi bi-diagram-3"></i></div></div></div>
        <div class="stat metric-card soft-enter soft-enter-delay-2"><div class="d-flex align-items-center justify-content-between"><div><div class="stat-label">Clientes de sistema</div><div class="stat-value" id="statAgents">0</div><div class="stat-trend">Agentes de servidores remotos</div></div><div class="metric-icon"><i class="bi bi-hdd-network"></i></div></div></div>
      </div>
      <div class="grid grid-2 mb-3 dashboard-panel">
        <div class="card chart-card soft-enter soft-enter-delay-1"><div class="card-header"><div><div class="card-title">Recursos por proyecto</div><div class="text-muted small">Distribución global de proxys por cliente.</div></div></div><div class="card-body"><canvas id="resourcesByProjectChart" aria-label="Recursos por proyecto" role="img"></canvas><div id="resourcesChartFallback" class="chart-fallback">Chart.js no está disponible. Revisa conexión al CDN o empaqueta los assets localmente.</div></div></div>
        <div class="card chart-card soft-enter soft-enter-delay-2"><div class="card-header"><div><div class="card-title">Estado global de recursos</div><div class="text-muted small">Activos contra suspendidos/inactivos.</div></div></div><div class="card-body"><canvas id="resourceStatusChart" aria-label="Estado global de recursos" role="img"></canvas><div id="statusChartFallback" class="chart-fallback">Chart.js no está disponible. Revisa conexión al CDN o empaqueta los assets localmente.</div></div></div>
      </div>
      <div class="card mb-3 soft-enter soft-enter-delay-3">
        <div class="card-header d-flex justify-content-between align-items-center"><div><div class="card-title">Estado operativo</div><div class="text-muted small">Publicación del panel, DNS y datos básicos del servidor.</div></div><a class="btn btn-sm btn-outline-secondary" href="/settings"><i class="bi bi-sliders"></i> Ajustes</a></div>
        <div class="card-body ops-list">
          <div class="ops-row"><div><div class="ops-label">Dominio del panel</div><div class="text-muted small">Entrada administrativa pública</div></div><div id="dashPanelDomain" class="ops-value">Sin configurar</div></div>
          <div class="ops-row"><div><div class="ops-label">IP pública detectada</div><div class="text-muted small">Referencia usada para validar DNS</div></div><div id="dashPublicIp" class="ops-value font-mono">No detectada</div></div>
          <div class="ops-row"><div><div class="ops-label">Validación DNS</div><div class="text-muted small">El dominio debe apuntar a este servidor</div></div><div id="dashDnsState" class="ops-value warn">Pendiente</div></div>
          <div class="ops-row"><div><div class="ops-label">Aplicación de cambios</div><div class="text-muted small">HTTP/HTTPS se actualiza sin cortar recursos existentes</div></div><div class="ops-value good">Automática</div></div>
        </div>
      </div>
      <div class="card">
        <div class="card-header">
          <div class="toolbar mb-0"><div><div class="card-title">Clientes / proyectos</div><div class="text-muted small">Administración principal de clientes y entornos.</div></div><div class="d-flex gap-2"><button class="btn btn-help" onclick="showHelp('Proyectos','Un proyecto es un cliente o entorno. El nombre no puede repetirse y dentro vivirán sus proxys, agentes y reglas de suspension.')"><i class="bi bi-question"></i></button><button class="btn btn-primary" onclick="openProjectModal()"><i class="bi bi-plus-lg"></i> Crear</button></div></div>
        </div>
        <div class="card-body table-responsive"><table class="table"><thead><tr><th>Estado</th><th>Nombre</th><th>Slug</th><th>Recursos</th><th>Agentes</th><th>Activos</th><th class="text-end">Acciones</th></tr></thead><tbody id="projectRows"></tbody></table></div>
      </div>
    </section>

    <section id="view-project" class="route-view">
      <div class="hero"><h1 id="projectHeroTitle">Proyecto</h1><p id="projectHeroText">Recursos y agentes separados por cliente.</p></div>
      <div class="grid grid-3 mb-3">
        <div class="stat"><div class="stat-label">Recursos del proyecto</div><div class="stat-value" id="projectResourceCount">0</div></div>
        <div class="stat"><div class="stat-label">Clientes de sistema</div><div class="stat-value" id="projectAgentCount">0</div></div>
        <div class="stat"><div class="stat-label">Activos</div><div class="stat-value" id="projectActiveCount">0</div></div>
      </div>
      <div class="card mb-3"><div class="card-header d-flex justify-content-between align-items-center"><div><div class="card-title">Acciones rapidas</div><div class="text-muted small">Gestion separada: primero clientes de sistema, despues recursos publicados.</div></div><button class="btn btn-help" onclick="showHelp('Proyecto','Cliente de sistema/agente es la identidad instalada en un servidor NAT. Recurso es lo que publicas: web, TCP o UDP. Separarlos evita confundir token de conexion con dominio o puerto expuesto.')"><i class="bi bi-question"></i></button></div><div class="card-body d-flex flex-wrap gap-2"><a id="goCreateResource" class="btn btn-primary" href="#"><i class="bi bi-diagram-3"></i> Crear recurso</a><a id="goCreateAgent" class="btn btn-outline-secondary" href="#"><i class="bi bi-hdd-network"></i> Crear cliente de sistema</a><a id="goResources" class="btn btn-outline-secondary" href="#"><i class="bi bi-list-ul"></i> Ver recursos</a><a id="goAgents" class="btn btn-outline-secondary" href="#"><i class="bi bi-router"></i> Ver clientes de sistema</a></div></div>
      <div class="grid grid-2 mb-3">
        <div class="card"><div class="card-header"><div class="card-title">Configuración del proyecto</div><div class="text-muted small">Renombra el proyecto y actualiza su descripción operativa.</div></div><div class="card-body"><form id="projectSettingsForm"><label class="form-label" for="projectEditName">Nombre</label><input id="projectEditName" class="form-control mb-3" required><label class="form-label" for="projectEditNotes">Descripción</label><textarea id="projectEditNotes" class="form-control mb-3" rows="4" placeholder="Descripción, contacto, notas de operación o cliente"></textarea><button class="btn btn-primary" type="submit"><i class="bi bi-save"></i> Guardar proyecto</button></form></div></div>
        <div class="card danger-zone"><div class="card-header"><div class="card-title">Zona de peligro</div><div class="text-muted small">Acciones irreversibles del proyecto seleccionado.</div></div><div class="card-body"><p class="danger-note" id="projectDangerHint">Para eliminar el proyecto primero debe quedar sin recursos ni clientes.</p><button id="deleteProjectBtn" class="btn btn-outline-danger" type="button" onclick="deleteCurrentProject()"><i class="bi bi-trash3"></i> Eliminar proyecto</button></div></div>
      </div>
      <div class="grid grid-2">
        <div class="card"><div class="card-header"><div class="card-title">Recursos recientes</div></div><div class="card-body table-responsive"><table class="table"><thead><tr><th>Estado</th><th>Tipo</th><th>Nombre</th><th>Entrada</th></tr></thead><tbody id="projectResourceRows"></tbody></table></div></div>
        <div class="card"><div class="card-header"><div class="card-title">Agentes</div></div><div class="card-body table-responsive"><table class="table"><thead><tr><th>Estado</th><th>Nombre</th><th>Ultima vez</th></tr></thead><tbody id="projectAgentRows"></tbody></table></div></div>
      </div>
    </section>

    <section id="view-resources" class="route-view">
      <div class="hero"><h1>Recursos del proyecto</h1><p>CRUD de dominios, paths y puertos del cliente seleccionado.</p></div>
      <div class="card"><div class="card-header d-flex justify-content-between align-items-center"><div><div class="card-title">Recursos</div><div class="text-muted small">Web, TCP o UDP publicados para este proyecto.</div></div><div class="d-flex gap-2"><button class="btn btn-help" onclick="showHelp('Recursos','Un recurso es el servicio publicado. Puede apuntar directo al host Pangolite o usar un cliente de sistema/agente para llegar a un backend dentro de una red NAT.')"><i class="bi bi-question"></i></button><button class="btn btn-outline-secondary" onclick="checkResourceHealth()"><i class="bi bi-heart-pulse"></i> Probar estado</button><a id="goCreateResourceFromList" class="btn btn-primary" href="#"><i class="bi bi-plus-lg"></i> Crear recurso</a></div></div><div class="card-body table-responsive"><table class="table"><thead><tr><th>Estado</th><th>Tipo</th><th>Nombre</th><th>Entrada</th><th>Servicio interno</th><th>Origen</th><th>Cliente</th><th>Health</th><th>Respuesta</th><th class="text-end">Acciones</th></tr></thead><tbody id="resourcesRows"></tbody></table></div></div>
      <div class="card mt-3"><div class="card-header"><div class="card-title">Suspension / respuesta personalizada</div></div><div class="card-body">
        <div class="grid grid-3">
          <div><label class="form-label">Recurso</label><select id="resourceControlSelect" class="form-select"></select></div>
          <div><label class="form-label">Estado</label><select id="resourceEnabled" class="form-select"><option value="true">Activo</option><option value="false">Suspendido</option></select></div>
          <div><label class="form-label">Respuesta</label><select id="disabledResponseMode" class="form-select"><option value="403">403 Prohibido</option><option value="404">404 No encontrado</option><option value="html">HTML personalizado</option></select></div>
        </div>
        <div class="grid grid-2 mt-3 html-control d-none"><div><label class="form-label">Preset</label><select id="disabledPreset" class="form-select"><option value="">Sin preset</option><option value="payment">Pago pendiente</option><option value="maintenance">Mantenimiento</option><option value="suspended">Servicio suspendido</option></select></div><div><label class="form-label">Codigo HTTP</label><select id="disabledStatusCode" class="form-select"><option value="403">403</option><option value="404">404</option><option value="200">200</option></select></div></div>
        <div class="mt-3 html-control d-none"><label class="form-label">HTML</label><textarea id="disabledHtml" class="form-control font-mono" rows="8"></textarea></div>
        <div class="mt-3 d-flex gap-2"><button class="btn btn-primary" onclick="saveResourceControl()"><i class="bi bi-save"></i> Guardar control</button><button class="btn btn-outline-secondary" onclick="activateSelectedResource()"><i class="bi bi-check2-circle"></i> Activar</button></div>
      </div></div>
    </section>

    <section id="view-agents" class="route-view">
      <div class="hero"><h1>Clientes de sistema</h1><p>Identidades instaladas en VPS/NAT o redes privadas. Un cliente de sistema se conecta hacia Pangolite y permite exponer recursos de ese servidor.</p></div>
      <div class="card"><div class="card-header d-flex justify-content-between align-items-center"><div><div class="card-title">Clientes de sistema</div><div class="text-muted small">ID y token para servidores remotos. Los tokens se muestran solo al crear o rotar.</div></div><div class="d-flex gap-2"><button class="btn btn-help" onclick="showHelp('Clientes de sistema','Un cliente de sistema no es un proxy. Es la credencial instalada en un servidor NAT/remoto. Los recursos despues eligen si usan este cliente o si apuntan directo al host Pangolite.')"><i class="bi bi-question"></i></button><a id="goCreateAgentFromList" class="btn btn-primary" href="#"><i class="bi bi-plus-lg"></i> Crear cliente</a></div></div><div class="card-body table-responsive"><table class="table"><thead><tr><th>Estado</th><th>Nombre</th><th>ID</th><th>Sistema</th><th>Recursos</th><th>Ultima vez</th><th class="text-end">Acciones</th></tr></thead><tbody id="agentsRows"></tbody></table><div id="agentTokenBox" class="pl-callout pl-primary token-box font-mono mt-3 d-none"></div></div></div>
    </section>

    <section id="view-create-agent" class="route-view">
      <div class="hero"><h1>Crear cliente de sistema</h1><p>Genera el ID y token que se instalaran en un servidor NAT, VPS de cliente o red remota. Este cliente no publica nada por si solo; solo habilita conexion saliente hacia Pangolite.</p></div>
      <div class="grid grid-2">
        <div class="card"><div class="card-header d-flex justify-content-between align-items-center"><div><div class="card-title">Datos del cliente</div><div class="text-muted small">Identidad de conexion remota.</div></div><button class="btn btn-help" onclick="showHelp('Crear cliente de sistema','Crea un cliente por servidor o red remota. Copia ID y TOKEN al instalar el agente. Despues los recursos podran seleccionar este cliente como origen del backend.')"><i class="bi bi-question"></i></button></div><div class="card-body"><label class="form-label">Nombre del cliente de sistema</label><input id="agentName" class="form-control mb-3" placeholder="vps-cliente-01"><div class="form-text mb-3">Usa nombres operativos: cliente-servidor, sucursal-vpn, vps-nat-01.</div><button class="btn btn-primary" onclick="createAgent()"><i class="bi bi-hdd-network"></i> Crear cliente de sistema</button><div id="agentTokenCreate" class="pl-callout pl-primary token-box mt-3 d-none"></div></div></div>
        <div class="card"><div class="card-header"><div class="card-title">Como se usa</div></div><div class="card-body"><ol class="text-muted mb-0"><li>Creas el cliente de sistema.</li><li>Copias ID y TOKEN en el servidor NAT/remoto.</li><li>El agente se conecta hacia Pangolite de forma saliente.</li><li>Al crear un recurso, eliges este cliente como origen si el backend vive en ese servidor.</li></ol></div></div>
      </div>
    </section>

    <section id="view-create-resource" class="route-view">
      <div class="hero"><h1>Crear recurso</h1><p>Un recurso es el servicio expuesto: web HTTP/HTTPS, TCP o UDP. Puede salir directo desde el host Pangolite o por un cliente de sistema.</p></div>
      <div class="card"><div class="card-header d-flex justify-content-between align-items-center"><div><div class="card-title">Datos del recurso</div><div class="text-muted small">Define qué se publica y dónde vive el servicio real.</div></div><button class="btn btn-help" onclick="showHelp('Crear recurso','Primero define que publicas. Luego elige origen: host local si Pangolite puede llegar directo al backend, o cliente de sistema si el backend vive dentro de un VPS/NAT/remoto.')"><i class="bi bi-question"></i></button></div><div class="card-body">
        <form id="resourceForm">
          <div class="grid grid-2"><div><label class="form-label">Nombre del recurso</label><input id="resourceName" class="form-control" placeholder="Panel administrativo"></div><div><label class="form-label">Tipo de publicacion</label><select id="mode" class="form-select"><option value="http">Web HTTP/HTTPS</option><option value="tcp">TCP</option><option value="udp">UDP</option></select></div></div>
          <div class="grid grid-2 mt-3"><div><label class="form-label">Ubicación del servicio</label><select id="originType" class="form-select"><option value="local">En este servidor Pangolite</option><option value="agent">En un servidor remoto conectado</option></select><div class="form-text">Usa “este servidor” si Pangolite puede llegar directo al servicio. Usa “servidor remoto” si el servicio vive en un VPS/NAT con agente instalado.</div></div><div id="agentOriginGroup" class="d-none"><label class="form-label">Servidor remoto conectado</label><select id="agentId" class="form-select"><option value="">Selecciona un cliente</option></select><div class="form-text">Servidor que tiene instalado el cliente de sistema.</div></div></div>
          <div id="agentTcpUdpNotice" class="alert alert-warning mt-3 d-none"><i class="bi bi-exclamation-triangle"></i> TCP remoto usa stream persistente por el cliente NAT. UDP remoto usa intercambio de datagramas con respuesta.</div>
          <div class="grid grid-2 mt-3 http-only"><div><label class="form-label">Dominio administrado</label><select id="domainSelect" class="form-select"></select><div class="form-text">Elige domain.tld configurado en Ajustes o Custom.</div></div><div id="managedDomainGroup"><label class="form-label">Subdominio</label><input id="subdomain" class="form-control" placeholder="app"><div class="form-text">Vacio usa el dominio base. Ejemplo: app.domain.tld</div></div></div>
          <div class="mt-3 http-only d-none" id="customDomainGroup"><label class="form-label">Dominio personalizado</label><input id="customDomain" class="form-control" placeholder="app.cliente.com"></div>
          <div class="grid grid-2 mt-3 http-only"><div><label class="form-label">Path</label><input id="pathPrefix" class="form-control" value="/"><div class="form-text">Usa / para todo el dominio o /app para una ruta especifica.</div></div><div><label class="form-label">TLS</label><select id="tls" class="form-select"><option value="true">HTTPS con ACME</option><option value="false">HTTP</option></select></div></div>
          <div class="mt-2 http-only"><span class="text-muted small">Vista previa: </span><span id="domainPreview" class="font-mono text-white">-</span></div>
          <div class="grid grid-2 mt-3 tcpudp-only d-none"><div><label class="form-label">Puerto publico</label><input id="publicPort" type="number" class="form-control" placeholder="2222"></div><div></div></div>
          <div class="grid grid-3 mt-3"><div class="http-only"><label class="form-label">Protocolo interno</label><select id="backendScheme" class="form-select"><option value="http">http</option><option value="https">https</option></select></div><div><label class="form-label">Host interno del servicio</label><input id="backendHost" class="form-control" value="127.0.0.1"><div class="form-text">Se resuelve desde la ubicación elegida. Ejemplo: 127.0.0.1 si el servicio corre en ese mismo servidor.</div></div><div><label class="form-label">Puerto interno del servicio</label><input id="backendPort" type="number" class="form-control" value="8080"></div></div>
          <button class="btn btn-primary mt-3" type="submit"><i class="bi bi-plus-circle"></i> Crear recurso</button>
        </form>
      </div></div>
    </section>

    <section id="view-logs" class="route-view">
      <div class="hero"><h1>Logs del sistema</h1><p>Ultimos eventos del panel, validaciones de puertos, errores API, Traefik y clientes NAT. El archivo se mantiene en maximo 1000 entradas para evitar crecimiento indefinido.</p></div>
      <div class="card"><div class="card-header d-flex justify-content-between align-items-center"><div><div class="card-title">Diagnostico operativo</div><div id="logsPath" class="log-path">Cargando ruta...</div></div><div class="d-flex gap-2"><button class="btn btn-outline-secondary" onclick="loadLogs()"><i class="bi bi-arrow-clockwise"></i> Actualizar</button><button class="btn btn-outline-secondary" onclick="copyLogs()"><i class="bi bi-clipboard"></i> Copiar</button></div></div><div class="card-body"><div id="logsBox" class="log-view font-mono">Cargando logs...</div></div></div>
    </section>



    <section id="view-maintenance" class="route-view">
      <div class="hero"><h1>Seguridad operativa</h1><p>Auditoría de cambios administrativos y respaldos seguros de SQLite antes de operaciones críticas.</p></div>
      <div class="grid grid-2 mb-3">
        <div class="card"><div class="card-header"><div class="toolbar mb-0"><div><div class="card-title">Respaldos SQLite</div><div id="backupDirHint" class="text-muted small">Cargando ruta...</div></div><button class="btn btn-primary" onclick="createBackup()"><i class="bi bi-database-add"></i> Crear respaldo</button></div></div><div class="card-body"><div id="backupRows" class="table-responsive"><div class="empty">Cargando respaldos...</div></div><div class="alert alert-warning mt-3 mb-0"><strong>Restauración segura:</strong> detén Pangolite, copia el respaldo elegido sobre la base activa y reinicia el servicio. Ejemplo: <code>sudo systemctl stop pangolite && sudo cp /opt/pangolite/data/backups/ARCHIVO.db /opt/pangolite/data/pangolite.db && sudo systemctl start pangolite</code>.</div></div></div>
        <div class="card"><div class="card-header"><div class="toolbar mb-0"><div><div class="card-title">Auditoría</div><div class="text-muted small">Cambios administrativos recientes sin exponer secretos.</div></div><button class="btn btn-outline-secondary" onclick="loadAudit()"><i class="bi bi-arrow-clockwise"></i> Actualizar</button></div></div><div class="card-body"><div id="auditRows" class="table-responsive"><div class="empty">Cargando auditoría...</div></div></div></div>
      </div>
    </section>

    <section id="view-settings" class="route-view">
      <div class="hero"><h1>Ajustes</h1><p>Dominio del panel, DNS, dominios administrados, ACME y Traefik.</p></div>
      <div class="card mb-3"><div class="card-header"><div class="toolbar mb-0"><div><div class="card-title">Dominio del dashboard</div><div class="text-muted small">Define el dominio publico del panel. Pangolite valida que apunte a la IP del servidor antes de guardarlo.</div></div><button class="btn btn-help" onclick="showHelp('Dominio del dashboard','Este dominio publica el panel administrativo por Traefik. Debe resolver a la IP publica detectada del servidor. El correo ACME se usa para Let\'s Encrypt.')"><i class="bi bi-question"></i></button></div></div><div class="card-body"><form id="dashboardSettingsForm"><div class="grid grid-2"><div><label class="form-label">Dominio del panel</label><input id="dashboardDomain" class="form-control" placeholder="pangolin.yahirex.us.kg"><div class="form-text">Debe apuntar a la IP publica del servidor.</div></div><div><label class="form-label">Correo ACME</label><input id="letsEncryptEmail" type="email" class="form-control" placeholder="admin@yahirex.us.kg"><div class="form-text">Usado por Let’s Encrypt para emitir/renovar certificados.</div></div></div><div class="grid grid-2 mt-3"><div class="stat"><div class="stat-label">IP publica detectada</div><div class="font-mono text-white" id="serverIpHint">-</div></div><div class="stat"><div class="stat-label">DNS del dashboard</div><div class="font-mono text-white" id="dashboardDnsHint">-</div></div></div><button class="btn btn-primary mt-3" type="submit"><i class="bi bi-save"></i> Guardar ajustes</button></form></div></div>
      <div class="card mb-3"><div class="card-header"><div class="toolbar mb-0"><div><div class="card-title">Dominios administrados</div><div class="text-muted small">Configura domain.tld para que al crear proxys solo elijas base + subdominio.</div></div><div class="d-flex gap-2"><button class="btn btn-help" onclick="showHelp('Dominios','Agrega dominios base controlados por el panel. Ejemplo: yahirex.us.kg. Al crear un proxy podras elegir app + yahirex.us.kg o Custom para un dominio completo.')"><i class="bi bi-question"></i></button><button class="btn btn-primary" onclick="openDomainModal()"><i class="bi bi-plus-lg"></i> Agregar dominio</button></div></div></div><div class="card-body table-responsive"><table class="table"><thead><tr><th>Dominio</th><th>Estado</th><th>Creado</th><th class="text-end">Acciones</th></tr></thead><tbody id="domainRows"></tbody></table></div></div>
      <div class="card"><div class="card-header d-flex justify-content-between align-items-center"><div><div class="card-title">Config dinamica</div><div class="text-muted small">Traefik lee automaticamente los cambios HTTP/HTTPS. Los puertos TCP/UDP se aplican con reinicio controlado cuando cambian entrypoints.</div></div><button class="btn btn-help" onclick="showHelp('Traefik','HTTP/HTTPS y el dominio del panel se aplican por configuracion dinamica vigilada por Traefik. Solo TCP/UDP nuevos requieren actualizar entrypoints estaticos.')"><i class="bi bi-question"></i></button></div><div class="card-body"><button class="btn btn-primary mb-3" onclick="renderTraefik()"><i class="bi bi-arrow-repeat"></i> Aplicar configuracion ahora</button><pre id="config" class="font-mono p-3 rounded" style="background:#000;border:1px solid var(--pg-border);max-height:520px;overflow:auto"></pre></div></div>
    </section>
  </main>
</div>
<div class="modal-lite" id="projectModal"><div class="modal-card"><div class="d-flex justify-content-between align-items-start gap-3 mb-3"><div><h2 class="h5 text-white">Crear proyecto</h2><p class="text-muted mb-0">Nombre unico por cliente, empresa, VPS o entorno.</p></div><button class="btn btn-outline-secondary" onclick="closeProjectModal()"><i class="bi bi-x-lg"></i> Cerrar</button></div><form id="projectForm"><label class="form-label" for="projectName">Nombre</label><input class="form-control mb-3" id="projectName" placeholder="Cliente ACME" required><label class="form-label" for="projectNotes">Notas</label><textarea class="form-control mb-3" id="projectNotes" rows="3" placeholder="Contacto, uso, fecha de pago, observaciones"></textarea><button class="btn btn-primary" type="submit"><i class="bi bi-plus-circle"></i> Crear proyecto</button></form></div></div>
<div class="modal-lite" id="domainModal"><div class="modal-card"><div class="d-flex justify-content-between align-items-start gap-3 mb-3"><div><h2 class="h5 text-white">Agregar dominio</h2><p class="text-muted mb-0">Registra domain.tld o un subdominio base controlado por el panel.</p></div><button class="btn btn-outline-secondary" onclick="closeDomainModal()"><i class="bi bi-x-lg"></i> Cerrar</button></div><form id="domainForm"><label class="form-label" for="managedDomainInput">Dominio</label><input class="form-control mb-3" id="managedDomainInput" placeholder="yahirex.us.kg" required><button class="btn btn-primary" type="submit"><i class="bi bi-plus-circle"></i> Agregar dominio</button></form></div></div>
<div class="modal-lite" id="resourceEditModal"><div class="modal-card"><div class="d-flex justify-content-between align-items-start gap-3 mb-3"><div><h2 class="h5 text-white">Editar recurso</h2><p class="text-muted mb-0">Actualiza entrada publica, ubicación y servicio interno. Pangolite aplicara Traefik automaticamente.</p></div><button class="btn btn-outline-secondary" onclick="closeResourceEditModal()"><i class="bi bi-x-lg"></i> Cerrar</button></div><form id="resourceEditForm" class="resource-edit-grid"><input type="hidden" id="editResourceId"><div class="grid grid-2"><div><label class="form-label">Nombre del recurso</label><input id="editResourceName" class="form-control" required></div><div><label class="form-label">Tipo de publicacion</label><select id="editMode" class="form-select"><option value="http">Web HTTP/HTTPS</option><option value="tcp">TCP</option><option value="udp">UDP</option></select></div></div><div class="grid grid-2 mt-3"><div><label class="form-label">Ubicación del servicio</label><select id="editOriginType" class="form-select"><option value="local">En este servidor Pangolite</option><option value="agent">En un servidor remoto conectado</option></select></div><div id="editAgentOriginGroup" class="d-none"><label class="form-label">Servidor remoto conectado</label><select id="editAgentId" class="form-select"><option value="">Selecciona un cliente</option></select></div></div><div id="editAgentTcpUdpNotice" class="alert alert-warning mt-3 d-none"><i class="bi bi-exclamation-triangle"></i> TCP remoto usa stream persistente por el cliente NAT. UDP remoto usa intercambio de datagramas con respuesta.</div><div class="grid grid-2 mt-3 edit-http-only"><div><label class="form-label">Dominio</label><input id="editDomain" class="form-control" placeholder="app.domain.tld"></div><div><label class="form-label">Path</label><input id="editPathPrefix" class="form-control" value="/"></div></div><div class="grid grid-2 mt-3 edit-http-only"><div><label class="form-label">TLS</label><select id="editTLS" class="form-select"><option value="true">HTTPS con ACME</option><option value="false">HTTP</option></select></div><div><label class="form-label">Protocolo interno</label><select id="editBackendScheme" class="form-select"><option value="http">http</option><option value="https">https</option></select></div></div><div class="grid grid-2 mt-3 edit-tcpudp-only d-none"><div><label class="form-label">Puerto publico</label><input id="editPublicPort" type="number" class="form-control" placeholder="2222"></div><div class="main-layout-note d-flex align-items-end">Si cambias el puerto publico TCP/UDP, Pangolite reiniciara Traefik de forma controlada porque cambia un entrypoint estatico.</div></div><div class="grid grid-2 mt-3"><div><label class="form-label">Host interno del servicio</label><input id="editBackendHost" class="form-control" required></div><div><label class="form-label">Puerto interno del servicio</label><input id="editBackendPort" type="number" class="form-control" required></div></div><div class="grid grid-2 mt-3"><div><label class="form-label">Estado</label><select id="editResourceEnabled" class="form-select"><option value="true">Activo</option><option value="false">Suspendido</option></select></div><div><label class="form-label">Respuesta al suspender</label><select id="editDisabledResponseMode" class="form-select"><option value="403">403 prohibido</option><option value="404">404 no encontrado</option><option value="html">HTML personalizado</option></select></div></div><div class="grid grid-2 mt-3 edit-html-control d-none"><div><label class="form-label">Status del HTML</label><select id="editDisabledStatusCode" class="form-select"><option value="403">403</option><option value="404">404</option><option value="200">200</option></select></div><div><label class="form-label">Plantilla</label><select id="editDisabledPreset" class="form-select"><option value="">Sin preset</option><option value="payment">Pago pendiente</option><option value="maintenance">Mantenimiento</option><option value="suspended">Servicio suspendido</option></select></div></div><div class="mt-3 edit-html-control d-none"><label class="form-label">HTML personalizado</label><textarea id="editDisabledHtml" class="form-control font-mono" rows="7"></textarea></div><div class="d-flex gap-2 mt-3"><button class="btn btn-primary" type="submit"><i class="bi bi-save"></i> Guardar cambios</button><button class="btn btn-outline-secondary" type="button" onclick="closeResourceEditModal()">Cancelar</button></div></form></div></div>
<div class="modal-lite" id="helpModal"><div class="modal-card"><div class="d-flex justify-content-between align-items-start gap-3"><div><h2 class="h5 text-white" id="helpTitle">Ayuda</h2><p id="helpBody" class="text-muted mb-0"></p></div><button class="btn btn-outline-secondary" onclick="closeHelp()"><i class="bi bi-x-lg"></i> Cerrar</button></div></div></div>
<div class="modal-lite" id="confirmModal" role="dialog" aria-modal="true"><div class="modal-card confirm-card"><div class="d-flex gap-3 align-items-start"><div class="confirm-icon"><i class="bi bi-exclamation-triangle"></i></div><div class="flex-grow-1"><h2 class="h5 text-white" id="confirmTitle">Confirmar accion</h2><p class="text-muted mb-0" id="confirmBody">Esta accion requiere confirmacion.</p></div></div><div class="d-flex justify-content-end gap-2 mt-4"><button class="btn btn-outline-secondary" type="button" id="confirmCancelBtn">Cancelar</button><button class="btn btn-outline-danger" type="button" id="confirmAcceptBtn">Confirmar</button></div></div></div>
<div class="modal-lite" id="passwordConfirmModal" role="dialog" aria-modal="true"><div class="modal-card confirm-card"><div class="d-flex gap-3 align-items-start"><div class="confirm-icon"><i class="bi bi-shield-lock"></i></div><div class="flex-grow-1"><h2 class="h5 text-white" id="passwordConfirmTitle">Confirmar con contraseña</h2><p class="text-muted mb-3" id="passwordConfirmBody">Escribe tu contraseña para continuar.</p><label class="form-label" id="passwordConfirmInputLabel" for="passwordConfirmInput">Contraseña del administrador</label><input id="passwordConfirmInput" class="form-control password-confirm-input" type="password" autocomplete="current-password"></div></div><div class="d-flex justify-content-end gap-2 mt-4"><button class="btn btn-outline-secondary" type="button" id="passwordConfirmCancelBtn">Cancelar</button><button class="btn btn-outline-danger" type="button" id="passwordConfirmAcceptBtn"><i class="bi bi-trash3"></i> Confirmar eliminación</button></div></div></div>
<div class="modal-lite" id="busyModal" role="dialog" aria-modal="true"><div class="modal-card busy-card"><div class="busy-spinner" aria-hidden="true"></div><h2 class="h5 text-white" id="busyTitle">Procesando</h2><p class="text-muted mb-0 busy-dots" id="busyBody">Validando y aplicando cambios</p><div class="main-layout-note mt-3">No cierres esta ventana. Algunas verificaciones de puertos y Traefik pueden tardar unos segundos.</div></div></div><div class="modal-lite" id="agentDetailModal"><div class="modal-card"><div class="d-flex justify-content-between align-items-start gap-3 mb-3"><div><h2 class="h5 text-white" id="agentDetailTitle">Cliente NAT</h2><p class="text-muted mb-0" id="agentDetailMeta">Estado y recursos asociados.</p></div><button class="btn btn-outline-secondary" onclick="closeAgentDetail()"><i class="bi bi-x-lg"></i> Cerrar</button></div><div id="agentDetailBody"></div></div></div>
<script>
let csrf='';let projects=[];let stats={};let resources=[];let agents=[];let domains=[];let panelSettings={};let networkInfo={};let currentProject=null;let charts={};let logsLines=[];let auditEvents=[];let backups=[];let resourceHealth={};let deletingResources=new Set();
const $=id=>document.getElementById(id);
const domainRe=/^([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/i;
const labelIcon={projects:'bi-building',settings:'bi-sliders'};
const presets={payment:'<!doctype html><html lang="es"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Pago pendiente</title><body style="margin:0;min-height:100vh;display:grid;place-items:center;background:#000;color:#fff;font-family:system-ui"><main style="max-width:560px;padding:28px;border:1px solid rgba(255,255,255,.2);border-radius:16px;background:rgba(255,255,255,.08)"><h1>Servicio temporalmente suspendido</h1><p>El acceso esta pausado por pago pendiente.</p></main></body></html>',maintenance:'<!doctype html><html lang="es"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Mantenimiento</title><body style="margin:0;min-height:100vh;display:grid;place-items:center;background:#000;color:#fff;font-family:system-ui"><main style="max-width:560px;padding:28px;border:1px solid rgba(255,255,255,.2);border-radius:16px;background:rgba(255,255,255,.08)"><h1>Mantenimiento programado</h1><p>El servicio volvera pronto.</p></main></body></html>',suspended:'<!doctype html><html lang="es"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Servicio suspendido</title><body style="margin:0;min-height:100vh;display:grid;place-items:center;background:#000;color:#fff;font-family:system-ui"><main style="max-width:560px;padding:28px;border:1px solid rgba(255,255,255,.2);border-radius:16px;background:rgba(255,255,255,.08)"><h1>Servicio no disponible</h1><p>Este recurso esta deshabilitado temporalmente.</p></main></body></html>'};
function esc(s){return String(s??'').replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]))}
function msg(t,bad=false){const m=$('msg');m.className='alert '+(bad?'alert-danger':'alert-success');m.textContent=t;m.classList.remove('d-none');setTimeout(()=>m.classList.add('d-none'),7000);scrollTo({top:0,behavior:'smooth'})}
function showHelp(t,b){$('helpTitle').textContent=t;$('helpBody').textContent=b;$('helpModal').classList.add('open')}function closeHelp(){$('helpModal').classList.remove('open')}
let commandCopies={};
function rememberCopy(value){const id='copy_'+Math.random().toString(36).slice(2);commandCopies[id]=String(value||'');return id}
function copyButton(id,label='Copiar'){return '<button class="btn btn-sm btn-outline-secondary" type="button" onclick="copyCommand(\''+id+'\')"><i class="bi bi-clipboard"></i> '+esc(label)+'</button>'}
function commandBlock(title,cmd){const id=rememberCopy(cmd);return '<div class="command-card"><div class="command-head"><span>'+esc(title)+'</span>'+copyButton(id)+'</div><pre><code>'+esc(cmd||'')+'</code></pre></div>'}
function secretBlock(label,value){const id=rememberCopy(value);return '<div class="secret-row"><div><div class="text-muted small">'+esc(label)+'</div><code>'+esc(value||'')+'</code></div>'+copyButton(id)+'</div>'}
async function copyCommand(id){const value=commandCopies[id]||'';try{if(navigator.clipboard&&window.isSecureContext){await navigator.clipboard.writeText(value)}else{const ta=document.createElement('textarea');ta.value=value;ta.style.position='fixed';ta.style.opacity='0';document.body.appendChild(ta);ta.focus();ta.select();document.execCommand('copy');ta.remove()}msg('Copiado al portapapeles')}catch(err){msg('No se pudo copiar automaticamente',true)}}
function renderAgentCredentials(a){const remove=a.removeCommand||'sudo /opt/pangolite-client/pangolite-client --remove';const wremove=a.windowsRemoveCommand||"Start-Process -Verb RunAs 'C:\\ProgramData\\Pangolite Client\\pangolite-client.exe' -ArgumentList '--remove'";return '<div class="command-stack"><div class="secret-grid">'+secretBlock('ID del cliente',a.id||'')+secretBlock('Token',a.token||'')+'</div>'+commandBlock('Instalar Linux systemd/OpenRC',a.installCommand||'')+commandBlock('Eliminar Linux por completo',remove)+commandBlock('Instalar Windows como servicio',a.windowsInstallCommand||'')+commandBlock('Eliminar Windows por completo',wremove)+'</div>'}
function showBusy(title,body){$('busyTitle').textContent=title||'Procesando';$('busyBody').textContent=body||'Validando y aplicando cambios';$('busyModal').classList.add('open')}
function hideBusy(){$('busyModal').classList.remove('open')}
function confirmAction(title,body,confirmText='Confirmar',danger=true){return new Promise(resolve=>{const modal=$('confirmModal');$('confirmTitle').textContent=title;$('confirmBody').textContent=body;$('confirmAcceptBtn').innerHTML='<i class="bi bi-check2"></i> '+esc(confirmText);$('confirmAcceptBtn').className=danger?'btn btn-outline-danger':'btn btn-primary';const finish=v=>{modal.classList.remove('open');resolve(v)};$('confirmCancelBtn').onclick=()=>finish(false);$('confirmAcceptBtn').onclick=()=>finish(true);modal.classList.add('open')})}
let passwordConfirmResolver=null;
function resetPasswordConfirmModal(){const input=$('passwordConfirmInput');$('passwordConfirmInputLabel').textContent='Contraseña del administrador';input.type='password';input.autocomplete='current-password';input.placeholder='';input.className='form-control password-confirm-input';setIfExists('passwordConfirmInput','');$('passwordConfirmAcceptBtn').className='btn btn-outline-danger';$('passwordConfirmAcceptBtn').innerHTML='<i class="bi bi-trash3"></i> Confirmar eliminación'}
function closePasswordConfirm(){const resolver=passwordConfirmResolver;passwordConfirmResolver=null;$('passwordConfirmModal').classList.remove('open');resetPasswordConfirmModal();if(resolver)resolver(null)}
function confirmInputAction(opts={}){return new Promise(resolve=>{const modal=$('passwordConfirmModal');const input=$('passwordConfirmInput');if(passwordConfirmResolver)passwordConfirmResolver(null);passwordConfirmResolver=resolve;$('passwordConfirmTitle').textContent=opts.title||'Confirmar accion';$('passwordConfirmBody').textContent=opts.body||'Confirma para continuar.';$('passwordConfirmInputLabel').textContent=opts.label||'Valor';input.type=opts.type||'text';input.autocomplete=opts.autocomplete||'off';input.placeholder=opts.placeholder||'';input.className='form-control '+(opts.inputClass||'');setIfExists('passwordConfirmInput',opts.value||'');const icon=opts.icon||'bi-check2';$('passwordConfirmAcceptBtn').innerHTML='<i class="bi '+icon+'"></i> '+esc(opts.confirmText||'Confirmar');$('passwordConfirmAcceptBtn').className=opts.danger?'btn btn-outline-danger':'btn btn-primary';const finish=accepted=>{if(passwordConfirmResolver!==resolve)return;passwordConfirmResolver=null;const value=fieldValue('passwordConfirmInput');modal.classList.remove('open');resetPasswordConfirmModal();resolve(accepted?value:null)};$('passwordConfirmCancelBtn').onclick=()=>finish(false);$('passwordConfirmAcceptBtn').onclick=()=>finish(true);input.onkeydown=e=>{if(e.key==='Enter'){e.preventDefault();finish(true)}else if(e.key==='Escape'){e.preventDefault();finish(false)}};modal.classList.add('open');setTimeout(()=>input.focus(),50)})}
function confirmPasswordAction(title,body,confirmText='Confirmar eliminación'){return confirmInputAction({title,body,label:'Contraseña del administrador',type:'password',autocomplete:'current-password',confirmText,icon:'bi-trash3',danger:true,inputClass:'password-confirm-input'})}
function openProjectModal(){$('projectModal').classList.add('open');setTimeout(()=>$('projectName').focus(),50)}function closeProjectModal(){$('projectModal').classList.remove('open')}
function openDomainModal(){$('domainModal').classList.add('open');setTimeout(()=>$('managedDomainInput').focus(),50)}function closeDomainModal(){$('domainModal').classList.remove('open')}
function closeResourceEditModal(){$('resourceEditModal').classList.remove('open')}
function toggleProjectSwitcher(){const sw=$('projectSwitcher');sw.classList.toggle('open');if(sw.classList.contains('open'))setTimeout(()=>$('projectSearch').focus(),50)}
function closeProjectSwitcher(){if($('projectSwitcher'))$('projectSwitcher').classList.remove('open')}
function toggleSidebar(){if(matchMedia('(max-width:960px)').matches){document.body.classList.toggle('sidebar-open')}else{document.body.classList.toggle('sidebar-collapsed');localStorage.setItem('pangolite.sidebarCollapsed',document.body.classList.contains('sidebar-collapsed')?'1':'0')}}
document.addEventListener('keydown',e=>{if(e.key==='Escape'){closeHelp();closeProjectModal();closeDomainModal();closeAgentDetail();closeResourceEditModal();closePasswordConfirm()}});['helpModal','projectModal','domainModal','resourceEditModal','agentDetailModal'].forEach(id=>$(id).addEventListener('click',e=>{if(e.target===$(id))$(id).classList.remove('open')}));$('passwordConfirmModal').addEventListener('click',e=>{if(e.target===$('passwordConfirmModal'))closePasswordConfirm()});
function shortID(id){id=String(id||'');return id.length>10?id.slice(0,6)+'...'+id.slice(-4):id}function fmt(v){if(!v||v==='0001-01-01T00:00:00Z')return '-';try{return new Date(v).toLocaleString()}catch{return v}}
async function api(url,opt={}){opt.headers=Object.assign({'Content-Type':'application/json'},opt.headers||{});if(opt.method&&opt.method!=='GET')opt.headers['X-CSRF-Token']=csrf;let res;try{res=await fetch(url,opt)}catch(err){throw new Error('No se pudo conectar con Pangolite. Si acabas de cambiar puertos TCP/UDP, espera unos segundos y vuelve a intentar.')}const text=await res.text();let data={};try{data=text?JSON.parse(text):{}}catch{data={raw:text}}if(!res.ok){if(res.status===401)location.href='/login';throw new Error(data.error||res.statusText)}return data}
function go(path){history.pushState({},'',path);route()}window.addEventListener('popstate',route);document.addEventListener('click',e=>{const a=e.target.closest('a[href^="/"]');if(!a)return;const href=a.getAttribute('href')||'';if(a.target||a.hasAttribute('download')||href.startsWith('/api/'))return;e.preventDefault();go(href)});
async function init(){const s=await fetch('/api/session').then(r=>r.json());if(!s.authenticated){location.href='/login';return}if(s.user.forcePasswordChange){location.href='/password';return}csrf=s.csrfToken;$('userLabel').textContent=s.user.username;await reloadDomains();await loadSettings();await reloadProjects();await route();}
async function reloadProjects(){const data=await api('/api/projects');projects=data.projects||[];stats=data.stats||{};paintProjectNav();paintProjectTable();let tr=0,ta=0,active=0;Object.values(stats).forEach(x=>{tr+=x.resources||0;ta+=x.agents||0;active+=x.activeResources||0});$('statProjects').textContent=projects.length;$('statResources').textContent=tr;$('statAgents').textContent=ta;if($('statActiveResources'))$('statActiveResources').textContent=active;renderGlobalDashboard();}
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
async function loadSettings(){const data=await api('/api/settings');panelSettings=data.settings||{};networkInfo=data.network||{};if($('dashboardDomain'))$('dashboardDomain').value=panelSettings.dashboardDomain||'';if($('letsEncryptEmail'))$('letsEncryptEmail').value=panelSettings.letsEncryptEmail||'';paintNetworkInfo()}
function paintNetworkInfo(){const ip=networkInfo.publicIp||'No detectada';const domain=panelSettings.dashboardDomain||'';const ips=(networkInfo.dashboardDomainIps||[]).join(', ');const dnsText=ips?(ips+(networkInfo.dnsMatchesServer?' ✓':' ✗')):'Sin DNS consultado';if($('serverIpHint'))$('serverIpHint').textContent=ip;if($('dashboardDnsHint'))$('dashboardDnsHint').textContent=dnsText;if($('dashPanelDomain'))$('dashPanelDomain').textContent=domain||'Sin configurar';if($('dashPublicIp'))$('dashPublicIp').textContent=ip;if($('dashDnsState')){const el=$('dashDnsState');el.classList.remove('good','warn','bad');if(!domain){el.textContent='Sin dominio';el.classList.add('warn')}else if(networkInfo.dnsMatchesServer){el.textContent='Correcto';el.classList.add('good')}else if(ips){el.textContent='No coincide';el.classList.add('bad')}else{el.textContent='Pendiente';el.classList.add('warn')}}}
async function saveSettings(e){e.preventDefault();try{const payload={dashboardDomain:$('dashboardDomain').value.trim().toLowerCase(),letsEncryptEmail:$('letsEncryptEmail').value.trim().toLowerCase()};const data=await api('/api/settings',{method:'PATCH',body:JSON.stringify(payload)});panelSettings=data.settings||{};networkInfo=data.network||{};paintNetworkInfo();const t=data.traefik||{};msg(t.message||'Ajustes guardados. Traefik se actualizo automaticamente.')}catch(err){msg(err.message,true)}}
async function reloadDomains(){const data=await api('/api/domains');domains=data.domains||[];paintDomains();fillDomainSelect();}

async function loadLogs(){try{const data=await api('/api/system/logs?limit=300');logsLines=data.lines||[];const box=$('logsBox');if($('logsPath'))$('logsPath').textContent='Archivo: '+(data.path||'no configurado')+' · maximo '+(data.maxEntries||1000)+' entradas';if(!box)return;if(!logsLines.length){box.textContent='Sin eventos registrados todavia.';return}box.innerHTML=logsLines.map(line=>'<span class="log-line '+(line.includes('level=ERROR')?'error':line.includes('level=WARN')?'warn':'')+'">'+esc(line)+'<\/span>').join('');box.scrollTop=box.scrollHeight}catch(err){if($('logsBox'))$('logsBox').textContent='No se pudieron cargar logs: '+err.message}}
async function copyLogs(){try{await navigator.clipboard.writeText(logsLines.join('\n'));msg('Logs copiados')}catch(err){msg('No se pudieron copiar logs: '+err.message,true)}}

async function loadAudit(){try{const data=await api('/api/audit?limit=200');auditEvents=data.events||[];paintAudit()}catch(err){const box=$('auditRows');if(box)box.innerHTML='<div class="empty">No se pudo cargar auditoría: '+esc(err.message)+'</div>'}}
function paintAudit(){const box=$('auditRows');if(!box)return;if(!auditEvents.length){box.innerHTML='<div class="empty">Sin eventos de auditoría todavía.</div>';return}let html='<table class="table"><thead><tr><th>Fecha</th><th>Acción</th><th>Entidad</th><th>Usuario</th><th>Detalle</th></tr></thead><tbody>';auditEvents.forEach(ev=>{let meta=ev.metadata||'';try{const obj=JSON.parse(meta);meta=Object.entries(obj).map(([k,v])=>k+': '+String(v)).join(' · ')}catch{}html+='<tr><td class="font-mono">'+esc(fmt(ev.createdAt))+'</td><td>'+esc(ev.action)+'</td><td><span class="font-mono">'+esc(ev.entityType)+'</span><div class="text-muted small font-mono">'+esc(ev.entityId||'-')+'</div></td><td>'+esc(ev.username||'-')+'</td><td class="small">'+esc(meta||ev.remoteIp||'-')+'</td></tr>'});html+='</tbody></table>';box.innerHTML=html}
async function loadBackups(){try{const data=await api('/api/backups');backups=data.backups||[];if($('backupDirHint'))$('backupDirHint').textContent='Ruta: '+(data.backupDir||'-');paintBackups()}catch(err){const box=$('backupRows');if(box)box.innerHTML='<div class="empty">No se pudieron cargar respaldos: '+esc(err.message)+'</div>'}}
function paintBackups(){const box=$('backupRows');if(!box)return;if(!backups.length){box.innerHTML='<div class="empty">Aún no hay respaldos. Crea uno antes de cambios destructivos.</div>';return}let html='<table class="table"><thead><tr><th>Archivo</th><th>Tamaño</th><th>Creado</th><th class="text-end">Acciones</th></tr></thead><tbody>';backups.forEach(b=>{html+='<tr><td class="font-mono">'+esc(b.name)+'</td><td>'+esc(formatBytes(b.sizeBytes||0))+'</td><td class="font-mono">'+esc(fmt(b.createdAt))+'</td><td class="text-end"><a class="btn btn-sm btn-outline-secondary" download href="/api/backups/'+encodeURIComponent(b.name)+'/download"><i class="bi bi-download"></i> Descargar</a></td></tr>'});html+='</tbody></table>';box.innerHTML=html}
function formatBytes(n){n=Number(n||0);if(n<1024)return n+' B';if(n<1024*1024)return (n/1024).toFixed(1)+' KB';return (n/1024/1024).toFixed(2)+' MB'}
async function createBackup(){try{const label=await confirmInputAction({title:'Crear respaldo SQLite',body:'Escribe un prefijo opcional para identificar el respaldo. Dejalo vacio para usar el nombre automatico.',label:'Prefijo del respaldo',placeholder:'antes-eliminar-cliente',confirmText:'Crear respaldo',icon:'bi-database-add',danger:false});if(label===null)return;showBusy('Creando respaldo','SQLite genera una copia consistente sin detener el panel');const backup=await api('/api/backups',{method:'POST',body:JSON.stringify({label})});msg('Respaldo creado: '+backup.name);await loadBackups();await loadAudit()}catch(err){msg(err.message,true)}finally{hideBusy()}}
async function loadMaintenance(){await Promise.all([loadBackups(),loadAudit()])}

function paintProjectNav(filter=''){const box=$('projectNav');box.innerHTML='';const q=String(filter||'').trim().toLowerCase();const list=projects.filter(p=>!q||p.name.toLowerCase().includes(q)||p.slug.toLowerCase().includes(q)||p.id.toLowerCase().includes(q));if(!list.length){box.innerHTML='<div class="empty py-3">Sin resultados</div>';return}list.forEach(p=>{const st=stats[p.id]||{};const btn=document.createElement('button');btn.type='button';btn.className='project-result'+(currentProject&&currentProject.id===p.id?' active':'');btn.dataset.projectNav=p.id;btn.innerHTML='<span><span class="fw-semibold">'+esc(p.name)+'</span><div class="project-meta">'+esc(p.slug)+' · '+(st.resources||0)+' recursos · '+(st.agents||0)+' agentes</div></span><i class="bi bi-arrow-right-short"></i>';btn.addEventListener('click',()=>{closeProjectSwitcher();go('/projects/'+p.id)});box.appendChild(btn)});updateProjectSwitcherLabel()}
function updateProjectSwitcherLabel(){const label=$('currentProjectLabel');if(!label)return;label.textContent=currentProject?currentProject.name:'Selecciona proyecto'}
function paintProjectTable(){const rows=$('projectRows');rows.innerHTML='';if(!projects.length){rows.innerHTML='<tr><td colspan="7" class="empty">No hay proyectos. Usa + Crear para agregar el primer cliente.</td></tr>';return}projects.forEach(p=>{const st=stats[p.id]||{};const tr=document.createElement('tr');tr.innerHTML='<td>'+(p.enabled?'<span class="state-pill on"><span class="status-dot ok"></span>Activo</span>':'<span class="state-pill off"><span class="status-dot"></span>Inactivo</span>')+'</td><td class="fw-semibold text-white">'+esc(p.name)+'</td><td class="font-mono">/'+esc(p.slug)+'</td><td>'+(st.resources||0)+'</td><td>'+(st.agents||0)+'</td><td>'+(st.activeResources||0)+'</td><td class="text-end"><a class="btn btn-sm btn-primary me-1" href="/projects/'+p.id+'"><i class="bi bi-box-arrow-in-right"></i> Abrir</a><a class="btn btn-sm btn-outline-secondary" href="/projects/'+p.id+'"><i class="bi bi-pencil-square"></i> Configurar</a></td>';rows.appendChild(tr)})}
function paintDomains(){const rows=$('domainRows');if(!rows)return;rows.innerHTML='';if(!domains.length){rows.innerHTML='<tr><td colspan="4" class="empty">Sin dominios administrados. Agrega domain.tld antes de crear proxys HTTP.</td></tr>';return}domains.forEach(d=>{const tr=document.createElement('tr');tr.innerHTML='<td class="font-mono text-white">'+esc(d.domain)+'</td><td>'+(d.enabled?'<span class="state-pill on"><span class="status-dot ok"></span>Activo</span>':'<span class="state-pill off"><span class="status-dot"></span>Inactivo</span>')+'</td><td>'+esc(fmt(d.createdAt))+'</td><td class="text-end"><button class="btn btn-sm btn-outline-danger" onclick="deleteDomain(\''+d.id+'\')"><i class="bi bi-trash"></i> Eliminar</button></td>';rows.appendChild(tr)})}
function fillDomainSelect(){const sel=$('domainSelect');if(!sel)return;sel.innerHTML='';domains.filter(d=>d.enabled).forEach(d=>{const opt=document.createElement('option');opt.value=d.domain;opt.textContent=d.domain;sel.appendChild(opt)});const custom=document.createElement('option');custom.value='custom';custom.textContent='Custom';sel.appendChild(custom);if(!domains.length)sel.value='custom';syncDomainMode()}
function paintProjectOverview(){$('projectHeroTitle').textContent=currentProject.name;$('projectHeroText').textContent=currentProject.notes||'Recursos y agentes separados por cliente.';$('projectResourceCount').textContent=resources.length;$('projectAgentCount').textContent=agents.length;$('projectActiveCount').textContent=resources.filter(r=>r.enabled).length;setIfExists('projectEditName',currentProject.name||'');setIfExists('projectEditNotes',currentProject.notes||'');const delBtn=maybeEl('deleteProjectBtn');const hint=maybeEl('projectDangerHint');const blocked=currentProject.id==='default'||resources.length>0||agents.length>0;if(delBtn){delBtn.disabled=blocked;delBtn.title=blocked?'Elimina primero recursos y clientes vinculados.':''}if(hint){if(currentProject.id==='default')hint.textContent='El proyecto General es base del sistema y no se puede eliminar.';else if(resources.length||agents.length)hint.textContent='Para eliminar este proyecto primero debe quedar sin recursos ni clientes. Actual: '+resources.length+' recurso(s), '+agents.length+' cliente(s).';else hint.textContent='El proyecto no tiene recursos ni clientes. Puedes eliminarlo si ya no se usa.'}const rr=$('projectResourceRows');rr.innerHTML='';resources.slice(0,6).forEach(r=>{rr.innerHTML+='<tr><td>'+state(r)+'</td><td>'+esc(r.mode.toUpperCase())+'</td><td>'+esc(r.name)+'</td><td class="font-mono">'+esc(label(r))+'</td></tr>'});if(!resources.length)rr.innerHTML='<tr><td colspan="4" class="empty">Sin recursos.</td></tr>';const ar=$('projectAgentRows');ar.innerHTML='';agents.slice(0,6).forEach(a=>{ar.innerHTML+='<tr><td>'+agentState(a)+'</td><td>'+esc(a.name)+'</td><td>'+esc(fmt(a.lastSeen))+'</td></tr>'});if(!agents.length)ar.innerHTML='<tr><td colspan="3" class="empty">Sin agentes.</td></tr>'}
function label(r){return r.mode==='http'?(r.domain+(r.pathPrefix||'/')):(r.mode.toUpperCase()+' :'+r.publicPort)}function state(r){return r.enabled?'<span class="state-pill on"><span class="status-dot ok"></span>Activo</span>':'<span class="state-pill off"><span class="status-dot"></span>Suspendido</span>'}
async function loadProjectData(id){resources=(await api('/api/resources?projectId='+encodeURIComponent(id))).resources||[];agents=(await api('/api/agents?projectId='+encodeURIComponent(id))).agents||[];paintResources();paintAgents();fillAgentSelect();}
function removeResourceLocal(id){resources=resources.filter(x=>x.id!==id);paintResources();if($('resourceControlSelect')&&$('resourceControlSelect').value===id){$('resourceControlSelect').value='';}}
async function refreshCurrentProjectSoft(){try{await reloadProjects();if(currentProject)await loadProjectData(currentProject.id);return true}catch(err){msg('Cambios guardados. Reintentando actualizar la tabla en unos segundos...');setTimeout(()=>{reloadProjects().then(()=>currentProject&&loadProjectData(currentProject.id)).catch(()=>{})},3500);return false}}
function healthBadge(id){const h=resourceHealth[id];if(!h)return '<span class="state-pill">Sin probar</span>';const cls=h.status==='ok'?'on':(h.status==='suspended'?'':'off');return '<span class="state-pill '+cls+'" title="'+esc(h.message||'')+'">'+esc(h.status||'unknown')+'</span>'}
function agentState(a){if(!a.enabled)return '<span class="status-dot"></span> Inactivo';if(a.online)return '<span class="status-dot ok"></span> Online';return '<span class="status-dot"></span> Offline'}
function paintResources(){const rows=$('resourcesRows');rows.innerHTML='';$('resourceControlSelect').innerHTML='<option value="">Selecciona recurso</option>';resources.forEach(r=>{const opt=document.createElement('option');opt.value=r.id;opt.textContent=r.name+' - '+label(r);$('resourceControlSelect').appendChild(opt);const response=r.enabled?'-':(r.disabledResponseMode==='html'?'HTML '+(r.disabledStatusCode||403):String(r.disabledStatusCode||r.disabledResponseMode));const origin=r.originType==='agent'?'Servidor remoto':'Este servidor';const tr=document.createElement('tr');tr.innerHTML='<td>'+state(r)+'</td><td><span class="badge badge-mode bg-primary">'+esc(r.mode.toUpperCase())+'</span></td><td class="fw-semibold">'+esc(r.name)+'</td><td class="font-mono">'+esc(label(r))+'</td><td class="font-mono">'+esc(r.backendHost+':'+r.backendPort)+'</td><td>'+esc(origin)+'</td><td>'+esc(r.agentId?shortID(r.agentId):'-')+'</td><td>'+healthBadge(r.id)+'</td><td>'+esc(response)+'</td><td class="text-end"><span class="resource-actions"></span></td>';const actions=tr.querySelector('.resource-actions');const primary=document.createElement('button');primary.className=r.enabled?'btn btn-sm btn-outline-danger me-1':'btn btn-sm btn-outline-secondary me-1';primary.innerHTML=r.enabled?'<i class="bi bi-pause-circle"></i> Suspender':'<i class="bi bi-play-circle"></i> Activar';primary.addEventListener('click',()=>r.enabled?quickSuspend(r.id,'403'):quickActivate(r.id));actions.appendChild(primary);const edit=document.createElement('button');edit.className='btn btn-sm btn-outline-secondary me-1';edit.innerHTML='<i class="bi bi-pencil-square"></i> Editar';edit.addEventListener('click',()=>openEditResource(r.id));actions.appendChild(edit);const control=document.createElement('button');control.className='btn btn-sm btn-outline-secondary me-1';control.innerHTML='<i class="bi bi-sliders"></i> Suspension';control.addEventListener('click',()=>selectResource(r.id));actions.appendChild(control);const del=document.createElement('button');del.className='btn btn-sm btn-outline-danger';del.innerHTML='<i class="bi bi-trash"></i> Eliminar';del.addEventListener('click',()=>deleteResource(r.id));actions.appendChild(del);rows.appendChild(tr)});if(!resources.length)rows.innerHTML='<tr><td colspan="10" class="empty">Sin recursos en este proyecto.</td></tr>'; }
function paintAgents(){const rows=$('agentsRows');rows.innerHTML='';agents.forEach(a=>{const system=[a.os,a.arch].filter(Boolean).join('/');const tr=document.createElement('tr');tr.innerHTML='<td>'+agentState(a)+'</td><td class="fw-semibold">'+esc(a.name)+'</td><td class="font-mono">'+esc(a.id)+'</td><td>'+esc(system||'-')+'<div class="project-meta">'+esc(a.hostname||a.privateIp||'')+'</div></td><td>'+esc(String(a.resourceCount||0))+'</td><td>'+esc(fmt(a.lastSeen))+'</td><td class="text-end"><span class="agent-actions"></span></td>';const actions=tr.querySelector('.agent-actions');const detail=document.createElement('button');detail.className='btn btn-sm btn-outline-secondary me-1';detail.innerHTML='<i class="bi bi-info-circle"></i> Detalle';detail.addEventListener('click',()=>showAgentDetail(a.id));actions.appendChild(detail);const rotate=document.createElement('button');rotate.className='btn btn-sm btn-outline-secondary me-1';rotate.innerHTML='<i class="bi bi-arrow-clockwise"></i> Rotar token';rotate.addEventListener('click',()=>rotateAgentToken(a.id));actions.appendChild(rotate);const disable=document.createElement('button');disable.className='btn btn-sm btn-outline-danger';disable.innerHTML='<i class="bi bi-trash3"></i> Eliminar';disable.addEventListener('click',()=>deleteAgent(a.id));actions.appendChild(disable);rows.appendChild(tr)});if(!agents.length)rows.innerHTML='<tr><td colspan="7" class="empty">Sin agentes en este proyecto.</td></tr>'; }

async function checkResourceHealth(){try{showBusy('Probando recursos','Consultando clientes NAT, puentes internos y backends');const data=await api('/api/resources/health?projectId='+encodeURIComponent(currentProject.id));resourceHealth={};(data.checks||[]).forEach(h=>resourceHealth[h.resourceId]=h);paintResources();msg('Health checks actualizados')}catch(err){msg(err.message,true)}finally{hideBusy()}}
async function showAgentDetail(id){try{const data=await api('/api/agents/'+id);const a=data.agent||{};const rs=data.resources||[];$('agentDetailTitle').textContent=a.name||'Cliente NAT';$('agentDetailMeta').textContent=(a.online?'Online':'Offline')+' · '+(a.os||'sistema desconocido')+'/'+(a.arch||'-')+' · '+(a.hostname||a.publicIp||'sin hostname');let html='<div class="grid grid-2 mb-3"><div class="ops-row"><div><div class="ops-label">IP publica</div><div class="ops-value">'+esc(a.publicIp||'-')+'</div></div></div><div class="ops-row"><div><div class="ops-label">IP privada</div><div class="ops-value">'+esc(a.privateIp||'-')+'</div></div></div><div class="ops-row"><div><div class="ops-label">Version</div><div class="ops-value">'+esc(a.version||'-')+'</div></div></div><div class="ops-row"><div><div class="ops-label">Ultima conexion</div><div class="ops-value">'+esc(fmt(a.lastSeen))+'</div></div></div></div>';html+='<div class="card"><div class="card-header"><div class="card-title">Recursos asociados</div></div><div class="card-body table-responsive"><table class="table"><thead><tr><th>Estado</th><th>Tipo</th><th>Nombre</th><th>Entrada</th><th>Backend</th></tr></thead><tbody>';if(rs.length){rs.forEach(r=>{html+='<tr><td>'+state(r)+'</td><td>'+esc(r.mode)+'</td><td>'+esc(r.name)+'</td><td class="font-mono">'+esc(label(r))+'</td><td class="font-mono">'+esc(r.backendHost+':'+r.backendPort)+'</td></tr>'})}else{html+='<tr><td colspan="5" class="empty">Este cliente no tiene recursos asociados.</td></tr>'}html+='</tbody></table></div></div>';$('agentDetailBody').innerHTML=html;$('agentDetailModal').classList.add('open')}catch(err){msg(err.message,true)}}
function closeAgentDetail(){$('agentDetailModal').classList.remove('open')}

function selectResource(id){const r=resources.find(x=>x.id===id);if(!r)return;$('resourceControlSelect').value=id;$('resourceEnabled').value=String(!!r.enabled);$('disabledResponseMode').value=r.disabledResponseMode||'403';$('disabledStatusCode').value=String(r.disabledStatusCode||403);$('disabledHtml').value=r.disabledHtml||'';$('disabledPreset').value='';syncDisabledMode();scrollTo({top:document.body.scrollHeight,behavior:'smooth'})}
async function saveResourceControl(){try{const id=$('resourceControlSelect').value;if(!id)throw new Error('Selecciona un recurso');await api('/api/resources/'+id,{method:'PATCH',body:JSON.stringify({enabled:$('resourceEnabled').value==='true',disabledResponseMode:$('disabledResponseMode').value,disabledStatusCode:Number($('disabledStatusCode').value),disabledHtml:$('disabledHtml').value})});msg('Control actualizado');await reloadProjects();await loadProjectData(currentProject.id)}catch(err){msg(err.message,true)}}
async function activateSelectedResource(){$('resourceEnabled').value='true';await saveResourceControl()}async function quickSuspend(id,mode){const r=resources.find(x=>x.id===id);if(!await confirmAction('Suspender recurso','El recurso '+(r?r.name:shortID(id))+' dejara de responder hacia su backend y mostrara la respuesta configurada.','Suspender',false))return;try{await api('/api/resources/'+id,{method:'PATCH',body:JSON.stringify({enabled:false,disabledResponseMode:mode,disabledStatusCode:mode==='404'?404:403,disabledHtml:''})});msg('Recurso suspendido');await reloadProjects();await loadProjectData(currentProject.id)}catch(err){msg(err.message,true)}}async function quickActivate(id){const r=resources.find(x=>x.id===id);if(!await confirmAction('Activar recurso','El recurso '+(r?r.name:shortID(id))+' volvera a enviar trafico al servicio interno.','Activar',false))return;try{await api('/api/resources/'+id,{method:'PATCH',body:JSON.stringify({enabled:true,disabledResponseMode:(r&&r.disabledResponseMode)||'403',disabledStatusCode:(r&&r.disabledStatusCode)||403,disabledHtml:(r&&r.disabledHtml)||''})});msg('Recurso activado');await reloadProjects();await loadProjectData(currentProject.id)}catch(err){msg(err.message,true)}}
async function saveProjectSettings(e){e.preventDefault();if(!currentProject){msg('Selecciona un proyecto',true);return false}try{const name=fieldValue('projectEditName');const notes=fieldValue('projectEditNotes');if(!name)throw new Error('Nombre de proyecto requerido');showBusy('Guardando proyecto','Actualizando nombre y descripción');const updated=await api('/api/projects/'+currentProject.id,{method:'PATCH',body:JSON.stringify({name,notes})});currentProject=updated;await reloadProjects();await loadProjectData(updated.id);paintProjectOverview();msg('Proyecto actualizado')}catch(err){msg(err.message,true)}finally{hideBusy()}return false}
async function deleteCurrentProject(){if(!currentProject)return;const st=stats[currentProject.id]||{};if(currentProject.id==='default'){msg('El proyecto General no se puede eliminar',true);return}if((st.resources||resources.length)>0||(st.agents||agents.length)>0){msg('Primero elimina todos los recursos y clientes de este proyecto',true);return}if(!await confirmAction('Eliminar proyecto','Se eliminara el proyecto '+currentProject.name+'. Esta accion no se puede deshacer.','Eliminar proyecto'))return;try{await api('/api/projects/'+currentProject.id,{method:'DELETE'});msg('Proyecto eliminado');currentProject=null;await reloadProjects();go('/projects')}catch(err){msg(err.message,true)}}
async function deleteAgent(id){const a=agents.find(x=>x.id===id);const count=a?(a.resourceCount||0):0;const body='Eliminar el cliente '+(a?a.name:shortID(id))+' tambien eliminara '+count+' recurso(s) vinculado(s). Escribe tu contraseña para confirmar.';const password=await confirmPasswordAction('Eliminar cliente NAT',body,'Eliminar cliente y recursos');if(!password)return;try{showBusy('Eliminando cliente NAT','Eliminando recursos vinculados y aplicando Traefik');const res=await api('/api/agents/'+id,{method:'DELETE',body:JSON.stringify({password})});msg('Cliente eliminado. Recursos eliminados: '+(res.deletedResources||0));await reloadProjects();if(currentProject)await loadProjectData(currentProject.id)}catch(err){msg(err.message,true)}finally{hideBusy()}}
async function rotateAgentToken(id){const a0=agents.find(x=>x.id===id);if(!await confirmAction('Rotar token del cliente','El token actual dejara de servir. Deberas reinstalar o actualizar el cliente '+(a0?a0.name:shortID(id))+' con el nuevo comando.','Rotar token'))return;try{const a=await api('/api/agents/'+id+'/token',{method:'POST',body:'{}'});$('agentTokenBox').innerHTML=renderAgentCredentials(a);$('agentTokenBox').classList.remove('d-none');msg('Token rotado. Copia el nuevo comando ahora.');await loadProjectData(currentProject.id)}catch(err){msg(err.message,true)}}
async function deleteDomain(id){if(!await confirmAction('Eliminar dominio administrado','Los recursos existentes no se borran, pero este dominio dejara de aparecer como opcion al crear recursos.','Eliminar dominio'))return;try{await api('/api/domains/'+id,{method:'DELETE'});msg('Dominio eliminado');await reloadDomains()}catch(err){msg(err.message,true)}}
async function renderTraefik(){try{const r=await api('/api/render-traefik',{method:'POST',body:'{}'});msg(r.message||'Traefik actualizado')}catch(err){msg(err.message,true)}}async function loadConfig(){$('config').textContent=await fetch('/api/v1/traefik-config').then(r=>r.text())}
async function route(){document.querySelectorAll('.route-view').forEach(v=>v.classList.remove('active'));document.querySelectorAll('.nav-link').forEach(a=>a.classList.remove('active'));const path=location.pathname;if(path==='/'||path==='/projects'){setTop('Dashboard','Operación global');$('view-projects').classList.add('active');document.querySelector('[data-nav="projects"]').classList.add('active');renderGlobalDashboard();return}if(path==='/logs'){setTop('Logs','Diagnostico del sistema');$('view-logs').classList.add('active');document.querySelector('[data-nav="logs"]').classList.add('active');await loadLogs();return}if(path==='/maintenance'){setTop('Seguridad','Auditoría y respaldos');$('view-maintenance').classList.add('active');document.querySelector('[data-nav="maintenance"]').classList.add('active');await loadMaintenance();return}if(path==='/settings'){setTop('Ajustes','Configuración del sistema');$('view-settings').classList.add('active');document.querySelector('[data-nav="settings"]').classList.add('active');await loadSettings();await reloadDomains();await loadConfig();return}const m=path.match(/^\/projects\/([^/]+)(?:\/(resources|agents)(?:\/(create))?|settings)?$/);if(!m){go('/projects');return}const id=m[1];const section=m[2]||'overview';const action=m[3]||'';currentProject=projects.find(p=>p.id===id);if(!currentProject){await reloadProjects();currentProject=projects.find(p=>p.id===id)}if(!currentProject){msg('Proyecto no encontrado',true);go('/projects');return}document.querySelectorAll('[data-project-nav="'+id+'"]').forEach(a=>a.classList.add('active'));paintProjectNav($('projectSearch')?$('projectSearch').value:'');updateProjectSwitcherLabel();await loadProjectData(id);$('goCreateResource').href='/projects/'+id+'/resources/create';$('goCreateAgent').href='/projects/'+id+'/agents/create';$('goResources').href='/projects/'+id+'/resources';$('goAgents').href='/projects/'+id+'/agents';const rbtn=$('goCreateResourceFromList');if(rbtn)rbtn.href='/projects/'+id+'/resources/create';const abtn=$('goCreateAgentFromList');if(abtn)abtn.href='/projects/'+id+'/agents/create';if(section==='resources'&&action==='create'){setTop(currentProject.name,'Crear recurso');$('view-create-resource').classList.add('active');fillDomainSelect();return}if(section==='agents'&&action==='create'){setTop(currentProject.name,'Crear cliente de sistema');$('view-create-agent').classList.add('active');return}if(section==='resources'){setTop(currentProject.name,'Recursos');$('view-resources').classList.add('active');return}if(section==='agents'){setTop(currentProject.name,'Clientes de sistema');$('view-agents').classList.add('active');return}setTop(currentProject.name,'Resumen');paintProjectOverview();$('view-project').classList.add('active')}
function setTop(crumb,title){$('crumb').textContent=crumb;$('topTitle').textContent=title}

function maybeEl(id){return document.getElementById(id)}
function fieldValue(id){const el=maybeEl(id);return el?String(el.value||'').trim():''}
function fieldNumber(id){const raw=fieldValue(id);return raw?Number(raw):0}
function setIfExists(id,value){const el=maybeEl(id);if(el)el.value=value}
function classToggleAll(selector,hide){document.querySelectorAll(selector).forEach(el=>el.classList.toggle('d-none',!!hide))}
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
  if(preview)preview.textContent=buildDomainFromCreateForm()||'-';
}
function syncMode(){
  const mode=fieldValue('mode')||'http';
  classToggleAll('.http-only',mode!=='http');
  classToggleAll('.tcpudp-only',!(mode==='tcp'||mode==='udp'));
  syncDomainMode();
  syncOrigin();
}
function syncOrigin(){
  const origin=fieldValue('originType')||'local';
  const group=maybeEl('agentOriginGroup');
  if(group)group.classList.toggle('d-none',origin!=='agent');
  const notice=maybeEl('agentTcpUdpNotice');
  const mode=fieldValue('mode')||'http';
  if(notice)notice.classList.toggle('d-none',!(origin==='agent'&&(mode==='tcp'||mode==='udp')));
}
function syncDisabledMode(){
  const mode=fieldValue('disabledResponseMode')||'403';
  document.querySelectorAll('.html-control').forEach(el=>el.classList.toggle('d-none',mode!=='html'));
}
function syncEditResourceMode(){
  const mode=fieldValue('editMode')||'http';
  document.querySelectorAll('.edit-http-only').forEach(el=>el.classList.toggle('d-none',mode!=='http'));
  document.querySelectorAll('.edit-tcpudp-only').forEach(el=>el.classList.toggle('d-none',!(mode==='tcp'||mode==='udp')));
  syncEditResourceOrigin();
}
function syncEditResourceOrigin(){
  const origin=fieldValue('editOriginType')||'local';
  const group=maybeEl('editAgentOriginGroup');
  if(group)group.classList.toggle('d-none',origin!=='agent');
  const notice=maybeEl('editAgentTcpUdpNotice');
  const mode=fieldValue('editMode')||'http';
  if(notice)notice.classList.toggle('d-none',!(origin==='agent'&&(mode==='tcp'||mode==='udp')));
}
function syncEditDisabledMode(){
  const mode=fieldValue('editDisabledResponseMode')||'403';
  document.querySelectorAll('.edit-html-control').forEach(el=>el.classList.toggle('d-none',mode!=='html'));
}
function disabledPresetHTML(value){
  if(value==='payment')return '<!doctype html><html><head><meta charset="utf-8"><title>Pago pendiente</title><style>body{font-family:system-ui;background:#0b0b10;color:#fff;display:grid;place-items:center;min-height:100vh;margin:0}.box{max-width:680px;padding:36px;border:1px solid rgba(255,255,255,.14);border-radius:24px;background:#14141d}p{color:#cfcfd8}</style></head><body><main class="box"><h1>Servicio suspendido temporalmente</h1><p>Este servicio se encuentra pausado por pago pendiente. Contacta al administrador para reactivarlo.</p></main></body></html>';
  if(value==='maintenance')return '<!doctype html><html><head><meta charset="utf-8"><title>Mantenimiento</title><style>body{font-family:system-ui;background:#0b0b10;color:#fff;display:grid;place-items:center;min-height:100vh;margin:0}.box{max-width:680px;padding:36px;border:1px solid rgba(255,255,255,.14);border-radius:24px;background:#14141d}p{color:#cfcfd8}</style></head><body><main class="box"><h1>Mantenimiento programado</h1><p>Estamos realizando tareas de mantenimiento. El servicio volvera a estar disponible pronto.</p></main></body></html>';
  if(value==='suspended')return '<!doctype html><html><head><meta charset="utf-8"><title>Servicio suspendido</title><style>body{font-family:system-ui;background:#0b0b10;color:#fff;display:grid;place-items:center;min-height:100vh;margin:0}.box{max-width:680px;padding:36px;border:1px solid rgba(255,255,255,.14);border-radius:24px;background:#14141d}p{color:#cfcfd8}</style></head><body><main class="box"><h1>Servicio no disponible</h1><p>Este recurso fue suspendido por el administrador de la plataforma.</p></main></body></html>';
  return '';
}
function fillAgentSelect(){
  ['agentId','editAgentId'].forEach(id=>{const sel=maybeEl(id);if(!sel)return;const current=sel.value;sel.innerHTML='<option value="">Selecciona un cliente</option>';agents.filter(a=>a.enabled!==false).forEach(a=>{const opt=document.createElement('option');opt.value=a.id;opt.textContent=a.name+' · '+shortID(a.id)+(a.online?' · online':' · offline');sel.appendChild(opt)});if(current)sel.value=current;});
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
async function createAgent(){
  if(!currentProject){msg('Selecciona un proyecto primero',true);return}
  try{
    const name=fieldValue('agentName');
    if(!name)throw new Error('Nombre del cliente requerido');
    showBusy('Creando cliente NAT','Generando ID, token y comandos de instalacion');
    const a=await api('/api/agents',{method:'POST',body:JSON.stringify({projectId:currentProject.id,name})});
    $('agentTokenCreate').innerHTML=renderAgentCredentials(a);
    $('agentTokenCreate').classList.remove('d-none');
    setIfExists('agentName','');
    await reloadProjects();
    await loadProjectData(currentProject.id);
    msg('Cliente creado. Copia el token ahora.');
  }catch(err){msg(err.message,true)}finally{hideBusy()}
}
function createResourcePayload(prefix=''){
  const isEdit=prefix==='edit';
  const mode=fieldValue(isEdit?'editMode':'mode')||'http';
  const originType=fieldValue(isEdit?'editOriginType':'originType')||'local';
  const payload={
    projectId: currentProject?currentProject.id:fieldValue('editProjectId'),
    name: fieldValue(isEdit?'editResourceName':'resourceName'),
    mode,
    originType,
    agentId: originType==='agent'?fieldValue(isEdit?'editAgentId':'agentId'):'',
    backendHost: fieldValue(isEdit?'editBackendHost':'backendHost'),
    backendPort: fieldNumber(isEdit?'editBackendPort':'backendPort'),
    enabled: isEdit ? fieldValue('editResourceEnabled')!=='false' : true,
    disabledResponseMode: isEdit ? (fieldValue('editDisabledResponseMode')||'403') : '403',
    disabledStatusCode: isEdit ? fieldNumber('editDisabledStatusCode')||403 : 403,
    disabledHtml: isEdit ? fieldValue('editDisabledHtml') : ''
  };
  if(mode==='http'){
    payload.domain=isEdit?fieldValue('editDomain').toLowerCase():buildDomainFromCreateForm();
    payload.pathPrefix=isEdit?fieldValue('editPathPrefix'):(fieldValue('pathPrefix')||'/');
    payload.backendScheme=isEdit?fieldValue('editBackendScheme'):(fieldValue('backendScheme')||'http');
    payload.tls=isEdit?fieldValue('editTLS')==='true':fieldValue('tls')==='true';
    payload.publicPort=0;
  }else{
    payload.domain='';payload.pathPrefix='';payload.backendScheme='';payload.tls=false;
    payload.publicPort=fieldNumber(isEdit?'editPublicPort':'publicPort');
  }
  return payload;
}
async function createResourceFromForm(e){
  e.preventDefault();
  if(!currentProject){msg('Selecciona un proyecto primero',true);return false}
  try{
    const payload=createResourcePayload();
    if(!payload.name)throw new Error('Nombre del recurso requerido');
    showBusy('Creando recurso','Validando puerto, cliente NAT, backend y aplicando Traefik');
    await api('/api/resources',{method:'POST',body:JSON.stringify(payload)});
    await reloadProjects();
    await loadProjectData(currentProject.id);
    msg('Recurso creado');
    go('/projects/'+currentProject.id+'/resources');
  }catch(err){msg(err.message,true)}finally{hideBusy()}
  return false;
}
function openEditResource(id){
  const r=resources.find(x=>x.id===id);if(!r){msg('Recurso no encontrado',true);return}
  setIfExists('editResourceId',r.id);setIfExists('editResourceName',r.name);setIfExists('editMode',r.mode||'http');setIfExists('editOriginType',r.originType||'local');
  fillAgentSelect();setIfExists('editAgentId',r.agentId||'');setIfExists('editDomain',r.domain||'');setIfExists('editPathPrefix',r.pathPrefix||'/');setIfExists('editTLS',String(!!r.tls));setIfExists('editBackendScheme',r.backendScheme||'http');setIfExists('editPublicPort',r.publicPort||'');setIfExists('editBackendHost',r.backendHost||'127.0.0.1');setIfExists('editBackendPort',r.backendPort||'');setIfExists('editResourceEnabled',String(!!r.enabled));setIfExists('editDisabledResponseMode',r.disabledResponseMode||'403');setIfExists('editDisabledStatusCode',r.disabledStatusCode||403);setIfExists('editDisabledHtml',r.disabledHtml||'');setIfExists('editDisabledPreset','');
  syncEditResourceMode();syncEditDisabledMode();
  $('resourceEditModal').classList.add('open');
}
async function saveResourceEdit(e){
  e.preventDefault();
  try{
    const id=fieldValue('editResourceId');if(!id)throw new Error('Recurso no seleccionado');
    const payload=createResourcePayload('edit');
    showBusy('Guardando recurso','Validando cambios y aplicando Traefik');
    await api('/api/resources/'+id,{method:'PATCH',body:JSON.stringify(payload)});
    closeResourceEditModal();
    await reloadProjects();
    if(currentProject)await loadProjectData(currentProject.id);
    msg('Recurso actualizado');
  }catch(err){msg(err.message,true)}finally{hideBusy()}
  return false;
}
async function deleteResource(id){
  const r=resources.find(x=>x.id===id);
  if(!await confirmAction('Eliminar recurso','Se eliminara '+(r?r.name:shortID(id))+' y Pangolite aplicara Traefik automaticamente.','Eliminar recurso'))return;
  try{
    await api('/api/resources/'+id,{method:'DELETE'});
    removeResourceLocal(id);
    msg('Recurso eliminado');
    refreshCurrentProjectSoft();
  }catch(err){msg(err.message,true)}
}
function setupForms(){
  const projectForm=maybeEl('projectForm');if(projectForm){projectForm.setAttribute('action','javascript:void(0)');projectForm.addEventListener('submit',createProjectFromForm)}
  const projectSettingsForm=maybeEl('projectSettingsForm');if(projectSettingsForm){projectSettingsForm.setAttribute('action','javascript:void(0)');projectSettingsForm.addEventListener('submit',saveProjectSettings)}
  const domainForm=maybeEl('domainForm');if(domainForm){domainForm.setAttribute('action','javascript:void(0)');domainForm.addEventListener('submit',createDomainFromForm)}
  const resourceForm=maybeEl('resourceForm');if(resourceForm){resourceForm.setAttribute('action','javascript:void(0)');resourceForm.addEventListener('submit',createResourceFromForm)}
  const editForm=maybeEl('resourceEditForm');if(editForm){editForm.setAttribute('action','javascript:void(0)');editForm.addEventListener('submit',saveResourceEdit)}
  const settingsForm=maybeEl('dashboardSettingsForm');if(settingsForm){settingsForm.setAttribute('action','javascript:void(0)');settingsForm.addEventListener('submit',saveSettings)}
  [['mode',syncMode],['originType',syncOrigin],['domainSelect',syncDomainMode],['subdomain',syncDomainMode],['customDomain',syncDomainMode],['disabledResponseMode',syncDisabledMode],['editMode',syncEditResourceMode],['editOriginType',syncEditResourceOrigin],['editDisabledResponseMode',syncEditDisabledMode]].forEach(([id,fn])=>{const el=maybeEl(id);if(el)el.addEventListener('input',fn);if(el)el.addEventListener('change',fn)});
  const preset=maybeEl('disabledPreset');if(preset)preset.addEventListener('change',()=>{const html=disabledPresetHTML(preset.value);if(html){setIfExists('disabledResponseMode','html');setIfExists('disabledStatusCode','403');setIfExists('disabledHtml',html);syncDisabledMode()}});
  const epreset=maybeEl('editDisabledPreset');if(epreset)epreset.addEventListener('change',()=>{const html=disabledPresetHTML(epreset.value);if(html){setIfExists('editDisabledResponseMode','html');setIfExists('editDisabledStatusCode','403');setIfExists('editDisabledHtml',html);syncEditDisabledMode()}});
}

$('logoutBtn').addEventListener('click',async()=>{await api('/api/logout',{method:'POST',body:'{}'});location.href='/login'});setupForms();syncMode();syncOrigin();init().catch(e=>msg(e.message,true));
</script>
</body>
</html>`
