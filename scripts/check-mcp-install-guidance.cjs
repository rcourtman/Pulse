// Offline source-derived README and real shipped Docs consumer proof.
// Run from the assigned repository root with pulse-worker-browser.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium } = require('playwright');

const root = process.cwd();
const frontend = path.join(root, 'frontend-modern');
const output = path.join(root, 'tmp/mcp-install-guidance');
const origin = 'http://127.0.0.1:5197';
const sources = ['cmd/pulse-mcp/README.md', 'docs/AGENT_SUBSTRATE.md',
  'frontend-modern/public/docs/AGENT_SUBSTRATE.md', 'scripts/install-mcp.sh',
  'scripts/install-mcp.ps1', 'frontend-modern/src/pages/Docs.tsx',
  'frontend-modern/src/features/docs/docMarkdown.ts', 'frontend-modern/src/index.css',
  'frontend-modern/browser-tests/docs-fragment-navigation.html',
  'frontend-modern/browser-tests/docs-fragment-navigation.tsx', 'frontend-modern/package-lock.json'];
const observations = [], errors = [], requests = [], captures = [];
function check(condition, message) {
  observations.push({ passed: Boolean(condition), message });
  if (!condition) throw new Error(message);
}
async function activate(link, narrow) {
  if (narrow) await link.tap();
  else { await link.focus(); await link.press('Enter'); }
}

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const readme = fs.readFileSync(path.join(root, sources[0]), 'utf8');
  process.chdir(frontend);
  const { createServer } = await import(path.join(frontend, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({ root: frontend, configFile: path.join(frontend, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5197, strictPort: true } });
  let browser, result;
  try {
    await server.listen();
    browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
    for (const [size, width, narrow] of [['desktop', 1440, false], ['narrow', 320, true]]) {
      const context = await browser.newContext({ viewport: { width, height: 1000 }, hasTouch: narrow,
        colorScheme: narrow ? 'dark' : 'light' });
      for (const scenario of ['readme', 'guide']) {
        const page = await context.newPage();
        page.on('pageerror', error => errors.push(error.message));
        page.on('response', response => { if (response.status() >= 400) errors.push(`${response.status()} ${response.url()}`); });
        await page.route('**/*', route => {
          const request = route.request();
          requests.push({ method: request.method(), url: request.url() });
          if (!request.url().startsWith(origin + '/') || request.method() !== 'GET' || request.url().includes('/api/')) {
            errors.push(`unexpected request: ${request.method()} ${request.url()}`);
            return route.abort();
          }
          if (request.url() === origin + '/docs/API.md') return route.fulfill({ status: 200,
            contentType: 'text/markdown', body: scenario === 'readme' ? readme
              : '# Existing operator guide\n\n[Adapter trust limits](AGENT_SUBSTRATE.md#known-rough-edges)' });
          return route.continue();
        });
        await page.goto(origin + '/browser-tests/docs-fragment-navigation.html');
        const article = page.locator('article');
        await article.waitFor();
        const capture = async (name, target) => {
          await target.evaluate(element => element.scrollIntoView({ block: 'start' }));
          check(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth),
            `${size}/${name}: no document horizontal overflow`);
          const file = path.join(output, `${size}-${name}.png`);
          await page.screenshot({ path: file });
          captures.push(path.relative(root, file));
        };
        if (scenario === 'readme') {
          const text = (await article.innerText()).replace(/\s+/g, ' ');
          for (const phrase of ['Do not pipe a download into Bash or PowerShell',
            'do not cryptographically authenticate the script itself',
            'Stop if the download fails', 'refuses installation if verification is unavailable or fails',
            'do not clear quarantine or override platform security policy',
            'A checksum fetched beside a binary is not by itself proof of origin']) {
            check(text.includes(phrase), `${size}: rendered README retains ${phrase}`);
          }
          const blocks = await article.locator('pre').allTextContents();
          check(blocks.some(text => text.includes('curl --disable') && text.includes('--max-time 60')
            && !/\|\s*(bash|sh)\b/.test(text)), `${size}: download-only Unix recipe`);
          check(blocks.some(text => text.includes('Invoke-WebRequest') && text.includes('-ErrorAction Stop')
            && !/\|\s*(iex|Invoke-Expression)\b/.test(text)), `${size}: download-only Windows recipe`);
          await capture('unix-download', article.getByRole('heading', { name: '1. Use a release installer', exact: true }));
          await capture('windows-download', article.getByText('On Windows, download only in PowerShell:', { exact: true }));
          await capture('manual-trust', article.getByRole('heading', {
            name: '2. Download from a GitHub Release (manual verification required)', exact: true }));
          const signing = article.getByRole('link', { name: 'Windows code-signing status', exact: true });
          check(await signing.getAttribute('href') === '/docs/INSTALL#windows-code-signing-status',
            `${size}: signing link resolves to the existing local guide`);
          await activate(signing, narrow);
          await page.waitForFunction(() => document.activeElement?.id === 'windows-code-signing-status');
          check(await article.getByRole('heading', { name: 'Windows code-signing status', exact: true }).isVisible(),
            `${size}: keyboard/touch navigation reaches actual signing status`);
        } else {
          await activate(article.getByRole('link', { name: 'Adapter trust limits', exact: true }), narrow);
          await page.waitForURL('**/docs/AGENT_SUBSTRATE#known-rough-edges');
          await page.waitForFunction(() => document.activeElement?.id === 'known-rough-edges');
          const text = (await article.innerText()).replace(/\s+/g, ' ');
          for (const phrase of ['Release signatures are not platform approval', 'Leave the block in place',
            'do not clear quarantine or bypass execution policy', 'Neither installer needs an API token']) {
            check(text.includes(phrase), `${size}: shipped guide retains ${phrase}`);
          }
          await capture('shipped-trust', article.getByRole('heading', { name: 'Known rough edges', exact: true }));
        }
        await page.close();
      }
      await context.close();
    }
    check(errors.length === 0, 'no page errors, failed responses or external/API/mutation requests');
    result = { result: 'passed' };
  } catch (error) { result = { result: 'failed', error: error.message }; }
  finally { if (browser) await browser.close(); await server.close(); }
  result = { ...result, observations, errors, requests, captures,
    playwright: require('playwright/package.json').version, chromium: browser?.version(),
    source_sha256: Object.fromEntries(sources.map(file => [file,
      crypto.createHash('sha256').update(fs.readFileSync(path.join(root, file))).digest('hex')])),
    cleanup: { browserClosed: !browser?.isConnected(), serverClosed: !server.httpServer?.listening },
    limits: 'README bytes use the production Docs renderer as an offline fixture, not remote GitHub layout. '
      + 'The operator/signing guides use actual shipped assets and production navigation. '
      + 'No installer, token, downloaded executable, platform bypass or remote service is exercised.' };
  fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result, null, 2));
  process.exitCode = result.result === 'passed' ? 0 : 1;
})();
