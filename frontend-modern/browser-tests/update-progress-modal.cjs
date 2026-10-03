// Offline browser proof: the update progress modal keeps moving when the
// update stream goes quiet, and reloads only on real restart evidence.
//
//   node frontend-modern/browser-tests/update-progress-modal.cjs
//
// Needs Playwright 1.56.1 resolvable (NODE_PATH works) with its Chromium.
// PULSE_BROWSER_ARTIFACTS picks the screenshot directory and
// PULSE_CHROMIUM_PATH an alternative Chromium executable. Every API answer is
// synthetic and off-origin traffic is aborted; no Pulse server is contacted.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');

const PORT = 5208;
const ORIGIN = `http://127.0.0.1:${PORT}`;
const HARNESS = `${ORIGIN}/browser-tests/update-progress-modal.html`;
const OLD = 'v6.4.4';
const NEW = 'v6.4.5';

const status = (stage, progress, message) => ({
  status: stage,
  progress,
  message,
  updatedAt: new Date().toISOString(),
});
const MESSAGES = {
  downloading: 'Downloading update...',
  verifying: 'Verifying download...',
  extracting: 'Extracting update...',
  'backing-up': 'Backing up current installation...',
  applying: 'Applying update...',
  restarting: 'Restarting Pulse...',
  completed: 'Update completed',
  idle: '',
};
const s = (stage, progress) => status(stage, progress, MESSAGES[stage]);

const ABORT = Symbol('abort');
const UNAVAILABLE = Symbol('unavailable');

async function openHarness(browser, viewport, server) {
  const context = await browser.newContext({ viewport });
  const page = await context.newPage();
  const record = {
    pageErrors: [],
    documentLoads: 0,
    statusRequests: [],
    versionRequests: [],
    offOrigin: 0,
  };
  page.on('pageerror', (error) => record.pageErrors.push(error.message));
  await page.route('**/*', (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.origin !== ORIGIN) {
      record.offOrigin += 1;
      return route.abort();
    }
    if (url.pathname === '/browser-tests/update-progress-modal.html') {
      record.documentLoads += 1;
      return route.continue();
    }
    const answer = (value, log) => {
      log.push(value === ABORT ? 'unreachable' : value === UNAVAILABLE ? '503' : value);
      if (value === ABORT) return route.abort('connectionrefused');
      if (value === UNAVAILABLE) return route.fulfill({ status: 503, body: 'unavailable' });
      return route.fulfill({ json: value });
    };
    if (url.pathname === '/api/updates/status') {
      return answer(server.status(), record.statusRequests);
    }
    if (url.pathname === '/api/version') {
      const version = server.version();
      return answer(
        typeof version === 'string' ? { version, build: 'release' } : version,
        record.versionRequests,
      );
    }
    if (url.pathname.startsWith('/api/')) return route.abort();
    return route.continue();
  });
  await page.goto(HARNESS, { waitUntil: 'domcontentloaded', timeout: 120_000 });
  await page.waitForFunction(() => window.__updateStream && window.__updateStream.open());
  await page.getByRole('dialog').waitFor();
  return { context, page, record };
}

const emit = (page, value) => page.evaluate((v) => window.__updateStream.emit(v), value);
const seen = (page) => page.evaluate(() => JSON.parse(JSON.stringify(window.__seen)));
const progressText = (page, value) =>
  page.getByRole('dialog').getByText(`${value}%`, { exact: true });

async function layoutCheck(page) {
  const dimensions = await page.evaluate(() => {
    const panel = document.querySelector('[role="dialog"]');
    const rect = panel ? panel.getBoundingClientRect() : null;
    return {
      scroll: document.documentElement.scrollWidth,
      inner: innerWidth,
      panelLeft: rect ? Math.round(rect.left) : null,
      panelRight: rect ? Math.round(rect.right) : null,
    };
  });
  assert.ok(dimensions.scroll <= dimensions.inner + 1, `horizontal overflow ${JSON.stringify(dimensions)}`);
  if (dimensions.panelLeft !== null) {
    assert.ok(dimensions.panelLeft >= 0 && dimensions.panelRight <= dimensions.inner + 1,
      `dialog outside viewport ${JSON.stringify(dimensions)}`);
  }
  return dimensions;
}

