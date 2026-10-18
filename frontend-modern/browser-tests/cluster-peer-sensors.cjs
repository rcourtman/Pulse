const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const {chromium, webkit} = require('playwright');
const root = '/workspace';
const dir = path.join(root, 'tmp/cluster-peer-sensors');
const staticRoot = path.join(dir, 'static');
const artifacts = path.join(dir, 'browser');
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
(async () => {
  fs.mkdirSync(artifacts, {recursive:true});
  const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
  for (const [file, expected] of Object.entries(binding.sources)) assert.equal(hash(path.join(root, 'frontend-modern', file)), expected, `source changed: ${file}`);
  for (const [file, expected] of Object.entries(binding.artifacts)) assert.equal(hash(path.join(staticRoot, file)), expected, `artifact changed: ${file}`);
  const states = JSON.parse(fs.readFileSync(path.join(dir, 'resources.json')));
  assert.equal(Object.keys(states).length, 8);
  // The backend's synthetic snapshot clock, not elapsed VM/queue transport,
  // decides freshness in this field-rendering fixture. Do not rewrite data.
  const receiptClock = Math.max(...Object.values(states).map(value => typeof value.lastSeen === 'number' ? value.lastSeen : Date.parse(value.lastSeen)));
  assert.ok(Number.isFinite(receiptClock));
  const server = http.createServer((req,res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (url.pathname === '/proof/states') { res.setHeader('Content-Type','application/json'); return res.end(JSON.stringify(states)); }
    if (url.pathname.startsWith('/api/')) {
      res.setHeader('Content-Type','application/json');
      if (url.pathname.includes('/metrics-store/history')) return res.end(JSON.stringify({resourceType:url.searchParams.get('resourceType'),resourceId:url.searchParams.get('resourceId'),range:url.searchParams.get('range'),start:1,end:2,metrics:{}}));
      if (url.pathname.includes('runtime-capabilities')) return res.end(JSON.stringify({capabilities:[],limits:[],max_history_days:7,hosted_mode:false,runtime:{build:'community'},blocked_capabilities:[]}));
      return res.end(JSON.stringify({data:[],enabled:false}));
    }
    const file = path.join(staticRoot, decodeURIComponent(url.pathname));
    if (!file.startsWith(staticRoot + path.sep) || !fs.existsSync(file) || !fs.statSync(file).isFile()) {res.statusCode=404;return res.end();}
    res.setHeader('Content-Type',file.endsWith('.js')?'text/javascript':file.endsWith('.css')?'text/css':'text/html');
    res.end(fs.readFileSync(file));
  });
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  const origin = `http://127.0.0.1:${server.address().port}`;
  let browser;
  const journeys = [];
  try {
    for (const [name, engine, width, height] of [['desktop',chromium,1440,1000],['narrow',webkit,320,900]]) {
      browser = await engine.launch(engine===chromium?{headless:true,channel:'chromium',args:['--no-sandbox']}:{headless:true});
      const page = await browser.newPage({viewport:{width,height},isMobile:name==='narrow',hasTouch:name==='narrow'});
      await page.clock.install({time:new Date(receiptClock)});
      const errors = [], requests = [];
      page.on('pageerror',e=>errors.push(e.message));
      await page.route('**/*', route => new URL(route.request().url()).origin === origin ? route.continue() : route.abort());
      page.on('request',req=>{if(req.url().includes('/api/metrics-store/history')) requests.push(req.url());});
      await page.goto(origin+'/browser-tests/cluster-peer-sensors.html');
      if (name === 'narrow') {
        const toggle = page.getByTestId('history-phone-toggle');
        await toggle.click();
        assert.equal(await toggle.getAttribute('aria-expanded'), 'true');
      }
      const steps=[];
      for (const state of ['id_rsa','denied','recovered','id_ed25519','partial','stale','foreign','local']) {
        await page.getByRole('button',{name:state,exact:true}).click();
        const thermals = page.locator('[data-history-group="thermals"]');
        await thermals.scrollIntoViewIfNeeded();
        await thermals.getByText('No stored history in this range',{exact:true}).waitFor();
        const current = thermals.locator('[data-history-current="temperature"]');
        const expected = ['denied','stale','foreign'].includes(state)?null:state==='local'?'70°C':'55°C';
        if (expected) { await current.getByText(expected,{exact:true}).waitFor(); }
        else { assert.equal(await current.count(),0, `${state} manufactured a reading`); }
        // A zero axis tick is a scale mark, not a manufactured live reading.
        assert.equal(await current.getByText('0°C',{exact:true}).count(),0);
        const dimensions = await page.evaluate(()=>({width:innerWidth,scroll:document.documentElement.scrollWidth}));
        assert.ok(dimensions.scroll<=dimensions.width+1,JSON.stringify(dimensions));
        // Retain complete success/failure/recovery pixels at both viewports.
        if (['id_rsa','denied','recovered','partial'].includes(state)) await page.screenshot({path:path.join(artifacts,`${name}-${state}.png`),fullPage:true});
        steps.push({state,expected,dimensions,current:await current.allTextContents()});
      }
      // Exercise the consuming drawer, not only text on a static image.
      await page.getByRole('tab',{name:'Manage',exact:true}).click();
      await page.getByRole('tab',{name:'Overview',exact:true}).click();
      assert.deepEqual(errors,[]);
      assert.ok(requests.length>0,'actual history consumer was not exercised');
      journeys.push({name,width,height,browser:browser.version(),receiptClock,steps,requests,errors});
      await browser.close(); browser=null;
    }
    fs.writeFileSync(path.join(artifacts,'result.json'),JSON.stringify({playwright:require('playwright/package.json').version,producer:hash(path.join(dir,'resources.json')),binding:hash(path.join(staticRoot,'binding.json')),journeys},null,2)+'\n');
    console.log(JSON.stringify({result:'passed',journeys:journeys.length,states:16,captures:8}));
  } finally {if(browser) await browser.close(); await new Promise(resolve=>server.close(resolve));}
})().catch(e=>{console.error(e);process.exitCode=1;});
