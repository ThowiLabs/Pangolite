(function(){
  function normalize(value){
    return String(value||'').normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLowerCase().trim();
  }

  function pageItems(current,total){
    if(total<=7)return Array.from({length:total},(_,index)=>index+1);
    const items=[1];
    const start=Math.max(2,current-1);
    const end=Math.min(total-1,current+1);
    if(start>2)items.push('ellipsis-left');
    for(let page=start;page<=end;page++)items.push(page);
    if(end<total-1)items.push('ellipsis-right');
    items.push(total);
    return items;
  }

  function initSSHConnections(){
    const grid=document.getElementById('sshConnectionGrid');
    if(!grid)return;

    const cards=Array.from(grid.querySelectorAll('[data-ssh-card]'));
    const input=document.getElementById('sshConnectionSearch');
    const clear=document.getElementById('sshConnectionClear');
    const emptyClear=document.getElementById('sshConnectionEmptyClear');
    const pageSize=document.getElementById('sshConnectionPageSize');
    const empty=document.getElementById('sshConnectionEmpty');
    const summary=document.getElementById('sshConnectionSummary');
    const pagination=document.getElementById('sshConnectionPagination');
    const params=new URLSearchParams(location.search);
    let currentPage=Math.max(1,Number.parseInt(params.get('page')||'1',10)||1);
    let searchTimer=null;

    if(input)input.value=params.get('q')||'';
    if(pageSize&&['9','18','30'].includes(params.get('perPage')||''))pageSize.value=params.get('perPage');

    function syncURL(query,perPage){
      const next=new URLSearchParams(location.search);
      if(query)next.set('q',query);else next.delete('q');
      if(currentPage>1)next.set('page',String(currentPage));else next.delete('page');
      if(perPage!==9)next.set('perPage',String(perPage));else next.delete('perPage');
      const suffix=next.toString();
      history.replaceState(null, '', suffix ? '/ssh?' + suffix : '/ssh');
    }

    function makeButton(label,page,options={}){
      const button=document.createElement('button');
      button.type='button';
      button.className='ssh-page-button'+(options.active?' active':'');
      button.disabled=!!options.disabled;
      button.setAttribute('aria-label',options.ariaLabel||String(label));
      if(options.active)button.setAttribute('aria-current','page');
      if(options.icon){
        const icon=document.createElement('i');
        icon.className='bi '+options.icon;
        button.appendChild(icon);
      }else{
        button.textContent=String(label);
      }
      if(!options.disabled&&!options.active)button.addEventListener('click',()=>{currentPage=page;render(true)});
      return button;
    }

    function render(scrollToGrid){
      const query=normalize(input&&input.value);
      const perPage=Math.max(1,Number.parseInt(pageSize&&pageSize.value||'9',10)||9);
      const filtered=cards.filter(card=>!query||normalize(card.dataset.search).includes(query));
      const totalPages=Math.max(1,Math.ceil(filtered.length/perPage));
      currentPage=Math.min(Math.max(1,currentPage),totalPages);
      const start=(currentPage-1)*perPage;
      const visible=new Set(filtered.slice(start,start+perPage));

      cards.forEach(card=>{card.hidden=!visible.has(card)});
      if(empty)empty.classList.toggle('d-none',filtered.length!==0);
      if(grid)grid.classList.toggle('d-none',filtered.length===0);
      if(clear)clear.classList.toggle('d-none',!query);

      if(summary){
        if(filtered.length===0){
          summary.textContent='0 conexiones encontradas';
        }else{
          const first=start+1;
          const last=Math.min(start+perPage,filtered.length);
          summary.textContent='Mostrando '+first+'â€“'+last+' de '+filtered.length+' conexiones';
        }
      }

      if(pagination){
        pagination.replaceChildren();
        pagination.classList.toggle('d-none',filtered.length===0||totalPages<=1);
        if(totalPages>1){
          pagination.appendChild(makeButton('Anterior',currentPage-1,{icon:'bi-chevron-left',disabled:currentPage===1,ariaLabel:'PÃ¡gina anterior'}));
          pageItems(currentPage,totalPages).forEach(item=>{
            if(typeof item==='string'){
              const dots=document.createElement('span');
              dots.className='ssh-page-ellipsis';
              dots.textContent='â€¦';
              dots.setAttribute('aria-hidden','true');
              pagination.appendChild(dots);
              return;
            }
            pagination.appendChild(makeButton(item,item,{active:item===currentPage,ariaLabel:'PÃ¡gina '+item}));
          });
          pagination.appendChild(makeButton('Siguiente',currentPage+1,{icon:'bi-chevron-right',disabled:currentPage===totalPages,ariaLabel:'PÃ¡gina siguiente'}));
        }
      }

      syncURL(input&&input.value.trim(),perPage);
      if(scrollToGrid&&grid)grid.scrollIntoView({behavior:'smooth',block:'start'});
    }

    function clearSearch(){
      if(input){input.value='';input.focus()}
      currentPage=1;
      render(false);
    }

    if(input){
      input.addEventListener('input',()=>{
        clearTimeout(searchTimer);
        searchTimer=setTimeout(()=>{currentPage=1;render(false)},100);
      });
      input.addEventListener('keydown',event=>{
        if(event.key==='Escape'&&input.value){event.preventDefault();clearSearch()}
      });
    }
    if(clear)clear.addEventListener('click',clearSearch);
    if(emptyClear)emptyClear.addEventListener('click',clearSearch);
    if(pageSize)pageSize.addEventListener('change',()=>{currentPage=1;render(false)});
    document.addEventListener('keydown',event=>{
      if(event.key==='/'&&!event.ctrlKey&&!event.metaKey&&!event.altKey&&document.activeElement!==input){
        const tag=(document.activeElement&&document.activeElement.tagName||'').toLowerCase();
        if(tag!=='input'&&tag!=='textarea'&&tag!=='select'){
          event.preventDefault();
          if(input)input.focus();
        }
      }
    });

    render(false);
  }

  document.addEventListener('DOMContentLoaded',initSSHConnections);
})();

