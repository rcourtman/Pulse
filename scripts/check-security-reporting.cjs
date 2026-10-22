// Exercise the existing Docs consumer and shipped security guide, not a
// replacement renderer. This opens no email client and sends no disclosure.
// Run from the workspace root: pulse-worker-browser scripts/check-security-reporting.cjs
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

const root = process.cwd();
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp', 'security-reporting');
const port = 5199;
const observations = [];
const errors = [];
const requests = [];
const captures = [];

function check(passed, message) {
  observations.push({ passed, message });
  if (!passed) throw new Error(message);
}

async function activate(link) {
  await link.focus();
  await link.press('Enter');
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
    // The real app auth probe has no backend in this Docs-only fixture.
    await page.route(`http://127.0.0.1:${port}/api/**`, (route) =>
      route.fulfill({
        status: 200, contentType: 'application/json',
        body: JSON.stringify({ hasAuthentication: false }),
      }),
    );
    await page.route('**/*', (route) => {
      const url = route.request().url();
      requests.push({ method: route.request().method(), url });
      if (!url.startsWith(`http://127.0.0.1:${port}/`)) {
        errors.push(`off-origin request: ${url}`);
        return route.abort();
      }
      return route.fallback();
    });

    const capture = async (name) => {
      check(!(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)), `${name}: no horizontal page overflow`);
      const file = path.join(output, `${name}.png`);
      await page.screenshot({ path: file });
      captures.push(path.relative(root, file));
    };

    for (const [size, width, height] of [['desktop', 1440, 900], ['phone', 320, 900]]) {
      await page.setViewportSize({ width, height });
      // Start at a supported existing fixture, then reach the policy through
      // actual index links with the keyboard. No fixture or app route added.
      await page.goto(`http://127.0.0.1:${port}/browser-tests/docs-fragment-navigation.html`);
      await activate(page.getByRole('link', { name: 'All documentation', exact: false }));
      await page.waitForURL(`**/docs/README`);
      await activate(page.getByRole('link', { name: 'Security guide', exact: true }));
      await page.waitForURL(`**/docs/SECURITY`);
      const article = page.locator('article');
      await article.getByRole('heading', { name: 'Report a vulnerability', exact: true }).waitFor();
      check((await article.locator('h2').first().innerText()) === 'Report a vulnerability', `${size}: private reporting precedes operational instructions`);
      const email = article.getByRole('link', { name: 'security@pulserelay.pro', exact: true }).first();
      check((await email.getAttribute('href')) === 'mailto:security@pulserelay.pro', `${size}: disclosure goes to the existing private email address`);
      check((await article.locator('a').first().getAttribute('href')) === 'mailto:security@pulserelay.pro', `${size}: the first action is not a public issue link`);
      await email.focus(); // Inspect the destination without opening a client.
      check(await email.evaluate((element) => element === document.activeElement), `${size}: private report link is keyboard reachable`);
      const text = await article.innerText();
      for (const phrase of ['Do not put exploit details or private evidence', 'Do not repeat an outage', 'A private email address is not a reason to send credentials', 'those conversations and their attachments are public']) {
        check(text.replace(/\s+/g, ' ').includes(phrase), `${size}: ${phrase}`);
      }
      await capture(`${size}-private-report`);

      await activate(article.getByRole('link', { name: 'disclosure containment steps', exact: true }));
      await page.waitForFunction(() => document.activeElement?.id === 'if-you-already-posted-private-information');
      check((await page.locator('article').innerText()).replace(/\s+/g, ' ').includes('Removing a Pulse connection or stopping an agent is not credential revocation.'), `${size}: containment target explains revocation separately`);
      await capture(`${size}-containment`);

      await page.goBack();
      await article.getByRole('heading', { name: 'Report a vulnerability', exact: true }).waitFor();
      await activate(article.getByRole('link', { name: 'safe reporting guidance', exact: true }).first());
      await page.waitForFunction(() => document.activeElement?.id === '-getting-help');
      check((await page.locator('article').innerText()).includes('Review before posting'), `${size}: ordinary help reaches public evidence precautions`);
      await capture(`${size}-ordinary-help`);
    }
    check(errors.length === 0, 'no page errors, failed responses or off-origin requests');
    check(requests.every((request) => request.method === 'GET'), 'reading and following guidance makes no mutation request');
    result = { result: 'passed' };
  } catch (error) {
    result = { result: 'failed', error: error.message };
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  result = {
    ...result, observations, errors, requests, captures,
    sourceAsset: 'frontend-modern/public/docs/SECURITY.md',
    playwright: require('playwright/package.json').version,
    chromium: browser?.version(),
    cleanup: { browserClosed: !browser?.isConnected(), serverClosed: !server.httpServer?.listening },
  };
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result, null, 2));
  process.exitCode = result.result === 'passed' ? 0 : 1;
})();
