/* Offline adversarial observer controls. These fixtures are not Pulse release,
 * systemd, installed update, Tailscale or qualification evidence. */
const assert = require('node:assert/strict');
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright');
const { validate, judge, runJourney } = require('./release-browser-journey.cjs');
const from = 'v6.4.5', to = 'v6.4.6-rc.1';
const downloadUrl = `https://github.com/rcourtman/Pulse/releases/download/${to}/pulse-${to}-linux-amd64.tar.gz`;

function pureControls() {
  const valid = { origin: 'https://ephemeral.tawny-powan.ts.net', mode: 'upgrade', from, to, expected: to };
  validate(valid);
  for (const origin of ['http://ephemeral.tawny-powan.ts.net', 'https://private.example.com',
    'https://ephemeral.other.ts.net', 'https://ephemeral.tawny-powan.ts.net:443', 'https://ephemeral.tawny-powan.ts.net/path']) {
    assert.throws(() => validate({ ...valid, origin }));
  }
  for (const delta of [{ mode: 'apply' }, { to: 'v6.4.6' }, { expected: from }]) assert.throws(() => validate({ ...valid, ...delta }));
  const receipt = { ...valid, login_status: 200, observed_version: to.slice(1), health: 'healthy',
    protected_read_status: 200, apply_requests: 1, apply_status: 200, selected_version: to.slice(1),
    sse_opens: 1, sse_progress: [{ status: 'downloading', progress: 10 }], progress_dialog_visible: false };
  judge(receipt);
  for (const delta of [{ login_status: 401 }, { observed_version: from.slice(1) }, { health: 'unhealthy' },
    { protected_read_status: 403 }, { apply_requests: 0 }, { apply_requests: 2 }, { apply_status: 500 },
    { selected_version: '6.5.0-rc.1' }, { sse_opens: 0 }, { sse_progress: [] }, { progress_dialog_visible: true }]) {
    assert.throws(() => judge({ ...receipt, ...delta }));
  }
}

function fixture(kind, mode) {
  let version = (mode === 'recovery' ? from : from).slice(1), applies = 0, logins = 0;
  let applied = false;
  const sockets = new Set();
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://localhost');
    const json = (code, data) => { res.writeHead(code, { 'content-type': 'application/json' }); res.end(JSON.stringify(data)); };
    if (url.pathname === '/api/login') {
      logins++;
      if (kind === 'login-401') return json(401, { error: 'fixture denial' });
      res.setHeader('Set-Cookie', 'fixture-session=true; Path=/; HttpOnly; SameSite=Lax');
      return json(200, { success: true });
    }
    if (url.pathname === '/api/version') return json(200, { version, isDocker: false, isSourceBuild: false });
    if (url.pathname === '/api/health') return json(200, { status: 'healthy' });
    if (url.pathname === '/api/config/nodes') return json(applied && kind === 'read-401' ? 401 : 200, []);
    if (url.pathname === '/api/updates/check') return json(200, {
      latestVersion: kind === 'wrong-preview' ? '6.5.0-rc.1' : to.slice(1), downloadUrl });
    if (url.pathname === '/api/updates/apply') {
      applies++; applied = true;
      setTimeout(() => { version = to.slice(1); }, 350);
      return json(200, { status: 'started' });
    }
    if (url.pathname === '/api/updates/stream') {
      res.writeHead(200, { 'content-type': 'text/event-stream', 'cache-control': 'no-cache' });
      res.write(': real loopback fixture stream\n\n');
      if (kind !== 'silent-stream') res.write(`data: ${JSON.stringify({ status: 'downloading', progress: 10, message: 'DO NOT RETAIN FIXTURE PRIVATE MESSAGE' })}\n\n`);
      req.on('close', () => res.end()); return;
    }
    res.writeHead(200, { 'content-type': 'text/html' });
    const signedIn = /fixture-session=true/.test(req.headers.cookie || '');
    res.end(`<!doctype html><html><head><title>Offline release observer control</title></head>
      <body><h1>Observer fixture — not Pulse/native qualification</h1>${signedIn ? `
      <h2>System updates</h2><button id="preview">Preview beta builds</button><button id="save">Save Changes</button>
      <button id="check">Check for Updates</button><button id="install" hidden>Install Update</button>
      <main id="dialogs"></main><p id="state">Published fixture ${version}</p>
      <script>
      document.querySelector('#check').onclick = async () => { await fetch('/api/updates/check?channel=rc'); document.querySelector('#install').hidden=false; };
      document.querySelector('#install').onclick = () => {
        document.querySelector('#dialogs').innerHTML='<section role="dialog" aria-label="Confirm update"><h2>Confirm Update</h2><label><input type="checkbox"> Acknowledge</label><button id="start">Start Update</button></section>';
        document.querySelector('#start').onclick = async () => {
          await fetch('/api/updates/apply', {method:'POST'});
          document.querySelector('#dialogs').innerHTML='<section role="dialog" aria-label="Updating Pulse"><h2>Updating Pulse</h2><p>Downloading update 10%</p></section>';
          const stream=new EventSource('/api/updates/stream');
          setTimeout(() => { ${kind === 'stuck-modal' ? '' : "document.querySelector('#dialogs').innerHTML='';"}
            document.querySelector('#state').textContent='Fixture ready'; stream.close(); }, 700);
        };
      };
      </script>` : `
      <input placeholder="Username"><input placeholder="Password" type="password"><button id="login">Sign in to Pulse</button>
      <script>document.querySelector('#login').onclick = async () => { const response=await fetch('/api/login', {method:'POST'}); if(response.ok)location.reload(); };</script>`}</body></html>`);
  });
  server.on('connection', socket => { sockets.add(socket); socket.on('close', () => sockets.delete(socket)); });
  return { server, counts: () => ({ applies, logins }), close: async () => {
    for (const socket of sockets) socket.destroy();
    await new Promise(resolve => server.close(resolve));
  } };
}

