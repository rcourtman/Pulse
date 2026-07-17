import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-resource-history-drawer.png';
const HOST_NAME = 'esxi-01.lab.local';

type WorkerFixtures = {
  authStorageStatePath: string;
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [async ({ browser }, use, workerInfo) => {
    const storageStatePath = path.resolve(
      __dirname,
      '..',
      '..',
      'tmp',
      'playwright-auth',
      `vmware-resource-history-drawer-${workerInfo.project.name}.json`,
    );
    fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
    await createAuthenticatedStorageState(browser, storageStatePath);
    try {
      await use(storageStatePath);
    } finally {
      fs.rmSync(storageStatePath, { force: true });
    }
  }, { scope: 'worker' }],
});

const vmwareActivityChange = (resourceId: string) => ({
  id: 'vmware-activity-1',
  observedAt: '2026-03-30T18:16:00Z',
  occurredAt: '2026-03-30T18:15:30Z',
  resourceId,
  kind: 'activity',
  sourceType: 'platform_event',
  sourceAdapter: 'vmware_adapter',
  confidence: 'high',
  actor: 'vCenter task: root@vsphere.local',
  reason: 'Enter maintenance mode (success)',
  metadata: {
    taskId: 'task-2049',
    taskState: 'success',
    entityType: 'HostSystem',
  },
});

const heuristicAlertChange = (resourceId: string) => ({
  id: 'heuristic-alert-1',
  observedAt: '2026-03-30T18:14:00Z',
  resourceId,
  kind: 'alert_fired',
  sourceType: 'heuristic',
  confidence: 'medium',
  reason: 'Pulse inferred elevated host risk from alarm churn',
  metadata: {
    incidentCategory: 'health',
  },
});

const unfilteredFacetBundle = (resourceId: string) => ({
  recentChanges: [vmwareActivityChange(resourceId), heuristicAlertChange(resourceId)],
  counts: {
    recentChanges: 2,
    recentChangeKinds: {
      activity: 1,
      alert_fired: 1,
    },
    recentChangeSourceTypes: {
      platform_event: 1,
      heuristic: 1,
    },
    recentChangeSourceAdapters: {
      vmware_adapter: 1,
    },
  },
});

const filteredFacetBundle = (resourceId: string) => ({
  recentChanges: [vmwareActivityChange(resourceId)],
  counts: {
    recentChanges: 1,
    recentChangeKinds: {
      activity: 1,
    },
    recentChangeSourceTypes: {
      platform_event: 1,
    },
    recentChangeSourceAdapters: {
      vmware_adapter: 1,
    },
  },
});

const actionAuditBundle = (resourceId: string) => ({
  available: true,
  count: 2,
  resourceId,
  audits: [
    {
      id: 'vmware-action-verified',
      createdAt: '2026-03-30T18:10:00Z',
      updatedAt: '2026-03-30T18:12:00Z',
      state: 'completed',
      request: {
        requestId: 'vmware-req-verified',
        resourceId,
        capabilityName: 'enter_maintenance_mode',
        reason: 'Place host in maintenance after alarm review',
        requestedBy: 'pulse_patrol',
      },
      plan: {
        actionId: 'vmware-action-verified',
        requestId: 'vmware-req-verified',
        allowed: true,
        requiresApproval: true,
        approvalPolicy: 'admin',
        rollbackAvailable: true,
      },
      result: {
        success: true,
        output: 'Maintenance mode requested',
        actionResultV2: {
          version: 2,
          execution: {
            status: 'succeeded',
            summary: 'Maintenance mode requested',
          },
          verification: {
            status: 'confirmed',
            evidenceClass: 'independent',
            summary: 'vCenter reported the host entering maintenance mode.',
          },
          compensation: {
            support: 'unavailable',
            status: 'not_available',
          },
        },
      },
    },
    {
      id: 'vmware-action-refused',
      createdAt: '2026-03-30T18:13:00Z',
      updatedAt: '2026-03-30T18:13:15Z',
      state: 'failed',
      request: {
        requestId: 'vmware-req-refused',
        resourceId,
        capabilityName: 'restart_host_agent',
        reason: 'Patrol proposed remediation while the host was locked',
        requestedBy: 'pulse_patrol',
      },
      plan: {
        actionId: 'vmware-action-refused',
        requestId: 'vmware-req-refused',
        allowed: true,
        requiresApproval: true,
        approvalPolicy: 'admin',
        rollbackAvailable: false,
      },
      result: {
        success: false,
        errorMessage: 'resource_remediation_locked: operator lock is active',
      },
      verificationOutcome: {
        status: 'unverified',
        evidenceSummary: 'No dispatch occurred, so no verification probe ran.',
      },
    },
  ],
});

