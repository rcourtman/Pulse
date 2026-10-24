import assert from 'node:assert/strict';
import fs from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { runInNewContext } from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const source = fs.readFileSync(
  process.env.PULSE_FIXTURE_AUTH_SOURCE || path.join(root, 'tests/helpers.ts'),
  'utf8',
);
const fragment = (start, end) => {
  const from = source.indexOf(start);
  const to = source.indexOf(end, from);
  assert.ok(from >= 0 && to > from, `missing helper boundary: ${start}`);
  return source.slice(from, to).replace(/^export /gm, '');
};
// Execute the real public helpers and their cookie-reuse implementation, not
// a second implementation. Strip types with the governed Node 24 runtime;
// stub only the browser/backend boundaries. No listener or credentials needed.
const helpers = stripTypeScriptTypes([
  source.match(/const AUTHENTICATED_URL =[\s\S]*?;\n/)[0],
  fragment('async function waitForAuthenticatedAppShell(', 'export async function waitForPulseReady('),
  fragment('export async function ensureAuthenticated(', 'async function storageStateHasLiveSession('),
  fragment('export async function apiRequest(', 'export async function isMultiTenantEnabled('),
  '({ ensureAuthenticated, ensureSessionAuthenticated, apiRequest });',
].join('\n'));

const appRuntimeSource = fs.readFileSync(
  path.resolve(root, '../../frontend-modern/src/useAppRuntimeState.ts'), 'utf8',
);
const preAuthPolicy = appRuntimeSource.match(/const isPreAuthLoginBootstrapPath =[\s\S]*?;\n/);
assert.ok(preAuthPolicy, 'missing app pre-auth entrypoint policy');
const isPreAuthLoginBootstrapPath = runInNewContext(stripTypeScriptTypes(
  `${preAuthPolicy[0]}\nisPreAuthLoginBootstrapPath;`,
));

const cookie = (name, value) => ({
  name, value, domain: '127.0.0.1', path: '/', expires: -1,
  httpOnly: name === 'pulse_session', secure: false, sameSite: 'Lax',
});
const response = status => ({ status: () => status, ok: () => status === 200 });

