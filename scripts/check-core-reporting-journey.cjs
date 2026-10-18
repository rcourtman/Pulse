// Run from the assigned root after the exact-source frontend build and the
// authenticated Go journey have emitted tmp/core-reporting-journey inputs.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium } = require('playwright');
const root = '/workspace';
const output = path.join(root, 'tmp/core-reporting-journey/browser');
const staticRoot = path.join(root, 'tmp/core-reporting-journey/static');
const origin = 'http://127.0.0.1:5369';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
(async () => {
 fs.mkdirSync(output, { recursive: true });
 const input = JSON.parse(fs.readFileSync(path.join(root, 'tmp/core-reporting-journey/source.json')));
 const samplesBytes = fs.readFileSync(path.join(root, 'tmp/core-reporting-journey/samples.json'));
 assert.equal(hash(samplesBytes), input.samples_sha256);
 const samples = JSON.parse(samplesBytes);
 const binding = JSON.parse(fs.readFileSync(path.join(staticRoot, 'binding.json')));
 assert.equal(binding.source_sha, input.source_sha);
 for (const [file, digest] of Object.entries(binding.sources))
  assert.equal(hash(fs.readFileSync(path.resolve(root, 'frontend-modern', file))), digest, file);
 const assets = new Map();
 for (const [file, digest] of Object.entries(binding.artifacts)) {
  const bytes = fs.readFileSync(path.join(staticRoot, file));
  assert.equal(hash(bytes), digest, file);
  assets.set('/' + file, bytes);
 }
 const result = { result: 'incomplete', source: input, playwright: require('playwright/package.json').version,
  cases: [], requests: [], screenshots: [], errors: [], cleanup: {},
  limits: 'Synthetic CORE REST fixture through production pinned-TLS/provider/poller/monitor/authenticated resource and chart HTTP captured in the Go VM; captured full JSON responses decoded/re-encoded only for local HTTP and served to the production decoder, TrueNAS table, inline drawer and charts. No appliance, installed or released recovery.',
 };
 assert.equal(result.playwright, JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages['node_modules/@playwright/test'].version);
 let phase = 'complete';
 const server = http.createServer((req, res) => {
  try {
   assert.equal(req.method, 'GET', 'no mutation is permitted');
   const url = new URL(req.url, origin);
   if (url.pathname.startsWith('/api/')) {
    result.requests.push({ phase, path: url.pathname, query: Object.fromEntries(url.searchParams) });
    res.setHeader('Content-Type', 'application/json');
    res.setHeader('Cache-Control', 'no-store');
    if (url.pathname === '/api/resources') { res.end(JSON.stringify(samples[phase].resources)); return; }
    if (url.pathname === '/api/metrics-store/history') {
     assert.equal(url.searchParams.get('resourceType'), 'agent');
     assert.equal(url.searchParams.get('resourceId'), 'core-rest-fixture');
     assert.equal(url.searchParams.get('maxPoints'), '240');
     const range = url.searchParams.get('range');
     assert(['1h', '24h'].includes(range), `uncaptured range ${range}`);
     const response = range === '24h' ? samples[phase].history24 : samples[phase].history;
     res.statusCode = response.status;
     res.end(JSON.stringify(response.body)); return;
    }
    res.end(JSON.stringify(url.pathname === '/api/license/runtime-capabilities' ? {
     capabilities: [], limits: [], max_history_days: 7, hosted_mode: false, runtime: { build: 'community' }, blocked_capabilities: [],
    } : { success: true, enabled: false, data: [], alerts: [], anomalies: [], config: null, capabilities: [], relationships: [], recentChanges: [], available: false, audits: [], count: 0 })); return;
   }
   if (url.pathname === '/favicon.ico') { res.writeHead(204); res.end(); return; }
   const bytes = assets.get(url.pathname);
   assert(bytes, `Unbound asset ${url.pathname}`);
   res.setHeader('Content-Type', { '.html':'text/html', '.js':'application/javascript', '.css':'text/css' }[path.extname(url.pathname)]);
   res.end(bytes);
  } catch (error) { result.errors.push(error.message); res.statusCode=500; res.end(); }
 });
 let browser;
 try {
  await new Promise((resolve) => server.listen(5369, '127.0.0.1', resolve));
  browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
  result.chromium = browser.version();
  for (const width of [1440, 320]) {
   const context = await browser.newContext({ viewport: { width, height: width===320 ? 1000 : 1100 }, hasTouch: width===320, isMobile: width===320, locale:'en-GB', timezoneId:'UTC' });
   const page = await context.newPage();
   page.setDefaultTimeout(12000);
   page.on('pageerror', (e) => result.errors.push(e.message));
   await page.route('**/*', (route) => new URL(route.request().url()).origin === origin ? route.continue() : route.abort());
   for (phase of input.phases || ['complete', 'rejected', 'partial', 'zero', 'recovered', 'retained']) {
    const record = { phase, width, checks: [] };
    result.cases.push(record);
    const check = (condition, label) => { assert(condition, label); record.checks.push(label); };
    const host = samples[phase].resources.data.find((resource) => resource.type === 'agent');
    await page.clock.setFixedTime(new Date(Date.parse(host.lastSeen) + 1000));
    await page.goto(origin + '/browser-tests/core-reporting-journey.html');
    const row = page.locator('tr').filter({ hasText: 'core-fixture' }).first();
    await row.waitFor();
    const rowText = await row.innerText();
    if (['complete', 'recovered'].includes(phase)) check(rowText.includes('12%') && rowText.includes('38%'), 'native CPU and rounded ARC-aware memory reach the table');
    if (phase === 'rejected' || phase === 'partial' || phase === 'retained') {
     const memoryCell = await row.locator('td').nth(2).innerText();
     check(!memoryCell.includes('%'), 'unavailable memory is not shown as idle');
     if (phase === 'rejected' || phase === 'retained') check(!(await row.locator('td').nth(1).innerText()).includes('%'), 'unavailable CPU is not shown as idle');
    }
    if (width === 1440 && phase === 'complete') {
     await row.locator('td').nth(2).locator('.relative').hover();
     await page.getByText('Used with ARC cache', { exact: true }).waitFor();
     const tooltip = page.getByText('Reclaimable cache', { exact: true }).locator('..').locator('..');
     record.memoryTooltip = await tooltip.innerText();
     check(['6.00 GB', '8.00 GB', '2.00 GB', '88%'].every((value) => record.memoryTooltip.includes(value)), 'free pages, ARC cache and used-with-cache comparison remain distinct');
     const file = '1440-memory.png';
     await page.screenshot({ path:path.join(output,file), animations:'disabled' });
     result.screenshots.push({ file, width, phase, sha256:hash(fs.readFileSync(path.join(output,file))) });
     await page.mouse.move(0,0);
    }
    if (width === 320) await row.tap();
    else {
     const disclosure = row.getByRole('button', { name: /expand details/i });
     await disclosure.focus(); await page.keyboard.press('Enter');
     check(await disclosure.getAttribute('aria-expanded') === 'true', 'keyboard opens the existing inline drawer');
    }
    const drawer = page.locator('[data-inline-detail-for]');
    await drawer.waitFor();
    const select = drawer.getByTestId('guest-history-range-control');
    check(await select.inputValue() === '24h', 'the existing drawer opens on its default day range');
    await drawer.locator('[data-history-group="utilization"]').getByRole('img').waitFor();
    await select.selectOption('1h');
    await page.waitForTimeout(400);
    const utilization = drawer.locator('[data-history-group="utilization"]');
    { 
     await utilization.waitFor();
     const chart = utilization.getByRole('img');
     await chart.waitFor();
     const inspection = utilization.getByRole('slider', { name:'Inspect Utilization history' });
     if (phase !== 'rejected' && await inspection.count()) {
      await inspection.focus(); await page.keyboard.press('Home');
      check(await inspection.inputValue() === '0', 'keyboard inspection selects the first actual native observation, not the later independent capacity sample');
     }
     if (phase === 'retained') check(samples.retained.history.body.metrics.cpu.length === 1 && samples.retained.history.body.metrics.memory.length === 1, 'warm rejection renders a single recorded CPU/memory observation, not invented optional readings');
     const description = await chart.evaluate((node) => document.getElementById(node.getAttribute('aria-describedby'))?.textContent || '');
     record.utilization = description;
     if (phase === 'rejected') check(!description.includes('CPU 0.0%') && !description.includes('Memory 0.0%') && !description.includes('12.0%') && !description.includes('37.5%'), 'missing CPU and memory are not idle, while capacity remains independently charted');
     else check(description.includes(phase==='zero' ? '0.0%' : '12.0%'), 'CPU History keeps its native value including measured zero');
     if (phase === 'partial') check(!description.includes('37.5%'), 'CPU-only history does not invent sibling memory');
     else if (phase !== 'rejected') check(description.includes(phase==='zero' ? '0.0%' : '37.5%'), 'memory History uses the same ARC-aware reading');
    }
    // At narrow widths the existing more-charts control reveals I/O/thermals.
    const more = drawer.getByTestId('history-phone-toggle');
    if (width === 320 && await more.isVisible()) { await more.click(); }
    const io = drawer.locator('[data-history-group="disk-io"]');
    if (phase !== 'rejected' && phase !== 'partial') {
     await io.getByRole('img').waitFor();
     record.io = await io.innerText();
     check((await io.innerText()).includes(phase==='zero' ? '0 B/s' : '1.00 KB/s'), 'native disk I/O reaches its own chart');
    }
    check(!(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)), 'no document horizontal overflow');
    if (['complete','rejected','zero','retained'].includes(phase)) {
     await utilization.scrollIntoViewIfNeeded();
     const file = `${width}-${phase}.png`;
     await page.screenshot({ path: path.join(output, file), animations:'disabled' });
     result.screenshots.push({ file, width, phase, sha256:hash(fs.readFileSync(path.join(output,file))) });
    }
   }
   await context.close();
  }
  assert.equal(result.errors.length, 0, JSON.stringify(result.errors));
  result.result = 'passed';
 } catch (error) {
  result.result = 'failed'; result.failure = error.stack;
 } finally {
  if (browser) { await browser.close(); result.cleanup.browser=true; }
  await new Promise((resolve) => server.close(resolve)); result.cleanup.server=true;
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result,null,2)+'\n');
  console.log(JSON.stringify(result,null,2));
 }
 process.exitCode = result.result==='passed' ? 0 : 1;
})();
