// Real Docs fetch/renderer/router and shipped Markdown/CSS. No backup or API call.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');

const root = '/workspace';
const preview = path.join(root, 'tmp/doc-command-help/static');
const output = path.join(root, 'tmp/backup-activity-help');
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const bindingBytes = fs.readFileSync(path.join(preview, 'binding.json'));
const binding = JSON.parse(bindingBytes);
const fragment = 'backup-activity-has-no-volume-or-fewer-rows-than-expected';
const result = {
  result: 'incomplete',
  source_sha: binding.source_sha,
  binding_sha256: hash(bindingBytes),
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require('playwright/package.json').version,
  cases: [],
  captures: [],
  errors: [],
  unexpected: [],
  served: [],
  cleanup: {},
};

(async () => {
  let server, browser;
  fs.mkdirSync(output, { recursive: true });
  try {
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json'))).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(hash(fs.readFileSync(path.join(root, 'frontend-modern', file))), digest, file);
    assert.equal(binding.sources['browser-tests/backup-activity-help.cjs'], result.protocol_sha256);
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(hash(bytes), digest, file);
      assets.set(file, bytes);
    }
    for (const slug of ['PBS', 'TROUBLESHOOTING', 'VM_DISK_MONITORING'])
      assert.deepEqual(
        assets.get(`docs/${slug}.md`),
        fs.readFileSync(path.join(root, 'docs', `${slug}.md`)),
      );
    server = http.createServer((req, res) => {
      const route = new URL(req.url, 'http://fixture.invalid').pathname;
      let file = route.slice(1);
      if (route.startsWith('/docs/') && !route.endsWith('.md'))
        file = 'browser-tests/doc-command-help.html';
      if (req.method !== 'GET' || !assets.has(file) || route.startsWith('/api/')) {
        result.unexpected.push({ method: req.method, route });
        res.writeHead(404);
        res.end();
        return;
      }
      const bytes = assets.get(file);
      result.served.push({ route, file, sha256: hash(bytes) });
      res.writeHead(200, {
        'Content-Type':
          {
            '.md': 'text/markdown',
            '.js': 'text/javascript',
            '.css': 'text/css',
            '.html': 'text/html',
          }[path.extname(file)] + '; charset=utf-8',
        'Cache-Control': 'no-store',
      });
      res.end(bytes);
    });
    await new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [
      ['desktop', chromium, 1280, false],
      ['narrow', webkit, 320, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height: 1000 },
        isMobile: width < 500,
        hasTouch: width < 500,
        locale: 'en-GB',
        colorScheme: dark ? 'dark' : 'light',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      page.on('pageerror', (error) => result.errors.push(String(error)));
      await page.addInitScript((isDark) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', isDark),
        );
      }, dark);
      const activate = async (link) => {
        await link.scrollIntoViewIfNeeded();
        if (width < 500) await link.tap();
        else {
          await link.focus();
          await page.keyboard.press('Enter');
        }
      };
      const capture = async (label, locator) => {
        await locator.evaluate((element) =>
          window.scrollBy(0, element.getBoundingClientRect().top - 50),
        );
        await page.evaluate(
          () =>
            new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
        );
        const geometry = await page.evaluate(() => ({
          viewport: innerWidth,
          page: document.documentElement.scrollWidth,
        }));
        assert.ok(geometry.page <= geometry.viewport + 1, `${name}: horizontal overflow`);
        const file = `${name}-${label}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          geometry,
        });
      };
      await page.goto(`${origin}/docs/TROUBLESHOOTING#backups-and-storage`);
      const entry = page.getByRole('link', {
        name: 'Count, range and size-evidence checks',
        exact: true,
      });
      await entry.waitFor();
      await capture('entry', entry.locator('..'));
      await activate(entry);
      await page.waitForURL(`**/docs/PBS#${fragment}`);
      await page.waitForFunction((id) => document.activeElement?.id === id, fragment);
      const heading = page.locator(`#${fragment}`);
      await capture('activity', heading);
      const words = await heading.evaluate((element) => {
        let text = '';
        for (
          let next = element.nextElementSibling;
          next && !/^H[1-3]$/.test(next.tagName);
          next = next.nextElementSibling
        )
          text += ' ' + next.textContent;
        return text.replace(/\s+/g, ' ');
      });
      for (const phrase of [
        'it is not a zero-byte backup',
        'Treat the total as partial',
        'not just the rows matching',
        'Clear date filter',
        "browser's local timezone",
        'not datastore space used, free space or bytes transferred',
        'not proof of protection, verification, restore readiness or guest thaw',
        'Do not run a backup, verification, restore, live diagnostics or guest-agent probe',
      ])
        assert.ok(words.includes(phrase), `${name}: ${phrase}`);
      await capture(
        'scope',
        page
          .locator('article p')
          .filter({ hasText: 'The activity chart covers the collected inventory' }),
      );
      const coverage = page.getByRole('link', { name: 'Coverage checks', exact: true });
      await activate(coverage);
      await page.waitForURL('**#backups-are-visible-but-coverage-says-unprotected');
      await page.waitForFunction(
        () => document.activeElement?.id === 'backups-are-visible-but-coverage-says-unprotected',
      );
      await page.goBack();
      await page.waitForURL(`**#${fragment}`);
      const safety = heading
        .locator('xpath=following-sibling::p')
        .filter({ hasText: 'Activity is not proof of protection' })
        .first();
      await capture('safety', safety);
      const safetyLink = safety.getByRole('link', { name: 'Backup safety', exact: true });
      await activate(safetyLink);
      await page.waitForURL('**/docs/VM_DISK_MONITORING#backup-safety');
      await page.waitForFunction(() => document.activeElement?.id === 'backup-safety');
      result.cases.push({
        name,
        width,
        height: 1000,
        browser: browser.version(),
        focusedCrossDocumentEntry: true,
        sameDocumentCoverageLink: true,
        historyReturn: true,
        focusedCrossDocumentSafetyLink: true,
        noPageOverflow: true,
      });
      await context.close();
      await browser.close();
      browser = undefined;
      result.cleanup[name] = true;
    }
    assert.deepEqual(result.errors, []);
    assert.deepEqual(result.unexpected, []);
    result.result = 'passed';
  } finally {
    if (browser) await browser.close();
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.browser = true;
    result.cleanup.server = true;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
  }
  console.log(JSON.stringify(result));
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