function fixture(t, options = {}) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-fixture-auth-'));
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  const statePath = path.join(directory, 'session.json');
  const sessions = new Set(['saved-live']);
  let loginCalls = 0;
  let probeCalls = 0;
  let disposed = 0;
  let primaryRevoked = false;
  let registeredWithToken = false;
  const bootstrapChecks = [];
  const requests = [];
  const primary = options.primary || '';
  const validSession = cookies => cookies.some(
    item => item.name === 'pulse_session' && sessions.has(item.value) &&
      item.domain === '127.0.0.1' && (item.expires === -1 || item.expires > Date.now() / 1000),
  );
  const fetch = (cookies, url, args = {}) => {
    const method = args.method || 'GET';
    const headers = args.headers || {};
    requests.push({ method, headers });
    const tokenAuth = !primaryRevoked && primary && headers['X-API-Token'] === primary;
    if (!validSession(cookies) && !tokenAuth) return response(401);
    if (validSession(cookies) && method !== 'GET' && headers['X-CSRF-Token'] !== 'csrf') {
      return response(403);
    }
    if (url.endsWith('/report') && method === 'POST') registeredWithToken = !!tokenAuth;
    if (url.endsWith('/fixture-host') && method === 'DELETE' && registeredWithToken) {
      primaryRevoked = true;
    }
    return response(200);
  };
  const createPage = () => {
    let cookies = [];
    let url = 'about:blank';
    let hasLoginHint = false;
    let bootstrapState = 'empty';
    let pendingBootstrap;
    let resolveWaitStarted;
    const waitStarted = new Promise(resolve => { resolveWaitStarted = resolve; });
    const navigations = [];
    const context = {
      cookies: async () => cookies,
      addCookies: async values => { cookies = [...cookies, ...values]; },
    };
    return {
      context: () => context,
      request: { fetch: async (target, args) => fetch(cookies, target, args) },
      goto: async target => {
        navigations.push(target);
        url = new URL(target, 'http://127.0.0.1:7655').href;
        bootstrapState = options.delayedBootstrap ? 'loading' : 'ready';
      },
      url: () => url,
      navigations,
      waitStarted,
      rememberLoginHint: () => { hasLoginHint = true; },
      completeBootstrap: () => {
        bootstrapState = 'ready';
        pendingBootstrap?.();
      },
      waitForFunction: async (predicate, arg, settings) => {
        assert.equal(arg, undefined);
        assert.equal(settings.timeout, 20_000);
        const admitted = () => validSession(cookies) && !options.browserRejectSession &&
          (!isPreAuthLoginBootstrapPath(new URL(url).pathname) || hasLoginHint);
        // Run the shipped predicate against a DOM-shaped boundary. Rejection
        // leaves the requested URL unchanged, just as the real app does.
        const evaluate = () => runInNewContext(`(${predicate.toString()})()`, {
          document: { querySelector: selector => {
            if (bootstrapState !== 'ready' || options.bootstrapFailure) return null;
            const present = selector === '.app-scroll-shell' ? admitted() : !admitted();
            return present ? { getClientRects: () => [1] } : null;
          } },
          getComputedStyle: () => ({ visibility: 'visible' }),
        });
        bootstrapChecks.push(evaluate());
        resolveWaitStarted();
        if (bootstrapState === 'loading') await new Promise(resolve => { pendingBootstrap = resolve; });
        const value = evaluate();
        if (!value) throw new Error('browser bootstrap timed out');
        return { jsonValue: async () => value, dispose: async () => {} };
      },
    };
  };
  const api = runInNewContext(helpers, {
    fs, path, helpersDir: path.join(root, 'tests'),
    process: { pid: process.pid, env: { PULSE_E2E_COOKIE_STATE_PATH: statePath } },
    preferredBrowserBaseURL: () => 'http://127.0.0.1:7655',
    preferredPlaywrightRouteBaseURL: () => 'http://127.0.0.1:7655',
    waitForPulseReady: async () => {},
    maybeCompleteSetupWizard: async () => {},
    waitForAppShell: async () => {},
    authenticateWithPrimaryAPIToken: async page => {
      if (!options.tokenAuth) return false;
      page.url = () => 'http://127.0.0.1:7655/proxmox';
      return true;
    },
    login: async page => {
      loginCalls++;
      if (options.loginFailure) throw new Error('Login failed: fixture rejection');
      const value = `login-${loginCalls}`;
      sessions.add(value);
      await page.context().addCookies([cookie('pulse_session', value), cookie('pulse_csrf', 'csrf')]);
      page.rememberLoginHint();
      await page.goto('/proxmox');
    },
    expect: page => ({ toHaveURL: async pattern => assert.match(page.url(), pattern) }),
    playwrightRequest: { newContext: async ({ storageState, baseURL }) => {
      assert.equal(baseURL, 'http://127.0.0.1:7655');
      assert.equal(storageState.origins.length, 0);
      return {
        get: async endpoint => {
          assert.equal(endpoint, '/api/state');
          probeCalls++;
          return response(options.probeStatus ?? (validSession(storageState.cookies) ? 200 : 401));
        },
        dispose: async () => { disposed++; },
      };
    } },
    configuredPrimaryAPIToken: () => primary,
    getTokenAuthRequestContext: async () => ({ fetch: async (url, args) => fetch([], url, args) }),
  });
  return {
    ...api, createPage, statePath, sessions, requests, bootstrapChecks,
    counts: () => ({ loginCalls, probeCalls, disposed, primaryRevoked }),
    save: (cookies, origins = []) => fs.writeFileSync(statePath, JSON.stringify({ cookies, origins })),
  };
}

