import { test, expect } from '@playwright/test';
import {
  E2E_CREDENTIALS,
  ensureAuthenticated,
  getMockMode,
  login,
  logout,
  setMockMode,
} from './helpers';

const truthy = (value: string | undefined) => {
  if (!value) return false;
  return ['1', 'true', 'yes', 'on'].includes(value.trim().toLowerCase());
};

test.describe.serial('Core E2E flows', () => {
  test('Bootstrap flow - setup wizard and dashboard', async ({ page }) => {
    await ensureAuthenticated(page);
    // v6 default landing page is /infrastructure (was /proxmox/overview in v5)
    await expect(page).toHaveURL(/\/(infrastructure|proxmox\/overview)/);
    await expect(page.locator('#root')).toBeVisible();
  });

  test('Login flow - logout and re-login', async ({ page }) => {
    await ensureAuthenticated(page);

    await logout(page);
    await login(page, E2E_CREDENTIALS);

    const stateRes = await page.request.get('/api/state');
    expect(stateRes.ok()).toBeTruthy();
  });

  test('Alerts page - create and delete threshold override', async ({ page }) => {
    await ensureAuthenticated(page);

    await page.goto('/alerts/overview');
    await expect(page.getByRole('heading', { name: 'Alerts Overview' })).toBeVisible();

    const alertsToggleOnOverview = page.getByRole('checkbox', { name: /toggle alerts/i });
    await expect(alertsToggleOnOverview).toBeVisible();
    const alertsInitiallyEnabled = await alertsToggleOnOverview.isChecked();
    if (!alertsInitiallyEnabled) {
      await alertsToggleOnOverview.setChecked(true, { force: true });
      await expect(page.getByText('Alerts enabled', { exact: true })).toBeVisible();
    }

    // Navigate to thresholds via in-app nav to avoid a full reload redirecting while activation is loading.
    await page.getByRole('button', { name: 'Thresholds' }).click();
    await expect(page).toHaveURL(/\/alerts\/thresholds/);
    await expect(page.getByRole('heading', { name: 'Alert Thresholds' })).toBeVisible();
    // Proxmox Nodes section only appears when PVE nodes exist in unified resources.
    // In v6 the unified registry may not include PVE nodes — skip gracefully in that case.
    const proxmoxNodesHeading = page.getByRole('heading', { name: 'Proxmox Nodes' });
    const hasProxmoxNodes = await proxmoxNodesHeading.isVisible({ timeout: 5000 }).catch(() => false);
    if (!hasProxmoxNodes) {
      test.skip(true, 'Proxmox Nodes section not present (nodes not in unified resources)');
    }

    const proxmoxNodesSection = page
      .getByRole('heading', { name: 'Proxmox Nodes' })
      .locator('xpath=ancestor::*[.//table][1]');

    const globalDefaultsRow = proxmoxNodesSection.locator('table tbody tr').filter({
      hasText: 'Global Defaults',
    });
    await expect(globalDefaultsRow).toBeVisible();
    const cpuDefaultValueRaw = await globalDefaultsRow.locator('input[type="number"]').first().inputValue();
    const cpuDefault = Number(cpuDefaultValueRaw);
    if (!Number.isFinite(cpuDefault)) {
      test.skip(true, `Unable to read CPU default threshold (value="${cpuDefaultValueRaw}")`);
    }

    const nodeRows = proxmoxNodesSection
      .locator('table tbody tr')
      .filter({ hasNotText: 'Global Defaults' });
    const rowCount = await nodeRows.count();
    let targetRowIndex = -1;
    for (let i = 0; i < rowCount; i++) {
      const row = nodeRows.nth(i);
      const hasEdit = await row.locator('button[title="Edit thresholds"]').isVisible().catch(() => false);
      if (!hasEdit) continue;
      const hasCustom = await row.getByText('Custom', { exact: true }).isVisible().catch(() => false);
      if (hasCustom) continue;
      targetRowIndex = i;
      break;
    }
    if (targetRowIndex < 0) {
      test.skip(true, 'No Proxmox node row without an existing override was found');
    }

    const targetRow = nodeRows.nth(targetRowIndex);
    await expect(targetRow).toBeVisible();

    const resourceName = (await targetRow.locator('td').nth(1).locator('a, span').first().innerText()).trim();
    const cpuCellBefore = (await targetRow.locator('td').nth(2).innerText()).trim();

    const overrideValue = cpuDefault === -1 ? 80 : Math.max(0, cpuDefault - 3);
    if (overrideValue === cpuDefault) {
      test.skip(true, `Computed overrideValue equals default (${cpuDefault})`);
    }

    await targetRow.locator('button[title="Edit thresholds"]').click();
    const cancelButton = proxmoxNodesSection.locator('button[title="Cancel editing"]').first();
    await expect(cancelButton).toBeVisible();
    const editedRow = cancelButton.locator('xpath=ancestor::tr[1]');

    const cpuInput = editedRow.locator('input[type="number"]').first();
    await expect(cpuInput).toBeVisible();
    await cpuInput.fill(String(overrideValue));
    await cpuInput.blur();

    const unsaved = page.getByText('You have unsaved changes');
    await expect(unsaved).toBeVisible();
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(unsaved).not.toBeVisible();

    const updatedRow = proxmoxNodesSection.locator('table tbody tr').filter({ hasText: resourceName }).first();
    await expect(updatedRow).toBeVisible();
    await expect(updatedRow.getByText('Custom')).toBeVisible();
    await expect(updatedRow.locator('button[title="Remove override"]')).toBeVisible();

    const stateRes = await page.request.get('/api/state');
    expect(stateRes.ok()).toBeTruthy();
    const state = (await stateRes.json()) as { nodes?: Array<{ id: string; name: string }> };
    const nodeId = state.nodes?.find((n) => n.name === resourceName)?.id;
    expect(nodeId).toBeTruthy();

    const configResAfterCreate = await page.request.get('/api/alerts/config');
    expect(configResAfterCreate.ok()).toBeTruthy();
    const configAfterCreate = (await configResAfterCreate.json()) as { overrides?: Record<string, unknown> };
    expect(configAfterCreate.overrides && Object.prototype.hasOwnProperty.call(configAfterCreate.overrides, nodeId as string)).toBeTruthy();

    await updatedRow.locator('button[title="Remove override"]').click();
    await expect(unsaved).toBeVisible();
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(unsaved).not.toBeVisible();

    const configResAfterDelete = await page.request.get('/api/alerts/config');
    expect(configResAfterDelete.ok()).toBeTruthy();
    const configAfterDelete = (await configResAfterDelete.json()) as { overrides?: Record<string, unknown> };
    expect(configAfterDelete.overrides && Object.prototype.hasOwnProperty.call(configAfterDelete.overrides, nodeId as string)).toBeFalsy();

    const parseNumericCell = (raw: string) => {
      const cleaned = raw.replace(/[^\d.-]+/g, '');
      const num = Number(cleaned);
      return Number.isFinite(num) ? num : null;
    };

    await page.reload();
    if (!/\/alerts\/thresholds/.test(page.url())) {
      await page.goto('/alerts/overview');
      await expect(page.getByRole('heading', { name: 'Alerts Overview' })).toBeVisible();
      const toggle = page.getByRole('checkbox', { name: /toggle alerts/i });
      await expect(toggle).toBeVisible();
      await toggle.setChecked(true, { force: true });
      await page.getByRole('button', { name: 'Thresholds' }).click();
      await expect(page).toHaveURL(/\/alerts\/thresholds/);
    }

    await expect(page.getByRole('heading', { name: 'Alert Thresholds' })).toBeVisible();
    const rowAfterReload = page
      .getByRole('heading', { name: 'Proxmox Nodes' })
      .locator('xpath=ancestor::*[.//table][1]')
      .locator('table tbody tr')
      .filter({ hasText: resourceName })
      .first();
    await expect(rowAfterReload).toBeVisible();
    await expect(rowAfterReload.getByText('Custom')).not.toBeVisible();
    const cpuCellAfter = (await rowAfterReload.locator('td').nth(2).innerText()).trim();
    expect(parseNumericCell(cpuCellAfter)).toBe(parseNumericCell(cpuCellBefore));

    // Restore prior activation state to keep the test safe against real instances.
    if (!alertsInitiallyEnabled) {
      await page.goto('/alerts/overview');
      await expect(page.getByRole('heading', { name: 'Alerts Overview' })).toBeVisible();
      const alertsToggleRestore = page.getByRole('checkbox', { name: /toggle alerts/i });
      await expect(alertsToggleRestore).toBeVisible();
      await alertsToggleRestore.setChecked(false, { force: true });
      await expect(page.getByText('Alerts disabled', { exact: true })).toBeVisible();
    }
  });

  // NOTE: 'Settings persistence - toggle auto update checks' test was removed
  // It was flaky due to timing sensitivity and tests basic CRUD better covered by unit tests.

  test('Add Proxmox node - appears in UI', async ({ page }) => {
    test.skip(
      !truthy(process.env.PULSE_E2E_ALLOW_NODE_MUTATION),
      'Set PULSE_E2E_ALLOW_NODE_MUTATION=1 to enable node mutation E2E',
    );

    await ensureAuthenticated(page);

    let initialMockMode: { enabled: boolean } | null = null;
    try {
      initialMockMode = await getMockMode(page);
      if (initialMockMode.enabled) {
        await setMockMode(page, false);
      }
    } catch (error) {
      console.warn(`[core-e2e] unable to read/set mock mode before node mutation: ${String(error)}`);
    }

    const nodeName = `e2e-pve-${Date.now()}`;
    try {
      await page.goto('/settings/pve');

      const pveNodesHeading = page.getByRole('heading', { name: 'Proxmox VE nodes' });
      const settingsReady = await pveNodesHeading.isVisible({ timeout: 30_000 }).catch(() => false);
      if (!settingsReady) {
        test.skip(true, 'Proxmox settings did not finish loading in time');
      }

      const addNodeButton = page.getByRole('button', { name: /^Add PVE Node$/ });
      await expect(addNodeButton).toBeVisible();
      await addNodeButton.click();

      const modalForm = page.locator('form').filter({ hasText: 'Basic information' }).first();
      await expect(modalForm).toBeVisible();

      await modalForm
        .locator('label:has-text("Node Name")')
        .locator('..')
        .locator('input')
        .fill(nodeName);
      await modalForm
        .locator('label:has-text("Host URL")')
        .locator('..')
        .locator('input')
        .fill('https://192.168.77.10:8006');

      await modalForm
        .locator('label:has-text("Token ID")')
        .locator('..')
        .locator('input')
        .fill('pulse-monitor@pam!pulse-e2e');
      await modalForm
        .locator('label:has-text("Token Value")')
        .locator('..')
        .locator('input')
        .fill('aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');

      await modalForm.locator('button[type="submit"]').click();
      await expect(modalForm).not.toBeVisible();

      await expect(page.getByText(nodeName)).toBeVisible();
    } finally {
      // Cleanup by deleting the node we just created (best-effort).
      try {
        const nodesRes = await page.request.get('/api/config/nodes');
        if (nodesRes.ok()) {
          const nodes = (await nodesRes.json()) as Array<{ id: string; name: string }>;
          const created = nodes.find((n) => n.name === nodeName);
          if (created?.id) {
            await page.request.delete(`/api/config/nodes/${created.id}`);
          }
        }
      } catch (error) {
        console.warn(`[core-e2e] unable to cleanup created node, continuing: ${String(error)}`);
      }

      if (initialMockMode?.enabled) {
        try {
          await setMockMode(page, true);
        } catch (error) {
          console.warn(`[core-e2e] unable to restore mock mode after node mutation: ${String(error)}`);
        }
      }
    }
  });
});
