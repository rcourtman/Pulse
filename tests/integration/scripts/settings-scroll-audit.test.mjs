import assert from 'node:assert/strict';
import test from 'node:test';
import { chromium, webkit } from '@playwright/test';
import { auditHorizontalOverflow, scrollSettingsToBottom, settingsScrollPosition } from './settings-scroll-audit.mjs';

// Browser geometry is essential here: a DOM mock cannot distinguish the
// document's scroll offset from the nested app-shell offset.
for (const [name, browserType] of Object.entries({ chromium, webkit })) {
  test(`Settings audit scrolls the content owner in ${name}`, async () => {
    const browser = await browserType.launch({ headless: true });
    try {
      for (const width of [320, 390, 430]) {
        const page = await browser.newPage({
          viewport: { width, height: 844 }, isMobile: true, hasTouch: true,
        });
        // Match App.tsx's h-screen/overflow-hidden parent and independently
        // scrolling flex child, with content taller than a phone viewport.
        await page.setContent(`<meta name="viewport" content="width=device-width, initial-scale=1"><style>
          body { margin: 0; overflow-x: hidden; overflow-y: auto; }
          .layout { display: flex; height: 100vh; overflow: hidden; }
          .app-scroll-shell { flex: 1; min-width: 0; overflow-y: scroll; }
          section { height: 3000px; }
        </style><div class="layout"><div class="app-scroll-shell">
          <section data-settings-content>Settings</section><footer>Last setting</footer>
        </div></div>`);
        // The old WebKit fallback cannot reach the actual content bottom.
        await page.evaluate(() => window.scrollBy(0, 12660));
        const before = await settingsScrollPosition(page);
        assert.equal(before.scrollTop, 0);
        assert.ok(before.maxScrollTop > 2000);
        await scrollSettingsToBottom(page);
        const after = await settingsScrollPosition(page);
        assert.ok(after.scrollTop >= after.maxScrollTop - 3, JSON.stringify({ name, width, after }));
        const footer = await page.locator('footer').boundingBox();
        assert.ok(footer.y >= 0 && footer.y + footer.height <= 844);
        assert.equal((await auditHorizontalOverflow(page)).overflowPx, 0);
        // Horizontal overflow is still measurable, not hidden by the helper.
        await page.locator('section').evaluate((element) => { element.style.width = '600px'; });
        assert.equal((await auditHorizontalOverflow(page)).pageWidth, 600);
        await page.close();
      }
    } finally {
      await browser.close();
    }
  });
}
