/* Offline adversarial observer controls. These fixtures are not Pulse release,
 * systemd, installed update, Tailscale or qualification evidence. */
const assert = require('node:assert/strict');
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
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
  const receipt = { ...valid, access_refused: false, login_status: 200, observed_version: to.slice(1), health: 'healthy',
    protected_read_status: 200, apply_requests: 1, apply_status: 200, selected_version: to.slice(1),
    sse_opens: 1, sse_progress: [{ status: 'downloading', progress: 10 }], progress_dialog_visible: false };
  judge(receipt);
  for (const delta of [{ access_refused: true }, { login_status: 401 }, { observed_version: from.slice(1) }, { health: 'unhealthy' },
    { protected_read_status: 403 }, { apply_requests: 0 }, { apply_requests: 2 }, { apply_status: 500 },
    { selected_version: '6.5.0-rc.1' }, { sse_opens: 0 }, { sse_progress: [] }, { progress_dialog_visible: true }]) {
    assert.throws(() => judge({ ...receipt, ...delta }));
  }
}

function fixture(kind) {
  let version = from.slice(1), applied = false, denied = false;
  const counters = { applies: 0, logins: 0, version_reads: 0, health_reads: 0,
    protected_reads: 0, streams: 0, denials: 0, anonymous_probes: 0,
    requests_after_denial: 0, transport_gaps: 0 };
  const denial = /^(.*)-(401|403)$/.exec(kind);
  const sockets = new Set(), timers = new Set();
  const later = (fn, ms) => { const timer = setTimeout(() => { timers.delete(timer); fn(); }, ms); timers.add(timer); };
  const server = http.createServer((req, res) => {
    if (denied) counters.requests_after_denial++;
    const url = new URL(req.url, 'http://localhost');
    const signedIn = /fixture-session=true/.test(req.headers.cookie || '');
    const json = (code, data) => { res.writeHead(code, { 'content-type': 'application/json' }); res.end(JSON.stringify(data)); };
    const refuse = endpoint => {
      if (denial?.[1] !== endpoint) return false;
      denied = true; counters.denials++;
      json(Number(denial[2]), { error: 'DO NOT RETAIN FIXTURE PRIVATE DENIAL' });
      return true;
    };
    if (url.pathname === '/api/login') {
      counters.logins++;
      if (refuse('login')) return;
      res.setHeader('Set-Cookie', 'fixture-session=true; Path=/; HttpOnly; SameSite=Lax');
      return json(200, { success: true });
    }
    if (url.pathname === '/api/security/status' || url.pathname === '/api/config/nodes' && !signedIn) {
      counters.anonymous_probes++;
      return json(url.pathname === '/api/security/status' ? 401 : 403, { error: 'normal anonymous auth probe' });
    }
    if (url.pathname === '/api/version') {
      counters.version_reads++;
      if (refuse(applied ? 'version' : 'initial-version')) return;
      if (applied && kind === 'restart-gap' && counters.transport_gaps === 0) {
        counters.transport_gaps++; res.destroy(); return;
      }
      return json(200, { version, isDocker: false, isSourceBuild: false });
    }
    if (url.pathname === '/api/health') {
      counters.health_reads++;
      if (refuse('health')) return;
      return json(200, { status: 'healthy' });
    }
    if (url.pathname === '/api/config/nodes') {
      counters.protected_reads++;
      if (applied && refuse('read')) return;
      return json(200, []);
    }
    if (url.pathname === '/api/settings/system') {
      if (refuse('authenticated-api')) return;
      return json(200, {});
    }
    if (url.pathname === '/api/updates/check') {
      if (refuse('check')) return;
      return json(200, { latestVersion: kind === 'wrong-preview' ? '6.5.0-rc.1' : to.slice(1), downloadUrl });
    }
    if (url.pathname === '/api/updates/apply') {
      counters.applies++;
      if (refuse('apply')) return;
      applied = true;
      later(() => { version = to.slice(1); }, 350);
      return json(200, { status: 'started' });
    }
    if (url.pathname === '/api/updates/stream') {
      counters.streams++;
      if (refuse('stream')) return;
      res.writeHead(200, { 'content-type': 'text/event-stream', 'cache-control': 'no-cache' });
      res.write(': real loopback fixture stream\n\n');
      if (kind !== 'silent-stream') res.write(`data: ${JSON.stringify({ status: 'downloading', progress: 10, message: 'DO NOT RETAIN FIXTURE PRIVATE MESSAGE' })}\n\n`);
      req.on('close', () => res.end()); return;
    }
    if (refuse(signedIn ? 'authenticated-navigation' : 'navigation')) return;
    res.writeHead(200, { 'content-type': 'text/html' });
    res.end(`<!doctype html><html><head><title>Offline release observer control</title></head>
      <body><h1>Observer fixture — not Pulse/native qualification</h1>${signedIn ? `
      <h2>System updates</h2><button id="preview">Preview beta builds</button><button id="save">Save Changes</button>
      <button id="check">Check for Updates</button><button id="install" hidden>Install Update</button>
      <main id="dialogs"></main><p id="state">Published fixture ${version}</p>
      <script>
      document.querySelector('#save').onclick = async () => { await fetch('/api/settings/system', {method:'POST'}); };
      document.querySelector('#check').onclick = async () => { await fetch('/api/updates/check?channel=rc'); document.querySelector('#install').hidden=false; };
      document.querySelector('#install').onclick = () => {
        document.querySelector('#dialogs').innerHTML='<section role="dialog" aria-label="Confirm update"><h2>Confirm Update</h2><label><input type="checkbox"> Acknowledge</label><button id="start">Start Update</button></section>';
        document.querySelector('#start').onclick = async () => {
          const response = await fetch('/api/updates/apply', {method:'POST'}); if (!response.ok) return;
          document.querySelector('#dialogs').innerHTML='<section role="dialog" aria-label="Updating Pulse"><h2>Updating Pulse</h2><p>Downloading update 10%</p></section>';
          const stream=new EventSource('/api/updates/stream');
          setTimeout(() => { ${kind === 'stuck-modal' ? '' : "document.querySelector('#dialogs').innerHTML='';"}
            document.querySelector('#state').textContent='Fixture ready'; stream.close(); }, 700);
        };
      };
      </script>` : `
      <input placeholder="Username"><input placeholder="Password" type="password"><button id="login">Sign in to Pulse</button>
      <script>
      const probes = ${kind === 'anonymous-probes' ? "Promise.all([fetch('/api/security/status'), fetch('/api/config/nodes')])" : 'Promise.resolve()'};
      document.querySelector('#login').onclick = async () => { await probes; const response=await fetch('/api/login', {method:'POST'}); if(response.ok)location.reload(); };
      </script>`}</body></html>`);
  });
  server.on('connection', socket => { sockets.add(socket); socket.on('close', () => sockets.delete(socket)); });
  return { server, counts: () => ({ ...counters }), close: async () => {
    for (const timer of timers) clearTimeout(timer);
    for (const socket of sockets) socket.destroy();
    await new Promise(resolve => server.close(resolve));
  } };
}

