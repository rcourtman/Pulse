const { chromium } = require('playwright');

const siteUrl = String(process.env.PULSE_PUBLIC_SITE_URL || '').trim();
const authUser = String(process.env.PULSE_DEMO_AUTH_USER || '').trim();
const authPass = String(process.env.PULSE_DEMO_AUTH_PASS || '');

// Demo mode signs a fresh visitor straight in (Login.tsx). The tab-scoped
// marker it writes after that attempt is what keeps the form on screen, so
// the login-form proof sets it before the app loads.
const DEMO_AUTO_LOGIN_KEY = 'pulse-demo-auto-login';

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function normalizeText(value) {
  return String(value || '')
    .replace(/\s+/g, ' ')
    .trim();
}

async function openDemo(browser, { suppressAutoLogin }) {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  if (suppressAutoLogin) {
    await context.addInitScript((key) => {
      window.sessionStorage.setItem(key, 'suppressed');
    }, DEMO_AUTO_LOGIN_KEY);
  }
  const page = await context.newPage();
  const response = await page.goto(siteUrl, {
    // Public demo shells can keep background activity alive long after the
    // page is usable, so wait for the first document and then assert the
    // actual UI instead of blocking on network quiescence.
    waitUntil: 'domcontentloaded',
    timeout: 120000,
  });
  assert(response, `No response received for ${siteUrl}`);
  assert(response.ok(), `Unexpected status ${response.status()} loading ${siteUrl}`);
  return page;
}

async function waitForConnectedWorkspace(page, startedAt, label) {
  await page.locator('main').waitFor({ state: 'visible', timeout: 60000 });
  await page
    .getByRole('status', { name: 'Backend and live data stream are connected.' })
    .waitFor({ state: 'visible', timeout: 60000 });

  const authenticatedBody = normalizeText(await page.locator('body').innerText());
  assert(authenticatedBody !== 'Loading...', 'Public demo remained on the authenticated loading shell');
  assert(authenticatedBody.includes('Pulse'), 'Authenticated public demo body is missing the Pulse workspace');
  console.log(`${label} workspace became connected in ${Date.now() - startedAt}ms`);
}

async function demoModeEnabled(page) {
  const response = await page.request.get(new URL('api/security/status', siteUrl).toString(), {
    timeout: 30000,
  });
  assert(response.ok(), `Unexpected status ${response.status()} reading demo security status`);
  const status = await response.json();
  return status?.presentationPolicy?.demoMode === true;
}

async function main() {
  assert(siteUrl, 'PULSE_PUBLIC_SITE_URL is required');
  assert(Boolean(authUser) === Boolean(authPass), 'PULSE_DEMO_AUTH_USER and PULSE_DEMO_AUTH_PASS must be provided together');

  const browser = await chromium.launch({ headless: true });
  try {
    // A first-time visitor must land on the connected workspace without typing
    // the published demo credentials.
    const visitorStartedAt = Date.now();
    const visitor = await openDemo(browser, { suppressAutoLogin: false });
    if (await demoModeEnabled(visitor)) {
      await waitForConnectedWorkspace(visitor, visitorStartedAt, 'auto-signed-in visitor');
    }
    await visitor.context().close();

    const page = await openDemo(browser, { suppressAutoLogin: true });
    await page.getByLabel('Username').waitFor({ state: 'visible', timeout: 120000 });
    await page.getByLabel('Password').waitFor({ state: 'visible', timeout: 120000 });
    await page.getByRole('button', { name: 'Sign in to Pulse' }).waitFor({ state: 'visible', timeout: 120000 });

    const title = normalizeText(await page.title());
    assert(title === 'Pulse', `Unexpected page title: ${title}`);

    const bodyText = normalizeText(await page.locator('body').innerText());
    for (const expected of ['Welcome to Pulse', 'Username', 'Password', 'Sign in to Pulse']) {
      assert(bodyText.includes(expected), `Public demo body missing ${JSON.stringify(expected)}`);
    }

    if (authUser) {
      const loginStartedAt = Date.now();
      await page.getByLabel('Username').fill(authUser);
      await page.getByLabel('Password').fill(authPass);
      await page.getByRole('button', { name: 'Sign in to Pulse' }).click();
      await waitForConnectedWorkspace(page, loginStartedAt, 'authenticated');
    }

    console.log(`public demo browser smoke passed for ${siteUrl}`);
  } finally {
    await browser.close();
  }
}

main().catch((error) => {
  const message = error instanceof Error ? error.stack || error.message : String(error);
  console.error(message);
  process.exit(1);
});
