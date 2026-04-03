import { expect, test, type Page } from "@playwright/test";

import { ensureAuthenticated } from "./helpers";

const QUICKSTART_EXHAUSTED_REASON =
  "Quickstart credits exhausted. Connect your API key to continue using AI Patrol.";
const QUICKSTART_ACTIVATION_REQUIRED_REASON =
  "Activate this install or start a trial to use AI Patrol quickstart. Otherwise connect your API key.";
const QUICKSTART_OFFLINE_REASON =
  "Quickstart credits require internet access. Connect your API key for offline AI Patrol.";

type AISettingsPayload = {
  enabled: boolean;
  model: string;
  chat_model?: string;
  patrol_model?: string;
  configured: boolean;
  custom_context: string;
  auth_method: string;
  oauth_connected: boolean;
  patrol_interval_minutes: number;
  patrol_enabled: boolean;
  patrol_auto_fix: boolean;
  alert_triggered_analysis: boolean;
  patrol_event_triggers_enabled: boolean;
  patrol_alert_triggers_enabled: boolean;
  patrol_anomaly_triggers_enabled: boolean;
  use_proactive_thresholds: boolean;
  available_models: Array<{ id: string; name: string }>;
  anthropic_configured: boolean;
  openai_configured: boolean;
  openrouter_configured: boolean;
  deepseek_configured: boolean;
  gemini_configured: boolean;
  ollama_configured: boolean;
  ollama_base_url: string;
  ollama_password_set: boolean;
  configured_providers: string[];
  control_level: string;
  protected_guests: string[];
  discovery_enabled: boolean;
  quickstart_credits_total: number;
  quickstart_credits_used: number;
  quickstart_credits_remaining: number;
  quickstart_credits_available: boolean;
  using_quickstart: boolean;
  quickstart_blocked_reason?: string;
};

type PatrolStatusPayload = {
  runtime_state: string;
  running: boolean;
  enabled: boolean;
  last_patrol_at: string;
  next_patrol_at: string;
  last_duration_ms: number;
  resources_checked: number;
  findings_count: number;
  error_count: number;
  healthy: boolean;
  interval_ms: number;
  fixed_count: number;
  blocked_reason: string;
  blocked_at: string;
  license_required: boolean;
  license_status: string;
  summary: {
    critical: number;
    warning: number;
    watch: number;
    info: number;
  };
  using_quickstart: boolean;
  quickstart_credits_total: number;
  quickstart_credits_remaining: number;
};

type PatrolSurfaceOverrides = {
  settings?: Partial<AISettingsPayload>;
  patrolStatus?: Partial<PatrolStatusPayload>;
  intelligenceSummary?: Record<string, unknown>;
};

const baseAISettings = (): AISettingsPayload => ({
  enabled: true,
  model: "quickstart:pulse-hosted",
  chat_model: "quickstart:pulse-hosted",
  patrol_model: "quickstart:pulse-hosted",
  configured: true,
  custom_context: "",
  auth_method: "api_key",
  oauth_connected: false,
  patrol_interval_minutes: 360,
  patrol_enabled: true,
  patrol_auto_fix: false,
  alert_triggered_analysis: true,
  patrol_event_triggers_enabled: true,
  patrol_alert_triggers_enabled: true,
  patrol_anomaly_triggers_enabled: true,
  use_proactive_thresholds: false,
  available_models: [],
  anthropic_configured: false,
  openai_configured: false,
  openrouter_configured: false,
  deepseek_configured: false,
  gemini_configured: false,
  ollama_configured: false,
  ollama_base_url: "http://localhost:11434",
  ollama_password_set: false,
  configured_providers: [],
  control_level: "read_only",
  protected_guests: [],
  discovery_enabled: false,
  quickstart_credits_total: 25,
  quickstart_credits_used: 0,
  quickstart_credits_remaining: 25,
  quickstart_credits_available: true,
  using_quickstart: true,
});