async function shot(page, artifacts, name, width) {
  await page.waitForTimeout(350); // let the dialog entrance settle
  const file = path.join(artifacts, `${name}-${width}.png`);
  await page.screenshot({ path: file });
  return file;
}

const scenarios = {
  // (a) One downloading event, then a held-open silent stream.
  async 'stalled-stream-fallback'(browser, viewport, artifacts) {
    const polls = [s('downloading', 35), s('downloading', 70), s('extracting', 80), s('applying', 90)];
    let pollIndex = 0;
    const server = {
      status: () => polls[Math.min(pollIndex++, polls.length - 1)],
      version: () => OLD,
    };
    const { context, page, record } = await openHarness(browser, viewport, server);
    await emit(page, s('downloading', 10));
    const silentSince = Date.now();
    await progressText(page, 10).waitFor();
    await page.getByText('Downloading update...').waitFor();
    const screenshots = [await shot(page, artifacts, 'a-stream-10pct', viewport.width)];
    await progressText(page, 35).waitFor({ timeout: 15_000 });
    const firstPollAfterMs = Date.now() - silentSince;
    await progressText(page, 90).waitFor({ timeout: 20_000 });
    await page.getByText('Applying update...').waitFor();
    const observed = await seen(page);
    assert.ok(firstPollAfterMs >= 5_000, `polling began before the silence window (${firstPollAfterMs}ms)`);
    assert.deepEqual(observed.progress, [10, 35, 70, 80, 90]);
    assert.equal(observed.restarting, false);
    assert.equal(await page.evaluate(() => window.__updateStream.open()), true, 'stream stays open alongside polling');
    screenshots.push(await shot(page, artifacts, 'a-polled-90pct', viewport.width));
    const dimensions = await layoutCheck(page);
    assert.deepEqual(record.pageErrors, []);
    await context.close();
    return { firstPollAfterMs, progress: observed.progress, statusPolls: record.statusRequests.length, dimensions, screenshots };
  },

  // (b) One failed poll mid-download must not look like a restart.
  async 'transient-poll-failure'(browser, viewport, artifacts) {
    const polls = [s('downloading', 40), ABORT, s('downloading', 60), s('downloading', 80)];
    let pollIndex = 0;
    const server = {
      status: () => polls[Math.min(pollIndex++, polls.length - 1)],
      version: () => OLD,
    };
    const { context, page, record } = await openHarness(browser, viewport, server);
    await emit(page, s('downloading', 20));
    await progressText(page, 20).waitFor();
    await page.evaluate(() => window.__updateStream.fail()); // stream closed, polling only
    await progressText(page, 80).waitFor({ timeout: 20_000 });
    await page.waitForTimeout(3_000); // one more poll cycle at the same stage
    const observed = await seen(page);
    assert.ok(record.statusRequests.includes('unreachable'), 'a poll failed');
    assert.equal(observed.restarting, false, 'restart phase entered on a single failed poll');
    assert.deepEqual(observed.progress, [20, 40, 60, 80]);
    await page.getByText('Please do not close this window or refresh the page during the update.').waitFor();
    assert.equal(record.documentLoads, 1);
    const screenshots = [await shot(page, artifacts, 'b-after-failed-poll-80pct', viewport.width)];
    const dimensions = await layoutCheck(page);
    assert.deepEqual(record.pageErrors, []);
    await context.close();
    return { statusRequests: record.statusRequests.map((r) => (typeof r === 'string' ? r : `${r.status} ${r.progress}`)), progress: observed.progress, dimensions, screenshots };
  },

  // (c) restarting status, then the server goes away, then the new version.
  async 'restart-evidence-reload'(browser, viewport, artifacts) {
    let restartServed = false;
    let versionAfterRestart = 0;
    const polls = [s('applying', 90), s('restarting', 100)];
    let pollIndex = 0;
    const server = {
      status: () => {
        if (restartServed) return ABORT;
        const next = polls[Math.min(pollIndex++, polls.length - 1)];
        if (next.status === 'restarting') restartServed = true;
        return next;
      },
      version: () => {
        if (!restartServed) return OLD;
        versionAfterRestart += 1;
        return versionAfterRestart <= 2 ? ABORT : NEW;
      },
    };
    const { context, page, record } = await openHarness(browser, viewport, server);
    await emit(page, s('downloading', 10));
    await progressText(page, 10).waitFor();
    await page.getByText('Pulse is restarting...').waitFor({ timeout: 20_000 });
    const screenshots = [await shot(page, artifacts, 'c-restarting', viewport.width)];
    const restartDimensions = await layoutCheck(page);
    await page.getByTestId('reloaded').waitFor({ timeout: 30_000 });
    await page.waitForTimeout(4_000); // no second reload
    assert.equal(record.documentLoads, 2, 'reloaded exactly once');
    const afterRestart = record.versionRequests.slice(record.versionRequests.findIndex((v) => v === 'unreachable'));
    assert.deepEqual(afterRestart.slice(0, 3), ['unreachable', 'unreachable', { version: NEW, build: 'release' }]);
    screenshots.push(await shot(page, artifacts, 'c-reloaded', viewport.width));
    assert.deepEqual(record.pageErrors, []);
    await context.close();
    return {
      documentLoads: record.documentLoads,
      versionRequests: record.versionRequests.map((v) => (typeof v === 'string' ? v : v.version)),
      dimensions: restartDimensions,
      screenshots,
    };
  },

  // (d) No pre-update version; requests fail late and a new version answers,
  // but the backend never confirmed completion.
  async 'no-baseline-no-autoreload'(browser, viewport, artifacts) {
    let failuresStarted = false;
    const polls = [s('applying', 95), ABORT, ABORT, ABORT, ABORT, s('idle', 0)];
    let pollIndex = 0;
    const server = {
      status: () => {
        const next = polls[Math.min(pollIndex++, polls.length - 1)];
        if (next === ABORT) failuresStarted = true;
        return next;
      },
      version: () => (failuresStarted ? NEW : UNAVAILABLE),
    };
    const { context, page, record } = await openHarness(browser, viewport, server);
    await emit(page, s('applying', 90));
    await progressText(page, 90).waitFor();
    await page.evaluate(() => window.__updateStream.fail());
    await page.getByText('Pulse is restarting...').waitFor({ timeout: 20_000 });
    // Past the idle answers and several healthy new-version probes.
    const deadline = Date.now() + 25_000;
    while (Date.now() < deadline && !(record.statusRequests.filter((r) => r.status === 'idle').length >= 2
      && record.versionRequests.filter((v) => v.version === NEW).length >= 3)) {
      await page.waitForTimeout(500);
    }
    await page.waitForTimeout(2_000);
    const idleAnswers = record.statusRequests.filter((r) => r.status === 'idle').length;
    const newVersionAnswers = record.versionRequests.filter((v) => v.version === NEW).length;
    assert.ok(idleAnswers >= 2, `idle answers ${idleAnswers}`);
    assert.ok(newVersionAnswers >= 3, `new-version answers ${newVersionAnswers}`);
    assert.ok(record.versionRequests.includes('503'), 'baseline probe was unavailable');
    assert.equal(record.documentLoads, 1, 'auto-reloaded without a baseline or confirmation');
    assert.equal(await page.getByTestId('reloaded').count(), 0);
    const observed = await seen(page);
    assert.equal(observed.completed, false, 'claimed completion without confirmation');
    const screenshots = [await shot(page, artifacts, 'd-unconfirmed-waiting', viewport.width)];
    const dimensions = await layoutCheck(page);
    assert.deepEqual(record.pageErrors, []);
    await context.close();
    return { documentLoads: record.documentLoads, idleAnswers, newVersionAnswers, dimensions, screenshots };
  },

  // (e) The stream delivers every stage through completion.
  async 'stream-happy-path'(browser, viewport, artifacts) {
    let completedEmitted = false;
    let versionAfterComplete = 0;
    const server = {
      status: () => s('applying', 95),
      version: () => {
        if (!completedEmitted) return OLD;
        versionAfterComplete += 1;
        if (versionAfterComplete === 1) return OLD; // old process still answering
        if (versionAfterComplete === 2) return ABORT; // it exits
        return NEW;
      },
    };
    const { context, page, record } = await openHarness(browser, viewport, server);
    const stages = [
      s('downloading', 10), s('downloading', 45), s('downloading', 80), s('verifying', 85),
      s('extracting', 88), s('backing-up', 90), s('applying', 95),
    ];
    const screenshots = [];
    for (const stage of stages) {
      await emit(page, stage);
      await progressText(page, stage.progress).waitFor();
      if (stage.status === 'applying') screenshots.push(await shot(page, artifacts, 'e-stream-applying', viewport.width));
      await page.waitForTimeout(300);
    }
    const midDimensions = await layoutCheck(page);
    completedEmitted = true;
    await emit(page, s('completed', 100));
    await page.getByText('Pulse is restarting...').waitFor({ timeout: 10_000 });
    screenshots.push(await shot(page, artifacts, 'e-restarting', viewport.width));
    await layoutCheck(page);
    await page.getByTestId('reloaded').waitFor({ timeout: 30_000 });
    await page.waitForTimeout(3_000);
    assert.equal(record.documentLoads, 2, 'reloaded exactly once');
    assert.equal(record.statusRequests.length, 0, 'fallback polling ran while the stream was live');
    screenshots.push(await shot(page, artifacts, 'e-reloaded', viewport.width));
    assert.deepEqual(record.pageErrors, []);
    await context.close();
    return {
      documentLoads: record.documentLoads,
      statusPolls: record.statusRequests.length,
      versionRequests: record.versionRequests.map((v) => (typeof v === 'string' ? v : v.version)),
      dimensions: midDimensions,
      screenshots,
    };
  },
};

