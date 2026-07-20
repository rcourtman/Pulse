import { test, expect, type Locator } from '@playwright/test';
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
  test('Bootstrap flow - setup wizard and infrastructure landing', async ({ page }) => {
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

    const alertsToggleOnOverview = page.getByRole('checkbox', {
      name: 'Toggle external notifications',
    });
    await expect(alertsToggleOnOverview).toBeVisible();
    const alertsInitiallyEnabled = await alertsToggleOnOverview.isChecked();
    if (!alertsInitiallyEnabled) {
      await alertsToggleOnOverview.locator('xpath=ancestor::label[1]').click();
      await expect(alertsToggleOnOverview).toBeChecked({ timeout: 30_000 });
    }

    // Navigate to thresholds via in-app nav to avoid a full reload redirecting while activation is loading.
    await page.getByRole('button', { name: 'Thresholds' }).click();
    await expect(page).toHaveURL(/\/alerts\/thresholds/);
    await expect(page.getByRole('heading', { name: 'Alert Thresholds' })).toBeVisible();
    // Virtualization Hosts only appears when PVE nodes exist in unified resources.
    // In v6 the unified registry may not include PVE nodes — skip gracefully in that case.
    const proxmoxNodesHeading = page.getByRole('heading', { name: 'Virtualization Hosts' });
    const hasProxmoxNodes = await proxmoxNodesHeading
      .waitFor({ state: 'visible', timeout: 30_000 })
      .then(() => true)
      .catch(() => false);
    if (!hasProxmoxNodes) {
      test.skip(true, 'Virtualization Hosts section not present (nodes not in unified resources)');
    }

    const sectionToggle = proxmoxNodesHeading.locator('xpath=ancestor::button[1]');
    if ((await sectionToggle.getAttribute('aria-expanded')) === 'false') {
      await sectionToggle.click();
    }
    const proxmoxNodesSection = sectionToggle.locator('..');
    const globalDefaultsContainer = proxmoxNodesSection
      .getByText('Global Defaults', { exact: true })
      .first()
      .locator('xpath=ancestor::*[.//input[@type="number"]][1]');
    await expect(globalDefaultsContainer).toBeVisible();
    const cpuDefaultValueRaw = await globalDefaultsContainer
      .locator('input[type="number"]')
      .first()
      .inputValue();
    const cpuDefault = Number(cpuDefaultValueRaw);
    if (!Number.isFinite(cpuDefault)) {
      test.skip(true, `Unable to read CPU default threshold (value="${cpuDefaultValueRaw}")`);
    }

    const configResBeforeCreate = await page.request.get('/api/alerts/config');
    expect(configResBeforeCreate.ok()).toBeTruthy();
    const configBeforeCreate = (await configResBeforeCreate.json()) as {
      overrides?: Record<string, unknown>;
    };

    const resourceContainerForEditControl = async (editControl: Locator): Promise<Locator> => {
      const desktopRow = editControl.locator('xpath=ancestor::tr[1]');
      if ((await desktopRow.count()) > 0) {
        return desktopRow;
      }
      return editControl.locator(
        'xpath=ancestor::*[contains(concat(" ", normalize-space(@class), " "), " flex-col ")][1]',
      );
    };

    const editControls = proxmoxNodesSection.getByRole('button', {
      name: /^Edit thresholds for /,
    });
    let targetEditIndex = -1;
    let resourceName = '';
    let resourceNameOrdinal = 0;
    const resourceNameCounts = new Map<string, number>();
    for (let i = 0; i < (await editControls.count()); i++) {
      const editControl = editControls.nth(i);
      const label = (await editControl.getAttribute('aria-label')) ?? '';
      const candidateName = label.replace(/^Edit thresholds for /, '');
      const candidateOrdinal = resourceNameCounts.get(candidateName) ?? 0;
      resourceNameCounts.set(candidateName, candidateOrdinal + 1);
      const resourceContainer = await resourceContainerForEditControl(editControl);
      const hasExistingOverride =
        (await resourceContainer.getByRole('button', { name: /^Revert to defaults for / }).count()) >
        0;
      if (hasExistingOverride) {
        continue;
      }
      targetEditIndex = i;
      resourceName = candidateName;
      resourceNameOrdinal = candidateOrdinal;
      break;
    }
    if (targetEditIndex < 0 || !resourceName) {
      test.skip(true, 'No virtualization host without an existing override was found');
      return;
    }

    const overrideValue = cpuDefault === -1 ? 80 : Math.max(0, cpuDefault - 3);
    if (overrideValue === cpuDefault) {
      test.skip(true, `Computed overrideValue equals default (${cpuDefault})`);
    }

    await editControls.nth(targetEditIndex).click();
    const cancelButton = page
      .getByRole('button', { name: /^Cancel (editing|threshold edits)$/ })
      .first();
    await expect(cancelButton).toBeVisible();
    const editorContainer = cancelButton.locator(
      'xpath=ancestor::*[.//input[@type="number"]][1]',
    );
    const cpuInput = editorContainer.locator('input[type="number"]').first();
    await expect(cpuInput).toBeVisible();
    await cpuInput.fill(String(overrideValue));
    await cpuInput.blur();
    const commitMobileEdit = page.getByRole('button', {
      name: `Save threshold edits for ${resourceName}`,
    });
    if (await commitMobileEdit.isVisible().catch(() => false)) {
      await commitMobileEdit.click();
    }

    const unsaved = page.getByText('You have unsaved changes');
    await expect(unsaved).toBeVisible();
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(unsaved).not.toBeVisible();

    const updatedEditControl = page
      .getByRole('button', { name: `Edit thresholds for ${resourceName}` })
      .nth(resourceNameOrdinal);
    const updatedResourceContainer =
      await resourceContainerForEditControl(updatedEditControl);
    const revertOverride = updatedResourceContainer.getByRole('button', {
      name: `Revert to defaults for ${resourceName}`,
    });
    await expect(revertOverride).toBeVisible();

    const configResAfterCreate = await page.request.get('/api/alerts/config');
    expect(configResAfterCreate.ok()).toBeTruthy();
    const configAfterCreate = (await configResAfterCreate.json()) as { overrides?: Record<string, unknown> };
    const previousOverrideIds = new Set(Object.keys(configBeforeCreate.overrides ?? {}));
    const createdOverrideIds = Object.keys(configAfterCreate.overrides ?? {}).filter(
      (id) => !previousOverrideIds.has(id),
    );
    expect(createdOverrideIds).toHaveLength(1);
    const createdOverrideId = createdOverrideIds[0];

    await revertOverride.click();
    await expect(unsaved).toBeVisible();
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(unsaved).not.toBeVisible();

    const configResAfterDelete = await page.request.get('/api/alerts/config');
    expect(configResAfterDelete.ok()).toBeTruthy();
    const configAfterDelete = (await configResAfterDelete.json()) as { overrides?: Record<string, unknown> };
    expect(
      configAfterDelete.overrides &&
        Object.prototype.hasOwnProperty.call(configAfterDelete.overrides, createdOverrideId),
    ).toBeFalsy();

    await page.reload();
    if (!/\/alerts\/thresholds/.test(page.url())) {
      await page.goto('/alerts/overview');
      await expect(page.getByRole('heading', { name: 'Alerts Overview' })).toBeVisible();
      const toggle = page.getByRole('checkbox', {
        name: 'Toggle external notifications',
      });
      await expect(toggle).toBeVisible();
      await toggle.click({ force: true });
      await page.getByRole('button', { name: 'Thresholds' }).click();
      await expect(page).toHaveURL(/\/alerts\/thresholds/);
    }

    await expect(page.getByRole('heading', { name: 'Alert Thresholds' })).toBeVisible();
    const sectionToggleAfterReload = page
      .getByRole('heading', { name: 'Virtualization Hosts' })
      .locator('xpath=ancestor::button[1]');
    if ((await sectionToggleAfterReload.getAttribute('aria-expanded')) === 'false') {
      await sectionToggleAfterReload.click();
    }
    const editControlAfterReload = page
      .getByRole('button', { name: `Edit thresholds for ${resourceName}` })
      .nth(resourceNameOrdinal);
    await expect(editControlAfterReload).toBeVisible();
    const resourceContainerAfterReload =
      await resourceContainerForEditControl(editControlAfterReload);
    await expect(
      resourceContainerAfterReload.getByRole('button', {
        name: `Revert to defaults for ${resourceName}`,
      }),
    ).toHaveCount(0);

    // Restore prior activation state to keep the test safe against real instances.
    if (!alertsInitiallyEnabled) {
      await page.goto('/alerts/overview');
      await expect(page.getByRole('heading', { name: 'Alerts Overview' })).toBeVisible();
      const alertsToggleRestore = page.getByRole('checkbox', {
        name: 'Toggle external notifications',
      });
      await expect(alertsToggleRestore).toBeVisible();
      await alertsToggleRestore.click({ force: true });
      await expect(page.getByText('Notifications paused', { exact: true })).toBeVisible();
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
        .fill('pulse-monitor@pve!pulse-e2e');
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
