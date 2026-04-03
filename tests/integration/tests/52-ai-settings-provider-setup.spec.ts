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

test.describe("AI settings provider setup", () => {
  test("OpenRouter setup submits credentials without a hardcoded model and renders backend-selected state", async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name.startsWith("mobile-"), "Desktop-only settings coverage");

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
        ? [{ id: "openrouter:runtime-selected-model", name: "Runtime Selected Model", notable: true }]
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
        body: JSON.stringify({ success: true, message: "Connection successful", provider: "openrouter" }),
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
      const body = (route.request().postDataJSON() ?? {}) as Record<string, unknown>;
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
    await expect(page.getByRole("heading", { name: "AI Services", level: 1 })).toBeVisible();

    await page.getByRole("button", { name: /enable ai services/i }).click();
    const setupDialog = page.getByRole("dialog", { name: "Set up Pulse Assistant" });
    await expect(setupDialog.getByText("Set Up Pulse Assistant")).toBeVisible();

    await setupDialog.getByRole("button", { name: /OpenRouter/i }).click();
    await setupDialog.getByPlaceholder("sk-or-...").fill("sk-or-runtime-selected");
    await setupDialog.getByRole("button", { name: "Enable Pulse Assistant" }).click();

    await expect.poll(() => updateRequests.length).toBe(1);
    expect(updateRequests[0]).toEqual({
      enabled: true,
      openrouter_api_key: "sk-or-runtime-selected",
    });
    expect(updateRequests[0]).not.toHaveProperty("model");

    await expect(page.getByText("Advanced Model Selection")).toBeVisible();
    await expect(page.getByRole("button", { name: /enable ai services/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });
});