(async () => {
  const root = path.resolve(__dirname, '..');
  const artifacts = process.env.PULSE_BROWSER_ARTIFACTS || path.join(root, 'browser-tests', 'update-progress-modal-proof');
  fs.mkdirSync(artifacts, { recursive: true });
  process.chdir(root);
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    server: { host: '127.0.0.1', port: PORT, strictPort: true },
    logLevel: 'warn',
  });
  let browser;
  try {
    await server.listen();
    browser = await chromium.launch({
      headless: true,
      args: ['--no-sandbox'],
      // Optional: a locally cached Chromium when Playwright's own build is absent.
      ...(process.env.PULSE_CHROMIUM_PATH ? { executablePath: process.env.PULSE_CHROMIUM_PATH } : {}),
    });
    const results = [];
    for (const viewport of [{ width: 1365, height: 900 }, { width: 390, height: 844 }]) {
      for (const [name, run] of Object.entries(scenarios)) {
        const started = Date.now();
        const detail = await run(browser, viewport, artifacts);
        results.push({ viewport: `${viewport.width}x${viewport.height}`, scenario: name, result: 'passed', ms: Date.now() - started, ...detail });
        console.error(`passed ${name} @ ${viewport.width}x${viewport.height}`);
      }
    }
    console.log(JSON.stringify({ result: 'passed', browser: browser.version(), playwright: require('playwright/package.json').version, results }, null, 2));
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
