import { expect, test, type Page } from '@playwright/test';

import { ensureAuthenticated } from './helpers';

const DESKTOP_VIEWPORT = { width: 1280, height: 900 };
const RECOVERY_SUBJECT_LABEL = 'Archive VM For Production Ledger Services';

const mockRecoveryData = {
  rollups: {
    data: [
      {
        rollupId: 'res:vm-archive-01',
        subjectResourceId: 'vm-archive-01',
        subjectRef: {
          type: 'proxmox-vm',
          namespace: 'prod',
          name: 'archive-ledger',
          id: 'vm-archive-01',
          class: 'cluster-a',
        },
        display: {
          subjectLabel: RECOVERY_SUBJECT_LABEL,
        },
        lastAttemptAt: '2026-03-24T04:03:04Z',
        lastSuccessAt: '2026-03-24T04:03:04Z',
        lastOutcome: 'success',
        providers: ['proxmox-pbs'],
      },
    ],
    meta: { page: 1, limit: 500, total: 1, totalPages: 1 },
  },
  points: {
    data: [
      {
        id: 'pbs-backup:archive-ledger-01',
        provider: 'proxmox-pbs',
        kind: 'backup',
        mode: 'remote',
        outcome: 'success',
        startedAt: '2026-03-24T04:02:12Z',
        completedAt: '2026-03-24T04:03:04Z',
        sizeBytes: 30546730222,
        verified: true,
        immutable: false,
        encrypted: true,
        entityId: '201',
        subjectResourceId: 'vm-archive-01',
        subjectRef: {
          type: 'proxmox-vm',
          namespace: 'prod',
          name: 'archive-ledger',
          id: 'vm-archive-01',
          class: 'cluster-a',
        },
        repositoryRef: {
          type: 'proxmox-pbs-datastore',
          namespace: 'pbs-prod',
          name: 'vault-main',
          class: 'cluster-a',
        },
        details: {
          summary: 'Nightly immutable backup retained for compliance validation.',
        },
        display: {
          subjectLabel: RECOVERY_SUBJECT_LABEL,
          subjectType: 'proxmox-vm',
          repositoryLabel: 'pbs-prod/vault-main',
          detailsSummary: 'Nightly immutable backup retained for compliance validation.',
        },
      },
    ],
    meta: { page: 1, limit: 500, total: 1, totalPages: 1 },
  },
  facets: {
    data: {
      clusters: [],
      nodesHosts: [],
      namespaces: [],
      hasSize: true,
      hasVerification: true,
      hasEntityId: false,
    },
  },
  series: {
    data: [{ day: '2026-03-24', total: 1, snapshot: 0, local: 0, remote: 1 }],
  },
};

async function mockRecoveryEndpoints(page: Page): Promise<void> {
  await page.route('**/api/recovery/rollups*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockRecoveryData.rollups),
    });
  });

  await page.route('**/api/recovery/points*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockRecoveryData.points),
    });
  });

  await page.route('**/api/recovery/facets*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockRecoveryData.facets),
    });
  });

  await page.route('**/api/recovery/series*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockRecoveryData.series),
    });
  });
}

test.describe('Recovery desktop layout guards', () => {
  test('history table keeps outcome visible within the desktop wrapper', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only recovery layout coverage');

    await page.setViewportSize(DESKTOP_VIEWPORT);
    await ensureAuthenticated(page);
    await mockRecoveryEndpoints(page);

    await page.goto('/recovery', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('recovery-page')).toBeVisible();

    const protectedRow = page.locator('[data-testid="recovery-page"] table tbody tr').first();
    await expect(protectedRow).toBeVisible();
    await protectedRow.click();

    await expect(page.getByText('Focused', { exact: true })).toBeVisible();
    await expect(page.getByText(/Showing 1 - 1 of 1 recovery points/i)).toBeVisible();

    const historyWrapper = page
      .locator('[data-testid="recovery-page"] div.overflow-x-auto')
      .filter({ has: page.locator('table') })
      .last();
    await expect(historyWrapper).toBeVisible();

    const overflowMetrics = await historyWrapper.evaluate((el) => {
      const wrapper = el as HTMLElement;
      const table = wrapper.querySelector('table') as HTMLElement | null;
      const style = window.getComputedStyle(wrapper);
      return {
        overflowX: style.overflowX,
        wrapperClientWidth: wrapper.clientWidth,
        wrapperScrollWidth: wrapper.scrollWidth,
        tableScrollWidth: table?.scrollWidth ?? 0,
      };
    });

    expect(['auto', 'scroll']).toContain(overflowMetrics.overflowX);
    expect(
      overflowMetrics.wrapperScrollWidth,
      `Recovery history wrapper should fit the default desktop column set without horizontal scrolling (wrapper=${overflowMetrics.wrapperClientWidth}, table=${overflowMetrics.tableScrollWidth})`,
    ).toBeLessThanOrEqual(overflowMetrics.wrapperClientWidth + 1);

    const outcomeHeader = historyWrapper.locator('th').filter({ hasText: /^Outcome$/ }).first();
    await expect(outcomeHeader).toBeVisible();

    const wrapperBox = await historyWrapper.boundingBox();
    const outcomeBox = await outcomeHeader.boundingBox();

    expect(wrapperBox, 'Expected recovery history wrapper bounds').toBeTruthy();
    expect(outcomeBox, 'Expected outcome column header bounds').toBeTruthy();

    const wrapperRight = (wrapperBox as { x: number; width: number }).x +
      (wrapperBox as { width: number }).width;
    const outcomeRight = (outcomeBox as { x: number; width: number }).x +
      (outcomeBox as { width: number }).width;
    expect(
      outcomeRight,
      `Recovery outcome header should stay inside the visible desktop wrapper (wrapperRight=${wrapperRight}, outcomeRight=${outcomeRight})`,
    ).toBeLessThanOrEqual(wrapperRight + 1);
  });
});