const basePatrolStatus = (): PatrolStatusPayload => ({
  runtime_state: "active",
  running: false,
  enabled: true,
  last_patrol_at: "2026-03-25T08:55:00Z",
  next_patrol_at: "2026-03-25T14:55:00Z",
  last_duration_ms: 42000,
  resources_checked: 24,
  findings_count: 0,
  error_count: 0,
  healthy: true,
  interval_ms: 21600000,
  fixed_count: 0,
  blocked_reason: "",
  blocked_at: "",
  license_required: false,
  license_status: "active",
  summary: {
    critical: 0,
    warning: 0,
    watch: 0,
    info: 0,
  },
  using_quickstart: true,
  quickstart_credits_total: 25,
  quickstart_credits_remaining: 25,
});

const healthyIntelligenceSummary = () => ({
  timestamp: "2026-03-25T09:05:00Z",
  overall_health: {
    score: 100,
    grade: "A",
    trend: "stable",
    factors: [],
    prediction: "Infrastructure is healthy with no significant issues detected.",
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
});

const baseRunHistory = () => [
  {
    id: "run-healthy-before-quickstart",
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

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

async function browserApiRequest(
  page: Page,
  endpoint: string,
  method = "GET",
  data?: Record<string, unknown>,
): Promise<{ ok: boolean; status: number; body: unknown }> {
  return page.evaluate(
    async ({ endpoint: path, method: httpMethod, data: requestBody }) => {
      const headers: Record<string, string> = {};
      if (requestBody !== undefined) {
        headers["Content-Type"] = "application/json";
      }
      if (!["GET", "HEAD", "OPTIONS"].includes(httpMethod)) {
        const csrf = document.cookie
          .split("; ")
          .find((cookie) => cookie.startsWith("pulse_csrf="))
          ?.split("=")[1];
        if (csrf) {
          headers["X-CSRF-Token"] = decodeURIComponent(csrf);
        }
      }

      const response = await fetch(path, {
        method: httpMethod,
        headers,
        body: requestBody === undefined ? undefined : JSON.stringify(requestBody),
      });
      const text = await response.text();
      let body: unknown = text;
      if (text) {
        try {
          body = JSON.parse(text);
        } catch {
          body = text;
        }
      }
      return { ok: response.ok, status: response.status, body };
    },
    { endpoint, method, data },
  );
}

async function mockQuickstartPatrolSurface(page: Page, overrides: PatrolSurfaceOverrides = {}) {
  const settings = {
    ...baseAISettings(),
    ...clone(overrides.settings ?? {}),
  };
  const patrolStatus = {
    ...basePatrolStatus(),
    ...clone(overrides.patrolStatus ?? {}),
  };
  const intelligenceSummary = {
    ...healthyIntelligenceSummary(),
    ...clone(overrides.intelligenceSummary ?? {}),
  };
  const runHistory = baseRunHistory();
  const updateRequests: Array<Record<string, unknown>> = [];
  const patrolRunRequests: Array<Record<string, unknown>> = [];

  await page.route("**/api/settings/ai", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(settings),
    });
  });

  await page.route("**/api/settings/ai/update", async (route) => {
    const body = (route.request().postDataJSON() as Record<string, unknown> | null) ?? {};
    updateRequests.push(body);

    if (typeof body.enabled === "boolean") {
      settings.enabled = body.enabled;
      settings.patrol_enabled = body.enabled;
      patrolStatus.enabled = body.enabled;
      if (body.enabled && settings.quickstart_credits_available && settings.configured_providers.length === 0) {
        settings.configured = true;
        settings.model = "quickstart:pulse-hosted";
        settings.chat_model = "quickstart:pulse-hosted";
        settings.patrol_model = "quickstart:pulse-hosted";
        settings.using_quickstart = true;
        patrolStatus.runtime_state = "active";
        patrolStatus.using_quickstart = true;
        patrolStatus.blocked_reason = "";
        patrolStatus.blocked_at = "";
        patrolStatus.healthy = true;
      }
    }

    if (typeof body.openai_api_key === "string" && body.openai_api_key.trim().length > 0) {
      settings.configured = true;
      settings.model = "openai:gpt-4o";
      settings.chat_model = "";
      settings.patrol_model = "";
      settings.openai_configured = true;
      settings.configured_providers = ["openai"];
      settings.using_quickstart = false;
      patrolStatus.using_quickstart = false;
      patrolStatus.runtime_state = "active";
      patrolStatus.blocked_reason = "";
      patrolStatus.blocked_at = "";
      patrolStatus.healthy = true;
    }

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(settings),
    });
  });

  await page.route("**/api/ai/patrol/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(patrolStatus),
    });
  });

  await page.route("**/api/ai/patrol/run", async (route) => {
    const body = (route.request().postDataJSON() as Record<string, unknown> | null) ?? {};
    patrolRunRequests.push(body);

    if (!patrolStatus.enabled || patrolStatus.runtime_state === "blocked") {
      await route.fulfill({
        status: 403,
        contentType: "application/json",
        body: JSON.stringify({ success: false, message: patrolStatus.blocked_reason }),
      });
      return;
    }

    if (patrolStatus.using_quickstart && patrolStatus.quickstart_credits_remaining > 0) {
      patrolStatus.quickstart_credits_remaining -= 1;
      settings.quickstart_credits_remaining = patrolStatus.quickstart_credits_remaining;
      settings.quickstart_credits_used =
        settings.quickstart_credits_total - patrolStatus.quickstart_credits_remaining;
    }

    runHistory.unshift({
      ...baseRunHistory()[0],
      id: `run-manual-${patrolRunRequests.length}`,
      started_at: "2026-03-25T09:15:00Z",
      completed_at: "2026-03-25T09:15:36Z",
      trigger_reason: "manual",
    });

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ success: true, message: "Patrol started" }),
    });
  });

  await page.route("**/api/ai/patrol/runs*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(runHistory),
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
    const models = settings.openai_configured
      ? [{ id: "openai:gpt-4o", name: "GPT-4o" }]
      : [];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ models }),
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
      body: JSON.stringify(intelligenceSummary),
    });
  });

  await page.route("**/api/ai/circuit/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        state: "closed",
        can_patrol: patrolStatus.runtime_state !== "blocked",
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

  await page.route("**/api/ai/remediation/plans", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ plans: [] }),
    });
  });

  return { settings, patrolStatus, updateRequests, patrolRunRequests };
}

