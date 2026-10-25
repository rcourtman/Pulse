import assert from 'node:assert/strict';
import test from 'node:test';
import { chromium, webkit } from '@playwright/test';
import { auditHorizontalOverflow, auditSettingsAtBottom, scrollSettingsToBottom, settingsScrollPosition } from './settings-scroll-audit.mjs';

// Browser geometry is essential here: a DOM mock cannot distinguish the
// document's scroll offset from the nested app-shell offset.
for (const [name, browserType] of Object.entries({ chromium, webkit })) {
  test(`Settings audit scrolls the content owner in ${name}`, async () => {
    const browser = await browserType.launch({ headless: true });
    try {
      for (const width of [320, 390, 430]) {
        for (const contentHeight of [3000, 30000]) {
          const page = await browser.newPage({
            viewport: { width, height: 844 }, isMobile: true, hasTouch: true,
          });
          // Match App.tsx's h-screen/overflow-hidden parent and independently
          // scrolling flex child, with content taller than a phone viewport.
          await page.setContent(`<meta name="viewport" content="width=device-width, initial-scale=1"><style>
            body { margin: 0; overflow-x: hidden; overflow-y: auto; }
            .layout { display: flex; height: 100vh; overflow: hidden; }
            .app-scroll-shell { flex: 1; min-width: 0; overflow-y: scroll; }
            section { height: ${contentHeight}px; }
          </style><div class="layout"><div class="app-scroll-shell">
            <section data-settings-content>Settings</section><footer>Last setting</footer>
          </div></div>`);
          // The old WebKit fallback cannot reach the actual content bottom.
          await page.evaluate(() => window.scrollBy(0, 12660));
          const before = await settingsScrollPosition(page);
          assert.equal(before.scrollTop, 0);
          assert.ok(before.maxScrollTop > 2000);
          const audit = await auditSettingsAtBottom(page);
          const after = audit.scrollPosition;
          assert.ok(after.scrollTop >= after.maxScrollTop - 3, JSON.stringify({ name, width, contentHeight, after }));
          const footer = await page.locator('footer').boundingBox();
          assert.ok(footer.y >= 0 && footer.y + footer.height <= 844);
          assert.equal(audit.overflowPx, 0);
          // Horizontal overflow is still measurable, not hidden by the helper.
          await page.locator('section').evaluate((element) => { element.style.width = '600px'; });
          assert.equal((await auditHorizontalOverflow(page)).pageWidth, 600);
          await page.close();
        }
      }
    } finally {
      await browser.close();
    }
  });

  test(`Settings audit follows reflow but rejects a scroll reset in ${name}`, async () => {
    const browser = await browserType.launch({ headless: true });
    try {
      const page = await browser.newPage({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
      const prepare = async (change) => {
        await page.setContent(`<meta name="viewport" content="width=device-width, initial-scale=1"><style>
          body { margin: 0; } .layout { height: 100vh; display: flex; overflow: hidden; }
          .app-scroll-shell { flex: 1; min-width: 0; overflow-y: scroll; }
          section { height: 3000px; }
        </style><div class="layout"><div class="app-scroll-shell"><section>Settings</section><footer>Last setting</footer></div></div>`);
        await page.locator('.app-scroll-shell').evaluate((element, change) => {
          const readHeight = Object.getOwnPropertyDescriptor(Element.prototype, 'scrollHeight').get;
          let changed = false;
          Object.defineProperty(element, 'scrollHeight', { get() {
            const height = readHeight.call(this);
            if (!changed && this.scrollTop >= height - this.clientHeight - 3) {
              changed = true;
              // Model data arriving immediately after the bottom snapshot.
              queueMicrotask(() => {
                if (change === 'grow') element.querySelector('section').style.height = '4500px';
                else element.scrollTop = 0;
              });
            }
            return height;
          } });
        }, change);
      };

      await prepare('grow');
      await scrollSettingsToBottom(page);
      const separatedRead = await settingsScrollPosition(page);
      assert.ok(separatedRead.scrollTop < separatedRead.maxScrollTop - 3, 'separate bottom reads must expose the controlled reflow');

      await prepare('grow');
      const audit = await auditSettingsAtBottom(page);
      assert.ok(audit.scrollPosition.scrollTop >= audit.scrollPosition.maxScrollTop - 3);
      assert.equal(audit.overflowPx, 0);
      const footer = await page.locator('footer').boundingBox();
      assert.ok(footer.y >= 0 && footer.y + footer.height <= 844);
      await page.locator('section').evaluate(element => { element.style.width = '600px'; });
      assert.equal((await auditSettingsAtBottom(page)).pageWidth, 600);

      await prepare('reset');
      await assert.rejects(() => auditSettingsAtBottom(page), /position moved before the audit without an extent change/);
    } finally {
      await browser.close();
    }
  });
}
