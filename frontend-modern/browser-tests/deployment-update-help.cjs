// Production Docs/router and real shipped Markdown; no backend or deployment.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  process.chdir(root);
  const artifacts = path.join(root, 'node_modules', 'deployment-update-help-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-deployment-update-help'),
    server: { host: '127.0.0.1', port: 5250, strictPort: true },
    optimizeDeps: { noDiscovery: true, include: [] },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [engine, width, tone] of [['chromium', 1280, 'light'], ['webkit', 390, 'dark']]) {
      browser = await { chromium, webkit }[engine].launch(
        engine === 'chromium'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: 900 },
        ...(engine === 'webkit' ? { isMobile: true, hasTouch: true } : {}),
      });
      const errors = [];
      page.on('pageerror', (e) => errors.push(e.message));
      page.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()); });
      await page.goto('http://127.0.0.1:5250/browser-tests/docs-fragment-navigation.html',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      await page.getByRole('link', { name: '← All documentation' }).click();
      await page.getByRole('link', { name: 'Deployment models', exact: true }).click();
      await page.getByRole('heading', { name: 'Updates by Model', exact: true }).waitFor();
      if (tone === 'dark') await page.evaluate(async () => {
        document.documentElement.classList.add('dark');
        await new Promise((resolve) => requestAnimationFrame(resolve));
        await Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => {})));
      });
      const text = await page.locator('article').innerText();
      for (const phrase of ['defaults, not a backup inventory', 'SQLite sidecar files',
        'snapshot is not necessarily a complete data backup', 'Persist the exact target',
        'Paid Pro installs must keep their private image', 'only the Pulse service',
        'stopping if the pull fails', 'existing Helm release and namespace',
        'current PVC, keys and settings']) assert.ok(text.includes(phrase), phrase);
      for (const unsafe of ['docker pull rcourtman/pulse:latest', 'docker compose up -d',
        'helm upgrade pulse pulse/pulse', 'swapping binaries/config safely'])
        assert.ok(!text.includes(unsafe), unsafe);
      await page.getByRole('heading', { name: 'Updates by Model', exact: true })
        .evaluate((heading) => heading.scrollIntoView({ block: 'start' }));
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await page.screenshot({ path: path.join(artifacts, `${engine}-overview.png`) });

      // Keyboard activation must reach the actual maintained procedure/anchor.
      const docker = page.getByRole('link', { name: 'Docker server updates', exact: true });
      await docker.focus();
      await page.keyboard.press('Enter');
      await page.getByRole('heading', { name: '🔄 Updates', exact: true }).waitFor();
      await page.waitForFunction(() => {
        const heading = document.getElementById('-updates');
        return heading && heading.getBoundingClientRect().top >= 0 && heading.getBoundingClientRect().top < innerHeight;
      });
      assert.ok(page.url().endsWith('/docs/DOCKER#-updates'));
      const dockerText = await page.locator('article').innerText();
      for (const phrase of ['persist PULSE_IMAGE', 'docker compose pull pulse',
        'docker compose up -d --no-deps pulse', 'paid Pro installs must keep the private image'])
        assert.ok(dockerText.includes(phrase), phrase);
      await page.screenshot({ path: path.join(artifacts, `${engine}-docker.png`) });
      await page.getByRole('link', { name: 'rollback and backup scope', exact: true }).click();
      await page.getByRole('heading', { name: 'Rollback', exact: true }).waitFor();
      assert.ok(page.url().endsWith('/docs/AUTO_UPDATE#rollback'));
      assert.ok((await page.locator('article').innerText()).includes('Copy errors can leave a partial snapshot'));

      await page.goBack();
      await page.goBack();
      await page.getByRole('heading', { name: 'Updates by Model', exact: true }).waitFor();
      await page.getByRole('link', { name: 'Kubernetes deployment settings', exact: true }).click();
      await page.waitForFunction(() => location.pathname === '/docs/KUBERNETES' &&
        document.querySelector('article')?.textContent.includes('Kubernetes cluster'));
      await page.goBack();
      await page.getByRole('link', { name: 'Metrics storage', exact: true }).click();
      await page.getByRole('heading', { name: 'Storage Location', exact: true }).waitFor();
      assert.ok(page.url().endsWith('/docs/METRICS_HISTORY#storage-location'));
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width, tone,
        overviewLimits: true, keyboardDockerLink: true, scopedDockerProcedure: true,
        rollbackLink: true, kubernetesLink: true, metricsLink: true, errors });
      await browser.close();
      browser = undefined;
    }
    const result = { playwright: require('playwright/package.json').version, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), `${JSON.stringify(result, null, 2)}\n`);
    process.stdout.write(`${JSON.stringify(result)}\n`);
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { process.stderr.write(`${error.stack}\n`); process.exitCode = 1; });
