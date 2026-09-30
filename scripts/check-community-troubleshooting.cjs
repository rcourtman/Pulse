// Render the real shipped troubleshooting guide with the production Docs page.
// Run from the assigned workspace root:
//   pulse-worker-browser scripts/check-community-troubleshooting.cjs
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

const root = process.cwd();
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp', 'community-troubleshooting');
const port = 5198;
const observations = [];
const errors = [];

function check(condition, message) {
  observations.push({ passed: condition, message });
  if (!condition) throw new Error(message);
}

(async () => {
  fs.mkdirSync(output, { recursive: true });
  // Tailwind resolves its config/content globs from the frontend working dir.
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
    // Only app-runtime auth probes are mocked; the Markdown asset is real.
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
      const article = page.locator('article');
      const text = await article.innerText();
      check(text.includes('browser-bound recovery session'), `${size}: browser-bound recovery is explained`);
      check(text.includes('A successful curl response does not unlock a separate browser'), `${size}: curl is not presented as a browser unlock`);
      check(!(await article.locator('pre').allTextContents()).some((code) => code.includes('X-Recovery-Token') || code.includes('generate_token')), `${size}: no recovery credential command is offered`);
      check(text.includes('Export for GitHub (sanitized)') && text.includes('Review before posting'), `${size}: safe reporting evidence is visible`);
      check(text.includes('Do not repeat an update, outage or notification storm'), `${size}: unsafe reproduction is not required`);
      check(!(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)), `${size}: no horizontal page overflow`);
      await page.screenshot({ path: path.join(output, `${size}-recovery.png`) });

      // The supported reset link must work with the real router and heading ID.
      await article.getByRole('link', { name: 'I forgot my password', exact: true }).click();
      await page.waitForFunction(() => document.activeElement?.id === 'i-forgot-my-password');
      check((await page.locator('#i-forgot-my-password').innerText()) === 'I forgot my password', `${size}: reset link reaches the deployment-specific steps`);
      await article.getByText(size === 'phone' ? 'Review before posting' : 'Keep the original evidence', { exact: true }).scrollIntoViewIfNeeded();
      await page.screenshot({ path: path.join(output, `${size}-help.png`) });
    }
    check(errors.length === 0, 'no page errors or failed HTTP responses');
    result = {
      result: 'passed', observations, errors, chromium: browser.version(),
      playwright: require('playwright/package.json').version,
      sourceAsset: 'frontend-modern/public/docs/TROUBLESHOOTING.md',
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
