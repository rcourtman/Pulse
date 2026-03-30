import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test as base, expect } from '@playwright/test';

import { createAuthenticatedStorageState } from './helpers';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = '/tmp/truenas-patrol-run-history.png';

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
      `truenas-patrol-history-breakdown-${workerInfo.project.name}.json`,
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

test.describe('TrueNAS patrol run history', () => {
  test.setTimeout(180_000);

  test('keeps TrueNAS patrol counts separate from agent hosts in run history', async ({ page }) => {
    await page.route('**/api/resources**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [],
          meta: { page: 1, limit: 200, total: 0, totalPages: 0 },
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
          last_activity_at: '2026-03-30T09:03:00Z',
          next_patrol_at: '2026-03-30T15:00:00Z',
          last_duration_ms: 180000,
          resources_checked: 2,
          findings_count: 0,
          error_count: 0,
          healthy: true,
          interval_ms: 21600000,
          fixed_count: 0,
          blocked_reason: '',
          blocked_at: '',
          license_required: false,
          license_status: 'active',
          summary: {
            critical: 0,
            warning: 0,
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
            id: 'run-truenas-breakdown',
            started_at: '2026-03-30T09:00:00Z',
            completed_at: '2026-03-30T09:03:00Z',
            duration_ms: 180000,
            type: 'full',
            trigger_reason: 'scheduled',
            resources_checked: 2,
            nodes_checked: 0,
            guests_checked: 0,
            docker_checked: 1,
            storage_checked: 0,
            hosts_checked: 0,
            truenas_checked: 1,
            pbs_checked: 0,
            pmg_checked: 0,
            kubernetes_checked: 0,
            new_findings: 0,
            existing_findings: 0,
            rejected_findings: 0,
            resolved_findings: 0,
            auto_fix_count: 0,
            findings_summary: 'TrueNAS patrol complete',
            finding_ids: [],
            error_count: 0,
            status: 'healthy',
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
          findings: [],
          count: 0,
          active_count: 0,
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
          timestamp: '2026-03-30T09:03:00Z',
          overall_health: {
            score: 100,
            grade: 'A',
            trend: 'stable',
            factors: [],
            prediction: 'Infrastructure is healthy with no significant issues detected.',
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
    });

    await page.route('**/api/ai/circuit/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          state: 'closed',
          can_patrol: true,
          consecutive_failures: 0,
          total_successes: 1,
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
    await page.getByRole('button', { name: 'Runs' }).click();
    await page.getByRole('button', { name: /Checked 2 resources/i }).click();

    await expect(page.getByText('1 container')).toBeVisible();
    await expect(page.getByText('1 TrueNAS system')).toBeVisible();
    await expect(page.getByText(/1 agent/)).toHaveCount(0);

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });
  });
});
