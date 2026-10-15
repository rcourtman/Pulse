// Serve the shipped TrueNAS guide through the production Docs page/router.
// No appliance request or monitoring result is mocked or exercised here.
// Run at the assigned root: pulse-worker-browser scripts/check-core-reporting-help.cjs
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium } = require('playwright');

const root = process.cwd();
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp', 'core-reporting-help');
const port = 5198;
const observations = [];
const errors = [];
const captures = [];
const cleanup = { browser: false, server: false };
const digest = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');

function check(passed, message) {
  observations.push({ passed, message });
  if (!passed) throw new Error(message);
}

(async () => {
  fs.mkdirSync(output, { recursive: true });
  process.chdir(frontend);
  const { createServer } = await import(
    path.join(frontend, 'node_modules', 'vite', 'dist', 'node', 'index.js')
  );
  const server = await createServer({
    root: frontend,
    configFile: path.join(frontend, 'vite.config.ts'),
    server: { host: '127.0.0.1', port, strictPort: true },
  });
  let browser;
  let result = { result: 'failed' };
  try {
    await server.listen();
    browser = await chromium.launch({
      headless: true, channel: 'chromium', args: ['--no-sandbox'],
    });
    const page = await browser.newPage();
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('response', (response) => {
      if (response.status() >= 400) errors.push(`${response.status()} ${response.url()}`);
    });
    // Only the fixture's app-auth probe is synthetic. The Markdown, router,
    // renderer, styles and linked target are the actual assigned source.
    await page.route(new RegExp(`^http://127\\.0\\.0\\.1:${port}/api/`), (route) =>
      route.fulfill({
        status: 200, contentType: 'application/json',
        body: JSON.stringify({ hasAuthentication: false }),
      }),
    );
    for (const [name, width, height] of [['desktop', 1440, 1000], ['phone', 390, 844]]) {
      await page.setViewportSize({ width, height });
      await page.goto(`http://127.0.0.1:${port}/browser-tests/docs-fragment-navigation.html?scenario=truenas`);
      const heading = page.locator('#inventory-works-but-cpu-memory-or-history-is-missing');
      await heading.waitFor();
      await heading.scrollIntoViewIfNeeded();
      const section = await heading.evaluate((node) => {
        let text = '';
        for (let next = node.nextElementSibling; next && next.tagName !== 'H3'; next = next.nextElementSibling) {
          text += ` ${next.textContent}`;
        }
        return text.replace(/\s+/g, ' ');
      });
      for (const text of [
        'The following attributes are not expected: query',
        'not measured idle values',
        'does not call for another graph capture, a replacement API key or weaker TLS',
        'Disk I/O and storage capacity are separate.',
        'do not sum dataset sizes or substitute zero for unknown pool bytes',
      ]) check(section.includes(text), `${name}: ${text}`);
      check(!(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)), `${name}: no horizontal overflow`);
      const capture = path.join(output, `${name}-rejection.png`);
      await page.screenshot({ path: capture });
      captures.push({ file: path.relative(root, capture), sha256: digest(capture) });

      const link = page.locator('article p').filter({ hasText: 'before graph data is returned' })
        .getByRole('link', { name: 'ordinary polling checks', exact: true });
      await link.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => document.activeElement?.id === 'stale-truenas-data');
      check(new URL(page.url()).hash === '#stale-truenas-data', `${name}: keyboard link reaches existing polling guidance`);
      check(await page.locator('#stale-truenas-data').isVisible(), `${name}: linked heading is visible`);
      const target = path.join(output, `${name}-polling.png`);
      await page.screenshot({ path: target });
      captures.push({ file: path.relative(root, target), sha256: digest(target) });
      await page.goBack();
      await heading.waitFor();
      check(await page.getByText('Disk I/O and storage capacity are separate.', { exact: true }).count() === 1, `${name}: browser Back preserves the guide`);
    }
    check(errors.length === 0, 'no page errors or failed HTTP responses');
    result = { result: 'passed', chromium: browser.version() };
  } catch (error) {
    result = { result: 'failed', error: error.message };
  } finally {
    if (browser) {
      await browser.close();
      cleanup.browser = true;
    }
    await server.close();
    cleanup.server = true;
  }
  Object.assign(result, {
    observations, errors, captures, cleanup,
    playwright: require('playwright/package.json').version,
    script_sha256: digest(__filename),
    document_sha256: digest(path.join(frontend, 'public/docs/TRUENAS.md')),
  });
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result, null, 2));
  process.exitCode = result.result === 'passed' ? 0 : 1;
})();