async function runControls(outputDir) {
  pureControls();
  const browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
  const results = [];
  try {
    for (const [kind, mode, expectedPass, expectedApplies] of [
      ['success', 'upgrade', true, 1], ['success', 'recovery', true, 0],
      ['login-401', 'upgrade', false, 0], ['wrong-preview', 'upgrade', false, 0],
      ['silent-stream', 'upgrade', false, 1], ['stuck-modal', 'upgrade', false, 1],
      ['read-401', 'upgrade', false, 1],
    ]) {
      const target = fixture(kind, mode);
      await new Promise(resolve => target.server.listen(0, '127.0.0.1', resolve));
      const folder = path.join(outputDir, `${kind}-${mode}`); fs.mkdirSync(folder, { recursive: true });
      let passed = false;
      try {
        await runJourney(browser, { origin: `http://127.0.0.1:${target.server.address().port}`, mode, from, to,
          expected: mode === 'upgrade' ? to : from, auth: { username: 'synthetic', password: 'synthetic' },
          timeout: 1500, readinessTimeout: 3000, outputDir: folder });
        passed = true;
      } catch (_) { /* observer must reject each deliberately adverse fixture */ }
      finally { await target.close(); }
      assert.equal(passed, expectedPass, `${kind}/${mode} observer verdict`);
      assert.deepEqual(target.counts(), { applies: expectedApplies, logins: 1 }, 'one login, no apply replay');
      const receipt = JSON.parse(fs.readFileSync(path.join(folder, `browser-${mode}.json`)));
      assert.ok(!JSON.stringify(receipt).includes('PRIVATE MESSAGE'));
      assert.ok(!JSON.stringify(receipt).includes('synthetic'));
      if (kind === 'read-401') assert.equal(receipt.protected_read_status, 401);
      results.push({ fixture: kind, mode, expected_rejection: !expectedPass, observed_status: receipt.status,
        active_step: receipt.active_step, ...target.counts() });
    }
    fs.writeFileSync(path.join(outputDir, 'observer-controls.json'), JSON.stringify({ scope: 'offline adversarial observer controls only',
      browser_version: browser.version(), client_version: require('playwright/package.json').version, results }, null, 2) + '\n');
    console.log(JSON.stringify({ observer_controls: results.length, scope: 'fixtures, not native qualification', results }));
  } finally { await browser.close(); }
}
module.exports = { runControls, pureControls };
if (require.main === module) runControls(path.resolve(process.argv[2] || 'tmp/browser-journey-proof')).catch(error => { console.error(error); process.exitCode = 1; });
