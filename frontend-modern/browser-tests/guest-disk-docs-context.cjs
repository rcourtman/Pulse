// Exercise the shipped guidance through the production row, drawer and Docs
// renderer. Synthetic observations only; no guest request or setup action.
const fs = require('node:fs');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { chromium, webkit } = require('playwright');
const {
  createPublicationFixtureServer,
} = require('/workspace/frontend-modern/browser-tests/publication-fixture-server.cjs');
const root = '/workspace/frontend-modern';
const output = '/workspace/tmp/guest-disk-docs-context';
const origin = 'http://127.0.0.1:5331';
const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex');

// Browser-computed colours and geometry, not class-string assertions. Solid
// focus outlines have deterministic colours; UA `auto` outlines do not.
const linkPresentation = (link) =>
  link.evaluate((element) => {
    const rgba = (colour) => {
      const channels = colour.match(/[\d.]+/g).map(Number);
      return [...channels.slice(0, 3), channels[3] ?? 1];
    };
    const luminance = (colour) => {
      const rgb = colour.slice(0, 3).map((channel) => {
        const value = channel / 255;
        return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
      });
      return rgb[0] * 0.2126 + rgb[1] * 0.7152 + rgb[2] * 0.0722;
    };
    const ratio = (a, b) => {
      const values = [luminance(a), luminance(b)].sort((x, y) => y - x);
      return (values[0] + 0.05) / (values[1] + 0.05);
    };
    const backgrounds = [];
    for (let node = element; node; node = node.parentElement) {
      const background = rgba(getComputedStyle(node).backgroundColor);
      backgrounds.push(background);
      if (background[3] === 1) break;
    }
    const background = backgrounds
      .reverse()
      .reduce(
        (base, layer) =>
          layer.slice(0, 3).map((value, index) => value * layer[3] + base[index] * (1 - layer[3])),
        [255, 255, 255],
      );
    const style = getComputedStyle(element),
      bounds = element.getBoundingClientRect();
    return {
      colour: style.color,
      background,
      text_contrast: ratio(rgba(style.color), background),
      width: bounds.width,
      height: bounds.height,
      focused: element === document.activeElement,
      focus_visible: element.matches(':focus-visible'),
      outline_style: style.outlineStyle,
      outline_width: style.outlineWidth,
      outline_offset: style.outlineOffset,
      outline_colour: style.outlineColor,
      outline_contrast: ratio(rgba(style.outlineColor), background),
      underlined: style.textDecorationLine.includes('underline'),
    };
  });

