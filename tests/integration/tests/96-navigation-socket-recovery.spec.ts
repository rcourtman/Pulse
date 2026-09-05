import { expect, test, type Page, type WebSocketRoute } from '@playwright/test';
import { ensureAuthenticated } from './helpers';

// Opt-in: uses synthetic inventory in an isolated real backend. Never interrupts
// a shared runtime or substitutes the application's websocket store.
const enabled = process.env.PULSE_E2E_NAVIGATION_RECOVERY === '1' &&
  process.env.PULSE_E2E_USE_LOCAL_BACKEND === '1';
const healthy = 'Backend and live data stream are connected.';

async function navigation(page: Page) {
  return page.locator('[aria-label="Primary navigation"]').evaluateAll(nodes =>
    nodes.map(node => ({
      visible: node.getBoundingClientRect().height > 0,
      tabs: Array.from(node.querySelectorAll('.tab')).map(tab => ({
        label: tab.getAttribute('aria-label')?.replace(/^\d+ /, '').split(':')[0],
        // Counts may change independently while the socket recovers.
        text: (tab as HTMLElement).innerText.replace(/\d+/g, '').replace(/\s+/g, ' ').trim(),
        icons: tab.querySelectorAll('svg').length,
        visible: tab.getBoundingClientRect().height > 0,
      })),
    })),
  );
}

for (const width of [1440, 1100, 390]) {
  test(`populated navigation survives socket loss at ${width}px`, async ({ page, browser }, testInfo) => {
    test.skip(!enabled, 'Requires isolated mock backend and explicit qualification opt-in');
    test.setTimeout(120_000);
    await page.setViewportSize({ width, height: 900 });
    let blocked = false;
    const sockets: WebSocketRoute[] = [];
    await page.routeWebSocket('**/ws*', async socket => {
      if (blocked) {
        await socket.close({ code: 1013, reason: 'qualification interruption' });
        return;
      }
      socket.connectToServer();
      sockets.push(socket);
    });
    await ensureAuthenticated(page);
    await page.goto('/proxmox');
    await expect(page.locator('[data-proxmox-host-row]').first()).toBeVisible({ timeout: 60_000 });
    await expect(page.locator('tr[data-guest-id]').first()).toBeVisible();
    await expect(page.getByRole('status', { name: healthy })).toBeVisible();
    // Verify both platform inventories before interrupting the live connection.
    await page.goto('/docker');
    await expect(page.locator('tr[data-docker-host-row]').first()).toBeVisible();
    await expect(page.locator('tr[data-docker-container-row]').first()).toBeVisible();
    await expect(page.getByRole('status', { name: healthy })).toBeVisible();
    const before = await navigation(page);
    expect(before).toHaveLength(1);
    expect(before[0].visible).toBe(width >= 1280);
    expect(before[0].tabs.map(tab => tab.label)).toEqual(expect.arrayContaining(['Proxmox', 'Docker']));
    const capture = async (stage: string) => {
      await testInfo.attach(stage, { body: await page.screenshot({ path: testInfo.outputPath(`${stage}.png`) }), contentType: 'image/png' });
    };
    await capture('healthy');
    const documentIdentity = await page.evaluate(() => performance.timeOrigin);
    blocked = true;
    for (const socket of sockets) await socket.close({ code: 1013, reason: 'qualification interruption' });
    await expect(page.getByRole('status', { name: 'Backend is healthy. Live updates are reconnecting.' })).toBeVisible({ timeout: 30_000 });
    expect((await page.request.get('/api/health')).ok()).toBeTruthy();
    await capture('reconnecting');
    expect.soft(await navigation(page)).toEqual(before);
    await expect(page.locator('tr[data-docker-container-row]').first()).toBeVisible();
    if (width >= 1280) {
      await page.locator('[aria-label="Primary navigation"]').getByRole('link', { name: /Alerts/ }).click();
      await expect(page.getByRole('button', { name: 'Acknowledge', exact: true }).first()).toBeEnabled();
      await capture('active-incidents-during-reconnect');
    }
    blocked = false;
    await expect(page.getByRole('status', { name: healthy })).toBeVisible({ timeout: 45_000 });
    expect.soft(await navigation(page)).toEqual(before);
    expect(await page.evaluate(() => performance.timeOrigin)).toBe(documentIdentity);
    await capture('recovered');
    await testInfo.attach('environment', { body: JSON.stringify({ browser: browser.version(), width, height: 900, zoom: 1, before }), contentType: 'application/json' });
  });
}
