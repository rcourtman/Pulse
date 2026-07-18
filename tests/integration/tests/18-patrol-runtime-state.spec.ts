import { expect, test, type Page, type Route } from "@playwright/test";

import { apiRequest, ensureAuthenticated, trackBrowserRequests } from "./helpers";

const PATROL_BLOCK_REASON =
  "Connect a provider to power Pulse Assistant and Patrol.";
const PATROL_REASONING_ONLY_REJECTION =
  "The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls. Patrol needs tool calling to inspect resources and create governed findings.";
const PATROL_EVENT_TRIGGERS_BLOCKED =
  "Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works.";

function todayAt(hours: number, minutes: number): string {
  const value = new Date();
  value.setHours(hours, minutes, 0, 0);
  return value.toISOString();
}

const blockedPatrolStatus = {
  runtime_state: "blocked",
  running: false,
  enabled: true,
  last_patrol_at: "2026-03-25T08:55:00Z",
  next_patrol_at: "2026-03-25T14:55:00Z",
  last_duration_ms: 42000,
  resources_checked: 24,
  findings_count: 0,
  error_count: 0,
  healthy: false,
  interval_ms: 21600000,
  fixed_count: 0,
  blocked_reason: PATROL_BLOCK_REASON,
  blocked_at: "2026-03-25T09:00:00Z",
  license_required: false,
  license_status: "active",
  summary: {
    critical: 0,
    warning: 0,
    watch: 0,
    info: 0,
  },
};

const staleHealthySummary = {
  timestamp: "2026-03-25T09:05:00Z",
  overall_health: {
    score: 100,
    grade: "A",
    trend: "stable",
    factors: [],
    prediction:
      "Infrastructure is healthy with no significant issues detected.",
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
    id: "run-healthy-before-block",
    started_at: "2026-03-25T08:55:00Z",
    completed_at: "2026-03-25T08:55:42Z",
    duration_ms: 42000,
    type: "full",
    trigger_reason: "scheduled",
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
    findings_summary: "No active findings",
    finding_ids: [],
    error_count: 0,
    status: "healthy",
    triage_flags: 0,
    tool_call_count: 0,
  },
];

function buildScopedTriggerRunHistory() {
  return [
    {
      id: "run-alert-scoped",
      started_at: todayAt(11, 0),
      completed_at: todayAt(11, 2),
      duration_ms: 120000,
      type: "scoped",
      trigger_reason: "alert_fired",
      resources_checked: 2,
      nodes_checked: 0,
      guests_checked: 0,
      docker_checked: 0,
      storage_checked: 0,
      hosts_checked: 0,
      pbs_checked: 0,
      pmg_checked: 0,
      kubernetes_checked: 0,
      new_findings: 1,
      existing_findings: 0,
      rejected_findings: 0,
      resolved_findings: 0,
      auto_fix_count: 0,
      findings_summary: "Scoped alert investigation",
      finding_ids: ["finding-triggered"],
      error_count: 1,
      status: "healthy",
      error_summary: "Selected model does not support Patrol tools",
      error_detail:
        "agentic patrol failed: API error (404): No endpoints found that support the provided 'tool_choice' value.",
      triage_flags: 0,
      tool_call_count: 0,
    },
    {
      id: "run-anomaly-scoped",
      started_at: todayAt(10, 15),
      completed_at: todayAt(10, 16),
      duration_ms: 60000,
      type: "scoped",
      trigger_reason: "anomaly",
      resources_checked: 1,
      nodes_checked: 0,
      guests_checked: 0,
      docker_checked: 0,
      storage_checked: 0,
      hosts_checked: 0,
      pbs_checked: 0,
      pmg_checked: 0,
      kubernetes_checked: 0,
      new_findings: 0,
      existing_findings: 0,
      rejected_findings: 0,
      resolved_findings: 0,
      auto_fix_count: 0,
      findings_summary: "Scoped anomaly investigation",
      finding_ids: [],
      error_count: 0,
      status: "healthy",
      triage_flags: 0,
      tool_call_count: 0,
    },
    {
      id: "run-full-review",
      started_at: todayAt(9, 0),
      completed_at: todayAt(9, 3),
      duration_ms: 180000,
      type: "full",
      trigger_reason: "scheduled",
      resources_checked: 58,
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
      findings_summary: "Full patrol complete",
      finding_ids: [],
      error_count: 0,
      status: "healthy",
      triage_flags: 0,
      tool_call_count: 0,
    },
  ];
}