test('twelve generic page fixtures reuse one password session, without backoff or retries', async t => {
  const f = fixture(t);
  for (let index = 0; index < 12; index++) await f.ensureAuthenticated(f.createPage());
  assert.equal(f.counts().loginCalls, 1);
  assert.equal(f.counts().probeCalls, 11);
  assert.equal(f.counts().disposed, 11);
  assert.equal(fs.statSync(f.statePath).mode & 0o777, 0o600);
  assert.deepEqual(JSON.parse(fs.readFileSync(f.statePath)).origins, []);
});

test('the existing admitted cookie session avoids another login and does not import org state', async t => {
  const f = fixture(t);
  f.save([
    cookie('pulse_session', 'saved-live'), cookie('pulse_csrf', 'csrf'),
    cookie('pulse_org_id', 'another-test-org'), cookie('unrelated', 'another-value'),
  ], [{ origin: 'http://127.0.0.1:7655', localStorage: [{ name: 'pulse_org_id', value: 'another-test-org' }] }]);
  const page = f.createPage();
  await f.ensureAuthenticated(page);
  assert.equal(f.counts().loginCalls, 0);
  assert.deepEqual((await page.context().cookies()).map(item => item.name), ['pulse_session', 'pulse_csrf']);
  assert.equal(f.counts().disposed, 1);
});

test('cookie-only reuse enters the protected workspace without manufacturing a local login hint', async t => {
  const f = fixture(t);
  f.save([cookie('pulse_session', 'saved-live'), cookie('pulse_csrf', 'csrf')]);
  const page = f.createPage();
  await f.ensureAuthenticated(page);
  assert.deepEqual(page.navigations, ['/infrastructure']);
  assert.equal(page.url(), 'http://127.0.0.1:7655/infrastructure');
  assert.deepEqual(f.bootstrapChecks, ['authenticated']);
  assert.deepEqual(f.counts(), { loginCalls: 0, probeCalls: 1, disposed: 1, primaryRevoked: false });
});

test('a protected URL and mounted loading root cannot complete delayed cookie bootstrap', async t => {
  const f = fixture(t, { delayedBootstrap: true });
  f.save([cookie('pulse_session', 'saved-live')]);
  const page = f.createPage();
  let completed = false;
  const authentication = f.ensureAuthenticated(page).then(() => { completed = true; });
  await page.waitStarted;
  assert.equal(page.url(), 'http://127.0.0.1:7655/infrastructure');
  assert.deepEqual(f.bootstrapChecks, [false]);
  assert.equal(completed, false);
  assert.equal(f.counts().disposed, 0);
  page.completeBootstrap();
  await authentication;
  assert.equal(completed, true);
  assert.equal(f.counts().disposed, 1);
  assert.equal(f.counts().loginCalls, 0);
});

test('API admission cannot hide browser rejection at the unchanged protected URL', async t => {
  const f = fixture(t, { browserRejectSession: true });
  f.save([cookie('pulse_session', 'saved-live')]);
  const page = f.createPage();
  await assert.rejects(() => f.ensureAuthenticated(page), /Session was rejected by browser bootstrap/);
  assert.equal(page.url(), 'http://127.0.0.1:7655/infrastructure');
  assert.deepEqual(page.navigations, ['/infrastructure']);
  assert.deepEqual(f.bootstrapChecks, ['login']);
  assert.equal(f.counts().loginCalls, 0);
  assert.equal(f.counts().disposed, 1);
  assert.equal(JSON.parse(fs.readFileSync(f.statePath)).cookies[0].value, 'saved-live');
});

test('an incomplete browser bootstrap propagates failure without another login or state replacement', async t => {
  const f = fixture(t, { bootstrapFailure: true });
  f.save([cookie('pulse_session', 'saved-live')]);
  const page = f.createPage();
  await assert.rejects(() => f.ensureAuthenticated(page), /browser bootstrap timed out/);
  assert.equal(f.counts().loginCalls, 0);
  assert.equal(f.counts().disposed, 1);
  assert.equal(JSON.parse(fs.readFileSync(f.statePath)).cookies[0].value, 'saved-live');
});

