import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { runInNewContext } from 'node:vm';
import { captureAuthRenderer } from './capture-auth-renderer.mjs';

function fixture({ name = 'chromium', fail = [], count = 1, css = '.probe{font-weight:800}', fonts, missingHeading = false } = {}) {
  const commands = [];
  let added;
  let detached = false;
  const session = {
    on(event, callback) { assert.equal(event, 'CSS.styleSheetAdded'); added = callback; },
    async send(method, args) {
      commands.push(method);
      if (fail.includes(method)) throw new Error('private-page-url-and-error');
      if (method === 'CSS.enable') {
        for (let i = 0; i < count; i++) added({ header: { styleSheetId: String(i), sourceURL: 'private-page-url' } });
      }
      if (method === 'DOM.getDocument') { assert.equal(args.depth, 0); return { root: { nodeId: 1 } }; }
      if (method === 'DOM.querySelector') { assert.equal(args.selector, 'h1'); return { nodeId: missingHeading ? 0 : 2 }; }
      if (method === 'CSS.getPlatformFontsForNode') return { fonts: fonts ?? [{ familyName: 'Actual font', postScriptName: 'ActualFont', isCustomFont: false, glyphCount: 16 }] };
      if (method === 'CSS.getStyleSheetText') return { text: css };
      return {};
    },
    async detach() { detached = true; if (fail.includes('detach')) throw new Error('private-page-url'); },
  };
  const context = {
    browser: () => ({ browserType: () => ({ name: () => name }), version: () => '141.0.7390.37' }),
    async newCDPSession() { if (fail.includes('session')) throw new Error('private-page-url'); return session; },
  };
  const page = {
    context: () => context,
    async evaluate(fn) {
      if (fail.includes('evaluate')) throw new Error('private-page-url');
      return runInNewContext(`(${fn})()`, {
        document: { querySelector(selector) { assert.equal(selector, 'h1'); return missingHeading ? null : { textContent: 'private-account-label' }; } },
        getComputedStyle: () => ({ fontFamily: 'system-ui', fontSize: '36px', fontWeight: '800', lineHeight: '40px', letterSpacing: 'normal' }),
        innerWidth: 1366, innerHeight: 900, devicePixelRatio: 1,
      });
    },
  };
  return { page, commands, detached: () => detached };
}

test('records actual fonts and complete UTF-8 CSS hashes, not computed family alone', async () => {
  const css = '.probe{font-family:"日本語"}';
  const f = fixture({ css });
  const result = await captureAuthRenderer(f.page);
  assert.deepEqual(result.gaps, []);
  assert.equal(result.heading_style.family, 'system-ui');
  assert.equal(result.fonts[0].family, 'Actual font');
  assert.deepEqual(result.stylesheets, [{ bytes: Buffer.byteLength(css), sha256: createHash('sha256').update(css).digest('hex') }]);
  assert.deepEqual(f.commands, ['DOM.enable', 'CSS.enable', 'DOM.getDocument', 'DOM.querySelector', 'CSS.getPlatformFontsForNode', 'CSS.getStyleSheetText']);
  assert.equal(f.detached(), true);
  assert.doesNotMatch(JSON.stringify(result), /private-|日本語|\.probe/);
});

test('independent font and CSS failures remain explicit and are never retried', async () => {
  const f = fixture({ fail: ['CSS.getPlatformFontsForNode', 'CSS.getStyleSheetText'] });
  const result = await captureAuthRenderer(f.page);
  assert.deepEqual(result.gaps, ['platform-fonts-unavailable', 'stylesheet-unavailable']);
  assert.equal(f.commands.filter((x) => x === 'CSS.getPlatformFontsForNode').length, 1);
  assert.equal(f.commands.filter((x) => x === 'CSS.getStyleSheetText').length, 1);
  assert.equal(f.detached(), true);
  assert.doesNotMatch(JSON.stringify(result), /private-/);
});

