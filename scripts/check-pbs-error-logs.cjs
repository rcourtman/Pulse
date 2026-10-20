// Actual PBS Settings error producer -> production SystemLogsPanel, with only
// captured synthetic HTTP/log frames replayed at the local consumer boundary.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');

(async () => {
  const out = path.resolve('tmp/pbs-error-logs');
  const staticRoot = path.resolve('tmp/pbs-status-logs/static');
  const producerBytes = fs.readFileSync(path.join(out, 'producer.json'));
  const frames = JSON.parse(producerBytes);
  const source = JSON.parse(fs.readFileSync(path.join(out, 'source.json')));
  const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
  assert.equal(source.head, binding.source);
  assert.equal(source.browserHelperSHA256, hash(fs.readFileSync(__filename)));
  assert.equal(source.producerSHA256, hash(producerBytes));
  for (const [file, digest] of Object.entries(source.producerFiles))
    assert.equal(hash(fs.readFileSync(file)), digest, 'producer differs from proved source');
  for (const [file, digest] of Object.entries(binding.compiledSources)) {
    if (file.startsWith('tmp/')) continue;
    assert.equal(hash(fs.readFileSync(path.join('frontend-modern', file))), digest, 'consumer differs from compiled source');
  }
  const version = require('playwright/package.json').version;
  assert.equal(version, JSON.parse(fs.readFileSync('tests/integration/package-lock.json')).packages['node_modules/playwright'].version);
  const keys = ['api-401','api-403','api-429','api-503','auth-401','auth-403','auth-429','auth-503'];
  assert.deepEqual(Object.keys(frames).sort(), [...keys].sort());
  assert(!producerBytes.includes(Buffer.from('synthetic-private-response-')));
  for (const key of keys) {
    const f = frames[key];
    assert.equal(f.lines.length, 1);
    assert.equal(f.httpStatus, 400);
    assert.equal(f.recoveryStatus, 200);
    assert.equal(f.verifySSL, true);
    assert.equal(JSON.parse(f.recoveryBody).datastoreCount, 0);
    assert(f.lines[0].includes(`HTTP ${f.providerStatus}`));
    assert(f.lines[0].includes('Operation failed'));
  }
  let level = 'info';
  const clients = new Set(), requests = [], unexpected = [], served = new Set();
  const emit = key => { for (const res of clients) for (const line of frames[key].lines) res.write(`data: ${line}\n\n`); };
  const server = http.createServer((req,res) => {
    const url = new URL(req.url, 'http://fixture.invalid');
    requests.push({method:req.method,path:url.pathname});
    if (url.pathname === '/api/logs/stream' && req.method === 'GET') {
      res.writeHead(200,{'Content-Type':'text/event-stream','Cache-Control':'no-cache',Connection:'keep-alive'});
      clients.add(res); res.on('close',()=>clients.delete(res));
      for (const line of frames['api-401'].lines) res.write(`data: ${line}\n\n`);
      return;
    }
    if (url.pathname === '/api/logs/level' && req.method === 'GET') {res.setHeader('Content-Type','application/json');return res.end(JSON.stringify({level}));}
    if (url.pathname === '/api/logs/level' && req.method === 'POST') {
      let body = '';req.on('data',bytes=>body+=bytes);
      req.on('end',()=>{assert.deepEqual(JSON.parse(body),{level:'debug'});assert.equal(req.headers['x-csrf-token'],'fixture-csrf');level='debug';res.end();emit('api-403');});return;
    }
    if (url.pathname === '/fixture/emit' && req.method === 'POST') {
      const key = url.searchParams.get('case');assert(keys.includes(key));emit(key);res.writeHead(204);return res.end();
    }
    if (url.pathname === '/favicon.ico') {res.writeHead(204);return res.end();}
    const key = url.pathname === '/' ? 'tmp/pbs-status-logs/index.html' : decodeURIComponent(url.pathname).slice(1);
    if (!Object.hasOwn(binding.artifacts,key)) {unexpected.push({method:req.method,path:url.pathname});res.writeHead(404);return res.end('Not found');}
    const bytes = fs.readFileSync(path.join(staticRoot,key));assert.equal(hash(bytes),binding.artifacts[key]);served.add(key);
    res.setHeader('Content-Type',{'.html':'text/html','.js':'text/javascript','.css':'text/css'}[path.extname(key)]||'application/octet-stream');res.end(bytes);
  });
  let browser, browserVersion, failure = null;
  const results = [], cleanup = {browserClosed:false,serverClosed:false,remainingSSE:null};
  try {
    await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});
    const origin = `http://127.0.0.1:${server.address().port}`;
    browser = await chromium.launch({headless:true,channel:'chromium',args:['--no-sandbox']});browserVersion=browser.version();
    for (const width of [1440,320]) {
      level='info';
      const context = await browser.newContext({viewport:{width,height:1100},hasTouch:width===320,colorScheme:width===320?'dark':'light'});
      await context.addCookies([{name:'pulse_csrf',value:'fixture-csrf',url:origin,sameSite:'Strict'}]);
      const page = await context.newPage();
      await page.addInitScript(dark=>{document.addEventListener('DOMContentLoaded',()=>document.documentElement.classList.toggle('dark',dark));},width===320);
      const errors=[], consoleErrors=[], requestFailures=[], captures=[], layouts=[];
      page.on('pageerror',e=>errors.push(e.message));page.on('console',m=>{if(m.type()==='error')consoleErrors.push(m.text());});page.on('requestfailed',r=>requestFailures.push({path:new URL(r.url()).pathname,error:r.failure()?.errorText}));
      const rows = page.locator('main .font-mono > div > span');
      const settle = async()=>{await page.evaluate(()=>document.fonts.ready);await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));};
      const count = async n=>{await page.waitForFunction(n=>document.querySelectorAll('main .font-mono > div > span').length===n,n);};
      const activate = async button=>{if(width===320)await button.tap();else{await button.focus();await button.press('Enter');}};
      const emitFromPage = key=>page.evaluate(async key=>{const r=await fetch('/fixture/emit?case='+key,{method:'POST'});if(r.status!==204)throw new Error('Fixture emission failed');await r.arrayBuffer();},key);
      const capture = async name=>{
        await settle();
        const layout=await page.evaluate(()=>({width:document.documentElement.clientWidth,scroll:document.documentElement.scrollWidth,consoleWidth:document.querySelector('.font-mono').clientWidth,consoleScroll:document.querySelector('.font-mono').scrollWidth}));
        layouts.push({name,...layout});assert(layout.scroll<=width,'document overflow');
        assert(!(await page.locator('main').innerText()).includes('synthetic-private-response-'));
        await page.screenshot({path:path.join(out,`${width}-${name}.png`),fullPage:true});captures.push(`${width}-${name}.png`);
      };
      await page.goto(origin+'/',{waitUntil:'domcontentloaded'});await count(1);
      assert((await rows.first().innerText()).includes('HTTP 401 Unauthorized'));
      await capture('failure');
      await page.getByRole('combobox',{name:'Server Log Level:'}).selectOption('debug');await count(2);
      await activate(page.getByRole('button',{name:'Pause Stream',exact:true}));
      assert.equal(await page.getByRole('button',{name:'Resume Stream',exact:true}).getAttribute('aria-pressed'),'true');
      await emitFromPage('auth-401');await settle();assert.equal(await rows.count(),2,'pause must hold display');
      await activate(page.getByRole('button',{name:'Resume Stream',exact:true}));
      for (const key of keys.slice(2)) await emitFromPage(key);
      await count(8);
      const shown=await rows.allInnerTexts();for(const key of keys)assert(shown.some(line=>line.includes(`HTTP ${frames[key].providerStatus}`)));
      await capture('all-failures');
      if(width===320){await page.locator('.font-mono').evaluate(el=>{el.scrollLeft=el.scrollWidth;});await capture('error-detail');}
      await activate(page.getByRole('button',{name:'Clear Log Output',exact:true}));
      await page.getByText('Waiting for log output.',{exact:true}).waitFor();assert.equal(await rows.count(),0);
      await emitFromPage('api-503');await count(1);assert((await rows.first().innerText()).includes('HTTP 503 Service Unavailable'));
      assert.deepEqual(errors,[]);assert.deepEqual(consoleErrors,[]);assert.deepEqual(requestFailures,[]);
      results.push({width,theme:width===320?'dark':'light',interaction:width===320?'touch':'keyboard',cases:keys,captures,layouts,errors,consoleErrors,requestFailures,passed:true});
      await context.close();
    }
    assert.deepEqual(unexpected,[]);
    assert.equal(requests.filter(r=>r.path==='/api/logs/level'&&r.method==='POST').length,2);
    assert(!requests.some(r=>r.path==='/api/logs/download'),'viewing failures must not download a bundle');
  } catch(error) {failure={name:error.name,message:error.message};process.exitCode=1;}
  finally {
    if(browser){await browser.close();cleanup.browserClosed=true;}
    for(const res of clients)res.end();server.closeAllConnections();
    await new Promise(resolve=>server.close(resolve));cleanup.serverClosed=true;
    await new Promise(resolve=>setImmediate(resolve));cleanup.remainingSSE=clients.size;
    fs.writeFileSync(path.join(out,'browser-result.json'),JSON.stringify({source,binding,producerSHA256:hash(producerBytes),playwright:version,browser:browserVersion,served:[...served],requests,unexpected,results,cleanup,failure},null,2)+'\n');
    console.log(JSON.stringify({results:results.length,cleanup,failure}));
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
