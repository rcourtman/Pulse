import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/truenas-patrol-finding-links.png';

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
      `truenas-patrol-finding-links-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS patrol finding links', () => {
  test.setTimeout(180_000);

  test('keeps TrueNAS Patrol findings on canonical surface handoff routes', async ({ page }) => {
    await page.route('**/api/resources**', async (route) => {
      const requestUrl = new URL(route.request().url());
      if (requestUrl.pathname !== '/api/resources') {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              id: 'truenas-main',
              type: 'truenas',
              name: 'truenas-main',
              displayName: 'TrueNAS Main',
              platformId: 'truenas-main',
              platformType: 'truenas',
              sourceType: 'hybrid',
              sources: ['agent', 'truenas'],
              status: 'online',
              lastSeen: '2026-03-30T10:00:00Z',
              canonicalIdentity: {
                displayName: 'TrueNAS Main',
                hostname: 'truenas-main',
                platformId: 'truenas-main',
              },
              platformData: {
                sources: ['agent', 'truenas'],
              },
            },
            {
              id: 'app-container:truenas-main:nextcloud',
              type: 'app-container',
              name: 'nextcloud',
              displayName: 'Nextcloud',
              parentId: 'truenas-main',
              parentName: 'TrueNAS Main',
              platformId: 'truenas-main',
              platformType: 'truenas',
              sourceType: 'api',
              sources: ['truenas'],
              status: 'running',
              lastSeen: '2026-03-30T10:00:00Z',
              platformData: {
                sources: ['truenas'],
              },
            },
          ],
          meta: {
            page: 1,
            limit: 200,
            total: 2,
            totalPages: 1,
          },
        }),
      });
    });

    await page.route('**/api/ai/patrol/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          runtime_state: 'active',
          running: false,
          enabled: true,
          last_patrol_at: '2026-03-30T09:00:00Z',
          last_activity_at: '2026-03-30T10:05:00Z',
          next_patrol_at: '2026-03-30T15:00:00Z',
          last_duration_ms: 180000,
          resources_checked: 42,
          findings_count: 2,
          error_count: 0,
          healthy: false,
          interval_ms: 21600000,
          fixed_count: 0,
          blocked_reason: '',
          blocked_at: '',
          license_required: false,
          license_status: 'active',
          summary: {
            critical: 1,
            warning: 1,
            watch: 0,
            info: 0,
          },
          using_quickstart: false,
          quickstart_credits_total: 0,
          quickstart_credits_remaining: 0,
        }),
      });
    });

    await page.route('**/api/ai/patrol/runs*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: 'run-truenas-links',
            started_at: '2026-03-30T09:00:00Z',
            completed_at: '2026-03-30T09:03:00Z',
            duration_ms: 180000,
            type: 'full',
            trigger_reason: 'scheduled',
            resources_checked: 42,
            findings_summary: '2 findings',
            finding_ids: ['finding-truenas-system', 'finding-truenas-app'],
            error_count: 0,
            status: 'warning',
            triage_flags: 0,
            tool_call_count: 0,
          },
        ]),
      });
    });

    await page.route('**/api/ai/patrol/autonomy', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          autonomy_level: 'monitor',
          full_mode_unlocked: false,
          investigation_budget: 15,
          investigation_timeout_sec: 300,
        }),
      });
    });

    await page.route('**/api/ai/models', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ models: [] }),
      });
    });

    await page.route('**/api/settings/ai', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          patrol_enabled: true,
          patrol_interval_minutes: 360,
          patrol_model: '',
          model: '',
          alert_triggered_analysis: true,
          patrol_alert_triggers_enabled: true,
          patrol_anomaly_triggers_enabled: true,
          patrol_event_triggers_enabled: true,
          patrol_auto_fix: false,
          auto_fix_model: '',
        }),
      });
    });

    await page.route('**/api/ai/unified/findings*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          findings: [
            {
              id: 'finding-truenas-system',
              severity: 'critical',
              category: 'availability',
              resource_id: 'truenas-main',
              resource_name: 'TrueNAS Main',
              resource_type: 'truenas',
              title: 'TrueNAS system offline',
              description: 'Pulse Patrol detected that the TrueNAS appliance stopped reporting.',
              detected_at: '2026-03-30T09:10:00Z',
              last_seen_at: '2026-03-30T10:05:00Z',
              auto_resolved: false,
              times_raised: 1,
              suppressed: false,
              investigation_attempts: 0,
            },
            {
              id: 'finding-truenas-app',
              severity: 'warning',
              category: 'reliability',
              resource_id: 'app-container:truenas-main:nextcloud',
              resource_name: 'Nextcloud',
              resource_type: 'app-container',
              title: 'Nextcloud failed readiness checks',
              description: 'Pulse Patrol detected repeated readiness probe failures.',
              detected_at: '2026-03-30T09:20:00Z',
              last_seen_at: '2026-03-30T10:05:00Z',
              auto_resolved: false,
              times_raised: 1,
              suppressed: false,
              investigation_attempts: 0,
            },
          ],
          count: 2,
          active_count: 2,
        }),
      });
    });

    await page.route('**/api/ai/intelligence/correlations*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ correlations: [], count: 0 }),
      });
    });

    await page.route('**/api/ai/intelligence', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          timestamp: '2026-03-30T10:05:00Z',
          overall_health: {
            score: 71,
            grade: 'C',
            trend: 'stable',
            factors: [],
            prediction: 'TrueNAS resources need attention.',
          },
          findings_count: {
            critical: 1,
            warning: 1,
            watch: 0,
            info: 0,
            total: 2,
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
    });

    await page.route('**/api/ai/circuit/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          state: 'closed',
          can_patrol: true,
          consecutive_failures: 0,
          total_successes: 42,
          total_failures: 0,
        }),
      });
    });

    await page.route('**/api/ai/approvals', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ approvals: [] }),
      });
    });

    await page.route('**/api/ai/remediation/plans', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ plans: [] }),
      });
    });

    await page.goto('/ai', { waitUntil: 'domcontentloaded' });

    await expect(page.getByRole('button', { name: 'Findings' })).toBeVisible();

    await page.getByText('TrueNAS system offline').click();
    const systemFinding = page.locator('#finding-finding-truenas-system');
    await expect(
      systemFinding.getByRole('link', {
        name: 'Open related infrastructure for TrueNAS Main',
      }),
    ).toHaveAttribute('href', '/infrastructure?resource=truenas-main');
    await expect(
      systemFinding.getByRole('link', { name: 'Open related workloads for TrueNAS Main' }),
    ).toHaveAttribute(
      'href',
      '/workloads?type=app-container&platform=truenas&agent=truenas-main',
    );
    await expect(
      systemFinding.getByRole('link', { name: 'Open related storage for TrueNAS Main' }),
    ).toHaveAttribute('href', '/storage?source=truenas&node=truenas-main');
    await expect(
      systemFinding.getByRole('link', { name: 'Open related recovery for TrueNAS Main' }),
    ).toHaveAttribute('href', '/recovery?platform=truenas&node=truenas-main');

    await page.getByText('Nextcloud failed readiness checks').click();
    const appFinding = page.locator('#finding-finding-truenas-app');
    await expect(
      appFinding.getByRole('link', {
        name: 'Open related infrastructure for Nextcloud',
      }),
    ).toHaveAttribute(
      'href',
      '/infrastructure?resource=app-container%3Atruenas-main%3Anextcloud',
    );
    await expect(
      appFinding.getByRole('link', { name: 'Open related workloads for Nextcloud' }),
    ).toHaveAttribute(
      'href',
      '/workloads?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
    );

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
