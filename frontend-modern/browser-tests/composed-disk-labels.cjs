const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');

const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/composed-disk-labels-settled';
const sha256 = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');
const runtime = [
  'frontend-modern/src/components/Workloads/stackedDiskBarModel.ts',
  'frontend-modern/src/components/Workloads/StackedDiskBar.tsx',
  'frontend-modern/src/components/Workloads/useStackedDiskBarState.ts',
  'frontend-modern/src/components/Workloads/GuestRow.tsx',
  'frontend-modern/src/components/Workloads/GuestDrawerOverview.tsx',
  'frontend-modern/src/components/Workloads/DiskList.tsx',
  'frontend-modern/src/utils/workloadGuestPresentation.ts',
  'frontend-modern/src/utils/format.ts',
  'frontend-modern/src/index.css',
];

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const playwrightVersion = require('playwright/package.json').version;
  assert.equal(
    playwrightVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const hashes = Object.fromEntries(
    runtime.map((file) => [file, sha256(fs.readFileSync(path.join('/workspace', file)))]),
  );
  assert.equal(
    hashes[runtime[0]],
    '9f50f630df8379673032052970e7cc6d287c02ecd212b3df5d16f1032be42413',
  );
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5301, strictPort: true },
  });
  const results = [],
    screenshots = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['chromium-desktop', chromium, 1365, false],
      ['webkit-phone', webkit, 390, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height: width === 390 ? 844 : 900 },
        isMobile: width === 390,
        hasTouch: width === 390,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      const page = await context.newPage();
      page.setDefaultTimeout(15000);
      page.setDefaultNavigationTimeout(60000);
      const errors = [],
        requests = [],
        checks = [],
        fits = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('request', (request) =>
        requests.push({
          method: request.method(),
          pathname: new URL(request.url()).pathname,
          origin: new URL(request.url()).origin,
        }),
      );
      await page.route('http://127.0.0.1:5301/api/**', (route) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            success: true,
            data: [],
            alerts: [],
            anomalies: [],
            config: null,
          }),
        }),
      );
      await page.addInitScript((value) => {
        document.addEventListener('DOMContentLoaded', () => {
          if (value) document.documentElement.classList.add('dark');
        });
      }, dark);
      const capture = async (state) => {
        const file = path.join(output, `${name}-${state}.png`);
        await page.screenshot({ path: file, fullPage: true });
        screenshots.push({
          path: path.relative('/workspace', file),
          sha256: sha256(fs.readFileSync(file)),
        });
      };
      const settle = () =>
        page.evaluate(
          () =>
            new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
        );
      await page.goto('http://127.0.0.1:5301/browser-tests/composed-disk-labels.html');
      await page.waitForFunction(() => window.__composedDiskLabels);
      await settle();
      const checkFit = async (expectedWidth, roomy) => {
        await page.waitForFunction(
          (value) =>
            Math.abs(
              document.querySelector('[data-fit="single"]').getBoundingClientRect().width - value,
            ) < 0.5,
          expectedWidth,
        );
        await settle();
        const state = await page.evaluate(() => {
          const boxes = [...document.querySelectorAll('[data-fit]')].map((box) => {
            const id = box.getAttribute('data-fit');
            const labels = [...box.querySelectorAll('span.whitespace-nowrap')];
            return {
              id,
              width: box.getBoundingClientRect().width,
              text: box.innerText,
              labels: labels.map((label) => ({
                text: label.textContent,
                scroll: label.scrollWidth,
                client: label.clientWidth,
              })),
              sublabels: box.querySelectorAll('.metric-sublabel').length,
            };
          });
          return { boxes, presentation: window.__composedDiskLabels.presentation() };
        });
        for (const box of state.boxes) {
          for (const label of box.labels)
            assert(
              label.scroll <= label.client + 1,
              `${name} ${box.id} ${expectedWidth}: rendered label must fit, not be ellipsised`,
            );
        }
        for (const id of ['single', 'anomaly', 'aggregate']) {
          const box = state.boxes.find((item) => item.id === id);
          assert.equal(box.sublabels, roomy ? 1 : 0, `${id}: optional detail matches actual width`);
        }
        assert.equal(state.presentation.single.displayLabel, '60%');
        assert.equal(state.presentation.anomaly.anomalyRatio, '2.5x');
        assert.equal(state.presentation.aggregate.showDiskCount, true);
        assert.equal(
          await page.locator('[data-fit="vertical"] [data-stacked-disk-max-label]').innerText(),
          '67%',
        );
        assert.equal(
          await page.locator('[data-fit="vertical"] [data-stacked-disk-fill="vertical"]').count(),
          2,
        );
        fits.push({ width: expectedWidth, boxes: state.boxes });
      };
      await checkFit(100, false);
      const compactInline = await page.locator('[data-fit="inline"]').innerText();
      assert(!compactInline.includes('MMMM'));
      assert(compactInline.includes('iiii 67%'));
      await capture('compact');
      const roomyButton = page.getByRole('button', { name: 'Use roomy bars', exact: true });
      if (engine === chromium) {
        await roomyButton.focus();
        await page.keyboard.press('Enter');
      } else await roomyButton.tap();
      await checkFit(320, true);
      assert((await page.locator('[data-fit="inline"]').innerText()).includes('MMMM 50%'));
      assert((await page.locator('[data-fit="inline"]').innerText()).includes('iiii 67%'));
      await capture('roomy');
      checks.push(
        'ResizeObserver drops optional detail at 100px and shows complete un-clipped single/anomaly/max/count and inline labels at 320px; vertical max remains truthful',
      );
      if (engine === chromium) {
        await page.locator('[data-fit="single"] [data-stacked-disk-trigger]').hover();
        const tip = page.locator('[data-tooltip-portal="true"]');
        await tip.waitFor({ state: 'visible' });
        assert((await tip.innerText()).includes('Guest reads paused while a backup is running.'));
        assert((await tip.innerText()).includes('/data'));
        await page.locator('h1').hover();
        await page.getByRole('button', { name: 'Mark observation fresh' }).click();
        await page.locator('[data-fit="single"] [data-stacked-disk-trigger]').hover();
        await tip.waitFor({ state: 'visible' });
        assert(!(await tip.innerText()).includes('Guest reads paused'));
        assert((await tip.innerText()).includes('/data'));
        await page.locator('h1').hover();
        checks.push(
          'Caller-owned paused-read tooltip survives measured model; fresh same-disk update removes only status',
        );
      }
      // Exercise the existing production guest callers, but only the combined
      // stale-to-fresh boundary, not an unchanged eight-reason acceptance run.
      await page.goto('http://127.0.0.1:5301/browser-tests/guest-disk-deferral.html');
      await page.waitForFunction(() => window.__guestDiskEvidence);
      const row = page.locator('[data-guest-id="fixture-pve:pve-a:101"]');
      const cell = row.locator('[data-workload-col="disk"]');
      if (engine === chromium) await row.click();
      else
        // WebKit's trusted tap emitted no click on this production row in the
        // separately retained input diagnostic. Check rendering via a mouse
        // click, without asserting touch acceptance or masking that failure.
        await row
          .locator('[data-workload-col="name"]')
          .getByText('backup-guest', { exact: true })
          .click();
      const drawer = page.getByRole('region', { name: 'Guest Overview' });
      await drawer.waitFor({ state: 'visible' });
      const paused = await cell.getAttribute('title');
      assert(paused.startsWith('Using last known disk stats. '));
      assert(paused.includes('VM operation lock, such as a backup.'));
      assert((await drawer.innerText()).includes(paused));
      assert(
        (await page.getByRole('region', { name: 'Guest disk list' }).innerText()).includes(paused),
      );
      assert((await drawer.innerText()).includes('/data'));
      if (engine === chromium) {
        await page.locator('h1').hover();
        await cell.locator('[data-stacked-disk-trigger]').hover();
        await page.locator('[data-tooltip-portal="true"]').waitFor({ state: 'visible' });
        assert((await page.locator('[data-tooltip-portal="true"]').innerText()).includes(paused));
        await page.locator('h1').hover();
      }
      await capture('guest-paused');
      await page.evaluate(() => {
        window.__composedOriginalRow = document.querySelector(
          '[data-guest-id="fixture-pve:pve-a:101"]',
        );
        window.__guestDiskEvidence.update({ reason: '', data: true, usage: 75 });
      });
      await page.waitForFunction(
        () => !document.querySelector('[data-workload-col="disk"]').hasAttribute('title'),
      );
      assert(!(await drawer.innerText()).includes('Using last known disk stats.'));
      assert.equal(await drawer.getByRole('progressbar').getAttribute('aria-valuenow'), '75');
      assert.equal(
        await page.evaluate(
          () =>
            window.__composedOriginalRow ===
            document.querySelector('[data-guest-id="fixture-pve:pve-a:101"]'),
        ),
        true,
      );
      // AnimatedNumber must finish; do not capture an intermediate old percent
      // beside the new capacity and claim that it is a settled observation.
      await page.waitForFunction(() =>
        document.querySelector('[data-workload-col="disk"]').textContent.includes('75%'),
      );
      await capture('guest-fresh');
      checks.push(
        'Production GuestRow, Overview and disk list retain paused provenance; same-VM resumption clears notice and updates to 75% without remount',
      );
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth),
        true,
      );
      assert.deepEqual(errors, []);
      assert(
        requests.every(
          (request) => request.origin === 'http://127.0.0.1:5301' && request.method === 'GET',
        ),
      );
      results.push({
        name,
        browserVersion: browser.version(),
        viewport: { width, height: width === 390 ? 844 : 900 },
        dark,
        checks,
        fits,
        errors,
        requestCount: requests.length,
      });
      await context.close();
      await browser.close();
      browser = null;
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: 'passed',
          verifiedAt: new Date().toISOString(),
          playwrightVersion,
          runtimeHashes: hashes,
          results,
          screenshots,
          limitations:
            'Synthetic component/CSS/browser composition only; not native QGA, thaw, appliance acceptance, installation, release qualification or availability.',
        },
        null,
        2,
      ),
    );
    process.stdout.write(
      `COMPOSED_DISK_LABELS_PASSED ${results.length} browsers / ${screenshots.length} screenshots\n`,
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  process.stderr.write(`${error.stack}\n`);
  process.exitCode = 1;
});
