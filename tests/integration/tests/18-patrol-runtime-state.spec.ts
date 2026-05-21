import { expect, test, type Page, type Route } from "@playwright/test";

import { ensureAuthenticated, trackBrowserRequests } from "./helpers";

const PATROL_BLOCK_REASON =
  "Connect a provider to power Pulse Assistant and Patrol.";
const PATROL_AUTONOMY_PRO_REQUIRED =
  "Investigation and auto-fix require Pulse Pro. Community tier is limited to Monitor (findings-only) autonomy.";
const PATROL_REASONING_ONLY_REJECTION =
  "The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls. Patrol needs tool calling to inspect resources and create governed findings.";

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

async function mockScopedTriggerPatrolRuntimeState(page: Page): Promise<void> {
  await page.route("**/api/ai/patrol/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(scopedTriggerPatrolStatus),
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

    await expect(page.getByText("Patrol Paused").first()).toBeVisible();
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

  test("shows the server reason when Patrol configuration save is rejected", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only Patrol configuration coverage",
    );

    let autonomyUpdateCount = 0;

    await mockRuntimeCapabilities(page, [
      "ai_patrol",
      "ai_autofix",
      "ai_alerts",
    ]);
    await ensureAuthenticated(page);
    await mockBlockedPatrolRuntimeState(page, {
      status: {
        ...blockedPatrolStatus,
        readiness: {
          status: "not_ready",
          ready: false,
          cause: "model_unsupported_tools",
          summary: PATROL_REASONING_ONLY_REJECTION,
          provider: "openrouter",
          model: "openrouter:deepseek/deepseek-r1",
          checks: [],
        },
      },
      autonomyRoute: async (route) => {
        if (route.request().method() !== "PUT") {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({
              autonomy_level: "monitor",
              full_mode_unlocked: false,
              investigation_budget: 15,
              investigation_timeout_sec: 300,
            }),
          });
          return;
        }

        autonomyUpdateCount += 1;
        if (autonomyUpdateCount === 1) {
          const payload = route.request().postDataJSON() as Record<
            string,
            unknown
          >;
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({
              success: true,
              settings: {
                autonomy_level: payload.autonomy_level ?? "assisted",
                full_mode_unlocked: payload.full_mode_unlocked ?? false,
                investigation_budget: payload.investigation_budget ?? 15,
                investigation_timeout_sec:
                  payload.investigation_timeout_sec ?? 300,
              },
            }),
          });
          return;
        }

        await route.fulfill({
          status: 402,
          contentType: "application/json",
          body: JSON.stringify({
            error: "license_required",
            code: "patrol_autonomy_pro_required",
            message: PATROL_AUTONOMY_PRO_REQUIRED,
            feature: "ai_autofix",
            upgrade_url: "https://www.pulseproxmox.com/pricing",
            details: {
              cause: "license_required",
              command: "systemctl restart pulse.service",
            },
          }),
        });
      },
    });

    await page.goto("/patrol", { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "Configure Patrol" }).click();
    const configPanel = page.getByRole("dialog", {
      name: "Patrol Configuration",
    });

    const remediateButton = configPanel.getByRole("button", {
      name: "Remediate",
    });
    await expect(remediateButton).toBeEnabled();
    await remediateButton.click();
    await expect.poll(() => autonomyUpdateCount).toBe(1);

    await configPanel.evaluate((element) => {
      element.scrollTop = element.scrollHeight;
    });
    const applyButton = configPanel.getByRole("button", {
      name: "Apply Configuration",
    });
    await applyButton.click();

    await expect(page.getByText(PATROL_AUTONOMY_PRO_REQUIRED)).toBeVisible();
    await expect(configPanel).toBeVisible();
    const inlineError = configPanel.getByTestId("patrol-configuration-error");
    await expect(inlineError).toBeVisible();
    await expect(inlineError).toContainText(PATROL_AUTONOMY_PRO_REQUIRED);
    await expect(inlineError).toContainText("patrol_autonomy_pro_required");
    await expect(inlineError).toContainText(PATROL_REASONING_ONLY_REJECTION);
    await expect(inlineError).toContainText("Provider: openrouter");
    await expect(inlineError).toContainText(
      "Model: openrouter:deepseek/deepseek-r1",
    );
    await expect(
      inlineError.getByRole("link", { name: "Open Patrol provider settings" }),
    ).toHaveAttribute("href", "/settings/system-ai");
    await inlineError
      .getByTestId("patrol-configuration-error-assistant-button")
      .click();
    await expect(configPanel).toBeHidden();
    const assistantContext = page.getByLabel("Assistant context");
    await expect(assistantContext).toBeVisible();
    await expect(assistantContext).toContainText(
      "Patrol configuration failure attached",
    );
    await expect(assistantContext).toContainText(
      "patrol_autonomy_pro_required",
    );
    await expect(assistantContext).toContainText(
      "Command: sensitive or command detail withheld",
    );
    await expect(
      assistantContext.getByText("systemctl restart pulse.service"),
    ).toHaveCount(0);
    await expect(
      page.getByText("Failed to save advanced settings"),
    ).toHaveCount(0);
  });

  test("surfaces model readiness blocker after Patrol provider-model save", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only Patrol configuration coverage",
    );

    let settingsUpdatePayload: Record<string, unknown> | null = null;
    const unsupportedModel = "ollama:deepseek-r1:7b";

    await mockRuntimeCapabilities(page, ["ai_patrol", "ai_alerts"]);
    await ensureAuthenticated(page);
    await page.route("**/api/settings/ai/update", async (route) => {
      settingsUpdatePayload = route.request().postDataJSON() as Record<
        string,
        unknown
      >;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          enabled: true,
          configured: true,
          model: "ollama:llama3",
          chat_model: "ollama:llama3",
          patrol_model: unsupportedModel,
          patrol_interval_minutes: 360,
          patrol_enabled: true,
          alert_triggered_analysis: false,
          patrol_alert_triggers_enabled: true,
          patrol_anomaly_triggers_enabled: true,
          patrol_event_triggers_enabled: true,
          patrol_auto_fix: false,
          anthropic_configured: false,
          openai_configured: false,
          openrouter_configured: false,
          deepseek_configured: false,
          gemini_configured: false,
          ollama_configured: true,
          ollama_base_url: "http://127.0.0.1:11434",
          configured_providers: ["ollama"],
          custom_context: "",
          auth_method: "api_key",
          oauth_connected: false,
          request_timeout_seconds: 300,
          control_level: "read_only",
          protected_guests: [],
          discovery_enabled: false,
          discovery_interval_hours: 0,
          patrol_readiness: {
            status: "not_ready",
            ready: false,
            cause: "model_unsupported_tools",
            summary: PATROL_REASONING_ONLY_REJECTION,
            provider: "ollama",
            model: unsupportedModel,
            checks: [
              {
                id: "tools",
                status: "not_ready",
                cause: "model_unsupported_tools",
                label: "Patrol tools",
                message: PATROL_REASONING_ONLY_REJECTION,
                action: "open_provider_settings",
              },
            ],
          },
        }),
      });
    });
    await mockBlockedPatrolRuntimeState(page, {
      models: [{ id: unsupportedModel, name: "DeepSeek R1" }],
    });

    await page.goto("/patrol", { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "Configure Patrol" }).click();
    const configPanel = page.getByRole("dialog", {
      name: "Patrol Configuration",
    });

    await configPanel
      .getByLabel("Provider model")
      .selectOption(unsupportedModel);
    await expect
      .poll(() => settingsUpdatePayload)
      .toMatchObject({ patrol_model: unsupportedModel });

    const inlineError = configPanel.getByTestId("patrol-configuration-error");
    await expect(inlineError).toBeVisible();
    await expect(inlineError).toContainText(
      "Patrol configuration needs attention",
    );
    await expect(inlineError).toContainText(
      "Patrol model was saved, but Patrol is not ready to run.",
    );
    await expect(inlineError).toContainText(PATROL_REASONING_ONLY_REJECTION);
    await expect(inlineError).toContainText(
      "patrol_readiness_not_ready · model_unsupported_tools",
    );
    await expect(inlineError).toContainText("Provider: ollama");
    await expect(inlineError).toContainText(`Model: ${unsupportedModel}`);
    await expect(page.getByText("Failed to update patrol model")).toHaveCount(
      0,
    );

    await inlineError
      .getByTestId("patrol-configuration-error-assistant-button")
      .click();
    await expect(configPanel).toBeHidden();
    await expect(page.getByLabel("Assistant context")).toContainText(
      "Patrol configuration issue attached",
    );
  });

  test("clamps stale full-mode state before monitor-only Patrol configuration save", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only Patrol configuration coverage",
    );

    let updatePayload: Record<string, unknown> | null = null;

    await mockRuntimeCapabilities(page, ["ai_patrol", "ai_alerts"]);
    await ensureAuthenticated(page);
    await mockBlockedPatrolRuntimeState(page, {
      autonomyRoute: async (route) => {
        if (route.request().method() !== "PUT") {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({
              autonomy_level: "full",
              full_mode_unlocked: true,
              investigation_budget: 15,
              investigation_timeout_sec: 300,
            }),
          });
          return;
        }

        updatePayload = route.request().postDataJSON() as Record<
          string,
          unknown
        >;
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            success: true,
            settings: {
              autonomy_level: "monitor",
              full_mode_unlocked: false,
              investigation_budget: updatePayload.investigation_budget ?? 15,
              investigation_timeout_sec:
                updatePayload.investigation_timeout_sec ?? 300,
            },
          }),
        });
      },
    });

    await page.goto("/patrol", { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: "Configure Patrol" }).click();
    const configPanel = page.getByRole("dialog", {
      name: "Patrol Configuration",
    });

    await expect(
      configPanel.getByRole("button", { name: "Remediate" }),
    ).toBeDisabled();
    await configPanel.evaluate((element) => {
      element.scrollTop = element.scrollHeight;
    });
    await configPanel
      .getByRole("button", { name: "Apply Configuration" })
      .click();

    await expect
      .poll(() => updatePayload)
      .toMatchObject({
        autonomy_level: "monitor",
        full_mode_unlocked: false,
      });
    await expect(configPanel).toBeHidden();
    await expect(
      page.getByText("Failed to save advanced settings"),
    ).toHaveCount(0);
    await expect(page.getByTestId("patrol-configuration-error")).toHaveCount(0);
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

  test("surfaces scoped trigger context inside the summary and split trigger controls on the Patrol page", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only Patrol runtime coverage",
    );

    await ensureAuthenticated(page);
    await mockScopedTriggerPatrolRuntimeState(page);

    await page.goto("/patrol", { waitUntil: "domcontentloaded" });

    await expect(
      page.getByText(
        "Breakdown: 1 full, 1 alert-triggered, 1 anomaly-triggered",
      ),
    ).toHaveCount(0);
    await expect(
      page.getByText(
        "Recent activity mix: 1 full, 1 alert-triggered, 1 anomaly-triggered",
      ),
    ).toBeVisible();
    await expect(
      page.getByText("Trigger mode: 4 queued · busy mode · anomalies off"),
    ).toBeVisible();

    await page.getByRole("button", { name: "Configure Patrol" }).click();

    await expect(page.getByText("Alert-Triggered Patrols")).toBeVisible();
    await expect(page.getByText("Anomaly-Triggered Patrols")).toBeVisible();
    await expect(
      page.getByText(
        "Alert and anomaly triggers run targeted scoped checks that update",
      ),
    ).toBeVisible();
    await expect(page.getByText("Last full patrol")).toBeVisible();

    await page.getByRole("button", { name: "Runs" }).click();
    await page.getByRole("button", { name: /Alert fired/i }).click();
    await expect(
      page.getByText("Selected model does not support Patrol tools"),
    ).toBeVisible();
    await expect(
      page.getByText("Provider rejected Patrol tool calls"),
    ).toBeVisible();
    await expect(page.getByText(/tool_choice/)).toHaveCount(0);
    await expect(page.getByText(/No endpoints found/)).toHaveCount(0);
    await expect(
      page.getByRole("link", { name: "Open Patrol provider settings" }),
    ).toHaveAttribute("href", "/settings/system-ai");

    await page.getByTestId("patrol-run-assistant-button").click();
    const assistantContext = page.getByLabel("Assistant context");
    await expect(assistantContext).toBeVisible();
    await expect(assistantContext).toContainText("Patrol run attached");
    await expect(assistantContext).toContainText("Scoped run run-alert-scoped");
    await expect(assistantContext).toContainText(
      "Review Patrol runtime failure",
    );
    await expect(assistantContext).toContainText(
      "Selected model does not support Patrol tools",
    );
  });
});
