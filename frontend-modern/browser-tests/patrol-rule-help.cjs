// Run from the Pulse workspace root with pulse-worker-browser.
// This exercises the documented snippet against a synthetic same-origin API,
// not a deployed Pulse installation or the still-missing rule-management UI.
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const { chromium, webkit } = require('playwright');

const guide = fs.readFileSync('docs/AI.md', 'utf8');
const section = guide.split('#### Remove a rule created by mistake\n')[1];
const snippet = section.match(/```javascript\n([\s\S]*?)\n```/)[1];
const output = 'frontend-modern/test-results/patrol-rule-help';
fs.mkdirSync(output, { recursive: true });
const escape = (text) => text.replaceAll('&', '&amp;').replaceAll('<', '&lt;');
const selected = { id: 'rule-one', created_from: 'manual', resource_name: 'Test VM',
  resource_id: 'vm-1', category: 'backup', description: 'Created by mistake' };
const other = { ...selected, id: 'rule-two', resource_id: 'vm-2', resource_name: 'Other VM' };
let scenario;
const server = http.createServer((request, response) => {
  if (request.url === '/') {
    response.setHeader('Content-Type', 'text/html; charset=utf-8');
    response.setHeader('Set-Cookie', [
      'pulse_session=synthetic-session; Path=/; HttpOnly; SameSite=Lax',
      'pulse_org_id=org-a; Path=/; SameSite=Lax',
      ...(scenario.noCSRF ? [] : ['pulse_csrf=synthetic-csrf; Path=/; SameSite=Lax']),
    ]);
    response.end(`<h1>Rule removal help — synthetic browser check</h1>
      <p>Cancel keeps both rules. A confirmed removal targets one exact ID.</p>
      <p id="result">No removal confirmed</p>
      <pre style="white-space:pre-wrap;max-width:1000px">${escape(snippet)}</pre>`);
    return;
  }
  if (!request.url.startsWith('/api/ai/patrol/suppressions')) {
    response.writeHead(404).end();
    return;
  }
  const hasSession = (request.headers.cookie || '').includes('pulse_session=synthetic-session');
  const expectedOrg = request.headers['x-pulse-org-id'] === 'org-a';
  const validCSRF = request.headers['x-csrf-token'] === 'synthetic-csrf';
  scenario.requests.push({ method: request.method, path: request.url,
    hasSession, expectedOrg, validCSRF });
  response.setHeader('Content-Type', 'application/json');
  if (!hasSession || !expectedOrg) {
    response.writeHead(403).end('{}');
    return;
  }
  if (request.method === 'DELETE') {
    assert.equal(request.url, '/api/ai/patrol/suppressions/rule-one');
    if (!validCSRF || scenario.rejectDelete) {
      response.writeHead(403).end('{}');
      return;
    }
    scenario.removed = true;
    response.end('{"success":true}');
    return;
  }
  assert.equal(request.method, 'GET');
  assert.equal(request.url, '/api/ai/patrol/suppressions');
  if (scenario.removed && scenario.readbackFailure) {
    response.writeHead(503).end('{}');
    return;
  }
  response.end(JSON.stringify(scenario.removed ? [other] : [selected, other]));
});

(async () => {
  let browser;
  const receipts = [];
  try {
    await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
    const url = `http://127.0.0.1:${server.address().port}/`;
    for (const [engineName, engine] of [['chromium', chromium], ['webkit', webkit]]) {
      browser = await engine.launch(engineName === 'chromium'
        ? { headless: true, channel: 'chromium', args: ['--no-sandbox'] }
        : { headless: true });
      const cases = engineName === 'chromium'
        ? ['prompt-cancel', 'confirm-cancel', 'success', 'missing-csrf', 'changed-org',
          'delete-rejected', 'readback-failed']
        : ['prompt-cancel', 'success'];
      for (const name of cases) {
        scenario = { name, requests: [], removed: false,
          noCSRF: name === 'missing-csrf', rejectDelete: name === 'delete-rejected',
          readbackFailure: name === 'readback-failed' };
        const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
        const page = await context.newPage();
        const messages = [];
        const dialogs = [];
        page.on('console', (message) => {
          if (['info', 'error'].includes(message.type())) {
            messages.push({ kind: message.type(), message: message.text() });
          }
        });
        page.on('dialog', async (dialog) => {
          dialogs.push({ type: dialog.type(), message: dialog.message() });
          if (dialog.type() === 'prompt') {
            if (name === 'changed-org') {
              await context.addCookies([{ name: 'pulse_org_id', value: 'org-b', url }]);
            }
            if (name === 'prompt-cancel') await dialog.dismiss();
            else await dialog.accept('rule-one');
          } else {
            if (name === 'confirm-cancel') await dialog.dismiss();
            else await dialog.accept();
          }
        });
        await page.goto(url);
        await page.evaluate((code) => window.eval(code), snippet);
        const deletes = scenario.requests.filter((request) => request.method === 'DELETE');
        const success = messages.some((message) => message.kind === 'info');
        if (name === 'success') {
          assert.equal(deletes.length, 1);
          assert.ok(deletes[0].validCSRF);
          assert.equal(scenario.requests.length, 3);
          assert.ok(scenario.removed);
          assert.ok(success);
        } else {
          assert.equal(success, false);
          if (['delete-rejected', 'readback-failed'].includes(name)) {
            assert.equal(deletes.length, 1);
          } else {
            assert.equal(deletes.length, 0);
          }
        }
        assert.ok(scenario.requests.every((request) => request.hasSession && request.expectedOrg));
        assert.ok(messages.every((message) => !message.message.includes('synthetic-csrf')));
        assert.ok(dialogs.every((dialog) => !dialog.message.includes('synthetic-csrf')));
        await page.locator('#result').evaluate((element, text) => { element.textContent = text; },
          success ? 'Selected rule absent after readback; other rule retained' : `${name}: no removal confirmed`);
        let screenshot;
        if (['success', 'confirm-cancel', 'delete-rejected'].includes(name)) {
          screenshot = path.join(output, `${engineName}-${name}.png`);
          await page.screenshot({ path: screenshot, fullPage: true });
        }
        receipts.push({ engine: engineName, browserVersion: browser.version(), name,
          requests: scenario.requests, messages, dialogs, removed: scenario.removed, screenshot });
        await context.close();
      }
      await browser.close();
      browser = null;
    }
    const result = { guideSHA256: crypto.createHash('sha256').update(guide).digest('hex'),
      playwrightVersion: require('playwright/package.json').version,
      evidenceKind: 'synthetic same-origin browser/API workaround; no installed Pulse or rule-management UI',
      scenarios: receipts };
    fs.writeFileSync(path.join(output, 'result.json'), JSON.stringify(result, null, 2) + '\n');
    console.log(JSON.stringify({ passed: receipts.length, guideSHA256: result.guideSHA256,
      playwrightVersion: result.playwrightVersion, result: path.join(output, 'result.json') }));
  } finally {
    if (browser) await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => { console.error(error); process.exitCode = 1; });