for (const scenario of ['revoked', 'expired', 'other-backend', 'missing-session', 'malformed-json']) {
  test(`${scenario} saved state is not accepted; ordinary login replaces it`, async t => {
    const f = fixture(t);
    if (scenario === 'malformed-json') fs.writeFileSync(f.statePath, '{');
    else {
      const saved = cookie(scenario === 'missing-session' ? 'pulse_csrf' : 'pulse_session', scenario);
      if (scenario === 'expired') Object.assign(saved, { value: 'saved-live', expires: 1 });
      if (scenario === 'other-backend') Object.assign(saved, { value: 'saved-live', domain: 'another-backend.test' });
      f.save([saved]);
    }
    const page = f.createPage();
    await f.ensureAuthenticated(page);
    assert.equal(f.counts().loginCalls, 1);
    assert.equal((await page.context().cookies()).some(item => item.value === scenario), false);
    assert.equal(JSON.parse(fs.readFileSync(f.statePath)).cookies[0].value, 'login-1');
    assert.equal(f.counts().disposed, f.counts().probeCalls);
  });
}

test('a denied shared session and denied password login remain a failure', async t => {
  const f = fixture(t, { loginFailure: true });
  f.save([cookie('pulse_session', 'revoked')]);
  await assert.rejects(() => f.ensureAuthenticated(f.createPage()), /Login failed: fixture rejection/);
  assert.equal(f.counts().disposed, 1);
  assert.equal(JSON.parse(fs.readFileSync(f.statePath)).cookies[0].value, 'revoked');
});

test('a failed backend probe is not hidden by another login', async t => {
  const f = fixture(t, { probeStatus: 500 });
  f.save([cookie('pulse_session', 'saved-live')]);
  await assert.rejects(() => f.ensureAuthenticated(f.createPage()), /Shared cookie session probe failed: 500/);
  assert.equal(f.counts().loginCalls, 0);
  assert.equal(f.counts().disposed, 1);
});

test('primary-token fixtures keep their token auth and import no ambient cookie session', async t => {
  const f = fixture(t, { tokenAuth: true });
  f.save([cookie('pulse_session', 'saved-live')]);
  const page = f.createPage();
  await f.ensureAuthenticated(page);
  assert.equal(f.counts().loginCalls, 0);
  assert.equal(f.counts().probeCalls, 0);
  assert.equal((await page.context().cookies()).length, 0);
});

function primaryBootstrapFixture({ rejected = false } = {}) {
  let currentURL = 'about:blank';
  let releaseBootstrap;
  const bootstrap = new Promise(resolve => { releaseBootstrap = resolve; });
  let notifyWaitStarted;
  const waitStarted = new Promise(resolve => { notifyWaitStarted = resolve; });
  let ready = false;
  const page = {
    goto: async target => { currentURL = new URL(target, 'http://127.0.0.1:7655').href; },
    url: () => currentURL,
    waitForURL: async () => { currentURL = 'http://127.0.0.1:7655/infrastructure'; },
    locator: () => ({ waitFor: () => new Promise(() => {}) }),
    waitForFunction: async predicate => {
      const evaluate = () => runInNewContext(`(${predicate.toString()})()`, {
        document: { querySelector: selector => {
          if (!ready) return null;
          const present = selector === '.app-scroll-shell' ? !rejected : rejected;
          return present ? { getClientRects: () => [1] } : null;
        } },
        getComputedStyle: () => ({ visibility: 'visible' }),
      });
      assert.equal(evaluate(), false, 'protected bootstrap is still loading');
      notifyWaitStarted();
      await bootstrap;
      return { jsonValue: async () => evaluate(), dispose: async () => {} };
    },
  };
  const authenticate = runInNewContext(stripTypeScriptTypes([
    source.match(/const AUTHENTICATED_URL =[\s\S]*?;\n/)[0],
    fragment('async function waitForAuthenticatedAppShell(', 'export async function waitForPulseReady('),
    fragment('async function authenticateWithPrimaryAPIToken(', 'export async function ensureAuthenticated('),
    'authenticateWithPrimaryAPIToken;',
  ].join('\n')), {
    URL, encodeURIComponent,
    primaryAPITokenCandidates: () => [{ token: 'fixture-token' }],
    apiRequest: async () => response(200),
    rememberPrimaryAPIToken: () => {},
    waitForAppShell: async () => {},
  });
  return {
    page, authenticate, waitStarted,
    completeBootstrap: () => {
      ready = true;
      currentURL = 'http://127.0.0.1:7655/proxmox/overview';
      releaseBootstrap();
    },
  };
}

