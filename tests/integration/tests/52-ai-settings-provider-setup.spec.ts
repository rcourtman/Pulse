import { expect, test } from "@playwright/test";

import { ensureAuthenticated } from "./helpers";

type MockAISettings = {
  enabled: boolean;
  model: string;
  chat_model?: string;
  patrol_model?: string;
  configured: boolean;
  custom_context: string;
  auth_method: string;
  oauth_connected: boolean;
  anthropic_configured: boolean;
  openai_configured: boolean;
  openrouter_configured: boolean;
  deepseek_configured: boolean;
  gemini_configured: boolean;
  ollama_configured: boolean;
  ollama_base_url: string;
  configured_providers: string[];
  available_models: Array<{ id: string; name: string; notable?: boolean }>;
  patrol_readiness?: {
    status: string;
    ready: boolean;
    cause?: string;
    summary: string;
    provider?: string;
    model?: string;
    checks: Array<Record<string, unknown>>;
  };
  patrol_model_readiness?: {
    probe_version: string;
    success: boolean;
    transport_healthy: boolean;
    patrol_capable: boolean;
    status: string;
    provider: string;
    model: string;
    duration_ms: number;
    cause: string;
    summary: string;
    recommendation: string;
    dimensions: Record<string, Record<string, unknown>>;
    modes: Record<string, Record<string, unknown>>;
    recorded_at: string;
    recorded_at_unix: number;
  };
};

const baseSettings = (): MockAISettings => ({
  enabled: false,
  model: "",
  chat_model: "",
  patrol_model: "",
  configured: false,
  custom_context: "",
  auth_method: "api_key",
  oauth_connected: false,
  anthropic_configured: false,
  openai_configured: false,
  openrouter_configured: false,
  deepseek_configured: false,
  gemini_configured: false,
  ollama_configured: false,
  ollama_base_url: "http://localhost:11434",
  configured_providers: [],
  available_models: [],
});

