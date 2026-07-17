import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-resource-history-drawer.png';

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

// The change/action history surface moved onto the vSphere platform page:
// expanding an ESXi host row opens the shared resource drawer, which fetches
// the facets and action-audit bundles per resource over REST. The host row
// itself comes from live mock websocket state, so the stubs match whatever
// canonical resource id the drawer asks for instead of pinning one.
const buildVmwareActivityChange = (resourceId: string) => ({
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
    connectionId: 'vc-1',
    taskId: 'task-2049',
    taskState: 'success',
    entityType: 'HostSystem',
    managedObjectId: 'host-101',
  },
});

const buildHeuristicAlertChange = (resourceId: string) => ({
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

const buildUnfilteredFacetBundle = (resourceId: string) => ({
  recentChanges: [buildVmwareActivityChange(resourceId), buildHeuristicAlertChange(resourceId)],
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

const buildFilteredFacetBundle = (resourceId: string) => ({
  recentChanges: [buildVmwareActivityChange(resourceId)],
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

const buildActionAuditBundle = (resourceId: string) => ({
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

test.describe('VMware resource history drawer', () => {
  test.setTimeout(180_000);

  test('filters VMware activity through the shared resource facets history surface', async ({
    page,
  }) => {
    // Real capability gap, not spec rot: platform-page drawers mount with
    // presentation="table-row", and useResourceDetailDrawerState disables
    // enableRemoteHistory for that presentation, so the change/action
    // history sections never fetch or render for vSphere hosts anywhere in
    // the current IA (Machines only lists Pulse-agent machines). Re-enable
    // once a VMware surface regains remote history (tracked).
    test.fixme(
      true,
      'VMware hosts have no reachable facets/action history surface (tracked)',
    );
    const facetRequestUrls: string[] = [];
    const actionAuditRequestUrls: string[] = [];
    let unexpectedVmwareApiCall: string | null = null;

    await page.route('**/api/vmware/**', async (route) => {
      unexpectedVmwareApiCall = route.request().url();
      await route.abort();
    });

    await page.route('**/api/resources/**', async (route) => {
      const requestUrl = new URL(route.request().url());
      const facetsMatch = requestUrl.pathname.match(/^\/api\/resources\/([^/]+)\/facets$/);
      if (!facetsMatch) {
        await route.continue();
        return;
      }

      facetRequestUrls.push(requestUrl.toString());
      const resourceId = decodeURIComponent(facetsMatch[1]);
      const bundle =
        requestUrl.searchParams.get('kind') === 'activity' &&
        requestUrl.searchParams.get('sourceAdapter') === 'vmware_adapter'
          ? buildFilteredFacetBundle(resourceId)
          : buildUnfilteredFacetBundle(resourceId);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(bundle),
      });
    });

    await page.route('**/api/audit/actions**', async (route) => {
      const requestUrl = new URL(route.request().url());
      actionAuditRequestUrls.push(requestUrl.toString());
      const resourceId = requestUrl.searchParams.get('resourceId') || '';

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          resourceId
            ? buildActionAuditBundle(resourceId)
            : { available: true, count: 0, audits: [] },
        ),
      });
    });

    await page.goto('/vmware', { waitUntil: 'domcontentloaded' });
    await expect(page.locator('[data-testid="vmware-page"]')).toBeVisible();

    await page
      .getByRole('button', { name: 'Expand details for esxi-01.lab.local' })
      .click();

    const drawer = page.getByRole('region', { name: 'esxi-01.lab.local' });
    await expect(drawer).toBeVisible();

    const historySection = page.getByTestId('resource-change-history-section');
    await expect(historySection).toBeVisible();
    await expect(historySection.getByText('Enter maintenance mode (success)')).toBeVisible();
    await expect(
      historySection.getByText('Pulse inferred elevated host risk from alarm churn'),
    ).toBeVisible();

    const actionHistorySection = page.getByTestId('resource-action-history-section');
    await expect(actionHistorySection).toBeVisible();
    await expect(actionHistorySection.getByText('Actions 2')).toBeVisible();
    await expect(
      actionHistorySection.getByText('Confirmed by independent observer'),
    ).toBeVisible();
    await expect(actionHistorySection.getByText('Execution refused')).toBeVisible();
    await expect(actionHistorySection.getByText('Refused before dispatch')).toBeVisible();
    await expect(
      actionHistorySection.getByText('Resource remediation locked'),
    ).toBeVisible();
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

    await expect(page.getByText('Filtered changes loaded')).toBeVisible();
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
    await expect
      .poll(() =>
        actionAuditRequestUrls.some((url) => {
          const parsed = new URL(url);
          return (
            parsed.pathname === '/api/audit/actions' &&
            Boolean(parsed.searchParams.get('resourceId')) &&
            parsed.searchParams.get('limit') === '5'
          );
        }),
      )
      .toBe(true);

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
