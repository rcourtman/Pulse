import { expect, test, type Page } from "@playwright/test";

import { ensureAuthenticated } from "./helpers";
import { PATROL_ROUTE } from "./routes";

type AISettingsPayload = {
  enabled: boolean;
  model: string;
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
};

type PatrolStatusPayload = {
  runtime_state: string;
  running: boolean;
  enabled: boolean;
  last_patrol_at?: string;
  next_patrol_at?: string;
  last_duration_ms: number;
  resources_checked: number;
  findings_count: number;
  error_count: number;
  healthy: boolean;
  interval_ms: number;
  fixed_count: number;
  blocked_reason?: string;
  blocked_at?: string;
  license_required: boolean;
  license_status: string;
  summary: {
    critical: number;
    warning: number;
    watch: number;
    info: number;
  };
};

const baseAISettings = (): AISettingsPayload => ({
  enabled: false,
  model: "",
  configured: false,
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
});

const basePatrolStatus = (): PatrolStatusPayload => ({
  runtime_state: "blocked",
  running: false,
  enabled: false,
  last_duration_ms: 0,
  resources_checked: 0,
  findings_count: 0,
  error_count: 0,
  healthy: true,
  interval_ms: 21600000,
  fixed_count: 0,
  blocked_reason: "Connect a provider to power Pulse Assistant and Patrol.",
  license_required: false,
  license_status: "active",
  summary: {
    critical: 0,
    warning: 0,
    watch: 0,
    info: 0,
  },
});

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
        body:
          requestBody === undefined ? undefined : JSON.stringify(requestBody),
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

async function mockRetiredQuickstartSurface(
  page: Page,
  overrides: {
    settings?: Partial<AISettingsPayload>;
    patrolStatus?: Partial<PatrolStatusPayload>;
  } = {},
) {
  const settings = {
    ...baseAISettings(),
    ...clone(overrides.settings ?? {}),
  };
  const patrolStatus = {
    ...basePatrolStatus(),
    ...clone(overrides.patrolStatus ?? {}),
  };
  const updateRequests: Array<Record<string, unknown>> = [];

  await page.route("**/api/settings/ai", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(settings),
    });
  });

  await page.route("**/api/settings/ai/update", async (route) => {
    const body =
      (route.request().postDataJSON() as Record<string, unknown> | null) ?? {};
    updateRequests.push(body);
    await route.fulfill({
      status: 400,
      contentType: "text/plain",
      body: "Please configure a provider (API key or Ollama URL) before enabling Pulse Assistant\n",
    });
  });

  await page.route("**/api/ai/patrol/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(patrolStatus),
    });
  });

  await page.route("**/api/ai/patrol/runs*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
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
      body: JSON.stringify({
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
      }),
    });
  });

  await page.route("**/api/ai/circuit/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        state: "closed",
        can_patrol: false,
        consecutive_failures: 0,
        total_successes: 0,
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

  return { settings, patrolStatus, updateRequests };
}

test.describe("Retired quickstart browser contract", () => {
  test("first enable stays on provider setup and does not create quickstart state", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only AI settings coverage",
    );

    await ensureAuthenticated(page);
    const surface = await mockRetiredQuickstartSurface(page);

    await page.goto("/settings/system-ai", { waitUntil: "domcontentloaded" });
    await expect(
      page.getByRole("heading", { name: "Provider & Models", level: 1 }),
    ).toBeVisible();

    await page
      .getByRole("button", { name: "Enable Pulse Intelligence" })
      .click();
    await expect.poll(() => surface.updateRequests.length).toBe(0);
    await expect(page.getByText(/quickstart/i)).toHaveCount(0);

    const response = await browserApiRequest(
      page,
      "/api/settings/ai/update",
      "PUT",
      {
        enabled: true,
      },
    );
    expect(response.ok).toBe(false);
    expect(response.status).toBe(400);
    expect(String(response.body).toLowerCase()).not.toContain("quickstart");
    expect(surface.updateRequests).toHaveLength(1);
  });

  test("AI and Patrol surfaces suppress legacy hosted blocked copy", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only Patrol coverage",
    );

    await ensureAuthenticated(page);
    await mockRetiredQuickstartSurface(page, {
      settings: {
        enabled: true,
        patrol_enabled: true,
      },
      patrolStatus: {
        runtime_state: "blocked",
        enabled: true,
        healthy: false,
        blocked_reason:
          "Quickstart credits exhausted. Connect your API key to continue using Patrol.",
        blocked_at: "2026-03-25T09:00:00Z",
      },
    });

    await page.goto(PATROL_ROUTE, { waitUntil: "domcontentloaded" });

    await expect(page.getByText("Patrol paused").first()).toBeVisible();
    await expect(
      page
        .getByText(
          "Connect your own AI provider or local model to use Pulse Patrol.",
        )
        .first(),
    ).toBeVisible();
    await expect(page.getByText(/Quickstart credits exhausted/i)).toHaveCount(
      0,
    );
    await expect(page.getByText(/Patrol quickstart/i)).toHaveCount(0);
  });
});
