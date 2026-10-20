process.env.RAYON_NUM_THREADS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
(async () => {
  const root='/workspace/frontend-modern';
  const artifacts='/workspace/tmp/unraid-alert-proof/browser';
  fs.mkdirSync(artifacts,{recursive:true});
  const source=JSON.parse(fs.readFileSync('/workspace/tmp/unraid-alert-proof/binding.json','utf8'));
  const fixtureBytes=fs.readFileSync('/workspace/frontend-modern/browser-tests/unraid-storage-frames.json');
  assert.equal(crypto.createHash('sha256').update(fixtureBytes).digest('hex'),source.frames_sha256);
  for(const [file,digest] of Object.entries(source.files)) assert.equal(crypto.createHash('sha256').update(fs.readFileSync(path.join('/workspace',file))).digest('hex'),digest,file);
  const frames=JSON.parse(fixtureBytes);
  const playwright=require('playwright/package.json').version;
  const expected=JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json','utf8')).packages['node_modules/@playwright/test'].version;
  assert.equal(playwright,expected);
  const {createServer} = await import(path.join(root,'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile:path.join(root,'vite.config.ts'),
    cacheDir:'/workspace/tmp/unraid-alert-proof/vite-cache',
    optimizeDeps:{noDiscovery:true,include:[]},
    server:{host:'127.0.0.1',port:5234,strictPort:true,fs:{allow:['/workspace']}},
  });
  const observations=[];
  const failures=[]; const requests=[]; const cleanup={};
  const equal=(actual,expected,label)=>{if(actual!==expected)failures.push({current,label,expected,actual});};
  const check=(actual,label)=>{if(!actual)failures.push({current,label,expected:true,actual});};
  let browser;
  let activePage; let current;
  try {
    await server.listen();
    for(const [name,engine,phone] of [['desktop',chromium,false],['narrow',webkit,true]]){
      browser=await engine.launch(engine===chromium?{headless:true,channel:'chromium',args:['--no-sandbox']}:{headless:true});
      const page=await browser.newPage({viewport:phone?{width:375,height:812}:{width:1440,height:900},hasTouch:phone,isMobile:phone});
      activePage=page;
      const errors=[];page.on('pageerror',e=>errors.push(e.message));
      await page.route('**/*',route=>{
        const url=new URL(route.request().url());
        if(url.origin!=='http://127.0.0.1:5234') {failures.push({external:url.origin});return route.abort();}
        if(!url.pathname.startsWith('/api/'))return route.continue();
        requests.push({method:route.request().method(),path:url.pathname});
        if(route.request().method()!=='GET'){failures.push({unexpected:route.request().method()+' '+url.pathname});return route.fulfill({status:405,json:{}});}
        if(url.pathname==='/api/metrics-store/history')return route.fulfill({json:{resourceType:url.searchParams.get('resourceType'),resourceId:url.searchParams.get('resourceId'),metric:url.searchParams.get('metric'),range:url.searchParams.get('range'),start:0,end:0,points:[],source:'unavailable'}});
        return route.fulfill({json:url.pathname==='/api/license/runtime-capabilities'?{capabilities:[],limits:[],max_history_days:7,hosted_mode:false,runtime:{build:'community'},blocked_capabilities:[]}:{data:[],enabled:false}});
      });
      await page.goto('http://127.0.0.1:5234/browser-tests/unraid-storage-alerts.html');
      if(phone)await page.evaluate(()=>document.documentElement.classList.add('dark'));
      const consumer=page.getByRole('region',{name:'Storage consumer'});
      for(const mode of ['legacy-17-empty','fresh-agent',...['explicit-missing','assigned-missing','assigned-disabled','assigned-invalid','no-parity'].flatMap(mode=>[mode,mode+'-recovered'])]){
        current={name,mode};
        await page.getByRole('button',{name:mode,exact:true}).click();
        const healthy=mode==='legacy-17-empty'||mode==='fresh-agent'||mode.endsWith('-recovered');
        const row=consumer.locator('tr[data-row-id]'); await row.waitFor();
        const rowText=await row.innerText();
        if(!phone){
          equal(/Protection reduced/.test(rowText),!healthy&&mode!=='no-parity',`${mode} protection: ${rowText}`);
          equal(/No parity/.test(rowText),mode==='no-parity',`${mode} parity: ${rowText}`);
        }
        if(healthy){
          check(!/missing|disabled|invalid|critical/i.test(rowText),`${mode}: ${rowText}`);
          equal(await row.locator('[data-storage-alert-headline]').count(),0,`${mode} healthy alert headline`);
          equal(await row.getAttribute('data-alert-state'),'none',`${mode} healthy alert state`);
        }else{
          const headline=row.locator('[data-storage-alert-headline]');
          equal(await headline.count(),1,`${mode} genuine alert headline presence`);
          equal(await row.getAttribute('data-alert-state'),'unacknowledged',`${mode} genuine alert state`);
          equal(await row.getAttribute('data-alert-severity'),mode==='no-parity'?'warning':'critical',`${mode} genuine alert severity`);
          if(await headline.count())check(/missing|disabled|invalid|parity/i.test(await headline.innerText()),`${mode} genuine alert explanation`);
        }
        // All three controls have the exact same displayed names as the
        // producer array, but must never borrow its alert by name/node/host.
        const isolation=[];
        for(const id of ['isolation-other-host','isolation-other-array','isolation-api']) {
          const control=page.getByRole('region',{name:id,exact:true}).locator('tr[data-row-id]');
          equal(await control.count(),1,`${mode} ${id} presence`);
          equal(await control.getAttribute('data-alert-state'),'none',`${mode} ${id} isolated state`);
          equal(await control.getAttribute('data-alert-severity'),'none',`${mode} ${id} isolated severity`);
          equal(await control.locator('[data-storage-alert-headline]').count(),0,`${mode} ${id} isolated headline`);
          isolation.push({id,text:await control.innerText()});
        }
        // Exercise the ordinary expandable production row with keyboard at desktop, touch on phone.
        const toggle=row.getByRole('button');
        if(phone)await row.tap();else {await toggle.focus();await page.keyboard.press('Enter');}
        await consumer.getByText('Topology',{exact:true}).waitFor();
        if(healthy){
          await consumer.getByText('Physical Disks (6)',{exact:true}).waitFor();
          equal(await consumer.getByText('parity2',{exact:true}).count(),0,`${mode} no empty parity member`);
          equal(await consumer.getByText('disk6',{exact:true}).count(),0,`${mode} no empty data member`);
        }
        const dimensions=await page.evaluate(()=>({scroll:document.documentElement.scrollWidth,width:innerWidth}));
        check(dimensions.scroll<=dimensions.width+1,JSON.stringify({mode,dimensions}));
        const screenshot=['legacy-17-empty','assigned-missing','assigned-missing-recovered'].includes(mode)?`${name}-${mode}.png`:null;
        if(screenshot)await page.screenshot({path:path.join(artifacts,screenshot),fullPage:true});
        observations.push({name,mode,healthy,browser:browser.version(),rowText,isolation,alertState:await row.getAttribute('data-alert-state'),alertSeverity:await row.getAttribute('data-alert-severity'),headlineCount:await row.locator('[data-storage-alert-headline]').count(),detail:await consumer.innerText(),dimensions,screenshot});
      }
      equal(errors.length,0,`${name} page error count`);
      await browser.close();browser=null;
    }
    fs.writeFileSync(path.join(artifacts,'result.json'),JSON.stringify({status:failures.length?'failed':'passed',source,playwright,expected,observations,failures,requests},null,2));
    console.log(JSON.stringify({status:failures.length?'failed':'passed',states:observations.length,failures:failures.length,frames_sha256:source.frames_sha256}));
    if(failures.length)process.exitCode=1;
  }catch(error){
    if(activePage){await activePage.screenshot({path:path.join(artifacts,'failed-state.png'),fullPage:true});fs.writeFileSync(path.join(artifacts,'failed-state.json'),JSON.stringify({current,error:String(error),observations,text:await activePage.locator('body').innerText()},null,2));}
    throw error;
  }finally{if(browser)await browser.close();await server.close();fs.writeFileSync(path.join(artifacts,'cleanup.json'),JSON.stringify({browserClosed:true,serverClosed:true}));}
})().catch(e=>{console.error(e);process.exitCode=1;});