test('WebKit retains computed facts without attempting unsupported CDP', async () => {
  const f = fixture({ name: 'webkit' });
  const result = await captureAuthRenderer(f.page);
  assert.deepEqual(result.gaps, ['platform-fonts-and-css-cdp-unavailable']);
  assert.equal(result.viewport.width, 1366);
  assert.deepEqual(f.commands, []);
});

test('creation, computed-style and protocol failures return explicit partial evidence', async () => {
  for (const failure of ['session', 'evaluate', 'DOM.enable']) {
    const f = fixture({ fail: [failure] });
    const result = await captureAuthRenderer(f.page);
    assert.ok(result.gaps.includes(failure === 'evaluate' ? 'computed-style-unavailable' : 'cdp-unavailable'));
    assert.equal(f.detached(), failure !== 'session');
    assert.doesNotMatch(JSON.stringify(result), /private-/);
  }
});

test('caps stylesheet/font counts and oversized CSS without implying completeness', async () => {
  const font = { familyName: 'Actual font', postScriptName: 'ActualFont', isCustomFont: false, glyphCount: 16 };
  const f = fixture({ count: 33, fonts: Array(33).fill(font), css: 'x'.repeat(1024 * 1024 + 1) });
  const result = await captureAuthRenderer(f.page);
  assert.equal(result.fonts.length, 32);
  assert.equal(result.stylesheets.length, 0);
  assert.ok(result.gaps.includes('font-limit'));
  assert.ok(result.gaps.includes('stylesheet-limit'));
  assert.ok(result.gaps.includes('stylesheet-byte-limit'));
  assert.equal(f.commands.filter((x) => x === 'CSS.getStyleSheetText').length, 32);
  assert.equal(f.detached(), true);
});

test('missing headings, empty fonts/stylesheets and detach failures stay visible', async () => {
  assert.ok((await captureAuthRenderer(fixture({ missingHeading: true }).page)).gaps.includes('heading-missing'));
  assert.ok((await captureAuthRenderer(fixture({ fonts: [] }).page)).gaps.includes('platform-fonts-empty'));
  assert.ok((await captureAuthRenderer(fixture({ count: 0 }).page)).gaps.includes('stylesheets-empty'));
  assert.ok((await captureAuthRenderer(fixture({ fail: ['detach'] }).page)).gaps.includes('cdp-detach-unavailable'));
});

test('every existing auth golden attaches facts before the unchanged strict assertions', () => {
  const spec = readFileSync(new URL('../tests/06-theme-visual.spec.ts', import.meta.url), 'utf8');
  assert.equal((spec.match(/await attachRenderer\(page\);/g) ?? []).length, 4);
  for (const name of ['auth-login-light', 'auth-login-dark', 'auth-settings-light', 'auth-settings-dark']) {
    assert.match(spec, new RegExp(`await attachRenderer\\(page\\);\\s+await expect\\(page\\).toHaveScreenshot\\('${name}\\.png'`));
  }
  assert.doesNotMatch(spec, /(?:maxDiffPixels|maxDiffPixelRatio|threshold|updateSnapshots)\s*:/);
  assert.match(spec, /getByText\('Organization', \{ exact: true \}\)/);
  assert.match(spec, /getByText\('Overview', \{ exact: true \}\)/);
});

test('normal E2E admission gates on these controls before installs or shard fan-out', () => {
  const workflow = readFileSync(new URL('../../../.github/workflows/test-e2e.yml', import.meta.url), 'utf8');
  const admission = workflow.split('\n  tier-selection:\n')[1].split('\n  offline-org-provisioning:\n')[0];
  const step = admission.split('- name: Validate read-only auth renderer diagnostics\n')[1].split('\n      - name:')[0];
  assert.match(step, /working-directory: tests\/integration/);
  assert.match(step, /run: node --test scripts\/capture-auth-renderer\.test\.mjs/);
  assert.doesNotMatch(step, /continue-on-error|\|\||\bif:/);
  assert.ok(admission.indexOf('Set up Node.js') < admission.indexOf('Validate read-only auth renderer diagnostics'));
  assert.ok(admission.indexOf('Validate read-only auth renderer diagnostics') < admission.indexOf('Install integration dependencies'));
});
