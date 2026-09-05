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

async function mobileDestinations(page: Page) {
  const nav = page.getByRole('navigation', { name: 'Mobile navigation' });
  await nav.getByRole('button', { name: 'More navigation', exact: true }).click();
  const more = page.getByRole('menu', { name: 'More navigation destinations' });
  await expect(more).toBeVisible();
  await expect(more.getByRole('menuitem', { name: /Settings/ })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(more).toBeHidden();
  await nav.getByRole('button', { name: 'More navigation', exact: true }).click();
  await more.getByRole('menuitem', { name: /Settings/ }).click();
  await expect(page).toHaveURL(/\/settings/);
  await expect(more).toBeHidden();
  for (const platform of ['Proxmox', 'Docker']) {
    await nav.locator('[data-tab-id="platform-switcher"]').click();
    const menu = page.getByRole('menu', { name: 'Switch platform', exact: true });
    await menu.getByRole('menuitem', { name: platform, exact: true }).click();
    await expect(menu).toBeHidden();
    await expect(page).toHaveURL(new RegExp('/' + platform.toLowerCase()));
    await expect(page.locator(platform === 'Docker'
      ? 'tr[data-docker-container-row]' : '[data-proxmox-host-row]').first()).toBeVisible();
  }
  await nav.locator('[data-tab-id="alerts"]').click();
  await expect(page.getByRole('button', { name: 'Acknowledge', exact: true }).first()).toBeEnabled();
  await nav.locator('[data-tab-id="platform-switcher"]').click();
  await page.getByRole('menu', { name: 'Switch platform', exact: true })
    .getByRole('menuitem', { name: 'Docker', exact: true }).click();
}

for (const admissionFailure of [false, true]) {
  for (const width of [1440, 1100, 390]) {
    test(`populated navigation survives socket loss at ${width}px (admission failure: ${admissionFailure})`, async ({ page, browser }, testInfo) => {
      test.skip(!enabled, 'Requires isolated mock backend and explicit qualification opt-in');
      test.setTimeout(120_000);
      await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
      let blocked = false;
      let failAdmission = false;
      let failedAdmissions = 0;
      await page.route('**/api/resources?*', async route => {
        const url = new URL(route.request().url());
        if (failAdmission && url.searchParams.get('page') === '1' && url.searchParams.get('limit') === '1') {
          failedAdmissions++;
          await route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"qualification admission unavailable"}' });
        } else {
          await route.continue();
        }
      });
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
      if (width === 390) await mobileDestinations(page);
      await capture('healthy');
      const documentIdentity = await page.evaluate(() => performance.timeOrigin);
      blocked = true;
      failAdmission = admissionFailure;
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
      if (width === 390) await mobileDestinations(page);
      const admissionResponse = admissionFailure
        ? page.waitForResponse(response => response.url().includes('/api/resources?page=1&limit=1') && response.status() === 503)
        : null;
      blocked = false;
      if (admissionResponse) await (await admissionResponse).finished();
      await expect(page.getByRole('status', { name: healthy })).toBeVisible({ timeout: 45_000 });
      if (admissionFailure) await expect.poll(() => failedAdmissions).toBeGreaterThan(0);
      expect.soft(await navigation(page)).toEqual(before);
      if (width === 390) await mobileDestinations(page);
      expect(await page.evaluate(() => performance.timeOrigin)).toBe(documentIdentity);
      await capture('recovered');
      if (width === 390 && !admissionFailure && process.env.PULSE_E2E_TABLE_ACCESS === '1') {
        const row = page.locator('tr[data-docker-container-row]').filter({ hasText: 'notification' }).first();
        const table = row.locator('xpath=ancestor::table');
        const wrapper = table.locator('..');
        const measure = () => wrapper.evaluate(el => ({
          width: el.clientWidth, scrollWidth: el.scrollWidth, left: el.scrollLeft,
          overflowX: getComputedStyle(el).overflowX, tabIndex: (el as HTMLElement).tabIndex,
          cells: Array.from(el.querySelector('tr[data-docker-container-row]')!.children).map(cell => ({
            text: cell.textContent, x: cell.getBoundingClientRect().x,
            width: cell.getBoundingClientRect().width,
            titles: Array.from(cell.querySelectorAll('[title]')).map(n => n.getAttribute('title')),
          })),
        }));
        await row.scrollIntoViewIfNeeded();
        const initial = await measure();
        // Text ranges catch clipped glyphs even when the wrapper itself has no
        // scrollable overflow. Check every rendered update state, not one row.
        await expect(table.locator('.docker-container-update-cell').filter({ hasText: 'Current' }).first()).toBeVisible();
        const clippedUpdateText = await table.locator('.docker-container-update-cell').evaluateAll(cells =>
          cells.flatMap(cell => {
            const bounds = cell.getBoundingClientRect();
            const walker = document.createTreeWalker(cell, NodeFilter.SHOW_TEXT);
            const clipped: string[] = [];
            while (walker.nextNode()) {
              if (!walker.currentNode.textContent?.trim()) continue;
              const range = document.createRange();
              range.selectNodeContents(walker.currentNode);
              if (Array.from(range.getClientRects()).some(rect =>
                rect.left < bounds.left - 1 || rect.right > bounds.right + 1)) {
                clipped.push(walker.currentNode.textContent!);
              }
            }
            return clipped;
          }),
        );
        expect(clippedUpdateText).toEqual([]);
        const currentLabel = table.locator('.docker-container-update-cell span').filter({ hasText: /^Current$/ }).last();
        expect(await currentLabel.evaluate(el => {
          const range = document.createRange();
          range.selectNodeContents(el);
          return range.getClientRects().length;
        })).toBe(1);

        expect(initial.overflowX).toBe('clip');
        expect(initial.scrollWidth).toBe(initial.width);

        await row.hover();
        await page.mouse.wheel(2000, 0);
        await page.waitForTimeout(350);
        const pointer = await measure();
        await capture('table-after-horizontal-wheel');
        await wrapper.evaluate(el => { el.scrollLeft = 0; });
        // Start on a real focusable descendant, without adding tabindex to the UI.
        const toggle = row.getByRole('button').first();
        await toggle.focus();
        await page.keyboard.press('ArrowRight');
        await page.waitForTimeout(350);
        const keyboard = await measure();
        const accessibleRow = await row.ariaSnapshot();
        await page.keyboard.press('Enter');
        await capture('table-keyboard-detail');
        const expanded = await toggle.getAttribute('aria-expanded');
        await expect(toggle).toHaveAttribute('aria-expanded', 'true');
        const detailId = await toggle.getAttribute('aria-controls');
        const detailText = detailId ? await page.locator(`[id="${detailId}"]`).innerText() : null;
        const heading = page.locator(`[id="${detailId}"] h2`).first();
        await expect(heading).toContainText('notification');
        expect(await heading.evaluate(el => el.scrollWidth <= el.clientWidth &&
          getComputedStyle(el).textOverflow !== 'ellipsis')).toBe(true);
        await heading.scrollIntoViewIfNeeded();
        await capture('table-full-detail-heading');

        await testInfo.attach('narrow-table-access', {
          body: JSON.stringify({ initial, pointer, keyboard, accessibleRow, expanded, detailText }, null, 2),
          contentType: 'application/json',
        });
      }

      await testInfo.attach('environment', { body: JSON.stringify({ browser: browser.version(), width, height: width === 390 ? 844 : 900, zoom: 1, admissionFailure, failedAdmissions, before }), contentType: 'application/json' });
    });
  }
}
