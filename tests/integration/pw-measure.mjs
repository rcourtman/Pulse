import { chromium } from '@playwright/test';

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  httpCredentials: { username: 'admin', password: 'admin' },
});
const page = await context.newPage();

// Navigate via Vite proxy which handles WebSocket
await page.goto('http://localhost:5173/dashboard', { waitUntil: 'networkidle', timeout: 30000 });
// Force a hard reload to clear HMR state
await page.reload({ waitUntil: 'networkidle', timeout: 30000 });
// Give websocket time to connect and data to load
await page.waitForTimeout(8000);

// Dismiss modal
const btn = page.locator('button:has-text("go")');
if (await btn.count() > 0) {
  await btn.first().click();
  await page.waitForTimeout(1000);
}

// Wait for dashboard content - look for KPI strip grid
try {
  await page.waitForFunction(() => {
    const s = document.querySelector('[data-testid="dashboard-page"] > section');
    return s && s.children.length >= 4;
  }, { timeout: 20000 });
  console.log('Dashboard loaded');
} catch (e) {
  console.log('Timeout waiting for dashboard content, continuing anyway');
}

await page.waitForTimeout(1000);
await page.screenshot({ path: '/tmp/dashboard-v8.png', fullPage: false });
await page.screenshot({ path: '/tmp/dashboard-v8-full.png', fullPage: true });

const info = await page.evaluate(() => {
  const s = document.querySelector('[data-testid="dashboard-page"] > section');
  if (s === null) {
    return { loaded: false, html: document.querySelector('[data-testid="dashboard-page"]')?.innerHTML?.substring(0, 200) };
  }
  const dashEl = document.querySelector('[data-testid="dashboard-page"]');
  const dashTop = Math.round(dashEl.getBoundingClientRect().top);
  const items = Array.from(s.children).map((el, i) => {
    const r = el.getBoundingClientRect();
    const prev = i > 0 ? s.children[i - 1].getBoundingClientRect().bottom : null;
    const cs = getComputedStyle(el);
    return {
      i,
      gap: prev !== null ? Math.round(r.top - prev) : 0,
      top: Math.round(r.top),
      h: Math.round(r.height),
      bot: Math.round(r.bottom),
      pt: cs.paddingTop,
      pb: cs.paddingBottom,
    };
  });
  return { loaded: true, dashTop, items };
});

if (info.loaded) {
  console.log('Dashboard starts at y=' + info.dashTop);
  for (const m of info.items) {
    console.log(`child-${m.i}: gap=${m.gap} top=${m.top} h=${m.h} bot=${m.bot} pt=${m.pt} pb=${m.pb}`);
  }
  const last = info.items[info.items.length - 1];
  console.log(`Content ends at y=${last.bot}, below-fold: ${900 - last.bot}px`);
} else {
  console.log('Dashboard not loaded:', info.html);
}

await browser.close();
