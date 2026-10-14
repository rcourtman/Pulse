// Direct production History/query + CSS with synthetic API data. Not installed PBS acceptance.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium, firefox, webkit } = require('playwright');

(async () => {
  const root = process.env.PULSE_FRONTEND_ROOT || path.resolve(__dirname, '..');
  process.chdir(root);
  const engine =
    process.argv.find((arg) => arg.startsWith('--engine='))?.split('=')[1] || 'chromium';
  assert.ok(['chromium', 'firefox', 'webkit'].includes(engine));
  const touch = process.argv.includes('--phone') || process.argv.includes('--touch');
  const width =
    Number(process.argv.find((arg) => arg.startsWith('--width='))?.split('=')[1]) ||
    (process.argv.includes('--phone') ? 390 : 1365);
  const artifacts = path.join(
    root,
    'node_modules',
    `history-inspection-${engine}-${width}${touch ? '-touch' : ''}`,
  );
  fs.mkdirSync(artifacts, { recursive: true });
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: fs.mkdtempSync(path.join(artifacts, 'vite-cache-')),
    server: { host: '127.0.0.1', port: 5223, strictPort: true },
  });
  let browser;
  let diagnosticPage;
  let diagnosticRequests;
  const observations = [];
  try {
    await server.listen();
    browser = await { chromium, firefox, webkit }[engine].launch(
      engine === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : engine === 'firefox'
          ? {
              headless: true,
              // Headless Firefox has no host pointer. Model the input device
              // explicitly: coarse=1, fine=2, hover=4.
              firefoxUserPrefs: {
                'ui.primaryPointerCapabilities': touch ? 1 : 6,
                'ui.allPointerCapabilities': touch ? 1 : 6,
              },
            }
          : { headless: true },
    );
    for (const theme of ['light', 'dark']) {
      const page = await browser.newPage({
        viewport: { width, height: width === 390 ? 844 : 900 },
        locale: 'en-GB',
        timezoneId: 'UTC',
        ...(touch ? { isMobile: engine !== 'firefox', hasTouch: true } : {}),
      });
      page.setDefaultTimeout(20_000);
      // Time flows normally; runFor advances it to fire the 30-second poll.
      await page.clock.install();
      const requests = [],
        errors = [];
      diagnosticPage = page;
      diagnosticRequests = requests;
      let action = 'original',
        held;
      page.on('pageerror', (error) => errors.push(error.message));
      const time = 1_700_000_000_000;
      const point = (offset, value) => ({
        timestamp: time + offset,
        value,
        min: value,
        max: value,
      });
      const metrics = (kind) => {
        if (kind === 'single') return { cpu: [point(0, 11)] };
        if (kind === 'empty') return {};
        if (kind === 'service')
          return {
            cpu: [point(0, 77), point(120_000, 88)],
            memory: [point(0, 33), point(120_000, 44)],
          };
        return {
          cpu:
            kind === 'updated'
              ? [
                  point(-60_000, 8),
                  point(0, 11),
                  point(60_000, 22),
                  point(120_000, 43),
                  point(180_000, 66),
                ]
              : [point(120_000, 43), point(0, 11), point(60_000, 22)],
          memory: [point(30_000, 30), point(120_000, 55)],
          // Zero throughput is an observation, not missing history.
          netin: [point(0, 0), point(120_000, 2048)],
          netout: [point(0, 1024), point(120_000, 4096)],
          diskread: [point(0, 2048), point(120_000, 8192)],
          diskwrite: [point(0, 0), point(120_000, 0)],
        };
      };
      const response = (id, range, kind) => ({
        resourceType: 'agent',
        resourceId: id,
        range,
        start: time,
        end: time + 180_000,
        source: 'store',
        metrics: metrics(kind),
      });
      await page.route('**/*', async (route) => {
        const url = new URL(route.request().url());
        if (url.origin !== 'http://127.0.0.1:5223') return route.abort();
        if (!url.pathname.startsWith('/api/')) return route.continue();
        if (url.pathname === '/api/metrics-store/history') {
          const id = url.searchParams.get('resourceId'),
            range = url.searchParams.get('range');
          assert.ok(['agent-three', 'pbs-three'].includes(id));
          const kind = action;
          action = id === 'pbs-three' ? 'service' : 'original';
          requests.push({ id, range, kind, method: route.request().method() });
          if (kind === 'hold') {
            held = route;
            return;
          }
          if (kind === 'failure')
            return route.fulfill({ status: 503, json: { error: 'Private fixture detail' } });
          return route.fulfill({ json: response(id, range, kind) });
        }
        if (url.pathname === '/api/license/runtime-capabilities')
          return route.fulfill({
            json: {
              capabilities: [],
              limits: [],
              max_history_days: 7,
              hosted_mode: false,
              runtime: { build: 'community', label: 'Pulse Community runtime' },
              blocked_capabilities: [],
            },
          });
        return route.fulfill({ json: { data: [], enabled: false } });
      });
      await page.goto('http://127.0.0.1:5223/browser-tests/pbs-history-refresh.html', {
        waitUntil: 'domcontentloaded',
      });
      await page.evaluate(
        (dark) => document.documentElement.classList.toggle('dark', dark),
        theme === 'dark',
      );
      const fixture = page.getByTestId('history-refresh-fixture');
      const utilization = fixture.locator('[data-history-group="utilization"]');
      const slider = fixture.getByRole('slider', { name: 'Inspect Utilization history' });
      const controls = fixture.getByTestId('guest-history-inspection-control');
      const inspectionControl = utilization.getByTestId('guest-history-inspection-control');
      await slider.waitFor();
      const countBeforeInspection = requests.length;
      const value = () => slider.getAttribute('aria-valuetext');
      const screenshot = async (state) => {
        await utilization.scrollIntoViewIfNeeded();
        const fineBefore = await page.evaluate(
          () => matchMedia('(hover: hover) and (pointer: fine)').matches,
        );
        // Resizing a tall element for a screenshot can change Chromium's
        // emulated pointer capability. Capture the supported viewport as-is.
        await page.screenshot({ path: path.join(artifacts, `${state}-${theme}.png`) });
        assert.equal(
          await page.evaluate(() => matchMedia('(hover: hover) and (pointer: fine)').matches),
          fineBefore,
        );
      };
      const assertValue = async (pattern) => assert.match(await value(), pattern);
      const layout = async () => {
        const dimensions = await page.evaluate(() => ({
          scroll: document.documentElement.scrollWidth,
          inner: innerWidth,
        }));
        assert.ok(dimensions.scroll <= dimensions.inner + 1, JSON.stringify(dimensions));
        const box = await slider.boundingBox();
        assert.ok(box && box.x >= 0 && box.x + box.width <= width + 1);
        assert.ok(box.width > 100, JSON.stringify(box));
        if (touch) assert.ok(box.height >= 44, JSON.stringify(box));
        return dimensions;
      };
      const collapsed = async (control = inspectionControl) => {
        const bounds = await control.boundingBox();
        assert.ok(bounds && bounds.width <= 1 && bounds.height <= 1, JSON.stringify(bounds));
      };
      assert.equal(
        await page.evaluate(() => matchMedia('(hover: hover) and (pointer: fine)').matches),
        !touch,
      );
      if (touch) {
        for (const control of await controls.all()) {
          const bounds = await control.boundingBox();
          assert.ok(bounds && bounds.width > 100 && bounds.height > 44, JSON.stringify(bounds));
        }
        for (const input of await fixture.getByRole('slider').all()) {
          const bounds = await input.boundingBox();
          assert.ok(bounds && bounds.width > 100 && bounds.height >= 44, JSON.stringify(bounds));
        }
      } else {
        for (const control of await controls.all()) await collapsed(control);
      }
      await screenshot('default-input-controls');
      // Tab reaches a native slider straight after the range control (there is
      // no manual refresh while reads succeed).
      await fixture.getByTestId('guest-history-range-control').focus();
      await page.keyboard.press('Tab');
      assert.equal(await slider.evaluate((el) => document.activeElement === el), true);
      const focusStyle = await slider.evaluate((el) => {
        const style = getComputedStyle(el);
        return {
          visible: el.matches(':focus-visible'),
          colour: style.outlineColor,
          width: style.outlineWidth,
          style: style.outlineStyle,
          background: getComputedStyle(el.closest('section')).backgroundColor,
        };
      });
      assert.equal(focusStyle.visible, true);
      assert.equal(focusStyle.colour, 'rgb(59, 130, 246)');
      assert.equal(focusStyle.style, 'solid');
      assert.ok(parseFloat(focusStyle.width) >= 2);
      const luminance = (colour) => {
        const channels = colour
          .match(/\d+/g)
          .slice(0, 3)
          .map((n) => {
            const s = Number(n) / 255;
            return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
          });
        return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
      };
      const a = luminance(focusStyle.colour),
        b = luminance(focusStyle.background);
      focusStyle.contrast = (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
      assert.ok(focusStyle.contrast >= 3, JSON.stringify(focusStyle));
      await assertValue(/CPU 43\.0%\. Memory 55\.0%\. Disk no observation\./);
      await slider.press('Home');
      assert.equal(await slider.inputValue(), '0');
      await assertValue(/CPU 11\.0%\. Memory no observation\. Disk no observation\./);
      await slider.press('ArrowRight');
      assert.equal(await slider.inputValue(), '1');
      await assertValue(/CPU no observation\. Memory 30\.0%\. Disk no observation\./);
      assert.match(await utilization.innerText(), /CPU\s*no reading/);
      assert.doesNotMatch(await utilization.innerText(), /53\.0%/);
      await layout();
      await screenshot('selected-sparse-time');
      await slider.press('End');
      assert.equal(await slider.inputValue(), '3');
      await slider.press('ArrowLeft');
      await assertValue(/CPU 22\.0%\. Memory no observation/);
      assert.equal(
        requests.length,
        countBeforeInspection,
        'inspection must not start another API read',
      );
      const selectedBeforePoll = await value();
      // Exercise an actual same-source background poll without moving focus.
      action = 'updated';
      await page.clock.runFor(30_000);
      await page.waitForFunction(
        () =>
          document.querySelector('input[aria-label="Inspect Utilization history"]')?.max === '5',
      );
      assert.equal(await slider.inputValue(), '3');
      assert.equal(await value(), selectedBeforePoll);
      assert.equal(await slider.evaluate((el) => document.activeElement === el), true);
      const plot = utilization.getByTestId('guest-history-plot');
      const box = await plot.boundingBox();
      await page.mouse.move(box.x + box.width - 4, box.y + box.height / 2);
      await page.mouse.move(1, 1);
      assert.equal(await value(), selectedBeforePoll);
      assert.match(await utilization.innerText(), /CPU\s*22\.0%/);
      // Native blur restores the latest legend; mouse inspection still works.
      await slider.press('Tab');
      assert.match(await utilization.innerText(), /CPU\s*66\.0%/);
      await page.mouse.move(box.x + (34 / 360) * box.width, box.y + box.height / 2);
      assert.match(await utilization.innerText(), /CPU\s*8\.0%/);
      await page.mouse.move(1, 1);

      if (!touch) {
        await collapsed();
        // Shift+Tab reveals the same native control again, without pointer focus.
        await page.keyboard.press('Shift+Tab');
        assert.equal(await slider.evaluate((el) => document.activeElement === el), true);
        await layout();
        await slider.press('Tab');
        await collapsed();
      }

      let hybridTouch = false;
      if (!touch && engine === 'chromium') {
        await fixture.getByTestId('guest-history-range-control').focus();
        const session = await page.context().newCDPSession(page);
        const tap = async (x, y) => {
          await session.send('Input.dispatchTouchEvent', {
            type: 'touchStart',
            touchPoints: [{ x, y }],
          });
          await session.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
        };
        await tap(box.x + box.width / 2, box.y + box.height / 2);
        // A trusted touch event on a fine-pointer device must expose all scrubbers.
        assert.equal(
          await page.evaluate(() => matchMedia('(hover: hover) and (pointer: fine)').matches),
          true,
        );
        for (const input of await fixture.getByRole('slider').all()) {
          const bounds = await input.boundingBox();
          assert.ok(bounds && bounds.width > 100 && bounds.height >= 44, JSON.stringify(bounds));
        }
        const touchBox = await slider.boundingBox();
        await tap(touchBox.x + 3, touchBox.y + touchBox.height / 2);
        await assertValue(/CPU 8\.0%\. Memory no observation/);
        await session.send('Input.dispatchTouchEvent', {
          type: 'touchStart',
          touchPoints: [{ x: touchBox.x + 3, y: touchBox.y + touchBox.height / 2 }],
        });
        await session.send('Input.dispatchTouchEvent', {
          type: 'touchMove',
          touchPoints: [
            { x: touchBox.x + touchBox.width - 3, y: touchBox.y + touchBox.height / 2 },
          ],
        });
        await session.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
        await assertValue(/CPU 66\.0%\. Memory no observation/);
        await screenshot('hybrid-touch-inspection');
        // Touch focus is browser-owned. Exercise a return to mouse without a
        // focused slider, then preserve an explicit keyboard selection too.
        await fixture.getByTestId('guest-history-range-control').focus();
        await page.mouse.move(box.x + (34 / 360) * box.width, box.y + box.height / 2);
        for (const control of await controls.all()) await collapsed(control);
        assert.match(await utilization.innerText(), /CPU\s*8\.0%/);
        await tap(box.x + box.width / 2, box.y + box.height / 2);
        await slider.focus();
        await slider.press('Home');
        const keyboardSelection = await value();
        await page.mouse.move(box.x + box.width - 4, box.y + box.height / 2);
        assert.equal(await slider.evaluate((el) => document.activeElement === el), true);
        assert.equal(await value(), keyboardSelection);
        await layout();
        await fixture.getByTestId('guest-history-range-control').focus();
        for (const control of await controls.all()) await collapsed(control);
        await screenshot('hybrid-mouse-resumed');
        await session.detach();
        hybridTouch = true;
      }

      if (touch) {
        await slider.scrollIntoViewIfNeeded();
        const touchBox = await slider.boundingBox();
        await page.touchscreen.tap(touchBox.x + 3, touchBox.y + touchBox.height / 2);
        await assertValue(/CPU 8\.0%\. Memory no observation/);
        assert.match(await utilization.innerText(), /CPU\s*8\.0%/);
        await page.touchscreen.tap(
          touchBox.x + touchBox.width - 3,
          touchBox.y + touchBox.height / 2,
        );
        await assertValue(/CPU 66\.0%\. Memory no observation/);
        assert.match(await utilization.innerText(), /CPU\s*66\.0%/);
      }
      action = 'failure';
      await page.clock.runFor(30_000);
      await fixture
        .getByRole('status', { name: 'History refresh status' })
        .getByText('History refresh failed. Showing previously loaded history.', { exact: true })
        .waitFor();
      await slider.focus();
      await slider.press('Home');
      await assertValue(/CPU 8\.0%\. Memory no observation/);
      assert.doesNotMatch(await fixture.innerText(), /Private fixture detail/);
      await layout();
      await screenshot('failed-refresh-inspection');
      const semantics = await slider.ariaSnapshot();
      action = 'single';
      await fixture.getByRole('button', { name: 'Retry history' }).click();
      await page.waitForFunction(
        () => document.querySelectorAll('input[type="range"]').length === 0,
      );
      const description = await utilization
        .getByRole('img')
        .evaluate((el) => document.getElementById(el.getAttribute('aria-describedby')).textContent);
      assert.match(description, /1 stored observation time\. .*CPU 11\.0%/);
      assert.equal(await utilization.locator('path[data-history-series]').count(), 0);
      action = 'empty';
      await page.clock.runFor(30_000);
      await page.waitForFunction(() => {
        const image = document.querySelector('[data-history-group="utilization"] svg[role="img"]');
        return (
          document.getElementById(image.getAttribute('aria-describedby')).textContent ===
          'No stored history observations.'
        );
      });
      assert.equal(await fixture.getByRole('slider').count(), 0);
      // Changing resource cannot carry an inspected former-host point into the new target.
      action = 'original';
      await page.clock.runFor(30_000);
      await slider.waitFor();
      await slider.focus();
      await slider.press('Home');
      action = 'hold';
      await page.getByRole('button', { name: 'Switch to service target' }).click();
      await page.waitForFunction(
        () => document.querySelectorAll('input[type="range"]').length === 0,
      );
      assert.equal(await fixture.locator('svg path').count(), 0);
      assert.equal(await fixture.getByTestId('guest-history-hover-time').count(), 0);
      assert.ok(held, 'new target read must be held');
      await held.fulfill({ json: response('pbs-three', '24h', 'service') });
      await slider.waitFor();
      assert.equal(await slider.inputValue(), '1');
      await assertValue(/CPU 88\.0%\. Memory 44\.0%\. Disk no observation/);
      const latestServiceTime = fixture.getByTestId('guest-history-hover-time');
      assert.match(await latestServiceTime.innerText(), /^Latest stored/);
      assert.equal(
        await latestServiceTime.getAttribute('datetime'),
        new Date(time + 120_000).toISOString(),
      );
      const readsBeforeLock = requests.length;
      await fixture.getByTestId('guest-history-range-control').selectOption('14d');
      await fixture.getByText(/14 days history requires a higher license plan/).waitFor();
      assert.equal(await fixture.getByRole('slider').count(), 0);
      assert.equal(requests.length, readsBeforeLock);
      assert.deepEqual(errors, []);
      observations.push({
        engine,
        version: browser.version(),
        width,
        theme,
        requests,
        errors,
        semantics,
        layout: { width },
        singleDescription: description,
        focusStyle,
        nativeKeys: ['Tab', 'Home', 'ArrowRight', 'End', 'ArrowLeft'],
        touch,
        hybridTouch,
      });
      console.log(JSON.stringify({ engine, width, theme, result: 'passed' }));
      await page.close();
    }
    fs.writeFileSync(
      path.join(artifacts, 'result.json'),
      JSON.stringify(
        {
          result: 'passed',
          engine,
          version: browser.version(),
          playwright: require('playwright/package.json').version,
          runtime_sha256: createHash('sha256')
            .update(
              fs.readFileSync(path.join(root, 'src/components/Workloads/GuestDrawerHistory.tsx')),
            )
            .digest('hex'),
          observations,
        },
        null,
        2,
      ),
    );
  } catch (error) {
    if (diagnosticPage && !diagnosticPage.isClosed()) {
      console.error(
        JSON.stringify({
          requests: diagnosticRequests,
          browser: await diagnosticPage.evaluate(() => ({
            width: innerWidth,
            scrollY,
            fineHover: matchMedia('(hover: hover) and (pointer: fine)').matches,
            coarse: matchMedia('(pointer: coarse)').matches,
            inspectionStyles: (() => {
              const matches = [];
              const walk = (rules, parents = []) => {
                for (const rule of rules) {
                  if (rule.selectorText?.includes('not-focus-within')) {
                    matches.push({
                      selector: rule.selectorText,
                      parents,
                      css: rule.style?.cssText,
                    });
                  }
                  if (rule.cssRules)
                    walk(rule.cssRules, [
                      ...parents,
                      rule.conditionText || rule.selectorText || rule.cssText.split('{')[0],
                    ]);
                }
              };
              for (const sheet of document.styleSheets) walk(sheet.cssRules);
              return matches;
            })(),
            active: document.activeElement?.outerHTML,
            controls: [
              ...document.querySelectorAll('[data-testid="guest-history-inspection-control"]'),
            ].map((el) => ({
              class: el.className,
              width: el.getBoundingClientRect().width,
              height: el.getBoundingClientRect().height,
              clip: getComputedStyle(el).clip,
              position: getComputedStyle(el).position,
            })),
            text: document.body.innerText,
          })),
        }),
      );
      await diagnosticPage.screenshot({ path: path.join(artifacts, 'failure.png') });
    }
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
