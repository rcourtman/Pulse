// Run the actual integration spec and full app shell offline. Only unrelated
// bootstrap/auth/state APIs are synthetic; the spec owns diagnostics responses.
// This is frontend integration proof, not backend authentication or appliance proof.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const http = require('node:http');
const { spawn } = require('node:child_process');
const { chromium, webkit } = require('playwright');
const { WebSocketServer } = require('/workspace/tests/integration/node_modules/ws');
const root = '/workspace';
const out = path.join(root, 'tmp/diagnostics-integration-proof');
const origin = 'http://127.0.0.1:5393';
const backendOrigin = 'http://127.0.0.1:5394';
const state = {
  resources: [],
  connectedInfrastructure: [],
  activeAlerts: [],
  recentlyResolved: [],
  lastUpdate: Date.now(),
};
const bootstrap = {
  '/api/health': { status: 'healthy' },
  '/api/state': state,
  '/api/security/status': {
    hasAuthentication: true,
    requiresAuth: false,
    settingsCapabilities: {
      infrastructureRead: true,
      systemSettingsRead: true,
      apiAccessRead: true,
      apiAccessWrite: true,
      authenticationRead: true,
      authenticationWrite: true,
    },
    sessionCapabilities: { isAdmin: true },
    presentationPolicy: {
      hideCommercial: true,
      hideUpgrade: true,
      demoMode: false,
      readOnly: false,
    },
  },
  '/api/config': { nodes: [], pbs: [], pmg: [], dockerHosts: [], theme: 'light' },
  '/api/version': { version: '6.4.5' },
  '/api/updates/check': { currentVersion: '6.4.5', latestVersion: '6.4.5', updateAvailable: false },
  '/api/license/status': { valid: false, tier: 'free', features: [] },
  '/api/ai/settings': { enabled: false },
  '/api/alerts/active': [],
};
(async () => {
  fs.mkdirSync(out, { recursive: true });
  const version = require('playwright/package.json').version;
  const lock = JSON.parse(fs.readFileSync(path.join(root, 'tests/integration/package-lock.json')));
  if (version !== lock.packages['node_modules/@playwright/test'].version)
    throw new Error('Playwright version mismatch');
  const paths = [
    'tests/integration/tests/69-diagnostics-onboarding.spec.ts',
    'tests/integration/playwright.config.ts',
    'frontend-modern/src/App.tsx',
    'frontend-modern/src/components/Settings/DiagnosticsPanel.tsx',
    'frontend-modern/src/components/Settings/DiagnosticsResultsPanel.tsx',
    'frontend-modern/src/components/Settings/diagnosticsModel.ts',
    'frontend-modern/src/components/Settings/useDiagnosticsPanelState.ts',
    'frontend-modern/src/utils/diagnosticsPresentation.ts',
    'frontend-modern/package-lock.json',
    'tests/integration/package-lock.json',
    'frontend-modern/browser-tests/diagnostics-integration.cjs',
  ];
  const source = Object.fromEntries(
    paths.map((p) => [
      p,
      crypto
        .createHash('sha256')
        .update(fs.readFileSync(path.join(root, p)))
        .digest('hex'),
    ]),
  );
  const requests = [];
  const backend = http.createServer((req, res) => {
    const pathname = new URL(req.url, backendOrigin).pathname;
    requests.push({ method: req.method, path: pathname });
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify(bootstrap[pathname] ?? {}));
  });
  const sockets = new WebSocketServer({ server: backend });
  sockets.on('connection', (socket) => {
    socket.send(JSON.stringify({ type: 'initialState', data: state }));
    socket.on('message', () => socket.send(JSON.stringify({ type: 'pong' })));
  });
  process.env.PULSE_DEV_API_URL = backendOrigin;
  process.env.PULSE_DEV_WS_URL = backendOrigin.replace('http:', 'ws:');
  const frontend = path.join(root, 'frontend-modern');
  const { createServer } = await import(
    path.join(frontend, 'node_modules/vite/dist/node/index.js')
  );
  const server = await createServer({
    root: frontend,
    configFile: path.join(frontend, 'vite.config.ts'),
    cacheDir: path.join(out, 'vite-cache'),
    server: { host: '127.0.0.1', port: 5393, strictPort: true },
  });
  // A cookie-backed synthetic bootstrap fixture avoids password/auth mutation.
  const cookieState = path.join(out, 'synthetic-session.json');
  fs.writeFileSync(
    cookieState,
    JSON.stringify({
      cookies: [
        {
          name: 'pulse_session',
          value: 'synthetic-offline-only',
          domain: '127.0.0.1',
          path: '/',
          expires: -1,
          httpOnly: true,
          secure: false,
          sameSite: 'Lax',
        },
      ],
      origins: [],
    }),
  );
  const config = path.join(out, 'playwright.config.cjs');
  fs.writeFileSync(
    config,
    `
    const { devices } = require('/workspace/tests/integration/node_modules/@playwright/test');
    module.exports = {
      testDir: '/workspace/tests/integration/tests', testMatch: '69-diagnostics-onboarding.spec.ts',
      workers: 1, retries: 0, timeout: 180000, expect: { timeout: 20000 },
      outputDir: '${out}/results', reporter: [['list'], ['json', { outputFile: '${out}/result.json' }]],
      use: { baseURL: '${origin}', trace: 'off', screenshot: 'only-on-failure', acceptDownloads: true },
      projects: [
        { name: 'chromium', use: { ...devices['Desktop Chrome'], launchOptions: { executablePath: ${JSON.stringify(chromium.executablePath())}, args: ['--no-sandbox'] } } },
        { name: 'mobile-chrome', use: { ...devices['Pixel 5'], launchOptions: { executablePath: ${JSON.stringify(chromium.executablePath())}, args: ['--no-sandbox'] } } },
        { name: 'mobile-safari', use: { ...devices['iPhone 12'], launchOptions: { executablePath: ${JSON.stringify(webkit.executablePath())} } } },
      ],
    };
  `,
  );
  let result, cleanup;
  try {
    await new Promise((resolve) => backend.listen(5394, '127.0.0.1', resolve));
    await server.listen();
    result = await new Promise((resolve, reject) => {
      const child = spawn(
        process.execPath,
        [
          path.join(root, 'tests/integration/node_modules/@playwright/test/cli.js'),
          'test',
          '--config',
          config,
        ],
        {
          cwd: path.join(root, 'tests/integration'),
          env: {
            ...process.env,
            PLAYWRIGHT_BASE_URL: origin,
            PULSE_E2E_COOKIE_STATE_PATH: cookieState,
            PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY: 'false',
            PULSE_E2E_RUNTIME_STATE_PATH: path.join(out, 'unused-runtime.json'),
          },
          stdio: 'inherit',
        },
      );
      child.on('error', reject);
      child.on('exit', (exit, signal) => resolve({ exit, signal }));
    });
  } finally {
    for (const socket of sockets.clients) socket.terminate();
    await new Promise((resolve) => sockets.close(resolve));
    await server.close();
    await new Promise((resolve) => backend.close(resolve));
    cleanup = {
      frontendClosed: true,
      socketsClosed: true,
      backendClosed: true,
      runnerTerminal: Boolean(result),
    };
    fs.writeFileSync(
      path.join(out, 'receipt.json'),
      JSON.stringify(
        {
          source,
          playwright: version,
          result,
          cleanup,
          boundary:
            'Actual full app shell and exact integration spec; synthetic offline session/bootstrap/state, no native API/guest-agent requests or production authentication.',
          requests,
        },
        null,
        2,
      ) + '\n',
    );
  }
  if (result.exit !== 0 || result.signal) throw new Error('Diagnostics integration failed');
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
