const { chromium } = require('playwright');

const siteUrl = String(process.env.PULSE_PUBLIC_SITE_URL || '').trim();

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

async function main() {
  assert(siteUrl, 'PULSE_PUBLIC_SITE_URL is required');

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    const response = await page.goto(siteUrl, {
      waitUntil: 'networkidle',
      timeout: 120000,
    });
    assert(response, `No response received for ${siteUrl}`);
    assert(response.ok(), `Unexpected status ${response.status()} loading ${siteUrl}`);

    await page.getByLabel('Username').waitFor();
    await page.getByLabel('Password').waitFor();
    await page.getByRole('button', { name: 'Sign in to Pulse' }).waitFor();

    const title = normalizeText(await page.title());
    assert(title === 'Pulse', `Unexpected page title: ${title}`);

    const bodyText = normalizeText(await page.locator('body').innerText());
    for (const expected of ['Welcome to Pulse', 'Username', 'Password', 'Sign in to Pulse']) {
      assert(bodyText.includes(expected), `Public demo body missing ${JSON.stringify(expected)}`);
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
