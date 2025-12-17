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
    await expect(page).toHaveURL(/\/proxmox\/overview/);
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

    await page.goto('/alerts/thresholds/proxmox');
    await expect(page.getByRole('heading', { name: 'Alert Thresholds' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Proxmox Nodes' })).toBeVisible();

    const proxmoxNodesSection = page
      .getByRole('heading', { name: 'Proxmox Nodes' })
      .locator('xpath=ancestor::*[.//table][1]');

    const firstRow = proxmoxNodesSection.locator('table tbody tr').first();
    await expect(firstRow).toBeVisible();

    await firstRow.locator('button[title="Edit thresholds"]').click();
    const firstMetricInput = firstRow.locator('input[type="number"]').first();
    await expect(firstMetricInput).toBeVisible();
    await firstMetricInput.fill('77');
    await page.keyboard.press('Tab');

    const unsaved = page.getByText('You have unsaved changes');
    await expect(unsaved).toBeVisible();
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(unsaved).not.toBeVisible();

    await expect(firstRow.getByText('Custom')).toBeVisible();
    await expect(firstRow.locator('button[title="Remove override"]')).toBeVisible();

    await firstRow.locator('button[title="Remove override"]').click();
    await expect(unsaved).toBeVisible();
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(unsaved).not.toBeVisible();

    await expect(firstRow.getByText('Custom')).not.toBeVisible();
  });

  test('Settings persistence - toggle auto update checks', async ({ page }) => {
    await ensureAuthenticated(page);

    await page.goto('/settings/system-updates');
    await expect(page.getByRole('heading', { name: 'Updates' })).toBeVisible();

    const toggle = page.getByTestId('updates-auto-check-toggle');
    const initial = await toggle.isChecked();
    await toggle.setChecked(!initial, { force: true });

    const unsaved = page.getByText('Unsaved changes');
    await expect(unsaved).toBeVisible();
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(unsaved).not.toBeVisible();

    await page.reload();
    await expect(page.getByRole('heading', { name: 'Updates' })).toBeVisible();
    expect(await page.getByTestId('updates-auto-check-toggle').isChecked()).toBe(!initial);

    // Restore previous state to keep the test safe against real instances
    await page.getByTestId('updates-auto-check-toggle').setChecked(initial, { force: true });
    await expect(page.getByText('Unsaved changes')).toBeVisible();
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(page.getByText('Unsaved changes')).not.toBeVisible();
  });

  test('Add Proxmox node - appears in UI', async ({ page }) => {
    test.skip(
      !truthy(process.env.PULSE_E2E_ALLOW_NODE_MUTATION),
      'Set PULSE_E2E_ALLOW_NODE_MUTATION=1 to enable node mutation E2E',
    );

    await ensureAuthenticated(page);

    const initialMockMode = await getMockMode(page);
    if (initialMockMode.enabled) {
      await setMockMode(page, false);
    }

    const nodeName = `e2e-pve-${Date.now()}`;

    await page.goto('/settings/pve');
    await page.getByRole('button', { name: 'Add PVE Node' }).click();

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

    // Cleanup by deleting the node we just created (best-effort).
    const nodesRes = await page.request.get('/api/config/nodes');
    expect(nodesRes.ok()).toBeTruthy();
    const nodes = (await nodesRes.json()) as Array<{ id: string; name: string }>;
    const created = nodes.find((n) => n.name === nodeName);
    expect(created).toBeTruthy();
    if (created?.id) {
      const delRes = await page.request.delete(`/api/config/nodes/${created.id}`);
      expect(delRes.ok()).toBeTruthy();
    }

    if (initialMockMode.enabled) {
      await setMockMode(page, true);
    }
  });
});
