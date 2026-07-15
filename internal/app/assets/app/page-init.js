function serverPageKey(){
  const bootKey=appBoot&&appBoot.pageKey?String(appBoot.pageKey):'';
  const bodyKey=document.body&&document.body.dataset?String(document.body.dataset.pageKey||''):'';
  return bootKey||bodyKey;
}

function serverCurrentProjectID(){
  const bootProject=appBoot&&appBoot.hasProject&&appBoot.currentProject?String(appBoot.currentProject.id||''):'';
  const bodyProject=document.body&&document.body.dataset?String(document.body.dataset.currentProject||''):'';
  return bootProject||bodyProject;
}

async function hydrateProjectPage(pageKey,initial){
  const projectID=serverCurrentProjectID();
  if(!currentProject&&projectID)currentProject=projects.find(p=>p.id===projectID||p.slug===projectID)||null;
  if(!currentProject)throw new Error('Go no entregó un proyecto válido para esta página.');

  if(!initial)await loadProjectData(currentProject.id);else fillAgentSelect();
  updateProjectSwitcherLabel();
  updateSidebarProjectContext();

  setHrefIfExists('goCreateResource','/projects/'+currentProject.id+'/resources/create');
  setHrefIfExists('goCreateAgent','/projects/'+currentProject.id+'/agents/create');
  setHrefIfExists('goResources','/projects/'+currentProject.id+'/resources');
  setHrefIfExists('goAgents','/projects/'+currentProject.id+'/agents');
  setHrefIfExists('goCreateResourceFromList','/projects/'+currentProject.id+'/resources/create');
  setHrefIfExists('goCreateAgentFromList','/projects/'+currentProject.id+'/agents/create');

  if(pageKey==='resource_create'){
    fillDomainSelect();
    fillAgentSelect();
    return;
  }
  if(pageKey==='resources'){
    if(resources.length)checkResourceHealth(true);
    return;
  }
  if(pageKey==='project'&&!initial)paintProjectOverview();
}

async function hydrateCurrentPage(options={}){
  const initial=!!options.initial;
  const pageKey=serverPageKey();
  if(!pageKey)throw new Error('Go no entregó el identificador de la página actual.');

  if(pageKey==='projects'){
    renderGlobalDashboard();
    return;
  }
  if(pageKey==='logs'){
    if(!initial)await loadLogs();
    return;
  }
  if(pageKey==='maintenance'){
    if(!initial)await loadMaintenance();
    return;
  }
  if(pageKey==='settings'){
    if(initial){paintDomains();fillDomainSelect();}
    else{await loadSettings();await reloadDomains();if(maybeEl('config'))await loadConfig();}
    return;
  }
  if(pageKey==='profile'){
    if(typeof setupProfilePage==='function')setupProfilePage();
    return;
  }
  if(pageKey==='ssh_connections'||pageKey==='terminal')return;
  if(['project','resources','resource_create','agents','agent_create'].includes(pageKey)){
    await hydrateProjectPage(pageKey,initial);
    return;
  }
  throw new Error('Página no reconocida por el servidor: '+pageKey);
}
