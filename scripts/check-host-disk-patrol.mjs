// Admission/UI contract proof with a production-emitted disk alert.
// GET estate/session data is synthetic. Patrol POSTs use the current Go handler
// from TestPatrolHostDiskBrowserBackend. No inference or infrastructure writes.
import { chromium, expect } from '../tests/integration/node_modules/@playwright/test/index.mjs';
import { mkdir, writeFile, readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';

const base = process.env.PLAYWRIGHT_BASE_URL;
const backend = process.env.PULSE_HOST_DISK_BROWSER_URL;
if (!base || !backend) throw new Error('Set PLAYWRIGHT_BASE_URL and PULSE_HOST_DISK_BROWSER_URL to isolated loopback servers');
for (const url of [base, backend]) {
  if (new URL(url).hostname !== '127.0.0.1') throw new Error('Proof servers must use loopback');
}
const output = new URL('../tmp/host-disk-patrol/', import.meta.url);
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ headless: true });
const results = [];
const errors = [];
const diagnostics = [];
let lastPage;
try {
  const request = await browser.newContext();
  const alert = await (await request.request.get(`${backend}/proof/alert`)).json();
  await request.close();
  expect(alert.resourceId).toBe('agent:repro-host-source-01/disk:docker-data');
  expect(alert.value).toBe(85);
  expect(alert.threshold).toBe(85);
  for (const width of [1440, 390]) {
    const context = await browser.newContext({ viewport: { width, height: 1000 } });
    const page = await context.newPage();
    lastPage = page;
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('console', (message) => {
      if (message.type() === 'error') diagnostics.push(message.text());
    });
    page.on('requestfailed', (request) => diagnostics.push(`${request.url()}: ${request.failure()?.errorText}`));
    let mode = 'accepted';
    const admissions = [];
    const state = { nodes: [], vms: [], containers: [], dockerHosts: [], hosts: [], storage: [], resources: [], activeAlerts: [alert], recentlyResolved: [], connectionHealth: {}, timestamp: Date.now() };
    await page.routeWebSocket('**/ws**', (socket) => socket.send(JSON.stringify({ type: 'initialState', data: state })));
    await page.route('**/api/**', async (route) => {
      const req = route.request();
      const path = new URL(req.url()).pathname;
      // Vite also serves source modules below /src/api/. Only stub HTTP API
      // endpoints; module requests must load the actual application source.
      if (!path.startsWith('/api/')) return route.continue();
      if (path === '/api/ai/patrol/run' && req.method() === 'POST') {
        const body = req.postDataJSON();
        expect(body).toEqual({ resource_ids: [alert.resourceId], alert_identifier: alert.id, alert_type: 'disk' });
        const response = await route.fetch({ url: `${backend}${path}?case=${mode}` });
        admissions.push({ mode, body, status: response.status(), response: await response.json() });
        return route.fulfill({ response });
      }
      if (req.method() !== 'GET') return route.fulfill({ status: 403, json: { error: 'Other writes disabled in proof' } });
      if (path === '/api/version') return route.fulfill({ json: { version: '6.5.0', build: 'proof', runtime: 'development', isDocker: false, isSourceBuild: true, isDevelopment: true } });
      if (path === '/api/updates/check') return route.fulfill({ json: { available: false, currentVersion: '6.5.0', latestVersion: '6.5.0', releaseNotes: '', downloadUrl: '', isPrerelease: false, isMajorUpgrade: false } });
      if (path === '/api/updates/status') return route.fulfill({ json: { status: 'idle', progress: 0, message: '', updatedAt: new Date().toISOString() } });
      if (path === '/api/security/status') return route.fulfill({ json: { hasAuthentication: true, hasProxyAuth: true, proxyAuthUsername: 'proof-operator', sessionCapabilities: { assistantEnabled: true, settingsRead: true, settingsWrite: true } } });
      if (path === '/api/license/runtime-capabilities') return route.fulfill({ json: { capabilities: ['ai_patrol', 'sso'], limits: [], blocked_capabilities: [], hosted_mode: false } });
      if (path === '/api/orgs') return route.fulfill({ json: [{ id: 'default', displayName: 'Proof' }] });
      if (path === '/api/resources') return route.fulfill({ json: { data: [], aggregations: { platformAdmission: { proxmox: false, docker: false, kubernetes: false, truenas: false, vmware: false, standalone: true } } } });
      if (path === '/api/state') return route.fulfill({ json: state });
      if (path === '/api/alerts/active') return route.fulfill({ json: [alert] });
      if (path === '/api/alerts/config') return route.fulfill({ json: { enabled: true, activationState: 'active', nodeDefaults: {}, guestDefaults: {}, agentDefaults: {}, overrides: {}, timeThresholds: {}, notifications: {} } });
      if (path === '/api/ai/settings') return route.fulfill({ json: { enabled: true, model: 'fixture:admission', control_level: 'read_only' } });
      if (/\/(history|events|delivery-diagnosis|sessions|models|findings|incidents)$/.test(path)) return route.fulfill({ json: [] });
      return route.fulfill({ json: {} });
    });
    await page.goto(`${base}/alerts`, { waitUntil: 'domcontentloaded' });
    const button = page.getByTitle('Have Patrol investigate this alert', { exact: true });
    await expect(button).toBeVisible({ timeout: 30000 });
    await button.scrollIntoViewIfNeeded();
    const rect = await button.boundingBox();
    expect(rect.x).toBeGreaterThanOrEqual(0);
    expect(rect.x + rect.width).toBeLessThanOrEqual(width);
    await page.screenshot({ path: new URL(`default-${width}.png`, output).pathname, fullPage: true });
    const menu = page.getByRole('button', { name: 'More alert actions', exact: true });
    await menu.click();
    await page.screenshot({ path: new URL(`menu-${width}.png`, output).pathname, fullPage: true });
    await page.keyboard.press('Escape');
    await expect(menu).toHaveAttribute('aria-expanded', 'false');
    await menu.click();
    await page.getByText('Active Alerts', { exact: true }).first().click();
    await expect(menu).toHaveAttribute('aria-expanded', 'false');
    for (const [scenario, status, text] of [
      ['accepted', 200, `Patrol is investigating ${alert.resourceName}`],
      ['busy', 409, 'Patrol is already running.'],
      ['unresolved', 422, 'One or more requested resource identities could not be resolved exactly'],
    ]) {
      mode = scenario;
      const before = admissions.length;
      await button.focus();
      await page.keyboard.press('Enter');
      await expect.poll(() => admissions.length).toBe(before + 1);
      expect(admissions.at(-1).status).toBe(status);
      await expect(page.getByText(text, { exact: false }).first()).toBeVisible();
      await page.screenshot({ path: new URL(`${scenario}-${width}.png`, output).pathname, fullPage: true });
    }
    mode = 'accepted';
    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(button).toBeVisible();
    await button.click();
    await expect.poll(() => admissions.at(-1).mode).toBe('accepted');
    await expect(page.getByText(`Patrol is investigating ${alert.resourceName}`, { exact: true })).toBeVisible();
    results.push({ width, admissions });
    await context.close();
  }
  expect(errors).toEqual([]);
  const paths = ['internal/unifiedresources/history_identity.go', 'internal/ai/patrol_alert_scope.go', 'internal/ai/patrol_state.go', 'internal/ai/patrol_run.go', 'internal/ai/patrol_ai.go', 'internal/ai/patrol_triggers.go'];
  const content_sha256 = Object.fromEntries(await Promise.all(paths.map(async (path) => [path, createHash('sha256').update(await readFile(new URL(`../${path}`, import.meta.url))).digest('hex')])));
  await writeFile(new URL('result.json', output), JSON.stringify({ result: 'passed', verified_at: new Date().toISOString(), frontend_base_sha: process.env.PROOF_BASE_SHA, content_sha256, routes: ['/alerts'], viewports: [1440, 390], states: ['default', 'menu open/closed', 'accepted', 'busy', 'unresolved owner', 'reload'], interactions: ['pointer', 'Enter', 'Escape', 'outside click', 'reload then re-submit'], results, errors, limits: 'Real API admission, synthetic estate/session, no inference or diagnosis qualification' }, null, 2));
} catch (error) {
  if (lastPage && !lastPage.isClosed()) {
    await lastPage.screenshot({ path: new URL('failure.png', output).pathname, fullPage: true });
    await writeFile(new URL('failure.txt', output), `${error.stack}\nPage errors: ${JSON.stringify(errors)}\nDiagnostics: ${JSON.stringify(diagnostics)}\nURL: ${lastPage.url()}\n${await lastPage.locator('body').innerText()}`);
  }
  throw error;
} finally {
  await browser.close();
}