// The vSphere hosts table opens ESXi host details inline through the shared
// table-row drawer presentation. Since the platform-first IA, that drawer's
// History disclosure is the only surface for per-resource change history and
// action audits, and its remote reads are fetch-on-expand: nothing hits
// /api/resources/:id/facets or /api/audit/actions until the user opens the
// disclosure. The host row itself comes from the live mock backend (VMware
// fixture inventory); the drawer's REST-only history reads are stubbed so the
// assertions stay deterministic.
test.describe('VMware resource history drawer', () => {
  test.setTimeout(180_000);

  test('starts remote history on disclosure expand and filters VMware activity', async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith('mobile-'),
      'Desktop-only platform table drawer coverage',
    );

    const facetRequestUrls: string[] = [];
    const actionAuditRequestUrls: string[] = [];
    let unexpectedVmwareApiCall: string | null = null;
    let stubbedResourceId: string | null = null;

    await page.route('**/api/vmware/**', async (route) => {
      unexpectedVmwareApiCall = route.request().url();
      await route.abort();
    });

    await page.route('**/api/resources/*/facets**', async (route) => {
      const requestUrl = new URL(route.request().url());
      facetRequestUrls.push(requestUrl.toString());
      const encodedId = requestUrl.pathname.split('/').at(-2) ?? '';
      const resourceId = decodeURIComponent(encodedId);
      stubbedResourceId = stubbedResourceId ?? resourceId;
      const bundle =
        requestUrl.searchParams.get('kind') === 'activity' &&
        requestUrl.searchParams.get('sourceAdapter') === 'vmware_adapter'
          ? filteredFacetBundle(resourceId)
          : unfilteredFacetBundle(resourceId);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(bundle),
      });
    });

    await page.route('**/api/audit/actions**', async (route) => {
      const requestUrl = new URL(route.request().url());
      actionAuditRequestUrls.push(requestUrl.toString());
      const resourceId = requestUrl.searchParams.get('resourceId') ?? '';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          resourceId
            ? actionAuditBundle(resourceId)
            : { available: true, count: 0, audits: [] },
        ),
      });
    });

    await page.route('**/api/ai/intelligence**', async (route) => {
      const requestUrl = new URL(route.request().url());
      const resourceId = requestUrl.searchParams.get('resource_id') ?? '';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          resource_id: resourceId,
          resource_name: HOST_NAME,
          resource_type: 'agent',
          health: {
            score: 81,
            grade: 'B',
            trend: 'stable',
            factors: [],
            prediction: 'VMware host activity is readable through shared history.',
          },
          dependencies: [],
          dependents: [],
          correlations: [],
          recent_changes: [],
          note_count: 0,
        }),
      });
    });

    await page.goto('/vmware', { waitUntil: 'domcontentloaded' });

    await expect(page.getByTestId('vmware-page')).toBeVisible();
    const hostRow = page.locator('tr').filter({ hasText: HOST_NAME }).first();
    await expect(hostRow).toBeVisible();
    await hostRow
      .getByRole('button', { name: `Expand details for ${HOST_NAME}` })
      .click();

    const detailRow = page.locator('[data-inline-platform-resource-detail-for]').first();
    await expect(detailRow).toBeVisible();

    // Fetch-on-expand pin: opening the row alone must not start any remote
    // history, intelligence, or action-audit reads.
    const historyDisclosure = detailRow.getByTestId('resource-row-history-disclosure');
    await expect(historyDisclosure).toBeVisible();
    await expect(detailRow.getByTestId('resource-change-history-section')).toHaveCount(0);
    await expect(detailRow.getByTestId('resource-action-history-section')).toHaveCount(0);
    expect(facetRequestUrls).toHaveLength(0);
    expect(actionAuditRequestUrls).toHaveLength(0);

    await historyDisclosure.getByRole('button', { name: 'Show history' }).click();

    const historySection = detailRow.getByTestId('resource-change-history-section');
    await expect(historySection).toBeVisible();
    await expect(historySection.getByText('Enter maintenance mode (success)')).toBeVisible();
    await expect(
      historySection.getByText('Pulse inferred elevated host risk from alarm churn'),
    ).toBeVisible();
    await expect(historySection.getByText('Activity', { exact: true })).toBeVisible();
    await expect(historySection.getByText('VMware adapter', { exact: true })).toBeVisible();

    const actionHistorySection = detailRow.getByTestId('resource-action-history-section');
    await expect(actionHistorySection).toBeVisible();
    await expect(actionHistorySection.getByText('Actions 2')).toBeVisible();
    await expect(
      actionHistorySection.getByText('Confirmed by independent observer'),
    ).toBeVisible();
    await expect(
      actionHistorySection.getByText('vCenter reported the host entering maintenance mode.'),
    ).toBeVisible();
    await expect(actionHistorySection.getByText('Refused', { exact: true })).toBeVisible();
    await expect(actionHistorySection.getByText('Execution refused')).toBeVisible();
    await expect(actionHistorySection.getByText('Resource remediation locked')).toBeVisible();
    await expect(
      actionHistorySection.getByText(
        'Pulse refused the action before dispatch because this resource is locked against automatic remediation.',
      ),
    ).toBeVisible();
    await expect(actionHistorySection.getByText('Verification not confirmed')).toBeVisible();
    await expect(actionHistorySection.getByText('resource_remediation_locked:')).toHaveCount(0);

    await historySection.getByRole('button', { name: 'Filter history' }).click();
    await historySection.getByLabel('Change kind').selectOption({ label: 'Activity' });
    await historySection.getByLabel('Source adapter').selectOption({ label: 'VMware adapter' });

    await expect(detailRow.getByText('Filtered changes loaded')).toBeVisible();
    await expect(historySection.getByText('Enter maintenance mode (success)')).toBeVisible();
    await expect(
      historySection.getByText('Pulse inferred elevated host risk from alarm churn'),
    ).toHaveCount(0);
    await expect(historySection.getByText('Change filters active')).toBeVisible();

    await expect
      .poll(() =>
        facetRequestUrls.some((url) => {
          const parsed = new URL(url);
          return (
            parsed.searchParams.get('limit') === '25' &&
            parsed.searchParams.get('kind') === 'activity' &&
            parsed.searchParams.get('sourceAdapter') === 'vmware_adapter'
          );
        }),
      )
      .toBe(true);

    expect(unexpectedVmwareApiCall).toBeNull();
    expect(stubbedResourceId).not.toBeNull();
    await expect
      .poll(() =>
        actionAuditRequestUrls.some((url) => {
          const parsed = new URL(url);
          return (
            parsed.pathname === '/api/audit/actions' &&
            parsed.searchParams.get('resourceId') === stubbedResourceId &&
            parsed.searchParams.get('limit') === '5'
          );
        }),
      )
      .toBe(true);

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
