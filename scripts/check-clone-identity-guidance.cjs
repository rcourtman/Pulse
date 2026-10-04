// Real shipped documents, production Docs renderer/router; no live host changes.
// Run from the assigned workspace root:
//   pulse-worker-browser scripts/check-clone-identity-guidance.cjs
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium } = require('playwright');

const root = process.cwd();
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp', 'clone-identity-guidance');
const port = 5198;
const observations = [];
const errors = [];

function check(condition, message) {
  observations.push({ passed: condition, message });
  if (!condition) throw new Error(message);
}

async function sectionText(page, id) {
  return page.locator(`#${id}`).evaluate((heading) => {
    const parts = [heading.textContent];
    for (let node = heading.nextElementSibling; node; node = node.nextElementSibling) {
      if (/^H[1-3]$/.test(node.tagName)) break;
      parts.push(node.textContent);
    }
    return parts.join('\n').replace(/\s+/g, ' ');
  });
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
  let result;
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
    // Authentication probing only; both Markdown documents are real assets.
    await page.route(new RegExp(`^http://127\\.0\\.0\\.1:${port}/api/`), (route) =>
      route.fulfill({
        status: 200, contentType: 'application/json',
        body: JSON.stringify({ hasAuthentication: false }),
      }),
    );

    for (const [size, width, height] of [['desktop', 1440, 900], ['phone', 390, 844]]) {
      await page.setViewportSize({ width, height });
      await page.goto(`http://127.0.0.1:${port}/browser-tests/docs-fragment-navigation.html?scenario=troubleshooting`);
      await page.waitForFunction(() => document.activeElement?.id === 'recovery-mode');
      await page.locator('#docker-hosts-appearingdisappearing').scrollIntoViewIfNeeded();
      const shortGuide = await sectionText(page, 'docker-hosts-appearingdisappearing');
      check(shortGuide.includes('saved Pulse agent ID'), `${size}: saved clone identity is identified`);
      check(shortGuide.includes('Do not delete the OS machine ID'), `${size}: no destructive OS reset is prescribed`);
      const link = page.getByRole('link', { name: 'Clone identity recovery', exact: true });
      check((await link.getAttribute('href')) === '/docs/UNIFIED_AGENT#duplicate-agents', `${size}: link targets the shipped agent guide`);
      check(!(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)), `${size}: troubleshooting does not overflow`);
      await page.screenshot({ path: path.join(output, `${size}-troubleshooting.png`) });

      await link.focus();
      await page.keyboard.press('Enter');
      await page.waitForFunction(() => document.activeElement?.id === 'duplicate-agents');
      const cloneGuide = await sectionText(page, 'duplicate-agents');
      check(cloneGuide.includes('argument wins over the environment variable'), `${size}: argument precedence is explicit`);
      check(cloneGuide.includes('PULSE_AGENT_ID_FILE'), `${size}: custom identity files are covered`);
      check(cloneGuide.includes('Environment="PULSE_AGENT_ID=vm-clone-02"'), `${size}: persistent systemd configuration is rendered`);
      check(cloneGuide.includes('fresh, distinct IDs') && cloneGuide.includes('does not split or recover historical'), `${size}: verification and history limitations are rendered`);
      check(cloneGuide.includes('Do not upload connection.env, token files'), `${size}: credential-bearing evidence is excluded`);
      check(!(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)), `${size}: agent guide does not overflow`);
      await page.locator('article pre').filter({ hasText: 'PULSE_AGENT_ID=vm-clone-02' }).scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(output, `${size}-agent-configuration.png`) });
    }
    check(errors.length === 0, 'no page errors or failed HTTP responses');
    result = {
      result: 'passed', observations, errors, chromium: browser.version(),
      playwright: require('playwright/package.json').version,
      sourceAssets: ['TROUBLESHOOTING.md', 'UNIFIED_AGENT.md'].map((name) => ({
        path: `frontend-modern/public/docs/${name}`,
        sha256: crypto.createHash('sha256').update(fs.readFileSync(path.join(frontend, 'public', 'docs', name))).digest('hex'),
      })),
    };
  } catch (error) {
    result = { result: 'failed', observations, errors, error: error.message };
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result, null, 2));
  process.exitCode = result.result === 'passed' ? 0 : 1;
})();