test.describe("Pulse Intelligence settings provider setup", () => {
  test("OpenRouter setup submits credentials without a hardcoded model and renders backend-selected state", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only settings coverage",
    );

    await ensureAuthenticated(page);

    const updateRequests: Array<Record<string, unknown>> = [];
    let settings = baseSettings();

    await page.route("**/api/settings/ai", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(settings),
      });
    });

    await page.route("**/api/ai/models", async (route) => {
      const models = settings.openrouter_configured
        ? [
            {
              id: "openrouter:runtime-selected-model",
              name: "Runtime Selected Model",
              notable: true,
            },
          ]
        : [];
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ models }),
      });
    });

    await page.route("**/api/ai/test/openrouter", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          success: true,
          message: "Connection successful",
          provider: "openrouter",
        }),
      });
    });

    await page.route("**/api/ai/chat/sessions", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([]),
      });
    });

    await page.route("**/api/settings/ai/update", async (route) => {
      const body = (route.request().postDataJSON() ?? {}) as Record<
        string,
        unknown
      >;
      updateRequests.push(body);
      settings = {
        ...settings,
        enabled: true,
        configured: true,
        model: "openrouter:runtime-selected-model",
        openrouter_configured: true,
        configured_providers: ["openrouter"],
      };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(settings),
      });
    });

    await page.goto("/settings/system-ai", { waitUntil: "domcontentloaded" });
    await expect(
      page.getByRole("heading", { name: "Provider & Models", level: 1 }),
    ).toBeVisible();

    // First enable routes through the Set up Pulse Intelligence dialog:
    // pick a provider, submit the key, and let the backend select models.
    // No model may be hardcoded into the update payload.
    await page
      .getByRole("button", { name: "Enable Pulse Intelligence" })
      .click();
    const setupDialog = page.getByRole("dialog", {
      name: "Set up Pulse Intelligence",
    });
    await expect(
      setupDialog.getByRole("heading", { name: "Set Up Pulse Intelligence" }),
    ).toBeVisible();

    await setupDialog.getByRole("button", { name: /Anthropic/ }).click();
    await setupDialog
      .getByPlaceholder("sk-ant-...")
      .fill("sk-ant-runtime-selected");
    await setupDialog
      .getByRole("button", { name: "Enable Pulse Intelligence" })
      .click();

    await expect.poll(() => updateRequests.length).toBe(1);
    const submitted = updateRequests[0] as Record<string, unknown>;
    expect(submitted.anthropic_api_key).toBe("sk-ant-runtime-selected");
    expect(submitted.enabled).toBe(true);
    expect(submitted.model ?? "").toBe("");

    // Per-model overrides moved into the section panels; the panel signals
    // enabled state and the backend-selected shared default model here.
    await expect(page.getByLabel("Enable Pulse Intelligence")).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  test("provider setup surfaces saved Patrol readiness warnings with provider and model context", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only settings coverage",
    );

    await ensureAuthenticated(page);

    let settings = baseSettings();

    await page.route("**/api/settings/ai", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(settings),
      });
    });

    await page.route("**/api/ai/models", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          models: [
            {
              id: "openrouter:deepseek/deepseek-r1",
              name: "DeepSeek R1",
              notable: true,
            },
          ],
        }),
      });
    });

    await page.route("**/api/ai/test/openrouter", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          success: true,
          message: "Connection successful",
          provider: "openrouter",
        }),
      });
    });

    await page.route("**/api/ai/chat/sessions", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([]),
      });
    });

    await page.route("**/api/settings/ai/update", async (route) => {
      settings = {
        ...settings,
        enabled: true,
        configured: true,
        model: "openrouter:deepseek/deepseek-r1",
        patrol_model: "openrouter:deepseek/deepseek-r1",
        openrouter_configured: true,
        configured_providers: ["openrouter"],
        patrol_readiness: {
          status: "not_ready",
          ready: false,
          cause: "model_unsupported_tools",
          summary:
            "The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.",
          provider: "openrouter",
          model: "openrouter:deepseek/deepseek-r1",
          checks: [],
        },
      };
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(settings),
      });
    });

    await page.goto("/settings/system-ai", { waitUntil: "domcontentloaded" });
    await page
      .getByRole("button", { name: "Enable Pulse Intelligence" })
      .click();
    const setupDialog = page.getByRole("dialog", {
      name: "Set up Pulse Intelligence",
    });
    await setupDialog.getByRole("button", { name: /Anthropic/ }).click();
    await setupDialog
      .getByPlaceholder("sk-ant-...")
      .fill("sk-ant-runtime-selected");
    await setupDialog
      .getByRole("button", { name: "Enable Pulse Intelligence" })
      .click();

    // The readiness warning carries the provider and model context from the
    // saved patrol_readiness payload.
    await expect(page.getByText(/but Patrol is not ready/)).toBeVisible();
    await expect(page.getByText("Provider: OpenRouter")).toBeVisible();
    await expect(
      page.getByText("Model: openrouter:deepseek/deepseek-r1"),
    ).toBeVisible();
  });

  test("keyless custom endpoint keeps provider health separate from Patrol tool capability", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only settings coverage",
    );
    await ensureAuthenticated(page);

    const settings: MockAISettings = {
      ...baseSettings(),
      enabled: true,
      configured: true,
      model: "openai:model-without-known-prefix",
      patrol_model: "openai:model-without-known-prefix",
      openai_configured: true,
      configured_providers: ["openai"],
      patrol_model_readiness: {
        probe_version: "patrol-readiness/v1",
        success: false,
        transport_healthy: true,
        patrol_capable: false,
        status: "warning",
        provider: "openai",
        model: "model-without-known-prefix",
        duration_ms: 60000,
        cause: "model_tool_support_unverified",
        summary:
          "The provider is healthy for ordinary chat, but the selected model did not demonstrate Patrol's streaming tool protocol.",
        recommendation:
          "Keep using this route for ordinary Assistant chat, or choose a Patrol model with reliable streaming tool use.",
        dimensions: {
          connectivity: {
            status: "pass",
            summary: "Provider and selected model are reachable.",
          },
          tool_protocol: {
            status: "fail",
            summary: "Exact tool protocol passed 0/3 scenarios.",
            attempts: 3,
            passed: 0,
          },
          context_quality: {
            status: "fail",
            summary: "Patrol-shaped context fixtures passed 0/2 scenarios.",
            attempts: 2,
            passed: 0,
          },
          latency: {
            status: "pass",
            summary:
              "Warm median 22.5s; projected 8-turn Watch-only loop 3m0s.",
          },
        },
        modes: {
          monitor: {
            status: "not_suitable",
            summary:
              "The model did not pass the minimum Watch-only protocol and context checks.",
          },
          approval: {
            status: "not_suitable",
            summary: "Ask first requires all Watch-only checks to pass first.",
          },
          assisted: {
            status: "not_assessed",
            summary:
              "Requires an extended remediation and verification canary.",
          },
          full: {
            status: "not_assessed",
            summary: "Requires an extended governed Autopilot canary.",
          },
        },
        recorded_at: "2026-07-24T10:00:00Z",
        recorded_at_unix: 1784887200,
      },
    };

    await page.route("**/api/settings/ai", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(settings),
      });
    });
    await page.route("**/api/ai/models", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          models: [
            {
              id: "openai:model-without-known-prefix",
              name: "Opaque local model",
              provider: "openai",
            },
          ],
        }),
      });
    });
    await page.route("**/api/ai/test/openai", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          success: true,
          message: "Connection successful",
          provider: "openai",
          model: "openai:model-without-known-prefix",
        }),
      });
    });
    await page.route("**/api/ai/chat/sessions", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([]),
      });
    });

    await page.goto("/settings/pulse-intelligence/patrol", {
      waitUntil: "domcontentloaded",
    });
    await expect(
      page.getByText("Provider connected; Patrol capability not verified"),
    ).toBeVisible();
    await expect(
      page.getByText("Provider and selected model are reachable."),
    ).toBeVisible();
    await expect(page.getByText("0/3 · Failed")).toBeVisible();
    await expect(page.getByText("Opaque local model").first()).toBeVisible();
  });

  test("settings save failure keeps provider preflight recommendation context", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only settings coverage",
    );

    await ensureAuthenticated(page);

    const settings: MockAISettings = {
      ...baseSettings(),
      enabled: true,
      configured: true,
      model: "openrouter:deepseek/deepseek-r1",
      patrol_model: "openrouter:deepseek/deepseek-r1",
      openrouter_configured: true,
      configured_providers: ["openrouter"],
    };
    let updateHits = 0;

    await page.route("**/api/settings/ai", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(settings),
      });
    });

    await page.route("**/api/ai/models", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          models: [
            {
              id: "openrouter:deepseek/deepseek-r1",
              name: "DeepSeek R1",
              notable: true,
            },
          ],
        }),
      });
    });

    await page.route("**/api/ai/test/openrouter", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          success: false,
          message: "Provider authentication issue",
          provider: "openrouter",
          model: "openrouter:deepseek/deepseek-r1",
          cause: "provider_auth",
          summary:
            "Pulse Patrol cannot analyze your infrastructure because the provider rejected the configured credentials or account access.",
          recommendation:
            "Check the API key or provider authentication in Patrol provider settings, then rerun Patrol.",
          action: "open_provider_settings",
        }),
      });
    });

    await page.route("**/api/ai/chat/sessions", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([]),
      });
    });

    await page.route("**/api/settings/ai/update", async (route) => {
      updateHits += 1;
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({
          error: "Failed to save Pulse Intelligence settings",
        }),
      });
    });

    await page.goto("/settings/system-ai", { waitUntil: "domcontentloaded" });
    await expect(
      page.getByRole("heading", { name: "Provider & Models", level: 1 }),
    ).toBeVisible();
    await expect(
      page.getByText("Provider authentication issue").first(),
    ).toBeVisible();
    await expect(
      page
        .getByText(
          "Check the API key or provider authentication in Patrol provider settings, then rerun Patrol.",
        )
        .first(),
    ).toBeVisible();

    await page.getByRole("button", { name: "Save provider settings" }).click();

    await expect.poll(() => updateHits).toBe(1);
    const failureMessage = page.getByText(
      /OpenRouter provider.*Provider authentication issue.*Failed to save Pulse Intelligence settings/i,
    );
    await expect(failureMessage).toBeVisible();
    await expect(failureMessage).toContainText(
      "Check the API key or provider authentication in Patrol provider settings",
    );
  });
});