(async () => {
  fs.mkdirSync(output, { recursive: true });
  const result = {
    result: 'incomplete',
    playwright: require('playwright/package.json').version,
    driver_sha256: hash(fs.readFileSync(__filename)),
    cases: [],
    captures: [],
    cleanup: { browser: false, server: false },
    limits:
      'Current-source presentation and guidance with safe fixtures; not PR #2893, native recovery, installed use or release acceptance.',
  };
  let server, browser;
  try {
    assert.equal(
      result.playwright,
      JSON.parse(fs.readFileSync('/workspace/tests/integration/package-lock.json')).packages[
        'node_modules/@playwright/test'
      ].version,
    );
    server = createPublicationFixtureServer(root, 5331, 'browser-tests/guest-reading-help.html');
    result.compiled = server.binding;
    const docHash = hash(fs.readFileSync(root + '/public/docs/VM_DISK_MONITORING.md'));
    assert.equal(server.binding.artifacts['docs/VM_DISK_MONITORING.md'], docHash);
    assert.equal(hash(fs.readFileSync('/workspace/docs/VM_DISK_MONITORING.md')), docHash);
    result.document_sha256 = docHash;
    await server.listen();
    for (const [name, engine, width, theme] of [
      ['chromium-desktop-light', chromium, 1365, 'light'],
      ['chromium-desktop-dark', chromium, 1365, 'dark'],
      ['webkit-phone-light', webkit, 390, 'light'],
      ['webkit-phone-dark', webkit, 390, 'dark'],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height: 900 },
        isMobile: width <= 768,
        hasTouch: width <= 768,
        locale: 'en-GB',
        colorScheme: theme,
        reducedMotion: 'reduce',
      });
      await context.addInitScript((theme) => {
        document.addEventListener('DOMContentLoaded', () =>
          document.documentElement.classList.toggle('dark', theme === 'dark'),
        );
      }, theme);
      const page = await context.newPage();
      page.setDefaultTimeout(12000);
      const record = {
        name,
        browser: browser.version(),
        width,
        theme,
        errors: [],
        requests: [],
        docs: [],
      };
      result.cases.push(record);
      page.on('pageerror', (error) => record.errors.push(error.message));
      page.on('response', (response) => {
        if (new URL(response.url()).pathname === '/docs/VM_DISK_MONITORING.md') {
          record.docs.push(
            response.body().then((body) => ({ status: response.status(), sha256: hash(body) })),
          );
        }
      });
      await page.route('**/*', (route) => {
        const request = route.request(),
          url = new URL(request.url());
        record.requests.push({
          method: request.method(),
          path: url.pathname,
          external: url.origin !== origin,
        });
        if (url.origin !== origin) return route.abort();
        if (url.pathname.startsWith('/api/'))
          return route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
              success: true,
              data: [],
              alerts: [],
              anomalies: [],
              config: null,
            }),
          });
        return route.continue();
      });
      await page.goto(origin, { waitUntil: 'domcontentloaded' });
      await page.waitForFunction(() => window.__readingHelp);
      const disclosure = page
        .locator('[data-guest-id="fixture-pve:pve-a:101"] [data-row-action]')
        .first();
      if (width <= 768) await page.getByText('reading-guest', { exact: true }).tap();
      else {
        await disclosure.focus();
        await page.keyboard.press('Enter');
      }
      const drawer = page.getByRole('region', { name: 'Guest details' });
      await drawer.waitFor({ state: 'visible' });
      const details = drawer.getByTestId('guest-technical-details');
      assert.match(await details.innerText(), /does not prove it is absent or stopped/);
      const guidance = details.getByRole('link', { name: 'Filesystem reading guidance' });
      record.unavailable_link = await linkPresentation(guidance);
      assert.ok(record.unavailable_link.text_contrast >= 4.5, 'unavailable-reading link contrast');
      if (width <= 768)
        assert.ok(record.unavailable_link.height >= 44, 'phone target smaller than 44px');
      else assert.ok(record.unavailable_link.height < 44, 'desktop link lost compact density');
      assert.equal(record.unavailable_link.underlined, true);
      await page.evaluate(() =>
        window.__readingHelp.update({ reason: 'vm-locked', retained: true, os: 'Windows 11' }),
      );
      assert.match(await details.innerText(), /Using last known disk stats/);
      assert.match(await details.innerText(), /VM operation lock, such as a backup/);
      // Trust a real keyboard focus indication, then activate the same native
      // destination by keyboard or touch. No setup/diagnostic action is used.
      await guidance.scrollIntoViewIfNeeded();
      await page.keyboard.press('Tab');
      await guidance.focus();
      record.retained_link_focus_before_frame = await linkPresentation(guidance);
      await page.evaluate(async () => {
        await document.fonts.ready;
        await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      });
      record.retained_link_focus = await linkPresentation(guidance);
      assert.equal(record.retained_link_focus.focused, true);
      assert.equal(record.retained_link_focus.focus_visible, true);
      assert.equal(record.retained_link_focus.outline_style, 'solid');
      assert.equal(record.retained_link_focus.outline_width, '2px');
      assert.equal(record.retained_link_focus.outline_offset, '2px');
      assert.ok(record.retained_link_focus.outline_contrast >= 3, 'focus outline contrast');
      assert.ok(record.retained_link_focus.text_contrast >= 4.5, 'retained-reading link contrast');
      const focusFile = name + '-guidance-focus.png';
      await page.screenshot({ path: output + '/' + focusFile });
      result.captures.push({
        file: focusFile,
        sha256: hash(fs.readFileSync(output + '/' + focusFile)),
      });
      if (width > 768) {
        await guidance.hover();
        record.hovered_link = await linkPresentation(guidance);
        assert.ok(record.hovered_link.text_contrast >= 4.5, 'hovered-reading link contrast');
      }
      if (width <= 768) await guidance.tap();
      else {
        await guidance.focus();
        await page.keyboard.press('Enter');
      }
      await page
        .locator('#a-missing-reading-is-not-an-installation-diagnosis')
        .waitFor({ state: 'visible' });
      const contextLink = page.getByRole('link', {
        name: "Check the reading's context",
        exact: true,
      });
      if (width <= 768) await contextLink.tap();
      else {
        await contextLink.focus();
        await page.keyboard.press('Enter');
      }
      const heading = page.locator('#current-retained-and-unavailable-disk-readings');
      await heading.waitFor({ state: 'visible' });
      await heading.scrollIntoViewIfNeeded();
      const text = (await page.locator('article').innerText()).replace(/\s+/g, ' ');
      for (const expected of [
        'select the disk reading indicator',
        'phone or with a keyboard',
        'presence or absence of a caption does not establish freshness',
        "same guest's explanation before treating it as an agent setup problem",
        'unknown, not zero',
        'not current free space',
        'not a monitoring-coverage check',
        'does not prove that the QEMU-only path recovered',
        'Do not bypass a guest-read pause or widen token access',
      ])
        assert.ok(text.includes(expected), `Missing rendered guidance: ${expected}`);
      assert.doesNotMatch(text, /wider rows use Last known and Unavailable/);
      record.guide = { url: page.url(), heading: await heading.innerText(), checks: 9 };
      record.layout = await page.evaluate(() => ({
        width: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }));
      assert.ok(
        record.layout.scrollWidth <= record.layout.width + 1,
        JSON.stringify(record.layout),
      );
      const file = name + '-reading-context.png';
      await page.screenshot({ path: output + '/' + file });
      result.captures.push({ file, sha256: hash(fs.readFileSync(output + '/' + file)) });
      record.docs = await Promise.all(record.docs);
      assert.ok(record.docs.length > 0, 'Docs did not consume the changed producer');
      for (const doc of record.docs) assert.deepEqual(doc, { status: 200, sha256: docHash });
      assert.deepEqual(record.errors, []);
      assert.equal(
        record.requests.filter((request) => !['GET', 'HEAD'].includes(request.method)).length,
        0,
      );
      assert.equal(record.requests.filter((request) => request.external).length, 0);
      await context.close();
      await browser.close();
      browser = null;
    }
    result.result = 'passed';
    result.verified_at = new Date().toISOString();
  } catch (error) {
    result.error = String(error.stack || error);
    throw error;
  } finally {
    if (browser) await browser.close();
    result.cleanup.browser = true;
    if (server) await server.close();
    result.cleanup.server = true;
    fs.writeFileSync(output + '/result.json', JSON.stringify(result, null, 2) + '\n');
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