const scopedTriggerPatrolStatus = {
  runtime_state: "active",
  running: false,
  enabled: true,
  last_patrol_at: todayAt(9, 3),
  last_activity_at: todayAt(11, 2),
  trigger_status: {
    running: false,
    pending_triggers: 4,
    current_interval_ms: 10000,
    recent_events: 12,
    is_busy_mode: true,
    alert_triggers_enabled: true,
    anomaly_triggers_enabled: false,
  },
  next_patrol_at: todayAt(15, 0),
  last_duration_ms: 180000,
  resources_checked: 58,
  findings_count: 1,
  error_count: 1,
  healthy: false,
  interval_ms: 21600000,
  fixed_count: 0,
  blocked_reason: "",
  blocked_at: "",
  license_required: false,
  license_status: "active",
  summary: {
    critical: 0,
    warning: 1,
    watch: 0,
    info: 0,
  },
};

const blockedEventTriggerPatrolStatus = {
  ...scopedTriggerPatrolStatus,
  trigger_status: {
    running: true,
    pending_triggers: 0,
    current_interval_ms: 900000,
    recent_events: 0,
    is_busy_mode: false,
    alert_triggers_enabled: true,
    anomaly_triggers_enabled: true,
    event_triggers_blocked: true,
    event_triggers_blocked_reason: "background_automation_disabled",
    event_triggers_blocked_message: PATROL_EVENT_TRIGGERS_BLOCKED,
  },
};

async function mockBlockedPatrolRuntimeState(
  page: Page,
  options: {
    autonomyRoute?: (route: Route) => Promise<void>;
    models?: Array<{
      id: string;
      name: string;
      description?: string;
      notable?: boolean;
    }>;
    status?: Record<string, unknown>;
  } = {},
): Promise<void> {
  await page.route("**/api/ai/patrol/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(options.status ?? blockedPatrolStatus),
    });
  });

  await page.route("**/api/ai/patrol/runs*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(healthyRunHistory),
    });
  });

  await page.route("**/api/ai/patrol/autonomy", async (route) => {
    if (options.autonomyRoute) {
      await options.autonomyRoute(route);
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        autonomy_level: "monitor",
        requested_autonomy_level: "monitor",
        effective_autonomy_level: "monitor",
        autopilot_acknowledgement: {
          code: "license_required",
          active: false,
          currentVersion: 1,
          acceptedAt: "0001-01-01T00:00:00Z",
          expiresAt: "0001-01-01T00:00:00Z",
          acceptedScope: [],
          acceptedLimits: {},
        },
        full_mode_unlocked: false,
        investigation_budget: 15,
        investigation_timeout_sec: 300,
      }),
    });
  });

  await page.route("**/api/ai/models", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ models: options.models ?? [] }),
    });
  });

  await page.route("**/api/settings/ai", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        patrol_enabled: true,
        patrol_interval_minutes: 360,
        patrol_model: "",
        model: "",
        alert_triggered_analysis: false,
        patrol_alert_triggers_enabled: true,
        patrol_anomaly_triggers_enabled: true,
        patrol_event_triggers_enabled: true,
        patrol_auto_fix: false,
        auto_fix_model: "",
      }),
    });
  });

  await page.route("**/api/ai/unified/findings*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ findings: [], count: 0, active_count: 0 }),
    });
  });

  await page.route("**/api/ai/intelligence/correlations*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ correlations: [], count: 0 }),
    });
  });

  await page.route("**/api/ai/intelligence", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(staleHealthySummary),
    });
  });

  await page.route("**/api/ai/circuit/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        state: "closed",
        can_patrol: true,
        consecutive_failures: 0,
        total_successes: 42,
        total_failures: 0,
      }),
    });
  });

  await page.route("**/api/ai/approvals", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ approvals: [] }),
    });
  });
}

async function mockRuntimeCapabilities(
  page: Page,
  capabilities: string[],
): Promise<void> {
  await page.route("**/api/license/runtime-capabilities", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        capabilities,
        limits: [],
        hosted_mode: false,
        max_history_days: 7,
        runtime: {
          build: "community",
          label: "Pulse Community runtime",
        },
        blocked_capabilities: [],
      }),
    });
  });
}