test('an admitted token and workspace URL cannot race a subsequent onboarding navigation', async () => {
  const f = primaryBootstrapFixture();
  let completed = false;
  const authentication = f.authenticate(f.page).then(value => { completed = true; return value; });
  await Promise.race([f.waitStarted, authentication]);
  assert.equal(completed, false, 'token auth must wait for the mounted authenticated shell');
  f.completeBootstrap();
  assert.equal(await authentication, true);
  await f.page.goto('/settings/infrastructure?add=agent');
  assert.equal(f.page.url(), 'http://127.0.0.1:7655/settings/infrastructure?add=agent');
});

test('a valid token probe cannot hide a rejected browser bootstrap', async () => {
  const f = primaryBootstrapFixture({ rejected: true });
  const authentication = f.authenticate(f.page);
  await Promise.race([f.waitStarted, authentication]);
  f.completeBootstrap();
  await assert.rejects(authentication, /Session was rejected by browser bootstrap/);
});

test('explicit session fixtures still get independent password sessions', async t => {
  const f = fixture(t, { tokenAuth: true });
  f.save([cookie('pulse_session', 'saved-live')]);
  await f.ensureSessionAuthenticated(f.createPage());
  await f.ensureSessionAuthenticated(f.createPage());
  assert.equal(f.counts().loginCalls, 2);
  assert.equal(f.counts().probeCalls, 0);
  assert.equal(JSON.parse(fs.readFileSync(f.statePath)).cookies[0].value, 'saved-live');
});

test('cookie-authenticated registration can verify deletion without binding the primary token', async t => {
  const f = fixture(t, { primary: 'fixture-primary', tokenAuth: true });
  const page = f.createPage();
  await f.ensureSessionAuthenticated(page);
  for (const [endpoint, method] of [
    ['/api/agents/agent/report', 'POST'], ['/api/agents/agent/fixture-host', 'DELETE'], ['/api/state', 'GET'],
  ]) assert.equal((await f.apiRequest(page, endpoint, { method })).status(), 200);
  assert.equal(f.counts().primaryRevoked, false);
  assert.equal(f.requests.some(item => item.headers['X-API-Token']), false);
  assert.equal(f.requests[0].headers['X-CSRF-Token'], 'csrf');
});

test('negative control: deletion revokes a bound token and rejects its subsequent read', async t => {
  const f = fixture(t, { primary: 'fixture-primary', tokenAuth: true });
  const page = f.createPage();
  await f.ensureAuthenticated(page);
  assert.equal((await f.apiRequest(page, '/api/agents/agent/report', { method: 'POST' })).status(), 200);
  assert.equal((await f.apiRequest(page, '/api/agents/agent/fixture-host', { method: 'DELETE' })).status(), 200);
  assert.equal((await f.apiRequest(page, '/api/state')).status(), 401);
  assert.equal(f.counts().primaryRevoked, true);
});

test('the agent journey selects session auth and retains deletion/state assertions', () => {
  const spec = fs.readFileSync(path.join(root, 'tests/journeys/04-agent-install-registration.spec.ts'), 'utf8');
  assert.match(spec, /ensureSessionAuthenticated/);
  assert.doesNotMatch(spec, /ensureAuthenticated/);
  assert.match(spec, /`Post-deletion state request failed: \$\{stateRes.status\(\)\}`/);
  assert.match(spec, /findRegisteredAgentResource\(state\),\s*"Host should be removed from state after deletion",\s*\).toBeFalsy\(\)/);
});
