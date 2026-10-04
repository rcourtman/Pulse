process.env.RAYON_NUM_THREADS = '2';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { chromium, webkit } = require('playwright');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/pbs-capacity/browser';
const origin = 'http://127.0.0.1:5321';
const sha256 = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const runtime = [
  'src/features/proxmox/ProxmoxBackupServersTable.tsx',
  'src/features/proxmox/proxmoxBackupsTablePresentation.ts',
  'src/types/resource.ts',
];

(async () => {
  fs.mkdirSync(output, { recursive: true });
  process.chdir(root);
  const clientVersion = require('playwright/package.json').version;
  assert.equal(
    clientVersion,
    JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
      'node_modules/@playwright/test'
    ].version,
  );
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    cacheDir: path.join(output, 'vite-cache'),
    optimizeDeps: { entries: ['browser-tests/pbs-capacity.html'] },
    server: { host: '127.0.0.1', port: 5321, strictPort: true },
  });
  const results = [];
  let browser;
  try {
    await server.listen();
    for (const [name, engine, width] of [
      ['chromium-desktop', chromium, 1365],
      ['webkit-phone', webkit, 390],
      ['webkit-narrow', webkit, 320],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      for (const transport of ['direct', 'canonical']) {
        const phone = engine === webkit;
        const page = await browser.newPage({
          viewport: { width, height: phone ? 844 : 900 },
          isMobile: phone,
          hasTouch: phone,
          colorScheme: phone ? 'dark' : 'light',
        });
        if (phone) await page.addInitScript(() => document.documentElement?.classList.add('dark'));
        page.setDefaultTimeout(20000);
        await page.clock.setFixedTime(new Date('2026-10-04T12:00:00Z'));
        const errors = [],
          offOrigin = [],
          requests = [],
          checks = [];
        page.on('pageerror', (error) => errors.push(error.message));
        await page.route('**/*', async (route) => {
          const req = route.request(),
            url = new URL(req.url());
          if (url.origin !== origin) {
            offOrigin.push(url.origin);
            return route.abort();
          }
          if (!url.pathname.startsWith('/api/')) return route.continue();
          requests.push({ path: url.pathname, method: req.method() });
          const data =
            url.pathname === '/api/backups/pve'
              ? { backupTasks: [], storageBackups: [], guestSnapshots: [] }
              : url.pathname === '/api/backups/pbs'
                ? {
                    backups: [
                      {
                        id: 'pbs-point',
                        vmid: '100',
                        backupType: 'vm',
                        backupTime: '2026-10-04T11:00:00Z',
                        files: ['index.json.blob'],
                        instance: 'pbs-main',
                        datastore: 'main',
                        protected: false,
                        verified: true,
                      },
                    ],
                  }
                : { postures: [] };
          return route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ data }),
          });
        });
        // Verify the actual table and its font metrics, not the dev server's
        // generic load event. Keep the same bounded navigation/locator deadline.
        await page.goto(`${origin}/browser-tests/pbs-capacity.html?transport=${transport}`, {
          waitUntil: 'domcontentloaded',
        });
        // Imported production theme stores can initialise after addInitScript.
        // Apply the requested CSS context after the fixture is loaded too.
        if (phone) await page.evaluate(() => document.documentElement.classList.add('dark'));
        const table = page.locator('[data-proxmox-backups-table="servers"]');
        await table.waitFor();
        await page.waitForFunction(() => document.fonts.status === 'loaded');
        const head = await table.locator('th').allTextContents();
        const usedIndex = head.indexOf('Used');
        assert(usedIndex >= 0);
        const cell = table.locator('tbody tr').first().locator('td').nth(usedIndex);
        await cell.locator('text=40.0%').waitFor();
        await cell.evaluate((node) => {
          node.dataset.identityWitness = 'original';
        });
        const activate = async (state) => {
          const control = page.getByRole('button', { name: state, exact: true });
          if (phone) await control.tap();
          else {
            await control.focus();
            await control.press('Enter');
          }
        };
        const checkNotice = async (state, label) => {
          await activate(state);
          await cell.getByText(label, { exact: true }).waitFor();
          const evidence = await cell.evaluate((node) => ({
            text: node.textContent,
            witness: node.dataset.identityWitness,
            success: !!node.querySelector('.bg-emerald-500'),
            title: node.querySelector('[title]')?.title,
          }));
          assert.equal(evidence.text, label);
          assert.equal(evidence.witness, 'original');
          assert.equal(evidence.success, false);
          assert(evidence.title.includes('does not mean the datastore is empty'));
          assert(!(await table.textContent()).includes('2.0×'));
          assert(
            !(await page.locator('body').innerHTML()).includes('PRIVATE_PROVIDER_ERROR_SENTINEL'),
          );
          checks.push({ state, evidence });
        };
        await checkNotice('Failure', 'Unavailable');
        await page.screenshot({
          path: path.join(output, `${name}-${transport}-unavailable.png`),
          fullPage: true,
        });
        await checkNotice('Unknown', 'Unknown');
        await checkNotice('Offline', 'Unavailable');
        await checkNotice('Error', 'Unavailable');
        await activate('Zero');
        await cell.getByText('0.0%', { exact: true }).waitFor();
        await activate('Full');
        await cell.getByText('95.0%', { exact: true }).waitFor();
        assert((await cell.locator('.text-red-600').count()) > 0);
        checks.push({
          state: 'valid zero / full',
          mounted: await cell.getAttribute('data-identity-witness'),
        });
        await activate('No stores');
        await cell.getByText('No datastore data').waitFor();
        await activate('Healthy');
        await cell.getByText('40.0%', { exact: true }).waitFor();
        await activate('Failure');
        await cell.getByText('Unavailable', { exact: true }).waitFor();
        const geometry = await cell.evaluate((node) => {
          const label = node.querySelector('span'),
            range = document.createRange();
          range.selectNodeContents(label);
          const text = range.getBoundingClientRect(),
            box = node.getBoundingClientRect();
          return {
            text: { x: text.x, right: text.right, width: text.width },
            cell: { x: box.x, right: box.right, width: box.width },
            pageWidth: document.documentElement.clientWidth,
            scrollWidth: document.documentElement.scrollWidth,
          };
        });
        assert(
          geometry.text.x >= geometry.cell.x && geometry.text.right <= geometry.cell.right,
          JSON.stringify({ name, geometry }),
        );
        assert(geometry.scrollWidth <= geometry.pageWidth + 1, JSON.stringify(geometry));
        checks.push({ state: 'warning fits', geometry });
        assert.equal(errors.length, 0, errors.join('\n'));
        assert.equal(offOrigin.length, 0);
        assert(requests.every((req) => req.method === 'GET'));
        results.push({
          name,
          transport,
          width,
          browserVersion: browser.version(),
          checks,
          requests,
          errors,
          offOrigin,
        });
        await page.close();
      }
      await browser.close();
      browser = undefined;
    }
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          result: 'passed',
          clientVersion,
          results,
          content_sha256: Object.fromEntries(
            runtime.map((file) => [`frontend-modern/${file}`, sha256(path.join(root, file))]),
          ),
        },
        null,
        2,
      ),
    );
    console.log(JSON.stringify({ result: 'passed', contexts: results.length, output }));
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failure.json'),
      JSON.stringify({ message: error.message, completedContexts: results }, null, 2),
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error.stack);
  process.exitCode = 1;
});
