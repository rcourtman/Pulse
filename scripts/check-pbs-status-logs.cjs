// Independent PBS status diagnostic producer -> existing real SystemLogsPanel.
// This is not a retry of, or evidence for, the retained snapshot-sizing browser.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');

(async () => {
  const out = path.resolve('tmp/pbs-status-logs');
  const staticRoot = path.join(out, 'static');
  const producerBytes = fs.readFileSync(path.join(out, 'producer.json'));
  const frames = JSON.parse(producerBytes);
  const source = JSON.parse(fs.readFileSync(path.join(out, 'source.json')));
  const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
  assert.match(source.head, /^[0-9a-f]{40}$/);
  assert.equal(source.producerSource, binding.source);
  assert.equal(source.compilerSource, binding.source);
  assert.equal(source.browserHelperSHA256, hash(fs.readFileSync(__filename)));
  for (const [name, digest] of Object.entries(source.producerFiles))
    assert.equal(hash(fs.readFileSync(name)), digest, 'producer input differs from proved containing source');
  assert.equal(source.producerSHA256, hash(producerBytes));
  for (const [name, digest] of Object.entries(binding.compiledSources)) {
    if (name.startsWith('tmp/')) continue; // Generated harness is bound separately.
    assert.equal(hash(fs.readFileSync(path.join('frontend-modern', name))), digest, 'compiled source differs from containing source');
  }
  const version = require('playwright/package.json').version;
  const lock = JSON.parse(fs.readFileSync('tests/integration/package-lock.json'));
  assert.equal(version, lock.packages['node_modules/playwright'].version);
  for (const level of ['info', 'debug']) for (const phase of ['node','rrd','status','gc','malformed']) assert(frames[`${level}/${phase}`]);
  assert.deepEqual(frames['info/node'].lines, []);
  assert.deepEqual(frames['info/rrd'].lines, []);
  assert.equal(frames['info/malformed'].lines.length, 1);
  assert(!producerBytes.includes(Buffer.from('synthetic-provider-extension-')));

  let level = 'info';
  const clients = new Set();
  const requests = [], unexpected = [], served = new Set();
  const emit = phase => { for (const res of clients) for (const line of frames[`${level}/${phase}`].lines) res.write(`data: ${line}\n\n`); };
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://fixture.invalid');
    requests.push({ method: req.method, path: url.pathname });
    if (url.pathname === '/api/logs/stream' && req.method === 'GET') {
      res.writeHead(200, { 'Content-Type':'text/event-stream', 'Cache-Control':'no-cache', Connection:'keep-alive' });
      clients.add(res);
      res.on('close', () => clients.delete(res));
      for (const line of frames['info/malformed'].lines) res.write(`data: ${line}\n\n`);
      return;
    }
    if (url.pathname === '/api/logs/level') {
      if (req.method === 'GET') { res.setHeader('Content-Type','application/json'); return res.end(JSON.stringify({level})); }
      if (req.method === 'POST') {
        let body = '';
        req.on('data', bytes => body += bytes);
        req.on('end', () => {
          assert.deepEqual(JSON.parse(body), {level:'debug'});
          assert.equal(req.headers['x-csrf-token'], 'fixture-csrf');
          level = 'debug'; res.end();
          for (const phase of ['node','status','gc','malformed']) emit(phase);
        });
        return;
      }
    }
    if (url.pathname === '/fixture/emit' && req.method === 'POST') { emit('rrd'); res.writeHead(204); return res.end(); }
    if (url.pathname === '/favicon.ico') { res.writeHead(204); return res.end(); }
    const key = url.pathname === '/' ? 'tmp/pbs-status-logs/index.html' : decodeURIComponent(url.pathname).slice(1);
    if (!Object.hasOwn(binding.artifacts, key)) { unexpected.push({method:req.method,path:url.pathname}); res.writeHead(404); return res.end('Not found'); }
    const artifact = fs.readFileSync(path.join(staticRoot,key));
    assert.equal(hash(artifact), binding.artifacts[key]);
    served.add(key);
    res.setHeader('Content-Type', {'.html':'text/html','.js':'text/javascript','.css':'text/css'}[path.extname(key)] || 'application/octet-stream');
    res.end(artifact);
  });
  let browser, browserVersion;
  const results = [];
  const cleanup = { browserClosed:false, serverClosed:false, remainingSSE:null };
  let failure = null;
  try {
    await new Promise((resolve,reject) => { server.once('error',reject); server.listen(0,'127.0.0.1',resolve); });
    const origin = `http://127.0.0.1:${server.address().port}`;
    browser = await chromium.launch({headless:true,channel:'chromium',args:['--no-sandbox']});
    browserVersion = browser.version();
    for (const width of [1440,320]) {
      level = 'info';
      const context = await browser.newContext({viewport:{width,height:1100},hasTouch:width===320,colorScheme:width===320?'dark':'light'});
      await context.addCookies([{ name:'pulse_csrf', value:'fixture-csrf', url:origin, sameSite:'Strict' }]);
      const page = await context.newPage();
      await page.addInitScript(dark => { document.addEventListener('DOMContentLoaded',()=>document.documentElement.classList.toggle('dark',dark)); }, width===320);
      const errors = [], consoleErrors = [], requestFailures = [], overflows = [], captures = [];
      page.on('pageerror', e=>errors.push(e.message));
      page.on('console', m=>{if(m.type()==='error')consoleErrors.push(m.text());});
      page.on('requestfailed', r=>requestFailures.push({url:new URL(r.url()).pathname,error:r.failure()?.errorText}));
      const startRequests = requests.length;
      const logRows = page.locator('main .font-mono > div > span');
      const settle = async () => {
        await page.evaluate(()=>document.fonts.ready);
        await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
      };
      const check = async phase => {
        await settle();
        const census = await page.evaluate(()=>({ document:{width:document.documentElement.clientWidth,scroll:document.documentElement.scrollWidth}, console:{width:document.querySelector('.font-mono').clientWidth,scroll:document.querySelector('.font-mono').scrollWidth} }));
        overflows.push({phase,...census});
        assert(census.document.scroll<=width, 'document overflow');
        assert(!((await page.locator('main').innerText()).includes('synthetic-provider-extension-')));
      };
      const capture = async name => { await check(name); await page.screenshot({path:path.join(out,`${width}-${name}.png`),fullPage:true}); captures.push(`${width}-${name}.png`); };
      const activate = async button => { if(width===320)await button.tap(); else{await button.focus();await button.press('Enter');} };
      await page.goto(origin+'/',{waitUntil:'domcontentloaded'});
      await logRows.first().waitFor();
      assert.equal(await logRows.count(),1);
      assert((await logRows.first().innerText()).includes('Failed to parse datastore status'));
      assert((await logRows.first().innerText()).includes('responseBytes'));
      assert.equal(await page.getByRole('combobox',{name:'Server Log Level:'}).inputValue(),'info');
      await capture('info-error');
      await page.getByRole('combobox',{name:'Server Log Level:'}).selectOption('debug');
      const debugCount = 1 + ['node','status','gc','malformed'].reduce((n,p)=>n+frames[`debug/${p}`].lines.length,0);
      await page.waitForFunction(n=>document.querySelectorAll('main .font-mono > div > span').length===n,debugCount);
      assert((await logRows.allInnerTexts()).some(l=>l.includes('PBS datastore status')));
      await capture('debug');
      await activate(page.getByRole('button',{name:'Pause Stream',exact:true}));
      assert.equal(await page.getByRole('button',{name:'Resume Stream',exact:true}).getAttribute('aria-pressed'),'true');
      await page.evaluate(async () => {
        const response = await fetch('/fixture/emit', {method:'POST'});
        if (response.status !== 204) throw new Error('Fixture emission did not complete');
        await response.arrayBuffer(); // Finish the control request, not just its headers.
      }); await settle();
      assert.equal(await logRows.count(),debugCount,'paused display must not add arrivals');
      await activate(page.getByRole('button',{name:'Resume Stream',exact:true}));
      await page.evaluate(async () => {
        const response = await fetch('/fixture/emit', {method:'POST'});
        if (response.status !== 204) throw new Error('Fixture emission did not complete');
        await response.arrayBuffer(); // Finish the control request, not just its headers.
      });
      const resumedCount = debugCount + frames['debug/rrd'].lines.length;
      await page.waitForFunction(n=>document.querySelectorAll('main .font-mono > div > span').length===n,resumedCount);
      await check('resumed');
      await activate(page.getByRole('button',{name:'Clear Log Output',exact:true}));
      await page.getByText('Waiting for log output.',{exact:true}).waitFor();
      assert.equal(await logRows.count(),0);
      await capture('cleared');
      const uiRequests = requests.slice(startRequests);
      const result = {width,theme:width===320?'dark':'light',interaction:width===320?'touch':'keyboard',infoRows:1,debugCount,resumedCount,errors,consoleErrors,requestFailures,overflows,captures,requests:uiRequests,passed:false};
      results.push(result); // Preserve the census even if a final assertion fails.
      assert.deepEqual(errors,[]); assert.deepEqual(consoleErrors,[]); assert.deepEqual(requestFailures,[]);
      assert.equal(uiRequests.filter(r=>r.path==='/api/logs/level'&&r.method==='POST').length,1);
      assert.equal(uiRequests.filter(r=>r.path==='/api/logs/stream').length,1);
      assert(!uiRequests.some(r=>r.path==='/api/logs/download'),'view controls cannot download a bundle');
      await context.close();
      result.passed = true;
    }
    assert.deepEqual(unexpected,[]);
  } catch(error) { failure = {name:error.name,message:error.message}; process.exitCode=1; }
  finally {
    if(browser){await browser.close();cleanup.browserClosed=true;}
    for(const res of clients)res.end();
    server.closeAllConnections();
    await new Promise(resolve=>server.close(resolve));cleanup.serverClosed=true;
    cleanup.remainingSSE=clients.size;
    // The close events from owned connections must complete before the receipt.
    await new Promise(resolve=>setImmediate(resolve));cleanup.remainingSSE=clients.size;
    const receipt={source,binding,producerSHA256:hash(producerBytes),playwright:version,browser:browserVersion,served:[...served],requests,unexpected,results,cleanup,failure};
    fs.writeFileSync(path.join(out,'browser-result.json'),JSON.stringify(receipt,null,2)+'\n');
    console.log(JSON.stringify({results:results.length,cleanup,failure}));
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
