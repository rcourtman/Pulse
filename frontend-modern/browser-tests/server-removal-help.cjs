// Render the shipped help and follow its recovery/agent links. Never execute
// any removal command: this verifies guidance, not native uninstall behaviour.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

(async () => {
  const root = '/workspace/frontend-modern';
  const artifacts = path.join(root, 'node_modules', 'server-removal-help-browser');
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(root, 'node_modules', '.vite-server-removal-help'),
    server: { host: '127.0.0.1', port: 5252, strictPort: true },
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
      await page.goto('http://127.0.0.1:5252/browser-tests/docs-fragment-navigation.html?scenario=installation',
        { waitUntil: 'domcontentloaded', timeout: 60_000 });
      const uninstall = page.getByRole('heading', { level: 2, name: '🗑️ Uninstall', exact: true });
      await uninstall.waitFor();
      if (engine === 'webkit') {
        await page.evaluate(async () => {
          document.documentElement.classList.add('dark');
          await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
          await Promise.all(document.getAnimations().map((animation) => animation.finished.catch(() => {})));
        });
      }
      const section = await uninstall.evaluate((heading) => {
        const nodes = [];
        for (let node = heading.nextElementSibling; node; node = node.nextElementSibling)
          nodes.push(node);
        return {
          text: nodes.map((node) => node.innerText).join(' ').replace(/\s+/g, ' '),
          commands: nodes.filter((node) => node.tagName === 'PRE').flatMap((node) =>
            node.innerText.replace(/\\\n\s*/g, ' ').trim().split('\n')
              .map((line) => line.replace(/\s+/g, ' ').trim())),
          commandBoxes: nodes.filter((node) => node.tagName === 'PRE').map((node) => ({
            clientWidth: node.clientWidth, scrollWidth: node.scrollWidth,
          })),
        };
      });
      for (const text of ['stops monitoring and alert delivery', 'Keep persistent data by default',
        'every effective data path', 'A configuration export alone is not a full backup',
        'container\'s writable layer or temporary storage', '--rm can also delete anonymous volumes',
        'reattach the same data mount', 'prefixes volume names', 'without a keep policy',
        'reclaim policy can delete its backing data', 'emptyDir storage, lost when the pod is removed',
        'inside the Pulse container', 'disable Pulse, not fully uninstall it',
        'without a keep-data prompt', 'Removing the server does not remove agents'])
        assert.ok(section.text.includes(text), text);
      assert.deepEqual(section.commands, [
        'docker stop pulse', 'docker rm pulse', 'docker compose stop pulse', 'docker compose rm pulse',
        'kubectl scale deployment pulse --namespace pulse --replicas=0',
        'sudo systemctl disable --now pulse-update.timer', 'sudo systemctl disable --now pulse.service',
      ]);
      assert.ok(section.commandBoxes.every((box) => box.scrollWidth <= box.clientWidth + 1),
        'removal commands must fit without phone clipping');
      for (const [name, heading] of [
        ['docker', uninstall],
        ['kubernetes', page.getByRole('heading', { level: 3,
          name: 'Kubernetes: check claim ownership before uninstalling', exact: true })],
        ['systemd', page.getByRole('heading', { level: 3,
          name: 'Systemd / Proxmox LXC: disable without erasing data', exact: true })],
      ]) {
        await heading.evaluate((element) => element.scrollIntoView({ block: 'start' }));
        await page.screenshot({ path: path.join(artifacts, `${engine}-${name}.png`) });
      }
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      const backup = page.getByRole('link', { name: 'full-state backup', exact: true });
      assert.equal(await backup.getAttribute('href'), '/docs/MIGRATION#full-state-recovery');
      await backup.focus();
      await page.keyboard.press('Enter');
      await page.waitForURL('**/docs/MIGRATION#full-state-recovery');
      await page.getByRole('heading', { level: 3, name: 'Full-state recovery', exact: true }).waitFor();
      assert.equal(await page.evaluate(() => document.activeElement?.id), 'full-state-recovery');
      await page.goBack();
      await uninstall.waitFor();
      const agent = page.getByRole('link', { name: 'uninstall procedure', exact: true });
      assert.equal(await agent.getAttribute('href'), '/docs/UNIFIED_AGENT#uninstall');
      await agent.focus();
      await page.keyboard.press('Enter');
      await page.waitForURL('**/docs/UNIFIED_AGENT#uninstall');
      await page.getByRole('heading', { level: 2, name: 'Uninstall', exact: true }).waitFor();
      assert.equal(await page.evaluate(() => document.activeElement?.id), 'uninstall');
      assert.deepEqual(errors, []);
      results.push({ engine, version: browser.version(), width,
        persistentDataWarning: true, commandsFit: true, noDocumentOverflow: true,
        helmRetentionWarning: true, installerErasureWarning: true,
        keyboardFullStateRecovery: true, keyboardAgentUninstall: true, errors });
      await browser.close();
      browser = undefined;
    }
    const files = ['public/docs/INSTALL.md', 'browser-tests/server-removal-help.cjs',
      'browser-tests/docs-fragment-navigation.tsx', 'src/features/docs/docMarkdown.ts',
      'src/pages/Docs.tsx'].map((file) => ({ file,
        sha256: crypto.createHash('sha256').update(fs.readFileSync(path.join(root, file))).digest('hex') }));
    const result = { result: 'passed', playwright: require('playwright/package.json').version,
      scope: 'shipped server-removal help rendering and keyboard links; no removal/native commands executed',
      dompurify: JSON.parse(fs.readFileSync(path.join(root, 'node_modules/dompurify/package.json'))).version,
      files, results };
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
