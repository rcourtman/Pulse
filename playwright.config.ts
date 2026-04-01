/**
 * Root-level Playwright configuration.
 *
 * This thin wrapper allows running journey tests from the workspace root:
 *
 *   npx playwright test tests/journeys --project=chromium
 *
 * It mirrors the settings in tests/integration/playwright.config.ts but
 * resolves testDir so that the Playwright runner and test files share the
 * same @playwright/test instance (avoiding the "two versions" error).
 */
import fs from "node:fs";
import path from "node:path";
import { defineConfig, devices } from "@playwright/test";

const trim = (value: unknown): string => String(value ?? "").trim();
const repoRoot = path.resolve(__dirname);
const managedHotDevPidPath = path.join(repoRoot, "tmp", "hot-dev.bg.pid");

const runtimeStatePath = (env: NodeJS.ProcessEnv = process.env): string => {
  const configuredPath = trim(env.PULSE_E2E_RUNTIME_STATE_PATH);
  if (configuredPath === "") {
    return path.resolve(repoRoot, "tmp", "e2e-runtime-state.json");
  }
  return path.isAbsolute(configuredPath)
    ? configuredPath
    : path.resolve(repoRoot, configuredPath);
};

const loadRuntimeBaseURL = (
  env: NodeJS.ProcessEnv = process.env,
): string | null => {
  try {
    const raw = fs.readFileSync(runtimeStatePath(env), "utf8");
    const parsed = JSON.parse(raw) as { baseURL?: string };
    return typeof parsed.baseURL === "string" && parsed.baseURL.trim() !== ""
      ? parsed.baseURL.trim()
      : null;
  } catch {
    return null;
  }
};

const managedDevBrowserBaseURL = (
  env: NodeJS.ProcessEnv = process.env,
): string | null => {
  try {
    const pid = Number.parseInt(
      fs.readFileSync(managedHotDevPidPath, "utf8").trim(),
      10,
    );
    if (!Number.isInteger(pid) || pid <= 0) {
      return null;
    }
    process.kill(pid, 0);
    const host = trim(env.FRONTEND_DEV_HOST) || "127.0.0.1";
    const port = trim(env.FRONTEND_DEV_PORT) || "5173";
    return `http://${host}:${port}`;
  } catch {
    return null;
  }
};

const preferredBrowserBaseURL = (
  env: NodeJS.ProcessEnv = process.env,
): string =>
  trim(env.PULSE_BASE_URL) ||
  trim(env.PLAYWRIGHT_BASE_URL) ||
  loadRuntimeBaseURL(env) ||
  managedDevBrowserBaseURL(env) ||
  "http://localhost:7655";

export default defineConfig({
  testDir: "./tests/integration/tests",

  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,

  reporter: [
    [
      "html",
      { outputFolder: "tests/integration/playwright-report", open: "never" },
    ],
    ["list"],
    ["junit", { outputFile: "tests/integration/test-results/junit.xml" }],
  ],

  timeout: 60_000,
  expect: { timeout: 10_000 },

  use: {
    baseURL: preferredBrowserBaseURL(),

    ignoreHTTPSErrors: ["1", "true", "yes", "on"].includes(
      String(process.env.PULSE_E2E_INSECURE_TLS || "")
        .trim()
        .toLowerCase(),
    ),

    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    navigationTimeout: 15_000,
    actionTimeout: 10_000,
  },

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
      testIgnore: ["**/04-mobile.spec.ts"],
    },
    {
      name: "mobile-chrome",
      use: { ...devices["Pixel 5"] },
      testIgnore: ["**/journeys/**"],
    },
    {
      name: "mobile-safari",
      use: { ...devices["iPhone 12"] },
      testIgnore: ["**/journeys/**"],
    },
  ],

  webServer: undefined,
});
