// New current-source static preview, not a replay of the stopped Vite request.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-reading-help';
const preview = output + '/preview';
const origin = 'http://127.0.0.1:5331';
const runtime = ['frontend-modern/src/utils/workloadGuestPresentation.ts', 'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx'];
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const load = name => import(require.resolve(name, { paths: [root] }));
(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete', playwright: require('playwright/package.json').version,
    runtime_hashes: Object.fromEntries(runtime.map(file => [file, hash('/workspace/' + file)])),
    fixtures: Object.fromEntries(['cjs','tsx','html'].map(ext => ['guest-reading-help.' + ext, hash(root + '/browser-tests/guest-reading-help.' + ext)])),
    cases: [], captures: [], cleanup: { browser: false, server: false }, phases: [],
    limits: 'Static production-component preview with synthetic observations/APIs. No native guest, collector, cause, thaw, writes, restart, installed or release acceptance.',
  };
  let server, browser;
  try {
    assert.equal(result.playwright, JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
    const { build } = await load('vite');
    const solidModule = await load('vite-plugin-solid');
    const solid = solidModule.default.default || solidModule.default;
    const { default: tailwind } = await load('@tailwindcss/vite');
    await build({ root, configFile: false, logLevel: 'warn', plugins: [solid(), tailwind()], resolve: { alias: { '@': root + '/src' } }, build: { target: 'esnext', outDir: preview, emptyOutDir: true, minify: false, rollupOptions: { input: root + '/browser-tests/guest-reading-help.html' } } });
    result.phases.push('production-component static preview compiled');
    server = http.createServer((req,res) => {
      const url = new URL(req.url, origin);
      let file = path.resolve(preview, '.' + decodeURIComponent(url.pathname));
      if (file !== preview && !file.startsWith(preview + '/')) { res.writeHead(403); return res.end(); }
      if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) file = preview + '/browser-tests/guest-reading-help.html';
      const ext = path.extname(file); res.setHeader('Content-Type', ({'.html':'text/html','.js':'text/javascript','.css':'text/css','.md':'text/plain','.json':'application/json'})[ext] || 'application/octet-stream');
      res.end(fs.readFileSync(file));
    });
    await new Promise(resolve => server.listen(5331, '127.0.0.1', resolve));
    result.phases.push('loopback-only static server listening');
    for (const [name, engine, width, dark] of [['chromium-desktop', chromium, 1365, false], ['webkit-phone', webkit, 390, true]]) {
      browser = await engine.launch(engine === chromium ? {headless:true, channel:'chromium', args:['--no-sandbox']} : {headless:true});
      const context = await browser.newContext({viewport:{width,height:900},isMobile:width<=768,hasTouch:width<=768,locale:'en-GB'});
      const page = await context.newPage();
      page.setDefaultTimeout(12000);
      const record = { name, browser: browser.version(), width, errors: [], requests: [], states: [] };
      result.cases.push(record);
      page.on('pageerror', err => record.errors.push(err.message));
      await page.route('**/*', route => {
        const request=route.request(), url=new URL(request.url());
        if (url.origin !== origin) { record.requests.push({method:request.method(),path:url.pathname,blocked:true}); return route.abort(); }
        if (url.pathname.startsWith('/api/')) {
          record.requests.push({method:request.method(),path:url.pathname,synthetic:true});
          return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({success:true,data:[],alerts:[],anomalies:[],config:null})});
        }
        return route.continue();
      });
      await page.addInitScript(dark => {
        window.__input = [];
        document.addEventListener('click', event => window.__input.push(event.isTrusted), true);
        document.addEventListener('DOMContentLoaded', () => { if (dark) document.documentElement.classList.add('dark'); });
      }, dark);
      await page.goto(origin, {waitUntil:'domcontentloaded'});
      await page.waitForFunction(() => window.__readingHelp);
      const disclosure=page.locator('[data-guest-id="fixture-pve:pve-a:101"] [data-row-action]').first();
      if (width<=768) await page.getByText('reading-guest', {exact:true}).tap(); else {await disclosure.focus();await page.keyboard.press('Enter');}
      const drawer=page.getByRole('region',{name:'Guest details'});
      await drawer.waitFor({state:'visible'});
      const technical=drawer.getByTestId('guest-technical-details');
      // The current production overview is a section, not a collapsed details element.
      for (const state of [
        {name:'missing-windows',reason:'agent-not-running',retained:false,os:'Windows 11'},
        {name:'retained-android',reason:'agent-not-running',retained:true,os:'Android'},
        {name:'disabled',reason:'agent-disabled',retained:false,os:'Windows 11'},
        {name:'unknown',reason:'future-private-detail',retained:false,os:'Android'},
      ]) {
        await page.evaluate(state => window.__readingHelp.update(state), state);
        await page.waitForFunction(() => document.querySelector('[data-testid="guest-technical-details"]').textContent.includes('Filesystem reading guidance'));
        const text=await technical.innerText();
        assert.doesNotMatch(text,/Install and start|Enable it in VM Options|may not be installed|qemu-guest-agent|future-private-detail/);
        assert.match(text,/backup/);assert.match(text,/guest incident/);
        if(state.retained) {assert.match(text,/Using last known disk stats/);assert.match(text,/Last known 50%/);assert.equal(await technical.getByRole('progressbar').count(),0);}
        if(state.name==='missing-windows') assert.match(text,/does not prove it is absent or stopped/);
        if(state.name==='unknown') assert.match(text,/cause is unknown/);
        const link=technical.getByRole('link',{name:'Filesystem reading guidance'});
        assert.equal(await link.getAttribute('href'),'/docs/VM_DISK_MONITORING#a-missing-reading-is-not-an-installation-diagnosis');
        const layout=await technical.evaluate(el=>({width:el.clientWidth,scroll:el.scrollWidth,document:document.documentElement.clientWidth,documentScroll:document.documentElement.scrollWidth}));
        assert.ok(layout.scroll<=layout.width+1,JSON.stringify(layout));assert.ok(layout.documentScroll<=layout.document+1,JSON.stringify(layout));
        const file=name+'-'+state.name+'.png';await technical.screenshot({path:output+'/'+file});
        result.captures.push({file,sha256:hash(output+'/'+file)});record.states.push({name:state.name,text,layout});
      }
      await page.evaluate(() => window.__readingHelp.update({reason:'',retained:true,os:'Windows 11'}));
      assert.equal(await technical.getByRole('link',{name:'Filesystem reading guidance'}).count(),0);
      assert.equal(await technical.getByRole('progressbar').getAttribute('aria-valuenow'),'50');
      record.fresh='Current same-guest data restores the utilization bar and removes uncertain-read guidance.';
      await page.evaluate(() => window.__readingHelp.update({reason:'agent-not-running',retained:false,os:'Windows 11'}));
      const link=technical.getByRole('link',{name:'Filesystem reading guidance'});
      if(width<=768)await link.tap();else {await link.focus();await page.keyboard.press('Enter');}
      const heading=page.locator('#a-missing-reading-is-not-an-installation-diagnosis');await heading.waitFor({state:'visible'});
      // Heading and actual renderer output, not a stand-in HTML help page.
      assert.match(await page.locator('article').innerText(),/does not establish whether an agent is absent or stopped/);
      assert.match(await page.locator('article').innerText(),/every\s+filesystem covered by the backup/);
      assert.match(await page.locator('article').innerText(),/Windows or Android/);
      record.help={url:page.url(),heading:await heading.innerText()};
      record.input=await page.evaluate(()=>window.__input);assert.ok(record.input.some(Boolean));
      assert.deepEqual(record.errors,[]);assert.equal(record.requests.filter(r=>!['GET','HEAD'].includes(r.method)).length,0);
      await context.close();await browser.close();browser=null;
    }
    result.result='passed';result.verified_at=new Date().toISOString();
  } catch(error) {result.error=String(error.stack||error);throw error;}
  finally {
    result.cleanup.browser_started = Boolean(browser) || result.cases.length > 0;
    if(browser)await browser.close();result.cleanup.browser=true;
    result.cleanup.server_started = Boolean(server);
    if(server)await new Promise(resolve=>server.close(resolve));result.cleanup.server=true;
    fs.writeFileSync(output+'/result.json',JSON.stringify(result,null,2)+'\n');
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
