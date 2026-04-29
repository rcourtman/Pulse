import { chromium } from '@playwright/test';

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  httpCredentials: { username: 'admin', password: 'admin' },
});
const page = await context.newPage();

// Navigate via Vite proxy which handles WebSocket
await page.goto('http://localhost:5173/infrastructure', { waitUntil: 'networkidle', timeout: 30000 });
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

// Wait for infrastructure content.
try {
  await page.waitForFunction(() => {
    const s = document.querySelector('[data-testid="infrastructure-page"]');
    return s && s.textContent?.includes('Infrastructure');
  }, { timeout: 20000 });
  console.log('Infrastructure loaded');
} catch (e) {
  console.log('Timeout waiting for infrastructure content, continuing anyway');
}

await page.waitForTimeout(1000);
await page.screenshot({ path: '/tmp/infrastructure-v8.png', fullPage: false });
await page.screenshot({ path: '/tmp/infrastructure-v8-full.png', fullPage: true });

const info = await page.evaluate(() => {
  const s = document.querySelector('[data-testid="infrastructure-page"]');
  if (s === null) {
    return { loaded: false, html: document.querySelector('[data-testid="infrastructure-page"]')?.innerHTML?.substring(0, 200) };
  }
  const surfaceEl = document.querySelector('[data-testid="infrastructure-page"]');
  const surfaceTop = Math.round(surfaceEl.getBoundingClientRect().top);
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
  return { loaded: true, surfaceTop, items };
});

if (info.loaded) {
  console.log('Infrastructure starts at y=' + info.surfaceTop);
  for (const m of info.items) {
    console.log(`child-${m.i}: gap=${m.gap} top=${m.top} h=${m.h} bot=${m.bot} pt=${m.pt} pb=${m.pb}`);
  }
  const last = info.items[info.items.length - 1];
  console.log(`Content ends at y=${last.bot}, below-fold: ${900 - last.bot}px`);
} else {
  console.log('Infrastructure not loaded:', info.html);
}

await browser.close();