async function mockScopedTriggerPatrolRuntimeState(
  page: Page,
  status: Record<string, unknown> = scopedTriggerPatrolStatus,
  settingsOverrides: Record<string, unknown> = {},
): Promise<void> {
  await page.route("**/api/ai/patrol/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(status),
    });
  });

  await page.route("**/api/ai/patrol/runs*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(buildScopedTriggerRunHistory()),
    });
  });

  await page.route("**/api/ai/patrol/autonomy", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        autonomy_level: "monitor",
        requested_autonomy_level: "monitor",
        effective_autonomy_level: "monitor",
        autopilot_acknowledgement: {
          code: "license_required",
          active: false,
          currentVersion: 1,
          acceptedAt: "0001-01-01T00:00:00Z",
          expiresAt: "0001-01-01T00:00:00Z",
          acceptedScope: [],
          acceptedLimits: {},
        },
        full_mode_unlocked: false,
        investigation_budget: 15,
        investigation_timeout_sec: 300,
      }),
    });
  });

  await page.route("**/api/ai/models", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ models: [] }),
    });
  });

  await page.route("**/api/settings/ai", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        patrol_enabled: true,
        patrol_interval_minutes: 180,
        patrol_model: "",
        model: "",
        alert_triggered_analysis: true,
        patrol_alert_triggers_enabled: true,
        patrol_anomaly_triggers_enabled: false,
        patrol_event_triggers_enabled: true,
        patrol_auto_fix: false,
        auto_fix_model: "",
        ...settingsOverrides,
      }),
    });
  });

  await page.route("**/api/ai/unified/findings*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        findings: [
          {
            id: "finding-triggered",
            severity: "warning",
            category: "reliability",
            resource_id: "resource-1",
            resource_name: "demo-resource",
            resource_type: "node",
            title: "Triggered scoped finding",
            description: "Scoped Patrol surfaced a warning.",
            detected_at: todayAt(11, 0),
            last_seen_at: todayAt(11, 2),
            auto_resolved: false,
            times_raised: 1,
            suppressed: false,
            investigation_attempts: 0,
          },
        ],
        count: 1,
        active_count: 1,
      }),
    });
  });

  await page.route("**/api/ai/intelligence/correlations*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ correlations: [], count: 0 }),
    });
  });

  await page.route("**/api/ai/intelligence", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        ...staleHealthySummary,
        timestamp: todayAt(11, 5),
        overall_health: {
          score: 72,
          grade: "C",
          trend: "stable",
          factors: [],
          prediction:
            "Patrol recently ran targeted scoped checks and still needs a clean full review.",
        },
        findings_count: {
          critical: 0,
          warning: 1,
          watch: 0,
          info: 0,
          total: 1,
        },
      }),
    });
  });

  await page.route("**/api/ai/circuit/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        state: "closed",
        can_patrol: true,
        consecutive_failures: 0,
        total_successes: 42,
        total_failures: 0,
      }),
    });
  });

  await page.route("**/api/ai/approvals", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ approvals: [] }),
    });
  });
}

