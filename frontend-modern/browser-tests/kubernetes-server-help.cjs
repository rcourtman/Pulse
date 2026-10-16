// Actual shipped Docs/router/Markdown/styles; synthetic shell only.
// No Helm, cluster, credential, administrative API or rollout is exercised.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace';
const frontend = path.join(root, 'frontend-modern');
const preview = path.join(root, 'tmp/doc-command-help/static');
const artifacts = path.join(root, 'tmp/kubernetes-server-help');
const hash = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const guide = fs.readFileSync(path.join(frontend, 'public/docs/KUBERNETES.md'), 'utf8');
const serverSection = guide.split('## 1. Deploying the Pulse Server\n')[1].split('\n## 2.')[0];
const openshiftSection = guide.split('### OpenShift profile (Helm)\n')[1].split('\n### ')[0];
const recipes = [...(serverSection + openshiftSection).matchAll(/```bash\n([\s\S]*?)```/g)]
  .map(match => match[1].trim());
const result = { source: binding.source_sha, binding_sha256: hash(bindingBytes),
  cases: [], captures: [], pageErrors: [], requests: [], served: [] };

(async () => {
  let server, browser;
  fs.mkdirSync(artifacts, { recursive: true });
  try {
    assert.equal(recipes.length, 5);
    for (const [file, expected] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(frontend, file))), expected, file);
    for (const [file, expected] of Object.entries(binding.artifacts))
      assert.equal(hash(fs.readFileSync(path.join(preview, file))), expected, file);
    server = http.createServer((req, res) => {
      const route = new URL(req.url, 'http://fixture.invalid').pathname;
      let asset = route.slice(1);
      if (route.startsWith('/docs/') && !route.endsWith('.md'))
        asset = 'browser-tests/doc-command-help.html';
      if (!Object.hasOwn(binding.artifacts, asset)) { res.writeHead(404); res.end(); return; }
      const bytes = fs.readFileSync(path.join(preview, asset));
      assert.equal(hash(bytes), binding.artifacts[asset]);
      result.served.push({ route, asset, sha256: hash(bytes) });
      const type = asset.endsWith('.md') ? 'text/markdown' : asset.endsWith('.css') ? 'text/css' :
        asset.endsWith('.js') ? 'text/javascript' : 'text/html';
      res.writeHead(200, { 'Content-Type': `${type}; charset=utf-8`, 'Cache-Control': 'no-store' });
      res.end(bytes);
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const origin = `http://127.0.0.1:${server.address().port}`;
    result.playwright = require('playwright/package.json').version;
    assert.equal(result.playwright, '1.56.1');
    for (const state of [
      { name: 'desktop-light', engine: 'chromium', width: 1440, height: 1000, dark: false },
      { name: 'phone-dark', engine: 'webkit', width: 320, height: 900, dark: true },
    ]) {
      browser = await { chromium, webkit }[state.engine].launch(state.engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] } : { headless: true });
      const context = await browser.newContext({ viewport: { width: state.width, height: state.height },
        isMobile: state.width < 500, hasTouch: state.width < 500 });
      const page = await context.newPage();
      page.on('pageerror', error => result.pageErrors.push(error.message));
      page.on('request', req => result.requests.push({ method: req.method(), url: req.url() }));
      await page.route('**/*', route => new URL(route.request().url()).origin === origin
        ? route.continue() : route.abort());
      await page.addInitScript(dark => {
        document.addEventListener('DOMContentLoaded', () => document.documentElement.classList.toggle('dark', dark));
      }, state.dark);
      const activate = async locator => {
        if (state.width < 500) await locator.tap();
        else { await locator.focus(); await page.keyboard.press('Enter'); }
      };
      const settledFragment = id => page.waitForFunction(fragment =>
        document.activeElement?.id === fragment, id);
      const capture = async (name, locator) => {
        await locator.evaluate(el => window.scrollBy(0, el.getBoundingClientRect().top - 64));
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
        const file = `${state.name}-${name}.png`;
        await page.screenshot({ path: path.join(artifacts, file) });
        result.captures.push({ file, sha256: hash(fs.readFileSync(path.join(artifacts, file))), state });
      };
      await page.goto(`${origin}/docs/INSTALL#1-kubernetes-helm`, { waitUntil: 'domcontentloaded' });
      await settledFragment('1-kubernetes-helm');
      await capture('install-entry', page.locator('[id="1-kubernetes-helm"]'));
      await activate(page.locator('a[href="/docs/KUBERNETES#option-a-using-helm-recommended"]'));
      await page.waitForURL('**/docs/KUBERNETES#option-a-using-helm-recommended');
      await settledFragment('option-a-using-helm-recommended');
      const article = await page.locator('article').innerText();
      for (const phrase of ['without a leading v', 'single Pulse server', 'brief monitoring interruption',
        'is not high availability', 'same installed chart version', 'not a version-upgrade recipe',
        'partial file can remain', 'name the namespace explicitly'])
        assert.ok(article.replace(/\s+/g, ' ').includes(phrase), phrase);
      const actual = (await page.locator('article pre code').allTextContents()).map(text => text.trim());
      for (const recipe of recipes) assert.ok(actual.includes(recipe), 'exact rendered recipe');
      const installation = page.locator('article pre').filter({ hasText: 'helm install pulse pulse/pulse' }).first();
      const installText = await installation.innerText();
      for (const flag of ['--version X.Y.Z', 'replicaCount=1', 'strategy.type=Recreate', 'persistence.enabled=true'])
        assert.ok(installText.includes(flag), flag);
      assert.ok(!installText.includes('upgrade --install'));
      await capture('new-install', installation);
      // Long code remains scrollable inside its own block on a narrow screen.
      const codeGeometry = await installation.evaluate(pre => {
        pre.scrollLeft = pre.scrollWidth;
        return { width: pre.clientWidth, content: pre.scrollWidth, offset: pre.scrollLeft };
      });
      if (codeGeometry.content > codeGeometry.width) assert.ok(codeGeometry.offset > 0);
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
      await capture('single-writer', page.locator('article p').filter({ hasText: 'Keep a single Pulse server' }));
      // Use the renderer's actual GitHub-compatible ID: removing the slash
      // from "Talos / GitOps" preserves the two surrounding spaces/hyphens.
      const staticId = await page.getByRole('heading', {
        name: 'Option B: Generating Static Manifests (For Talos / GitOps)', exact: true,
      }).getAttribute('id');
      assert.ok(staticId);
      await page.goto(`${origin}/docs/KUBERNETES#${staticId}`,
        { waitUntil: 'domcontentloaded' });
      await settledFragment(staticId);
      await capture('static-namespace', page.locator('article p').filter({ hasText: 'Do not apply after a render error' }));
      await page.goto(`${origin}/docs/KUBERNETES#openshift-profile-helm`, { waitUntil: 'domcontentloaded' });
      await settledFragment('openshift-profile-helm');
      const update = page.locator('article pre').filter({ hasText: 'helm upgrade pulse pulse/pulse' });
      assert.ok((await update.innerText()).includes('--reuse-values'));
      assert.ok(!(await update.innerText()).includes('--install'));
      await capture('collector-update', page.locator('article p').filter({ hasText: 'same installed chart version' }));
      await activate(page.locator('a[href="/docs/DEPLOYMENT_MODELS#kubernetes-helm"]').first());
      await page.waitForURL('**/docs/DEPLOYMENT_MODELS#kubernetes-helm');
      await settledFragment('kubernetes-helm');
      const updateText = await page.locator('article').innerText();
      assert.ok(updateText.includes('failure is not proof that nothing changed'));
      assert.ok(updateText.includes('does not guarantee restoration of Pulse data'));
      await capture('existing-rollout', page.locator('#kubernetes-helm'));
      await page.goBack({ waitUntil: 'domcontentloaded' });
      await settledFragment('openshift-profile-helm');
      await page.reload({ waitUntil: 'domcontentloaded' });
      await settledFragment('openshift-profile-helm');
      for (const name of ['KUBERNETES', 'INSTALL', 'DEPLOYMENT_MODELS']) {
        const response = await page.request.get(`${origin}/docs/${name}.md`);
        assert.equal(response.status(), 200);
        assert.equal(hash(await response.body()), binding.sources[`public/docs/${name}.md`]);
      }
      result.cases.push({ ...state, browser: browser.version(), exactRenderedRecipes: recipes.length,
        installAndUpdateLinks: true, interaction: state.width < 500 ? 'touch' : 'keyboard',
        codeGeometry, fragmentFocusAndReload: true, noDocumentOverflow: true, exactFetchedGuides: true });
      await context.close();
      await browser.close();
      browser = undefined;
    }
    assert.deepEqual(result.pageErrors, []);
    assert.ok(result.requests.every(request => request.method === 'GET'
      && new URL(request.url).origin === origin && !new URL(request.url).pathname.startsWith('/api/')));
    result.result = 'passed';
  } catch (error) {
    result.result = 'failed'; result.error = String(error.stack || error); process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    if (server) await new Promise(resolve => server.close(resolve));
    fs.writeFileSync(path.join(artifacts, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify(result));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
