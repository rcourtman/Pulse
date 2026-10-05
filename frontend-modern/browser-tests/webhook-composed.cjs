// Fresh proof of the composed heading and multiline instructions, not a replay
// of an old browser operation or installed notification-delivery acceptance.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/webhook-composed-proof-stable';
const hash = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const runtimePath = 'frontend-modern/src/components/Alerts/WebhookConfigForm.tsx';
const expectedHash = '0286a39d8405d214df78d8806f1792991a46e0583ab20d1b1fafb276b3c9160d';
const integration = '5e967c49f55d0fc2423e9d7ba63c7ed250069a87';
const sourceFiles = [
  runtimePath,
  'frontend-modern/src/components/Alerts/WebhookConfig.tsx',
  'frontend-modern/src/components/Alerts/useWebhookConfigState.ts',
  'frontend-modern/src/utils/alertWebhookPresentation.ts',
  'frontend-modern/src/index.css',
  'frontend-modern/package-lock.json',
  'tests/integration/package-lock.json',
  'internal/notifications/webhook_templates.go',
  'frontend-modern/browser-tests/webhook-composed.html',
  'frontend-modern/browser-tests/webhook-composed.tsx',
  'frontend-modern/browser-tests/webhook-composed.cjs',
];

// Read the committed server catalogue, not a second copy of its instructions.
// This bounded extractor is only for the selected public fixture fields, not
// a Go evaluator or a substitute for a backend response/receiver test.
const templateSource = fs.readFileSync(
  '/workspace/internal/notifications/webhook_templates.go',
  'utf8',
);
const starts = [...templateSource.matchAll(/^\s*Service:\s*"([^"]+)"/gm)];
const templates = ['generic', 'discord', 'telegram'].map((service) => {
  const i = starts.findIndex((match) => match[1] === service);
  assert.ok(i >= 0, `catalogue service ${service}`);
  const block = templateSource.slice(
    starts[i].index,
    starts[i + 1]?.index ?? templateSource.length,
  );
  const value = (field, optional = false) => {
    const match = block.match(new RegExp(`^\\s*${field}:\\s*("(?:[^"\\\\]|\\\\.)*")`, 'm'));
    assert.ok(optional || match, `catalogue field ${service}.${field}`);
    return match ? JSON.parse(match[1]) : '';
  };
  const payload = block.match(/^\s*PayloadTemplate:\s*`([\s\S]*?)`,/m);
  assert.ok(payload, `catalogue payload ${service}`);
  assert.match(block, /Headers:\s*map\[string\]string\{"Content-Type": "application\/json"\}/);
  return {
    service,
    label: value('Label'),
    name: value('Name'),
    description: value('Description'),
    urlPattern: value('URLPattern'),
    method: value('Method'),
    headers: { 'Content-Type': 'application/json' },
    mentionPlaceholder: value('MentionPlaceholder', true),
    mentionHelp: value('MentionHelp', true),
    payloadTemplate: payload[1],
    instructions: value('Instructions'),
  };
});

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const playwrightVersion = require('playwright/package.json').version;
  const declared = JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json'))
    .packages['node_modules/@playwright/test'].version;
  assert.equal(playwrightVersion, declared);
  assert.equal(hash(path.join('/workspace', runtimePath)), expectedHash);
  const files = Object.fromEntries(
    sourceFiles.map((file) => [file, hash(path.join('/workspace', file))]),
  );
  const { createServer } = await import(path.join(root, 'node_modules/vite/dist/node/index.js'));
  const server = await createServer({
    root,
    configFile: path.join(root, 'vite.config.ts'),
    cacheDir: path.join(output, 'vite-cache'),
    optimizeDeps: { noDiscovery: true, include: [] },
    server: { host: '127.0.0.1', port: 5307, strictPort: true, watch: null },
  });
  const origin = 'http://127.0.0.1:5307';
  const results = [],
    screenshots = [],
    observations = [];
  let browser,
    phase = 'server',
    completed = false;
  try {
    await server.listen();
    for (const [name, engine, width, dark] of [
      ['chromium-desktop-light', chromium, 1440, false],
      ['chromium-desktop-dark', chromium, 1440, true],
      ['webkit-phone-light', webkit, 390, false],
      ['webkit-phone-dark', webkit, 390, true],
    ]) {
      phase = name;
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const page = await browser.newPage({
        viewport: { width, height: width < 500 ? 844 : 900 },
        isMobile: width < 500,
        hasTouch: width < 500,
        locale: 'en-GB',
        timezoneId: 'UTC',
      });
      page.setDefaultTimeout(10000);
      page.setDefaultNavigationTimeout(45000);
      const errors = [],
        unexpectedRequests = [],
        requests = [],
        previewSockets = [],
        checks = [];
      page.on('pageerror', (error) => errors.push(error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
      await page.addInitScript((dark) => {
        const apply = () => document.documentElement.classList.toggle('dark', dark);
        if (document.documentElement) apply();
        else document.addEventListener('DOMContentLoaded', apply, { once: true });
      }, dark);
      // Vite's own HMR client uses this fixed loopback endpoint. Closing it
      // creates a real console error even though the form renders correctly.
      // Permit that preview-only socket, not application/provider sockets;
      // keep the zero-error assertion rather than filtering its diagnostic.
      await page.routeWebSocket(/.*/, (socket) => {
        const url = new URL(socket.url());
        if (url.origin === 'ws://127.0.0.1:5307' && url.pathname === '/') {
          previewSockets.push(url.pathname);
          socket.connectToServer();
        } else {
          unexpectedRequests.push({ origin: url.origin, path: url.pathname, method: 'WebSocket' });
          socket.close();
        }
      });
      await page.route('**/*', async (route) => {
        const request = route.request(),
          url = new URL(request.url());
        if (url.origin !== origin || request.method() !== 'GET') {
          unexpectedRequests.push({ url: request.url(), method: request.method() });
          return route.abort();
        }
        if (!url.pathname.startsWith('/api/')) return route.continue();
        requests.push(url.pathname);
        if (url.pathname === '/api/notifications/webhook-templates')
          return route.fulfill({ json: templates });
        unexpectedRequests.push({ url: request.url(), method: request.method() });
        return route.abort();
      });
      await page.goto(`${origin}/browser-tests/webhook-composed.html`, {
        waitUntil: 'networkidle',
      });
      await page.getByRole('button', { name: '+ Add Webhook', exact: true }).click();
      await page
        .getByRole('button', { name: 'Discord Discord server webhook', exact: true })
        .waitFor();
      for (const service of ['discord', 'telegram']) {
        phase = `${name}: ${service}`;
        const template = templates.find((candidate) => candidate.service === service);
        if (service === 'telegram') {
          const toggle = page.getByRole('button', { name: 'Discord →', exact: true });
          await toggle.focus();
          await page.keyboard.press('Enter');
        }
        const choice = page.getByRole('button', {
          name: `${template.label} ${template.description}`,
          exact: true,
        });
        if (width < 500) await choice.tap();
        else {
          await choice.focus();
          await page.keyboard.press('Enter');
        }
        const heading = page.getByRole('heading', { name: 'Setup Instructions', exact: true });
        await heading.waitFor();
        const panel = heading.locator('..'),
          instructions = panel.locator('p');
        assert.equal(await instructions.textContent(), template.instructions);
        assert.equal(
          await page.getByLabel('Webhook URL', { exact: true }).getAttribute('placeholder'),
          template.urlPattern,
        );
        await heading.evaluate((element) => element.scrollIntoView({ block: 'start' }));
        await page.evaluate(async () => {
          await document.fonts.ready;
          await new Promise((resolve) =>
            requestAnimationFrame(() => requestAnimationFrame(resolve)),
          );
        });
        const layout = await panel.evaluate((element) => {
          const heading = element.querySelector('h4'),
            p = element.querySelector('p');
          const style = getComputedStyle(heading),
            pStyle = getComputedStyle(p);
          const walker = document.createTreeWalker(p, NodeFilter.SHOW_TEXT);
          const node = walker.nextNode();
          if (!node || node.textContent !== p.textContent)
            throw new Error('Instructions must remain one complete text node');
          let offset = 0;
          const linePositions = p.textContent.split('\n').map((line) => {
            const range = document.createRange();
            range.setStart(node, offset);
            range.setEnd(node, offset + 2);
            offset += line.length + 1;
            const bounds = range.getBoundingClientRect();
            const fragments = [...range.getClientRects()].map((rect) => ({
              x: rect.x,
              y: rect.y,
              width: rect.width,
              height: rect.height,
            }));
            // A union bound can include a zero-width newline on the preceding
            // line. Measure the numbered glyphs, not that whitespace fragment.
            const glyphs = fragments.filter((rect) => rect.width > 0 && rect.height > 0);
            if (glyphs.length !== 1)
              throw new Error('Each numbered prefix needs one painted rectangle');
            return {
              number: line.slice(0, 2),
              x: glyphs[0].x,
              y: glyphs[0].y,
              bounds: { x: bounds.x, y: bounds.y, width: bounds.width, height: bounds.height },
              fragments,
            };
          });
          const canvas = document.createElement('canvas');
          canvas.width = canvas.height = 1;
          const context = canvas.getContext('2d');
          context.fillStyle = style.color;
          context.fillRect(0, 0, 1, 1);
          const ancestorOpacity = [];
          for (let current = heading; current; current = current.parentElement)
            ancestorOpacity.push(getComputedStyle(current).opacity);
          return {
            headingClass: heading.className,
            color: style.color,
            alpha: context.getImageData(0, 0, 1, 1).data[3],
            ancestorOpacity,
            whiteSpace: pStyle.whiteSpace,
            overflowWrap: pStyle.overflowWrap,
            linePositions,
            panelOverflow: element.scrollWidth - element.clientWidth,
            documentOverflow: document.documentElement.scrollWidth - innerWidth,
          };
        });
        observations.push({ name, service, layout });
        const file = `${name}-${service}.png`;
        await page.screenshot({ path: path.join(output, file), fullPage: true });
        screenshots.push({ file, sha256: hash(path.join(output, file)), width, dark, service });
        assert.ok(layout.headingClass.split(' ').includes('text-blue-900'));
        assert.ok(layout.headingClass.split(' ').includes('dark:text-blue-100'));
        assert.equal(layout.alpha, 255, 'heading colour is not translucent');
        assert.ok(layout.ancestorOpacity.every((opacity) => opacity === '1'));
        assert.equal(layout.whiteSpace, 'pre-line');
        assert.equal(layout.overflowWrap, 'break-word');
        assert.equal(layout.linePositions.length, service === 'discord' ? 4 : 5);
        layout.linePositions.forEach((line, i, lines) => {
          assert.equal(line.number, `${i + 1}.`);
          if (i)
            assert.ok(line.y > lines[i - 1].y + 5, 'numbered steps need separate rendered lines');
          assert.ok(
            Math.abs(line.x - lines[0].x) <= 1,
            'each numbered step starts at the left edge',
          );
        });
        assert.ok(layout.panelOverflow <= 1);
        assert.ok(layout.documentOverflow <= 1);
        assert.equal(await page.getByLabel('Notification operations').textContent(), '0');
        checks.push({
          service,
          templateInstructionsExact: true,
          layout,
          noNotificationOperation: true,
        });
      }
      await page.getByRole('button', { name: 'Cancel', exact: true }).click();
      await page.getByRole('button', { name: '+ Add Webhook', exact: true }).waitFor();
      assert.equal(
        await page.getByRole('heading', { name: 'Setup Instructions', exact: true }).count(),
        0,
      );
      assert.equal(await page.getByLabel('Notification operations').textContent(), '0');
      assert.deepEqual(errors, []);
      assert.deepEqual(unexpectedRequests, []);
      results.push({
        name,
        browserVersion: browser.version(),
        width,
        dark,
        phoneEmulated: width < 500,
        checks,
        cancelReturnsToAdd: true,
        requests,
        previewSockets,
        errors,
        unexpectedRequests,
      });
      await browser.close();
      browser = undefined;
    }
    completed = true;
  } catch (error) {
    fs.writeFileSync(
      path.join(output, 'failure.json'),
      JSON.stringify(
        { phase, error: error.message, files, results, screenshots, observations },
        null,
        2,
      ),
    );
    throw error;
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
  if (completed) {
    fs.writeFileSync(
      path.join(output, 'result.json'),
      JSON.stringify(
        {
          integration,
          playwrightVersion,
          declaredPlaywrightVersion: declared,
          files,
          results,
          screenshots,
          observations,
          verifiedAt: new Date().toISOString(),
          browserAndServerClosed: true,
          limits:
            'Production WebhookConfig/form/state/CSS in a standalone offline fixture. Source-exact public template fields, synthetic API. No full app navigation, real credentials, receiver, Test/save/delivery, native phone or installed acceptance.',
        },
        null,
        2,
      ),
    );
    console.log(
      `Verified ${results.length} theme/viewport states and ${screenshots.length} source-exact service captures; browser and server closed.`,
    );
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
