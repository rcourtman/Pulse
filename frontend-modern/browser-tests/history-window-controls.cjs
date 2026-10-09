const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const parent = Boolean(global.__historyWindowParentControl);
const output = '/workspace/tmp/history-window-controls/' + (parent ? 'parent' : 'final');
const preview = output + '/preview';
const origin = 'http://127.0.0.1:5332';
const runtime = 'frontend-modern/src/components/shared/HistoryChartHeader.tsx';
const parentFile = '/workspace/tmp/history-window-controls/parent.tsx';
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const load = name => import(require.resolve(name, { paths: [root] }));
(async () => {
  fs.mkdirSync(output, {recursive:true});
  const result = { result:'incomplete', control:parent ? 'exact parent expected defects' : 'final',
    playwright:require('playwright/package.json').version,
    runtime_hash:hash(parent ? parentFile : '/workspace/' + runtime),
    script_hash:hash(root + '/browser-tests/history-window-controls.cjs'),
    cases:[], captures:[], cleanup:{browser:false,server:false},
    limits:'Production header and CSS with synthetic state. No native PBS/NAS, collector, physical device, assistive-device, installed or release acceptance.' };
  let server, browser;
  try {
    assert.equal(result.playwright,JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages['node_modules/@playwright/test'].version);
    const {build} = await load('vite');
    const solidModule = await load('vite-plugin-solid');
    const solid = solidModule.default.default || solidModule.default;
    const {default:tailwind} = await load('@tailwindcss/vite');
    const plugins = [];
    if(parent) plugins.push({name:'exact-parent-header-control',enforce:'pre',load(id) {
      if(id.split('?')[0] === '/workspace/' + runtime) return fs.readFileSync(parentFile,'utf8');
    }});
    plugins.push(solid(),tailwind());
    await build({root,configFile:false,logLevel:'warn',plugins,resolve:{alias:{'@':root + '/src'}},
      build:{target:'esnext',outDir:preview,emptyOutDir:true,minify:false,rollupOptions:{input:root + '/browser-tests/history-window-controls.html'}}});
    server = http.createServer((req,res) => {
      const url=new URL(req.url,origin);
      let file=path.resolve(preview,'.' + decodeURIComponent(url.pathname));
      if(file!==preview && !file.startsWith(preview + '/')) {res.writeHead(403);return res.end();}
      if(!fs.existsSync(file)||fs.statSync(file).isDirectory()) file=preview+'/browser-tests/history-window-controls.html';
      res.setHeader('Content-Type',({'.html':'text/html','.js':'text/javascript','.css':'text/css'})[path.extname(file)]||'application/octet-stream');
      res.end(fs.readFileSync(file));
    });
    await new Promise(resolve=>server.listen(5332,'127.0.0.1',resolve));
    for(const [name,engine,width,dark] of [['chromium-desktop',chromium,1280,false],['webkit-phone',webkit,390,true]]) {
      browser=await engine.launch(engine===chromium ? {headless:true,channel:'chromium',args:['--no-sandbox']} : {headless:true});
      const context=await browser.newContext({viewport:{width,height:760},isMobile:width<=768,hasTouch:width<=768,locale:'en-GB'});
      const page=await context.newPage(); page.setDefaultTimeout(10000);
      const record={name,browser:browser.version(),width,errors:[],blocked:[]}; result.cases.push(record);
      page.on('pageerror',error=>record.errors.push(error.message));
      await page.route('**/*',route=> {
        if(new URL(route.request().url()).origin!==origin) {record.blocked.push(route.request().url());return route.abort();}
        return route.continue();
      });
      await page.addInitScript(dark=> {
        window.__trustedHistoryClicks=[];
        document.addEventListener('click',event=>window.__trustedHistoryClicks.push(event.isTrusted),true);
        document.addEventListener('DOMContentLoaded',()=>{if(dark)document.documentElement.classList.add('dark');});
      },dark);
      await page.goto(origin,{waitUntil:'domcontentloaded'});
      await page.waitForFunction(()=>window.__historyWindows);
      const cpu=page.locator('[data-chart="CPU"]');
      const memory=page.locator('[data-chart="Memory"]');
      record.initialLayout=await cpu.evaluate(el=>({width:el.clientWidth,scroll:el.scrollWidth,document:document.documentElement.clientWidth,documentScroll:document.documentElement.scrollWidth}));
      if(parent) {
        assert.equal(await cpu.getByRole('group').count(),0);
        assert.equal(await cpu.locator('[aria-pressed]').count(),0);
        await cpu.getByRole('button',{name:'24h',exact:true}).click();
        assert.equal(await page.evaluate(()=>window.__historyWindows.submissions()),1);
        if(width<=768) assert.ok(record.initialLayout.scroll>record.initialLayout.width+1,JSON.stringify(record.initialLayout));
        record.expectedDefects={noNamedGroup:true,noSelectionState:true,nativeFormSubmitted:true,phoneOverflow:width<=768};
      } else {
        const cpuGroup=cpu.getByRole('group',{name:'CPU history window'});
        const memoryGroup=memory.getByRole('group',{name:'Memory history window'});
        assert.equal(await cpuGroup.getByRole('button',{pressed:true}).count(),1);
        assert.equal(await cpuGroup.getByRole('button',{name:'1h',exact:true}).getAttribute('aria-pressed'),'true');
        if(width<=768) {
          await cpuGroup.getByRole('button',{name:'7d',exact:true}).tap();
          assert.equal(await cpuGroup.getByRole('button',{name:'7d',exact:true}).getAttribute('aria-pressed'),'true');
          await memoryGroup.getByRole('button',{name:'6h',exact:true}).tap();
          await page.waitForFunction(()=>Array.from(document.querySelectorAll('[data-chart="Memory"] button')).some(button=>button.textContent.trim()==='6h' && button.getAttribute('aria-pressed')==='true'));
          assert.equal(await memoryGroup.getByRole('button',{name:'6h',exact:true}).getAttribute('aria-pressed'),'true');
          assert.equal(await cpuGroup.getByRole('button',{name:'7d',exact:true}).getAttribute('aria-pressed'),'true');
        } else {
          const button=cpuGroup.getByRole('button',{name:'24h',exact:true});
          await button.focus();await page.keyboard.press('Enter');
          assert.equal(await button.getAttribute('aria-pressed'),'true');
          assert.equal(await button.evaluate(el=>el===document.activeElement),true);
          await page.keyboard.press('Tab');
          assert.equal(await cpuGroup.getByRole('button',{name:'7d',exact:true}).evaluate(el=>el===document.activeElement),true);
          await page.keyboard.press('Space');
          assert.equal(await cpuGroup.getByRole('button',{name:'7d',exact:true}).getAttribute('aria-pressed'),'true');
          assert.equal(await memoryGroup.getByRole('button',{name:'1h',exact:true}).getAttribute('aria-pressed'),'true');
        }
        assert.equal(await cpuGroup.getByRole('button',{pressed:true}).count(),1);
        assert.equal(await memoryGroup.getByRole('button',{pressed:true}).count(),1);
        assert.equal(await page.evaluate(()=>window.__historyWindows.submissions()),0);
        assert.equal(await page.locator('[data-chart="hidden"]').getByRole('group').count(),0);
        assert.ok(record.initialLayout.scroll<=record.initialLayout.width+1,JSON.stringify(record.initialLayout));
        assert.ok(record.initialLayout.documentScroll<=record.initialLayout.document+1,JSON.stringify(record.initialLayout));
        record.selection=await page.locator('[data-chart] button').evaluateAll(buttons=>buttons.map(button=>({text:button.textContent.trim(),pressed:button.getAttribute('aria-pressed'),class:button.className})));
        for(const button of record.selection) assert.equal(button.class.includes('bg-surface text-base-content'),button.pressed==='true');
        record.calls=await page.evaluate(()=>window.__historyWindows.calls);
        record.trusted=await page.evaluate(()=>window.__trustedHistoryClicks);
        assert.ok(record.trusted.length>=2 && record.trusted.every(Boolean));
      }
      assert.deepEqual(record.errors,[]);assert.deepEqual(record.blocked,[]);
      await page.evaluate(async()=> {
        // Observe completed finite colour transitions, not a transient tap frame.
        await Promise.all(document.getAnimations().filter(animation=>Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation=>animation.finished.catch(()=>{})));
        await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
      });
      const capture=name+'.png';await page.screenshot({path:output+'/'+capture,fullPage:true});
      result.captures.push({file:capture,sha256:hash(output+'/'+capture)});
      await browser.close();browser=undefined;
    }
    result.result='passed';
  } finally {
    if(browser)await browser.close(); result.cleanup.browser=true;
    if(server)await new Promise(resolve=>server.close(resolve)); result.cleanup.server=true;
    fs.writeFileSync(output+'/result.json',JSON.stringify(result,null,2)+'\n');
  }
  console.log(JSON.stringify(result));
})().catch(error=>{console.error(error);process.exitCode=1;});
