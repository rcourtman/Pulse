// Render the existing probe help and its outage/retirement links with the real
// Docs page. No agent, destination, watchdog or Mobile operation is performed.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules', 'probe-guidance-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-probe-guidance'),
    server: { host: '127.0.0.1', port: 5253, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width] of [['chromium', 1280], ['webkit', 390]]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.goto('http://127.0.0.1:5253/browser-tests/docs-fragment-navigation.html?scenario=configuration',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      await page.getByRole('heading', { name: 'External probes (Pro)', exact: true }).waitFor();
      if (engine === 'webkit') await page.evaluate(() => document.documentElement.classList.add('dark'));
      const table = page.getByRole('table').filter({ has: page.getByRole('columnheader',
        { name: 'Situation', exact: true }) });
      assert.equal(await table.getByRole('row').count(), 4);
      assert.ok(await table.evaluate((element) => {
        const parent = element.parentElement;
        return parent.dataset.docTableScroll !== undefined &&
          getComputedStyle(parent).overflowX === 'auto' &&
          [...element.querySelectorAll('thead th')].every((header) => header.scope === 'col');
      }));
      const assertBoundaries = async () => {
        const body = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
        for (const text of ['The agent does not send notifications directly.',
          'Pulse server must be running and able to reach the notification destination',
          'Observation locations', 'This Pulse server', 'not a universal outage',
          'not proof the target is down', 'five minutes or three check intervals',
          '31 March 2027', 'Relay is no longer sold', 'not a permanent substitute']) {
          assert.ok(body.includes(text), text);
        }
        assert.ok(!body.includes('cannot disappear silently'));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      };
      const screenshot = async (name, heading) => {
        await page.getByRole('heading', { name: heading, exact: true })
          .evaluate((element) => element.scrollIntoView({ block: 'start' }));
        await page.evaluate(async () => {
          await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
          await Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => {})));
        });
        await page.screenshot({ path: path.join(artifacts, `${engine}-${name}.png`) });
      };
      const follow = async (name, href, heading, id) => {
        const link = page.getByRole('link', { name, exact: true });
        assert.equal(await link.getAttribute('href'), href);
        await link.focus();
        await page.keyboard.press('Enter');
        await page.waitForURL(`**${href}`);
        await page.getByRole('heading', { name: heading, exact: true }).waitFor();
        if (id) await page.waitForFunction((id) => document.activeElement?.id === id, id);
      };
      await assertBoundaries();
      await screenshot('configuration', 'External probes (Pro)');
      await follow('agent probe guide', '/docs/UNIFIED_AGENT#external-probes-pro',
        'External Probes (Pro)', 'external-probes-pro');
      await assertBoundaries();
      const agentBody = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const text of ['up to 200 observations', 'Oldest pending observations are dropped',
        'agent restart loses the queue', 'not a complete outage record']) assert.ok(agentBody.includes(text), text);
      await screenshot('agent', 'External Probes (Pro)');
      await follow('external watchdog', '/docs/TROUBLESHOOTING#no-alert-when-pulse-power-or-internet-goes-down',
        'No alert when Pulse, power or internet goes down', 'no-alert-when-pulse-power-or-internet-goes-down');
      assert.ok((await page.locator('article').innerText()).includes('period plus grace'));
      await screenshot('watchdog', 'No alert when Pulse, power or internet goes down');
      await page.goBack();
      await page.getByRole('heading', { name: 'External Probes (Pro)', exact: true }).waitFor();
      await follow('Mobile retirement', '/docs/RELAY', 'Relay / Pulse Mobile', '');
      assert.ok((await page.locator('article').innerText()).includes('31 March 2027'));
      await page.goBack();
      await page.getByRole('heading', { name: 'External Probes (Pro)', exact: true }).waitFor();
      await follow('configuration guide', '/docs/CONFIGURATION#external-probes-pro',
        'External probes (Pro)', 'external-probes-pro');
      await follow('target fields', '/docs/API#availability-checks',
        'Availability Checks', 'availability-checks');
      assert.ok((await page.locator('article').innerText()).includes('clear probeAgentId to ""'));
      await page.goBack();
      await page.getByRole('heading', { name: 'External probes (Pro)', exact: true }).waitFor();
      await follow('external watchdog', '/docs/TROUBLESHOOTING#no-alert-when-pulse-power-or-internet-goes-down',
        'No alert when Pulse, power or internet goes down', 'no-alert-when-pulse-power-or-internet-goes-down');
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width,
        scope: engine === 'webkit' ? 'phone-emulated, not a native device' : 'desktop',
        serverDependencyVisible: true, semanticContainedCoverageTable: true,
        noDocumentOverflow: true, queueAndRetirementLimitsVisible: true,
        keyboardAgentGuide: true, keyboardConfigurationGuide: true,
        keyboardWatchdogLinks: 2, keyboardRetirementLink: true, keyboardCanonicalApiFields: true, errors });
      await browser.close();
      browser = undefined;
    }
    const files = ['public/docs/CONFIGURATION.md', 'public/docs/UNIFIED_AGENT.md', 'public/docs/API.md',
      'browser-tests/probe-guidance.cjs', 'browser-tests/docs-fragment-navigation.tsx',
      'src/features/docs/docMarkdown.ts', 'src/pages/Docs.tsx'].map((file) => ({ file,
        sha256: crypto.createHash('sha256').update(fs.readFileSync(path.join(root, file))).digest('hex') }));
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      scope: 'existing shipped Docs rendering and keyboard links only; no native outage or delivery acceptance',
      files, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