test.describe("Patrol runtime-state browser contract", () => {
  test("blocked runtime state overrides stale healthy summary copy on the Patrol page", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only Patrol runtime coverage",
    );

    const commercialPostureRequests = trackBrowserRequests(
      page,
      "/api/license/commercial-posture",
    );
    const entitlementsRequests = trackBrowserRequests(
      page,
      "/api/license/entitlements",
    );

    await ensureAuthenticated(page);
    commercialPostureRequests.clear();
    entitlementsRequests.clear();
    await mockBlockedPatrolRuntimeState(page);

    await page.getByRole("tab", { name: "Patrol" }).click();
    await expect(page).toHaveURL(/\/patrol/);

    // Banner and badge both use sentence case now.
    await expect(page.getByText("Patrol paused").first()).toBeVisible();
    await expect(page.getByText(PATROL_BLOCK_REASON).first()).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Run Patrol" }),
    ).toBeDisabled();
    await expect(page.getByText(/Patrol quickstart/i)).toHaveCount(0);
    await expect(page.getByText("Health A · 100/100")).toHaveCount(0);
    expect(commercialPostureRequests.count()).toBe(0);
    expect(entitlementsRequests.count()).toBe(0);
    commercialPostureRequests.stop();
    entitlementsRequests.stop();
  });

  test("shows the server reason when a stale manual Patrol run is rejected", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only Patrol runtime coverage",
    );

    let manualRunRequests = 0;

    await ensureAuthenticated(page);
    await mockScopedTriggerPatrolRuntimeState(page);
    await page.route("**/api/ai/patrol/run", async (route) => {
      manualRunRequests += 1;
      expect(route.request().method()).toBe("POST");
      await route.fulfill({
        status: 409,
        contentType: "application/json",
        body: JSON.stringify({
          error: PATROL_REASONING_ONLY_REJECTION,
          code: "patrol_readiness_not_ready",
          status_code: 409,
          timestamp: Math.floor(Date.now() / 1000),
          details: {
            status: "not_ready",
            provider: "ollama",
            model: "ollama:deepseek-r1:7b-llama-distill-q4_K_M",
          },
        }),
      });
    });

    await page.goto("/patrol", { waitUntil: "domcontentloaded" });

    const runButton = page.getByRole("button", { name: "Run Patrol" });
    await expect(runButton).toBeEnabled();
    await runButton.click();

    await expect.poll(() => manualRunRequests).toBe(1);
    await expect(page.getByText(PATROL_REASONING_ONLY_REJECTION)).toBeVisible();
    await expect(page.getByText("Failed to start patrol run")).toHaveCount(0);
  });

  test("surfaces the server reason when a Patrol settings save is rejected", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only Patrol settings coverage",
    );

    await ensureAuthenticated(page);

    const serverReason =
      "Patrol schedule rejected: interval conflicts with the maintenance window";
    await page.route("**/api/settings/ai/update", async (route) => {
      if (route.request().method() !== "PUT") {
        await route.continue();
        return;
      }
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ error: serverReason }),
      });
    });

    await page.goto("/settings/pulse-intelligence/patrol", {
      waitUntil: "domcontentloaded",
    });
    await expect(
      page.getByRole("heading", { level: 1, name: "Patrol" }),
    ).toBeVisible();

    await page.getByRole("combobox", { name: "Schedule" }).selectOption("Every hour");
    await page.getByRole("button", { name: "Save Patrol settings" }).click();

    await expect(page.getByRole("alert").filter({ hasText: serverReason })).toBeVisible();
  });

  test("surfaces the Patrol readiness blocker when a settings save returns not-ready", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only Patrol settings coverage",
    );

    await ensureAuthenticated(page);

    const settingsRes = await apiRequest(page, "/api/settings/ai");
    const currentSettings = (await settingsRes.json()) as Record<string, unknown>;

    await page.route("**/api/settings/ai/update", async (route) => {
      if (route.request().method() !== "PUT") {
        await route.continue();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          ...currentSettings,
          patrol_readiness: {
            status: "not_ready",
            ready: false,
            cause: "model_unsupported_tools",
            summary: PATROL_REASONING_ONLY_REJECTION,
            provider: "openrouter",
            model: "openrouter:deepseek/deepseek-r1",
            checks: [],
          },
        }),
      });
    });

    await page.goto("/settings/pulse-intelligence/patrol", {
      waitUntil: "domcontentloaded",
    });
    await expect(
      page.getByRole("heading", { level: 1, name: "Patrol" }),
    ).toBeVisible();

    await page.getByRole("combobox", { name: "Schedule" }).selectOption("Every hour");
    await page.getByRole("button", { name: "Save Patrol settings" }).click();

    const readinessAlert = page
      .getByRole("alert")
      .filter({ hasText: "but Patrol is not ready" });
    await expect(readinessAlert).toBeVisible();
    await expect(readinessAlert).toContainText(PATROL_REASONING_ONLY_REJECTION);
    await expect(readinessAlert).toContainText("Provider: OpenRouter");
    await expect(readinessAlert).toContainText(
      "Model: openrouter:deepseek/deepseek-r1",
    );
  });

  test("keeps the Patrol mode clamped to Watch only when the runtime lacks autonomy", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only Patrol runtime coverage",
    );

    await ensureAuthenticated(page);

    // Community runtime: autonomy capability blocked at the runtime layer.
    await page.route("**/api/license/runtime-capabilities", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          capabilities: ["ai_patrol", "ai_alerts"],
          limits: [],
          hosted_mode: false,
          max_history_days: 7,
          runtime: {
            build: "community",
            label: "Pulse Community runtime",
          },
          blocked_capabilities: [
            {
              key: "ai_autofix",
              reason: "paid_runtime_required",
              action_url: "https://pulserelay.pro/download",
            },
          ],
        }),
      });
    });

    // A stale server-side autonomy level above what the runtime allows must
    // not surface as the active mode.
    await page.route("**/api/ai/patrol/autonomy", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          autonomy_level: "full",
          requested_autonomy_level: "full",
          effective_autonomy_level: "full",
          autopilot_acknowledgement: {
            code: "ok",
            active: true,
            currentVersion: 1,
            acceptedAt: "2026-07-01T00:00:00Z",
            expiresAt: "2027-07-01T00:00:00Z",
            acceptedScope: [],
            acceptedLimits: {},
          },
          full_mode_unlocked: true,
          investigation_budget: 15,
          investigation_timeout_sec: 300,
        }),
      });
    });

    await page.goto("/patrol", { waitUntil: "domcontentloaded" });

    const modeGroup = page.getByRole("group", { name: "Patrol mode" });
    await expect(modeGroup).toBeVisible();
    await expect(
      modeGroup.getByRole("button", { name: "Watch only" }),
    ).toHaveAttribute("aria-pressed", "true");
    await expect(
      modeGroup.getByRole("button", { name: /Autopilot/ }),
    ).toBeDisabled();
    await expect(
      modeGroup.getByRole("button", { name: /Safe auto-fix/ }),
    ).toBeDisabled();
    await expect(
      page.getByText(
        "Install the Pulse Pro runtime to use Patrol modes.",
        { exact: false },
      ),
    ).toBeVisible();
  });
});
