// Diagnostic only: proves a blank-page suspension control, not Pulse recovery.
// Run through pulse-heavy-run -- node scripts/check-browser-lifecycle-control.mjs
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { chromium } from '@playwright/test';

const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const browser = await chromium.launch({ headless: true });
try {
  const require = createRequire(import.meta.url);
  console.log(JSON.stringify({ browser: browser.version(),
    playwright: require('@playwright/test/package.json').version,
    node: process.version, platform: process.platform, arch: process.arch }));
  const results = [];
  // Preselected comparison; independent contexts prevent carry-over.
  for (const transition of [false, true]) {
    const context = await browser.newContext();
    try {
      const page = await context.newPage();
      await page.evaluate(() => {
        window.probe = { ticks: 0, events: [] };
        setInterval(() => window.probe.ticks++, 50);
        for (const type of ['freeze', 'resume', 'visibilitychange']) {
          document.addEventListener(type, () => window.probe.events.push({
            type, ticks: window.probe.ticks, visibility: document.visibilityState,
          }));
        }
      });
      const session = await context.newCDPSession(page);
      const commands = [];
      const send = async (method, params) => {
        commands.push({ method, params, response: await session.send(method, params) });
      };
      if (transition) await send('Emulation.setFocusEmulationEnabled', { enabled: true });
      await send('Emulation.setFocusEmulationEnabled', { enabled: false });
      await wait(250);
      const before = await page.evaluate(() => ({ ...window.probe,
        visibility: document.visibilityState, focus: document.hasFocus() }));
      await send('Page.setWebLifecycleState', { state: 'frozen' });
      // Do not evaluate the page while frozen. Wait on the host instead.
      await wait(2100);
      await send('Page.setWebLifecycleState', { state: 'active' });
      await wait(250);
      const after = await page.evaluate(() => ({ ...window.probe,
        visibility: document.visibilityState, focus: document.hasFocus() }));
      const freeze = after.events.find((event) => event.type === 'freeze');
      const resume = after.events.find((event) => event.type === 'resume');
      const valid = Boolean(freeze && resume && freeze.ticks === resume.ticks &&
        after.events.indexOf(freeze) < after.events.indexOf(resume) &&
        after.ticks > resume.ticks);
      const result = { transition, before, after, commands, valid };
      results.push(result);
      console.log(JSON.stringify(result));
    } finally {
      await context.close();
    }
  }
  assert.ok(results[1].valid, 'true-to-false intervention did not establish suspension');
} finally {
  await browser.close();
}