function recoveryGuard(folder, refused) {
  // Exercise the ACTUAL shell guard with this browser's receipt. The permitted
  // branch's process is a fixed local sentinel, not installation or recovery.
  const state = path.join(folder, 'state'); fs.mkdirSync(state);
  fs.copyFileSync(path.join(folder, 'browser-upgrade.json'), path.join(state, 'browser-upgrade.json'));
  const invocation = path.join(folder, 'guard-process-started');
  const sentinel = path.join(folder, 'process-sentinel.sh');
  fs.writeFileSync(sentinel, `#!/bin/sh\nprintf started > ${JSON.stringify(invocation)}\n`, { mode: 0o700 });
  const harness = fs.readFileSync(path.resolve(__dirname, '../../../scripts/release_lifecycle_rehearsal.sh'), 'utf8');
  const body = 'run_browser_journey() {' + harness.split('run_browser_journey() {')[1].split('\n}\n')[0] + '\n}';
  const result = spawnSync('bash', ['-c', `set -euo pipefail\n${body}\nrun_browser_journey recovery v6.4.5\n`], {
    env: { PATH: '/usr/local/bin:/usr/bin:/bin', WORK_DIR: folder, ROOT_DIR: path.resolve(__dirname, '../../..'),
      SEED_ADMIN_USER: 'synthetic', ADMIN_PASSWORD: 'synthetic', PULSE_REHEARSAL_NODE: sentinel,
      PULSE_REHEARSAL_BROWSER_ORIGIN: 'https://ephemeral.tawny-powan.ts.net', FROM_TAG: from, TO_TAG: to },
    encoding: 'utf8', timeout: 5000,
  });
  assert.equal(result.error, undefined, 'shell guard must execute');
  assert.equal(result.stderr, '', 'no raw error/receipt contents');
  assert.equal(result.status, refused ? 1 : 0, 'actual recovery guard verdict');
  assert.equal(fs.existsSync(invocation), !refused, 'no later browser process after refusal');
  assert.equal(fs.existsSync(path.join(state, 'browser-auth.json')), !refused, 'no later auth preparation after refusal');
  if (refused) assert.deepEqual(JSON.parse(fs.readFileSync(path.join(state, 'browser-recovery.json'))),
    { status: 'not-executed', reason: 'stopped-access-no-reauthentication' });
  else fs.writeFileSync(path.join(state, 'browser-auth.json'), '');
  return { executed_guard: true, exit: result.status, browser_process_started: !refused, auth_prepared: !refused };
}