test.describe("Quickstart cross-surface browser contract", () => {
  test("activated install first AI enable surfaces quickstart-backed Patrol without BYOK", async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only quickstart coverage");

    await ensureAuthenticated(page);
    const surface = await mockQuickstartPatrolSurface(page, {
      settings: {
        enabled: false,
        configured: false,
        model: "",
        chat_model: "",
        patrol_model: "",
        using_quickstart: false,
      },
      patrolStatus: {
        runtime_state: "blocked",
        enabled: false,
        healthy: false,
        using_quickstart: false,
        blocked_reason: "Awaiting AI provider configuration",
      },
    });

    await page.goto("/settings/system-ai", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "AI Services", level: 1 })).toBeVisible();

    const response = await browserApiRequest(page, "/api/settings/ai/update", "PUT", {
      enabled: true,
    });
    expect(response.ok).toBe(true);
    expect(surface.updateRequests).toHaveLength(1);

    await page.goto("/ai", { waitUntil: "domcontentloaded" });
    await expect(page.getByText("Patrol enabled")).toBeVisible();
    await expect(page.getByText("Patrol quickstart: 25/25 runs left")).toBeVisible();
    await expect(page.getByRole("button", { name: "Run Patrol" })).toBeEnabled();
  });

  test("unactivated Community surfaces activation-or-BYOK messaging instead of quickstart", async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only quickstart coverage");

    await ensureAuthenticated(page);
    await mockQuickstartPatrolSurface(page, {
      settings: {
        enabled: true,
        configured: false,
        model: "",
        chat_model: "",
        patrol_model: "",
        quickstart_credits_total: 0,
        quickstart_credits_used: 0,
        quickstart_credits_remaining: 0,
        quickstart_credits_available: false,
        using_quickstart: false,
      },
      patrolStatus: {
        runtime_state: "blocked",
        enabled: true,
        healthy: false,
        using_quickstart: false,
        quickstart_credits_total: 0,
        quickstart_credits_remaining: 0,
        blocked_reason: QUICKSTART_ACTIVATION_REQUIRED_REASON,
        blocked_at: "2026-03-25T09:00:00Z",
      },
    });

    await page.goto("/ai", { waitUntil: "domcontentloaded" });

    await expect(page.getByText("Patrol paused").first()).toBeVisible();
    await expect(page.getByText(QUICKSTART_ACTIVATION_REQUIRED_REASON).first()).toBeVisible();
    await expect(page.getByRole("button", { name: "Run Patrol" })).toBeDisabled();
    await expect(page.getByText(/Patrol quickstart:/)).toHaveCount(0);
  });

  test("successful quickstart Patrol run refreshes the credit badge from server state", async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only quickstart coverage");

    await ensureAuthenticated(page);
    const surface = await mockQuickstartPatrolSurface(page, {
      settings: {
        quickstart_credits_remaining: 12,
        quickstart_credits_used: 13,
      },
      patrolStatus: {
        quickstart_credits_remaining: 12,
      },
    });

    await page.goto("/ai", { waitUntil: "domcontentloaded" });
    await expect(page.getByText("Patrol quickstart: 12/25 runs left")).toBeVisible();

    const response = await browserApiRequest(page, "/api/ai/patrol/run", "POST", {});
    expect(response.ok).toBe(true);
    expect(surface.patrolRunRequests).toHaveLength(1);

    await page.getByRole("button", { name: "Refresh" }).click();
    await expect(page.getByText("Patrol quickstart: 11/25 runs left")).toBeVisible();
  });

  test("BYOK override suppresses exhausted quickstart copy once Patrol is no longer using quickstart", async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only quickstart coverage");

    await ensureAuthenticated(page);
    await mockQuickstartPatrolSurface(page, {
      settings: {
        model: "openai:gpt-4o",
        chat_model: "",
        patrol_model: "",
        openai_configured: true,
        configured_providers: ["openai"],
        using_quickstart: false,
        quickstart_credits_remaining: 0,
        quickstart_credits_used: 25,
      },
      patrolStatus: {
        using_quickstart: false,
        quickstart_credits_remaining: 0,
        healthy: true,
      },
    });

    await page.goto("/ai", { waitUntil: "domcontentloaded" });

    await expect(page.getByText("Patrol enabled")).toBeVisible();
    await expect(page.getByText("Health A · 100/100")).toBeVisible();
    await expect(page.getByText("Patrol quickstart exhausted")).toHaveCount(0);
    await expect(page.getByText(QUICKSTART_EXHAUSTED_REASON)).toHaveCount(0);
  });

  test("unreachable quickstart proxy pauses Patrol with accurate offline guidance", async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only quickstart coverage");

    await ensureAuthenticated(page);
    await mockQuickstartPatrolSurface(page, {
      settings: {
        enabled: true,
        configured: false,
        model: "",
        chat_model: "",
        patrol_model: "",
        using_quickstart: false,
        quickstart_credits_remaining: 25,
        quickstart_credits_used: 0,
      },
      patrolStatus: {
        runtime_state: "blocked",
        healthy: false,
        using_quickstart: false,
        blocked_reason: QUICKSTART_OFFLINE_REASON,
        blocked_at: "2026-03-25T09:00:00Z",
      },
    });

    await page.goto("/ai", { waitUntil: "domcontentloaded" });

    await expect(page.getByText("Patrol Paused").first()).toBeVisible();
    await expect(page.getByText("Patrol paused").first()).toBeVisible();
    await expect(page.getByText(QUICKSTART_OFFLINE_REASON).first()).toBeVisible();
    await expect(page.getByRole("button", { name: "Run Patrol" })).toBeDisabled();
    await expect(page.getByText("Patrol quickstart exhausted")).toHaveCount(0);
    await expect(page.getByText("Health A · 100/100")).toHaveCount(0);
  });

  test("activated-install first AI enable toggle should not force BYOK setup when quickstart credits are available", async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only quickstart coverage");

    await ensureAuthenticated(page);
    const surface = await mockQuickstartPatrolSurface(page, {
      settings: {
        enabled: false,
        configured: false,
        model: "",
        chat_model: "",
        patrol_model: "",
        using_quickstart: false,
      },
      patrolStatus: {
        runtime_state: "blocked",
        enabled: false,
        healthy: false,
        using_quickstart: false,
        blocked_reason: "Awaiting AI provider configuration",
      },
    });

    await page.goto("/settings/system-ai", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "AI Services", level: 1 })).toBeVisible();

    await page.getByRole("button", { name: "Enable AI services" }).click();

    expect(surface.updateRequests).toHaveLength(1);
    await expect(page.getByText("Choose a provider to get started")).toHaveCount(0);
  });
});
