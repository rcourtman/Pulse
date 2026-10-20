// Run with pulse-worker-browser from the assigned repository root. No key,
// registry, Docker socket, provider, update, install or live Pulse is used.
process.env.RAYON_NUM_THREADS = '2';
process.env.UV_THREADPOOL_SIZE = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit, expect } = require('playwright/test');
const root = '/workspace';
const outputRoot = path.join(root, 'tmp/pro-registry-guidance-final');
const binding = JSON.parse(fs.readFileSync(path.join(outputRoot, 'binding.json')));
const origin = 'http://127.0.0.1:5215';
const digest = 'sha256:' + 'ab'.repeat(32);
const image = 'license.pulserelay.pro/pulse-pro';
const prompt = `docker login license.pulserelay.pro --username 'pulse-pro'`;
// Only the login field below is the owner-returned producer value. The other
// public endpoint fields remain synthetic: this is not a full handler wire or
// proof of the pending pinned-image server reader repair.
const legacy =
  "printf '%s' '<activation-key>' | docker login license.pulserelay.pro -u 'pulse-pro' --password-stdin";
const producer = JSON.parse(fs.readFileSync(path.join(outputRoot, 'producer-command.json')));
assert.equal(producer.command, "docker login 'license.pulserelay.pro' --username 'pulse-pro'");
const marker = 'ppk_live_SYNTHETIC_NOT_A_CREDENTIAL';
const cases = [
  ['legacy', legacy, true],
  ['producer-command', producer.command, true],
  ['secret-rejected', legacy.replace('<activation-key>', marker), false],
  ['missing', undefined, false],
];
const baseDocker = {
  version: 'v6.6.0',
  image,
  imageDigest: digest,
  composePullCommand: `PULSE_IMAGE='${image}@${digest}' docker compose pull`,
  composeUpCommand: `PULSE_IMAGE='${image}@${digest}' docker compose up -d`,
};
module.exports = async (profile) => {
  assert.ok(['desktop', 'phone'].includes(profile), 'select one bounded viewport');
  const output = path.join(outputRoot, profile);
  const report = {
    binding,
    producer,
    profile,
    startedAt: new Date().toISOString(),
    playwright: require('playwright/package.json').version,
    cases: [],
    captures: [],
    cleanup: {},
  };
  let server, browser;
  const save = (phase) => {
    report.phase = phase;
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(report, null, 2) + '\n');
  };
  const sha = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');

  fs.mkdirSync(output, { recursive: true });
  save('preparing-preview');
  try {
    const { createServer } = await import(
      path.join(root, 'frontend-modern/node_modules/vite/dist/node/index.js')
    );
    server = await createServer({
      root: path.join(root, 'frontend-modern'),
      configFile: path.join(root, 'frontend-modern/vite.config.ts'),
      server: { host: '127.0.0.1', port: 5215, strictPort: true },
    });
    await server.listen();
    save('preview-listening');
    for (const [engine, label, viewport] of [
      [chromium, 'desktop', { width: 1365, height: 900 }],
      [webkit, 'phone', { width: 320, height: 844 }],
    ].filter(([, label]) => label === profile)) {
      browser = await engine.launch(
        label === 'desktop'
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      report.browserVersion = browser.version();
      save('browser-launched');
      const context = await browser.newContext({ viewport, hasTouch: label === 'phone' });
      for (const mode of ['switch', 'update'])
        for (const [variant, loginCommand, recognised] of cases) {
          const result = {
            browser: label,
            browserVersion: browser.version(),
            viewport,
            mode,
            variant,
            requests: [],
            unexpected: [],
            errors: [],
          };
          report.cases.push(result);
          save('case-started');
          const docker = { ...baseDocker, loginCommand };
          const page = await context.newPage();
          page.on('pageerror', (error) => result.errors.push(error.message));
          await page.addInitScript(() => {
            window.__copied = [];
            window.__dateCalls = [];
            const nativeDate = Date.prototype.toLocaleDateString;
            Date.prototype.toLocaleDateString = function (...args) {
              const start = performance.now();
              const result = Reflect.apply(nativeDate, this, args);
              window.__dateCalls.push({ elapsedMs: performance.now() - start, result });
              return result;
            };
            Object.defineProperty(navigator, 'clipboard', {
              configurable: true,
              value: {
                writeText: async (text) => {
                  window.__copied.push(text);
                },
              },
            });
          });
          await page.route('**/*', async (route) => {
            const req = route.request();
            const url = new URL(req.url());
            if (url.origin !== origin) {
              result.unexpected.push('off-origin');
              return route.abort();
            }
            if (!url.pathname.startsWith('/api/')) return route.continue();
            result.requests.push(`${req.method()} ${url.pathname}`);
            if (req.method() !== 'GET') {
              result.unexpected.push('write');
              return route.abort();
            }
            if (url.pathname === '/api/license/runtime-capabilities')
              return route.fulfill({
                json: {
                  capabilities: [],
                  limits: [],
                  runtime: {
                    build: mode === 'switch' ? 'community' : 'pro',
                    label: 'Fixture runtime',
                  },
                },
              });
            if (url.pathname === '/api/version')
              return route.fulfill({
                json: {
                  version: '6.5.0',
                  build: 'release',
                  runtime: 'go',
                  isDocker: true,
                  isSourceBuild: false,
                  isDevelopment: false,
                  deploymentType: 'docker',
                },
              });
            if (url.pathname === '/api/updates/history') return route.fulfill({ json: [] });
            if (url.pathname === '/api/updates/pro-runtime' && mode === 'switch')
              return route.fulfill({
                json: {
                  available: true,
                  currentVersion: '6.5.0',
                  version: '6.6.0',
                  installInPlace: false,
                  docker,
                },
              });
            if (url.pathname === '/api/updates/check' && mode === 'update')
              return route.fulfill({
                json: {
                  available: true,
                  currentVersion: '6.5.0',
                  latestVersion: '6.6.0',
                  releaseDate: '2026-09-10T14:06:01Z',
                  releaseNotes: '',
                  downloadUrl: '',
                  isPrerelease: false,
                  isMajorUpgrade: false,
                  dockerUpdate: docker,
                },
              });
            result.unexpected.push(url.pathname);
            return route.fulfill({ status: 405, body: 'Unexpected fixture request' });
          });
          await page.goto(`${origin}/browser-tests/pro-registry-guidance.html?mode=${mode}`, {
            waitUntil: 'domcontentloaded',
          });
          await page.waitForFunction(() => document.documentElement.dataset.ready === 'true');
          if (mode === 'update') {
            await expect(page.getByText(/^Released /)).toBeVisible();
            result.dateCalls = await page.evaluate(() => window.__dateCalls);
            assert.ok(result.dateCalls.length >= 2, 'the real release-date consumer must render');
            assert.ok(result.dateCalls.every((c) => c.result === result.dateCalls[0].result));
          }
          if (label === 'phone')
            await page.evaluate(() => document.documentElement.classList.add('dark'));
          const guide =
            mode === 'switch'
              ? page.getByTestId('pro-runtime-switch')
              : page.locator('div.overflow-hidden.border-green-200').filter({
                  has: page.getByRole('heading', { name: 'Update Available', exact: true }),
                });
          await expect(guide).toContainText(baseDocker.composePullCommand);
          await expect(guide).toContainText(baseDocker.composeUpCommand);
          await expect(guide).not.toContainText('<activation-key>');
          await expect(guide).not.toContainText(marker);
          await expect(guide).not.toContainText('printf');
          await expect(guide).not.toContainText('rcourtman/pulse:');
          if (recognised) {
            await expect(guide).toContainText(prompt);
            await expect(guide).toContainText('Docker’s hidden Password prompt');
            await expect(guide).toContainText('Ctrl+C cancels.');
          } else if (loginCommand) {
            await expect(guide).toContainText('could not be checked safely');
            await expect(guide).toContainText('https://pulserelay.pro/download.html');
            await expect(guide).not.toContainText(prompt);
          } else await expect(guide).not.toContainText('Password prompt');
          const copies = guide.getByRole('button', { name: 'Copy to clipboard' });
          await expect(copies).toHaveCount(recognised ? 3 : 2);
          for (let i = 0; i < (await copies.count()); i++) {
            if (label === 'desktop') {
              await copies.nth(i).focus();
              await page.keyboard.press('Enter');
            } else await copies.nth(i).tap();
          }
          const expected = [baseDocker.composePullCommand, baseDocker.composeUpCommand];
          if (recognised) mode === 'switch' ? expected.unshift(prompt) : expected.push(prompt);
          const copied = await page.evaluate(() => window.__copied);
          assert.deepEqual(copied, expected);
          assert.ok(!copied.join('\n').includes(marker));
          result.copied = copied;
          const geometry = await page.evaluate(() => ({
            width: innerWidth,
            scrollWidth: document.documentElement.scrollWidth,
          }));
          assert.ok(geometry.scrollWidth <= geometry.width + 1, JSON.stringify(geometry));
          result.geometry = geometry;
          const reading = await guide.evaluate((element) => {
            const rect = (node) => {
              const r = node.getBoundingClientRect();
              return { left: r.left, right: r.right, top: r.top, bottom: r.bottom };
            };
            const lines = (node) => {
              const range = document.createRange();
              range.selectNodeContents(node);
              return [...range.getClientRects()].map((r) => ({
                left: r.left,
                right: r.right,
                top: r.top,
                bottom: r.bottom,
              }));
            };
            return {
              paragraphs: [...element.querySelectorAll('p')].map((node) => ({
                box: rect(node),
                lines: lines(node),
              })),
              commands: [...element.querySelectorAll('code')].map((node) => ({
                lines: lines(node),
                button: rect(node.parentElement.querySelector('button')),
              })),
            };
          });
          for (const paragraph of reading.paragraphs)
            for (const line of paragraph.lines) {
              assert.ok(
                line.left >= paragraph.box.left - 1 && line.right <= paragraph.box.right + 1,
                'guidance must not clip inside an overflow-hidden panel',
              );
            }
          for (const command of reading.commands)
            for (const line of command.lines) {
              const overlaps =
                line.left < command.button.right &&
                line.right > command.button.left &&
                line.top < command.button.bottom &&
                line.bottom > command.button.top;
              assert.ok(!overlaps, 'copy button must not cover command text');
            }
          result.readingGeometryChecked = true;
          assert.equal(await page.getByTestId('settings-dirty').innerText(), 'saved');
          if (
            variant === 'legacy' ||
            variant === 'producer-command' ||
            variant === 'secret-rejected'
          ) {
            await guide.scrollIntoViewIfNeeded();
            await page.waitForFunction(() => document.getAnimations().length === 0);
            const file = `${label}-${mode}-${variant}.png`;
            await guide.screenshot({ path: path.join(output, file), animations: 'disabled' });
            report.captures.push({ file, sha256: sha(fs.readFileSync(path.join(output, file))) });
            if (mode === 'update' && variant === 'legacy') {
              const dateFile = `${label}-release-date.png`;
              await page
                .getByText(/^Released /)
                .locator('..')
                .screenshot({ path: path.join(output, dateFile), animations: 'disabled' });
              report.captures.push({
                file: dateFile,
                sha256: sha(fs.readFileSync(path.join(output, dateFile))),
              });
            }
          }
          assert.deepEqual(result.errors, []);
          assert.deepEqual(result.unexpected, []);
          await page.close();
          result.completed = true;
          save('case-complete');
        }
      await context.close();
      report.cleanup.contextClosed = true;
      await browser.close();
      report.cleanup.browserClosed = true;
      browser = undefined;
      save('browser-closed');
    }
    assert.equal(report.cases.length, 8);
    assert.ok(report.cases.every((c) => c.completed));
    report.result = 'passed';
  } catch (error) {
    report.result = 'failed';
    report.error = error.stack;
    throw error;
  } finally {
    if (browser) {
      await browser.close();
      report.cleanup.browserClosedAfterFailure = true;
    }
    if (server) await server.close();
    report.cleanup.serverClosed = true;
    report.finishedAt = new Date().toISOString();
    save('complete');
    console.log(JSON.stringify(report));
  }
};
