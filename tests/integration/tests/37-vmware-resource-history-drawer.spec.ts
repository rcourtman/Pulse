import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test as base } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_PATH = '/tmp/vmware-resource-history-drawer.png';
const RESOURCE_ID = 'vc-1:host:host-21';
const RESOURCE_ID_ENCODED = encodeURIComponent(RESOURCE_ID);

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

const vmwareActivityChange = {
  id: 'vmware-activity-1',
  observedAt: '2026-03-30T18:16:00Z',
  occurredAt: '2026-03-30T18:15:30Z',
  resourceId: RESOURCE_ID,
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
    managedObjectId: 'host-21',
  },
};

const heuristicAlertChange = {
  id: 'heuristic-alert-1',
  observedAt: '2026-03-30T18:14:00Z',
  resourceId: RESOURCE_ID,
  kind: 'alert_fired',
  sourceType: 'heuristic',
  confidence: 'medium',
  reason: 'Pulse inferred elevated host risk from alarm churn',
  metadata: {
    incidentCategory: 'health',
  },
};

const unfilteredFacetBundle = {
  recentChanges: [vmwareActivityChange, heuristicAlertChange],
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
};

const filteredFacetBundle = {
  recentChanges: [vmwareActivityChange],
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
};

test.describe('VMware resource history drawer', () => {
  test.setTimeout(180_000);

  test('filters VMware activity through the shared resource facets history surface', async ({
    page,
  }) => {
    const facetRequestUrls: string[] = [];
    let unexpectedVmwareApiCall: string | null = null;

    await page.route('**/api/vmware/**', async (route) => {
      unexpectedVmwareApiCall = route.request().url();
      await route.abort();
    });

    await page.route('**/api/resources**', async (route) => {
      const requestUrl = new URL(route.request().url());

      if (requestUrl.pathname === '/api/resources') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: [
              {
                id: RESOURCE_ID,
                type: 'agent',
                name: 'esxi-01.lab.local',
                displayName: 'ESXi 01',
                platformId: RESOURCE_ID,
                platformType: 'vmware-vsphere',
                sourceType: 'api',
                sources: ['vmware-vsphere'],
                status: 'online',
                lastSeen: '2026-03-30T18:16:00Z',
                canonicalIdentity: {
                  displayName: 'ESXi 01',
                  hostname: 'esxi-01.lab.local',
                  platformId: RESOURCE_ID,
                },
                agent: {
                  hostname: 'esxi-01.lab.local',
                  platform: 'VMware ESXi',
                  uptimeSeconds: 604800,
                },
                vmware: {
                  connectionId: 'vc-1',
                  connectionName: 'Lab VC',
                  vcenterHost: 'vc.lab.local',
                  managedObjectId: 'host-21',
                  entityType: 'HostSystem',
                  overallStatus: 'green',
                  activeAlarmCount: 1,
                  activeAlarmSummary: 'Host fan degraded',
                  recentTaskCount: 1,
                  recentTaskSummary: 'Enter maintenance mode (success)',
                },
                platformData: {
                  sources: ['vmware-vsphere'],
                },
              },
            ],
            meta: {
              page: 1,
              limit: 100,
              total: 1,
              totalPages: 1,
            },
          }),
        });
        return;
      }

      if (requestUrl.pathname === `/api/resources/${RESOURCE_ID_ENCODED}/facets`) {
        facetRequestUrls.push(requestUrl.toString());
        const bundle =
          requestUrl.searchParams.get('kind') === 'activity' &&
          requestUrl.searchParams.get('sourceAdapter') === 'vmware_adapter'
            ? filteredFacetBundle
            : unfilteredFacetBundle;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(bundle),
        });
        return;
      }

      await route.continue();
    });

    await page.route('**/api/ai/intelligence**', async (route) => {
      const requestUrl = new URL(route.request().url());
      if (requestUrl.searchParams.get('resource_id') !== RESOURCE_ID) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            timestamp: '2026-03-30T18:16:00Z',
            overall_health: {
              score: 81,
              grade: 'B',
              trend: 'stable',
              factors: [],
              prediction: 'Infrastructure posture is stable.',
            },
            findings_count: {
              critical: 0,
              warning: 0,
              watch: 0,
              info: 0,
              total: 0,
            },
            predictions_count: 0,
            recent_changes_count: 0,
            recent_changes: [],
            learning: {
              resources_with_knowledge: 0,
              total_notes: 0,
              resources_with_baselines: 0,
              patterns_detected: 0,
              correlations_learned: 0,
              incidents_tracked: 0,
            },
          }),
        });
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          resource_id: RESOURCE_ID,
          resource_name: 'ESXi 01',
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

    await page.goto(
      `/infrastructure?source=vmware-vsphere&resource=${encodeURIComponent(RESOURCE_ID)}`,
      {
        waitUntil: 'domcontentloaded',
      },
    );

    const historySection = page.getByTestId('resource-change-history-section');

    await expect(page.getByTestId('infrastructure-page')).toBeVisible();
    await expect(page.locator('#infra-source-filter')).toHaveValue('vmware-vsphere');
    await expect(
      page.locator('div[title="ESXi 01"]').filter({ hasText: 'ESXi 01' }).first(),
    ).toBeVisible();
    await expect(historySection).toBeVisible();
    await expect(historySection.getByText('Enter maintenance mode (success)')).toBeVisible();
    await expect(
      historySection.getByText('Pulse inferred elevated host risk from alarm churn'),
    ).toBeVisible();
    await expect(historySection.getByText('Activity', { exact: true })).toBeVisible();
    await expect(historySection.getByText('VMware adapter', { exact: true })).toBeVisible();

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
            parsed.pathname === `/api/resources/${RESOURCE_ID_ENCODED}/facets` &&
            parsed.searchParams.get('limit') === '25' &&
            parsed.searchParams.get('kind') === 'activity' &&
            parsed.searchParams.get('sourceAdapter') === 'vmware_adapter'
          );
        }),
      )
      .toBe(true);

    expect(unexpectedVmwareApiCall).toBeNull();

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
