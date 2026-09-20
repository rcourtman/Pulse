// Offline real-browser regression for #2130.
//
// Mounts the production Alert Thresholds card list (two Proxmox hosts, 30
// guests, the leading item being a short group header) in a fixed-height
// native scroll container. It records the virtual window's spacer heights
// relative to a real card height, then scrolls a little and checks the window
// did not over-advance. Run with:
//   pulse-worker-browser scripts/check-thresholds-windowed-list.cjs
const path = require('node:path');
const { chromium } = require('playwright');

// pulse-worker-browser copies this file to /request.cjs and runs it with the
// assigned workspace mounted at /workspace, so resolve from the process cwd.
const ROOT = path.resolve(process.cwd(), 'frontend-modern');

const launchOptions = { headless: true, channel: 'chromium', args: ['--no-sandbox'] };

const readWindow = () =>
  (() => {
    const anchor = document.querySelector('[data-platform-window-spacer="top"]');
    if (!anchor) return null;
    const header = anchor.nextElementSibling;
    const firstCard = header ? header.nextElementSibling : null;
    const bottom = document.querySelector('[data-platform-window-spacer="bottom"]');
    return {
      topSpacer: parseFloat(anchor.style.height || '0'),
      bottomSpacer: bottom ? parseFloat(bottom.style.height || '0') : -1,
      headerHeight: header ? header.getBoundingClientRect().height : -1,
      cardHeight: firstCard ? firstCard.getBoundingClientRect().height : -1,
      scrollHeight: document.getElementById('thresholds-scroll').scrollHeight,
      scrollTop: document.getElementById('thresholds-scroll').scrollTop,
    };
  })();

(async () => {
  process.chdir(ROOT);
  const { createServer } = await import(
    path.join(ROOT, 'node_modules', 'vite', 'dist', 'node', 'index.js')
  );
  const server = await createServer({
    root: ROOT,
    configFile: path.join(ROOT, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: 5199, strictPort: true },
  });
  let browser;
  const errors = [];
  const httpErrors = [];
  try {
    await server.listen();
    browser = await chromium.launch(launchOptions);
    const page = await browser.newPage({ viewport: { width: 480, height: 800 } });
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('console', (message) => {
      if (message.type() !== 'error') return;
      // A missing static asset is recorded through the response listener; the
      // generic browser console line carries no URL and is not a render fault.
      if (message.text().includes('Failed to load resource')) return;
      errors.push(`console: ${message.text()}`);
    });
    page.on('response', (response) => {
      if (response.status() >= 400) httpErrors.push(`${response.status()} ${response.url()}`);
    });

    await page.goto('http://127.0.0.1:5199/browser-tests/thresholds-windowed-list.html');
    await page.getByText('gateway01', { exact: true }).first().waitFor({ timeout: 20000 });
    await page.waitForSelector('[data-platform-window-spacer="bottom"]');

    const initial = await page.evaluate(readWindow);
    if (!initial) throw new Error('window spacers not rendered');
    if (!(initial.cardHeight > 80)) {
      throw new Error(`card height not measured (${initial.cardHeight})`);
    }

    // The estimate must track the real card, not the short group header.
    const expectedBottom = 8 * initial.cardHeight;
    if (!(initial.bottomSpacer > expectedBottom * 0.6)) {
      throw new Error(
        `bottom spacer ${initial.bottomSpacer} collapsed below card-scaled ${expectedBottom} ` +
          `(header ${initial.headerHeight}, card ${initial.cardHeight})`,
      );
    }

    // Scrolling 70px must not advance the window past the first group header.
    await page.evaluate(() => {
      document.getElementById('thresholds-scroll').scrollTop = 70;
    });
    await page.waitForTimeout(250);
    const afterScroll = await page.evaluate(readWindow);
    if (!(afterScroll.topSpacer < 1)) {
      throw new Error(
        `window over-advanced after a 70px scroll: top spacer ${afterScroll.topSpacer}px`,
      );
    }

    // The last guest must be reachable at the bottom of the list.
    await page.evaluate(() => {
      const el = document.getElementById('thresholds-scroll');
      el.scrollTop = el.scrollHeight;
    });
    await page.waitForTimeout(250);
    await page.getByText('production-vm-14', { exact: true }).first().waitFor({ timeout: 5000 });

    await page.screenshot({ path: path.join(ROOT, 'browser-tests', 'thresholds-narrow.png') });
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.waitForTimeout(150);
    await page.screenshot({ path: path.join(ROOT, 'browser-tests', 'thresholds-wide.png') });

    if (errors.length > 0) throw new Error(`page errors: ${errors.join(' | ')}`);

    console.log(
      JSON.stringify(
        {
          result: 'passed',
          initial,
          afterScroll,
          expectedBottom,
          scrollHeight: afterScroll.scrollHeight,
          httpErrors,
        },
        null,
        2,
      ),
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error('FAILED:', error && error.stack ? error.stack : error);
  process.exitCode = 1;
});
