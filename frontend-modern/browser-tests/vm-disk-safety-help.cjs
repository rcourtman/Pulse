// Exercise production Docs/router/Markdown. No Proxmox or guest is contacted.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = path.resolve('frontend-modern');
  const artifacts = path.join(root, 'node_modules', 'vm-disk-safety-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const guides = Object.fromEntries(['VM_DISK_MONITORING', 'TROUBLESHOOTING'].map(name =>
    [name, fs.readFileSync(path.join(root, 'public/docs', `${name}.md`), 'utf8')]));
  const recipe = guides.VM_DISK_MONITORING.match(/```bash\n([\s\S]*?)```/)[1].trim();
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root, configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-vm-disk-safety'),
    server: { host: '127.0.0.1', port: 5265, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, tone] of [['chromium', 1280, 'light'], ['webkit', 390, 'dark']]) {
      browser = await { chromium, webkit }[engine].launch(engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const page = await browser.newPage({ viewport: { width, height: 1000 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}) });
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
      await page.goto('http://127.0.0.1:5265/browser-tests/docs-fragment-navigation.html',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      if (tone === 'dark') await page.evaluate(() => document.documentElement.classList.add('dark'));
      await page.getByRole('link', { name: '← All documentation' }).click();
      const entry = page.locator('a[data-doc-link][href="/docs/VM_DISK_MONITORING"]').first();
      await entry.focus();
      await page.keyboard.press('Enter');
      const safety = page.getByRole('heading', { name: 'Backup safety', exact: true });
      await safety.waitFor();
      await safety.scrollIntoViewIfNeeded();
      let text = await page.locator('article').innerText();
      for (const phrase of ['Do not run manual guest-agent probes', 'monitoring and alerts',
        'does not prove thaw', 'temporary precaution', 'not established as a reproduced cause',
        'clear backup locks', 'host-root diagnostic does not test']) {
        assert.ok(text.replace(/\s+/g, ' ').includes(phrase), phrase);
      }
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-backup-safety.png`) });
      const passive = page.getByRole('heading', { name: 'Passive host preflight', exact: true });
      await passive.scrollIntoViewIfNeeded();
      const code = page.locator('pre').filter({ hasText: 'sudo bash ./scripts/test-vm-disk.sh 100' });
      assert.equal((await code.locator('code').textContent()).trim(), recipe);
      const section = text.replace(/\s+/g, ' ').split('Passive host preflight')[1];
      for (const phrase of ['no guest-agent commands', 'older copies', 'bounded timeouts',
        'non-zero exit', 'does not verify disk freshness', 'before running it']) {
        assert.ok(section.includes(phrase), phrase);
      }
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-passive-preflight.png`) });
      const related = page.getByRole('link', { name: 'Getting help', exact: true });
      await related.focus();
      await page.keyboard.press('Enter');
      await page.getByRole('heading', { name: '🆘 Getting Help', exact: true }).waitFor();
      text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const phrase of ['Do not run guest-agent probes during backup freeze/thaw',
        'passive host preflight', 'they do not by themselves establish why disk usage is absent']) {
        assert.ok(text.includes(phrase), phrase);
      }
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      assert.deepEqual(errors, []);
      results.push({ engine, browserVersion: browser.version(), width, tone,
        keyboardEntry: true, backupPrecautionAndOutage: true, exactCopiedShell: true,
        passiveLimits: true, relatedHelpNavigation: true, noBodyOverflow: true, errors });
      await browser.close();
      browser = undefined;
    }
    const result = { result: 'passed', playwrightVersion: require('playwright/package.json').version,
      scope: 'rendered production help and copied recipe; no native guest or backup acceptance',
      docs: Object.fromEntries(Object.entries(guides).map(([name, content]) =>
        [`frontend-modern/public/docs/${name}.md`, createHash('sha256').update(content).digest('hex')])),
      results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