async function runControls(outputDir) {
  pureControls();
  const browser = await chromium.launch({ headless: true, channel: 'chromium', args: ['--no-sandbox'] });
  const results = [];
  try {
    const controls = [
      ['success', 'upgrade', true, 1, 1], ['success', 'recovery', true, 0, 1],
      ['anonymous-probes', 'upgrade', true, 1, 1], ['restart-gap', 'upgrade', true, 1, 1],
      ['wrong-preview', 'upgrade', false, 0, 1], ['silent-stream', 'upgrade', false, 1, 1],
      ['stuck-modal', 'upgrade', false, 1, 1],
    ];
    for (const endpoint of ['navigation', 'authenticated-navigation', 'login', 'initial-version',
      'authenticated-api', 'check', 'apply', 'stream', 'version', 'health', 'read']) {
      for (const status of [401, 403]) {
        controls.push([`${endpoint}-${status}`, 'upgrade', false,
          ['apply', 'stream', 'version', 'health', 'read'].includes(endpoint) ? 1 : 0,
          endpoint === 'navigation' ? 0 : 1]);
      }
    }
    for (const [kind, mode, expectedPass, expectedApplies, expectedLogins] of controls) {
      const target = fixture(kind);
      await new Promise(resolve => target.server.listen(0, '127.0.0.1', resolve));
      const folder = path.resolve(outputDir, `${kind}-${mode}`); fs.mkdirSync(folder, { recursive: true });
      let passed = false;
      try {
        await runJourney(browser, { origin: `http://127.0.0.1:${target.server.address().port}`, mode, from, to,
          expected: mode === 'upgrade' ? to : from, auth: { username: 'synthetic', password: 'synthetic' },
          timeout: 1500, readinessTimeout: 3000, outputDir: folder });
        passed = true;
      } catch (_) { /* observer must reject each deliberately adverse fixture */ }
      finally { await target.close(); }
      assert.equal(passed, expectedPass, `${kind}/${mode} observer verdict`);
      const counts = target.counts();
      assert.equal(counts.applies, expectedApplies, 'one apply, no replay');
      assert.equal(counts.logins, expectedLogins, 'no login after navigation denial');
      assert.equal(counts.requests_after_denial, 0, 'page stops after denial, including SSE reconnect');
      const receipt = JSON.parse(fs.readFileSync(path.join(folder, `browser-${mode}.json`)));
      assert.ok(!JSON.stringify(receipt).includes('PRIVATE MESSAGE'));
      assert.ok(!JSON.stringify(receipt).includes('synthetic'));
      assert.equal(receipt.context_closed, true, 'browser context cleaned up');
      const refusal = /-(401|403)$/.test(kind);
      assert.equal(receipt.access_refused, refusal, 'only actual access refusal stops later access');
      if (refusal) {
        assert.equal(receipt.access_refusals[0].status, Number(kind.slice(-3)));
        assert.equal(counts.denials, 1, 'denied request is not repeated');
        assert.ok(!JSON.stringify(receipt).includes('PRIVATE DENIAL'));
      }
      if (kind.startsWith('read-')) assert.equal(receipt.protected_read_status, Number(kind.slice(-3)));
      if (kind === 'anonymous-probes') assert.equal(counts.anonymous_probes, 2, 'normal pre-login denial controls ran');
      if (kind === 'restart-gap') assert.equal(counts.transport_gaps, 1, 'restart transport control ran');
      const recovery = mode === 'upgrade' ? recoveryGuard(folder, refusal) : null;
      results.push({ fixture: kind, mode, expected_rejection: !expectedPass, observed_status: receipt.status,
        active_step: receipt.active_step, ...counts, recovery_guard: recovery, receipt });
    }
    fs.writeFileSync(path.join(outputDir, 'observer-controls.json'), JSON.stringify({ scope: 'offline adversarial observer controls only',
      browser_version: browser.version(), client_version: require('playwright/package.json').version, results }, null, 2) + '\n');
    console.log(JSON.stringify({ observer_controls: results.length, scope: 'fixtures, not native qualification', results }));
  } finally { await browser.close(); }
}
module.exports = { runControls, pureControls };
if (require.main === module) runControls(path.resolve(process.argv[2] || 'tmp/browser-journey-proof')).catch(error => { console.error(error); process.exitCode = 1; });
