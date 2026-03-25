import { expect, test, type Page } from '@playwright/test';

import { ensureAuthenticated } from './helpers';

const PATROL_BLOCK_REASON =
  'Quickstart credits exhausted. Connect your API key to continue using AI Patrol.';

const blockedPatrolStatus = {
  runtime_state: 'blocked',
  running: false,
  enabled: true,
  last_patrol_at: '2026-03-25T08:55:00Z',
  next_patrol_at: '2026-03-25T14:55:00Z',
  last_duration_ms: 42000,
  resources_checked: 24,
  findings_count: 0,
  error_count: 0,
  healthy: false,
  interval_ms: 21600000,
  fixed_count: 0,
  blocked_reason: PATROL_BLOCK_REASON,
  blocked_at: '2026-03-25T09:00:00Z',
  license_required: false,
  license_status: 'active',
  summary: {
    critical: 0,
    warning: 0,
    watch: 0,
    info: 0,
  },
  using_quickstart: true,
  quickstart_credits_total: 25,
  quickstart_credits_remaining: 0,
};

const staleHealthySummary = {
  timestamp: '2026-03-25T09:05:00Z',
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
};

const healthyRunHistory = [
  {
    id: 'run-healthy-before-block',
    started_at: '2026-03-25T08:55:00Z',
    completed_at: '2026-03-25T08:55:42Z',
    duration_ms: 42000,
    type: 'full',
    trigger_reason: 'scheduled',
    resources_checked: 24,
    nodes_checked: 2,
    guests_checked: 8,
    docker_checked: 4,
    storage_checked: 3,
    hosts_checked: 2,
    pbs_checked: 1,
    pmg_checked: 0,
    kubernetes_checked: 4,
    new_findings: 0,
    existing_findings: 0,
    rejected_findings: 0,
    resolved_findings: 0,
    auto_fix_count: 0,
    findings_summary: 'No active findings',
    finding_ids: [],
    error_count: 0,
    status: 'healthy',
    triage_flags: 0,
    tool_call_count: 0,
  },
];

async function mockBlockedPatrolRuntimeState(page: Page): Promise<void> {
  await page.route('**/api/ai/patrol/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(blockedPatrolStatus),
    });
  });

  await page.route('**/api/ai/patrol/runs*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(healthyRunHistory),
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
        alert_triggered_analysis: false,
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
      body: JSON.stringify({ findings: [], count: 0, active_count: 0 }),
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
      body: JSON.stringify(staleHealthySummary),
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
}

test.describe('Patrol runtime-state browser contract', () => {
  test('blocked runtime state overrides stale healthy summary copy on the Patrol page', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith('mobile-'), 'Desktop-only Patrol runtime coverage');

    await ensureAuthenticated(page);
    await mockBlockedPatrolRuntimeState(page);

    await page.goto('/ai', { waitUntil: 'domcontentloaded' });

    await expect(page.getByText('Patrol Paused').first()).toBeVisible();
    await expect(page.getByText('Patrol paused').first()).toBeVisible();
    await expect(page.getByText(PATROL_BLOCK_REASON).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Run Patrol' })).toBeDisabled();
    await expect(page.getByText(/Credits exhausted/)).toBeVisible();
    await expect(page.getByText('Health A · 100/100')).toHaveCount(0);
  });
});
