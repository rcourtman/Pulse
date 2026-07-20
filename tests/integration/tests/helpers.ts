import { execFileSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  Browser,
  Page,
  Request,
  expect,
  request as playwrightRequest,
  type APIRequestContext,
} from "@playwright/test";
import {
  browserBaseURLIsImplicitDevRuntime,
  preferredBrowserBaseURL,
  preferredPlaywrightRouteBaseURL,
  readRuntimeState,
  runtimeStatePath,
} from "./runtime-defaults";

const helpersDir = path.dirname(fileURLToPath(import.meta.url));

const runtimePrimaryAPIToken = (): string => {
  const parsed = readRuntimeState();
  return typeof parsed?.primaryAPIToken === "string"
    ? parsed.primaryAPIToken.trim()
    : "";
};

const explicitPrimaryAPIToken = (): string =>
  String(process.env.PULSE_E2E_PRIMARY_API_TOKEN || "").trim();

const configuredPrimaryAPIToken = (): string =>
  explicitPrimaryAPIToken() || runtimePrimaryAPIToken();

/**
 * Default admin credentials for testing
 */
export const ADMIN_CREDENTIALS = {
  username: "admin",
  // Pulse enforces a minimum password length of 12 characters.
  password: "adminadminadmin",
};

const DEFAULT_E2E_BOOTSTRAP_TOKEN =
  "0123456789abcdef0123456789abcdef0123456789abcdef";

export const E2E_CREDENTIALS = {
  bootstrapToken:
    process.env.PULSE_E2E_BOOTSTRAP_TOKEN || DEFAULT_E2E_BOOTSTRAP_TOKEN,
  primaryApiToken: configuredPrimaryAPIToken(),
  username: process.env.PULSE_E2E_USERNAME || ADMIN_CREDENTIALS.username,
  password: process.env.PULSE_E2E_PASSWORD || ADMIN_CREDENTIALS.password,
};

const SETUP_HANDOFF_STORAGE_KEY = "pulse_setup_handoff";
let ignoreRuntimePrimaryAPIToken = false;

type PrimaryAPITokenSource = "env" | "memory" | "runtime";

type PrimaryAPITokenCandidate = {
  source: PrimaryAPITokenSource;
  token: string;
};

const primaryAPITokenCandidates = (): PrimaryAPITokenCandidate[] => {
  const seen = new Set<string>();
  const candidates: PrimaryAPITokenCandidate[] = [];
  const add = (source: PrimaryAPITokenSource, token: unknown) => {
    const nextToken = String(token || "").trim();
    if (!nextToken || seen.has(nextToken)) {
      return;
    }
    seen.add(nextToken);
    candidates.push({ source, token: nextToken });
  };

  add("env", explicitPrimaryAPIToken());
  add("memory", E2E_CREDENTIALS.primaryApiToken);
  if (!ignoreRuntimePrimaryAPIToken) {
    add("runtime", runtimePrimaryAPIToken());
  }
  return candidates;
};

const rememberPrimaryAPIToken = (token: unknown) => {
  const nextToken = String(token || "").trim();
  if (nextToken) {
    E2E_CREDENTIALS.primaryApiToken = nextToken;
    ignoreRuntimePrimaryAPIToken = false;
    persistRuntimePrimaryAPIToken(nextToken);
  }
};

const rememberSetupCredentials = (setup: CompleteSetupWizardResult) => {
  const username = String(setup.username || "").trim();
  const password = String(setup.password || "").trim();
  if (username) {
    E2E_CREDENTIALS.username = username;
  }
  if (password) {
    E2E_CREDENTIALS.password = password;
  }
  rememberPrimaryAPIToken(setup.apiToken);
};

const forgetRejectedPrimaryAPIToken = (
  candidate: PrimaryAPITokenCandidate,
) => {
  if (
    candidate.source === "runtime" ||
    candidate.token === runtimePrimaryAPIToken()
  ) {
    ignoreRuntimePrimaryAPIToken = true;
  }
  if (E2E_CREDENTIALS.primaryApiToken === candidate.token) {
    E2E_CREDENTIALS.primaryApiToken = "";
  }
};

const persistRuntimePrimaryAPIToken = (token: string) => {
  try {
    const runtimePath = runtimeStatePath();
    const currentState = readRuntimeState() || {};
    fs.writeFileSync(
      runtimePath,
      `${JSON.stringify({ ...currentState, primaryAPIToken: token }, null, 2)}\n`,
      "utf8",
    );
  } catch {
    // Runtime-state persistence is a managed-local optimization. In plain
    // Playwright runs the in-process credential update is still sufficient.
  }
};

export async function waitForAppShell(page: Page, timeoutMs = 20_000) {
  await page.waitForLoadState("domcontentloaded");

  // The raw HTML shell contains a noscript fallback. Wait for the SPA to
  // mount before making route-specific assertions against wizard/login UI.
  await page.waitForFunction(
    () => {
      const root = document.getElementById("root");
      return root !== null && root.children.length > 0;
    },
    undefined,
    { timeout: timeoutMs },
  );
}

export async function waitForPulseReady(page: Page, timeoutMs = 120_000) {
  const startedAt = Date.now();
  let lastError: unknown = null;
  while (Date.now() - startedAt < timeoutMs) {
    try {
      const res = await page.request.get("/api/health");
      if (res.ok()) {
        return;
      }
      lastError = new Error(`Health check returned ${res.status()}`);
    } catch (err) {
      lastError = err;
    }
    await page.waitForTimeout(1000);
  }
  throw lastError ?? new Error("Timed out waiting for Pulse to become ready");
}

export function trackBrowserRequests(page: Page, matcher: string | RegExp) {
  const matchedURLs: string[] = [];
  const matches = (url: string) =>
    typeof matcher === "string" ? url.includes(matcher) : matcher.test(url);
  const handleRequest = (request: Request) => {
    const url = request.url();
    if (matches(url)) {
      matchedURLs.push(url);
    }
  };

  page.on("request", handleRequest);

  return {
    clear: () => {
      matchedURLs.length = 0;
    },
    count: () => matchedURLs.length,
    stop: () => page.off("request", handleRequest),
    urls: () => [...matchedURLs],
  };
}

type SecurityStatus = {
  hasAuthentication?: boolean;
};

type ResetFirstRunResponse = {
  bootstrapToken?: string;
};

type SetupCompletionTarget = "sources" | "agent" | "none";

type CompleteSetupWizardOptions = {
  completionTarget?: SetupCompletionTarget;
};

type CompleteSetupWizardResult = {
  apiToken?: string;
  password?: string;
  username?: string;
};

const AUTHENTICATED_URL =
  /\/(proxmox|nodes|hosts|machines|docker|infrastructure)/;

const SETUP_COMPLETION_HANDOFFS: Record<
  Exclude<SetupCompletionTarget, "none">,
  { buttonName: string; path: string; urlPattern: RegExp }
> = {
  sources: {
    buttonName: "Add infrastructure",
    path: "/settings/infrastructure?add=pick",
    urlPattern: /\/settings\/infrastructure\?add=pick$/,
  },
  agent: {
    buttonName: "Install Pulse Agent",
    path: "/settings/infrastructure?add=linux-host",
    urlPattern: /\/settings\/infrastructure\?add=linux-host$/,
  },
};

async function completeFirstRunViaAPI(
  page: Page,
  bootstrapToken: string,
): Promise<CompleteSetupWizardResult> {
  const apiToken = randomBytes(24).toString("hex");
  const res = await apiRequest(page, "/api/security/quick-setup", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Setup-Token": bootstrapToken,
    },
    data: {
      username: E2E_CREDENTIALS.username,
      password: E2E_CREDENTIALS.password,
      apiToken,
      force: false,
      setupToken: bootstrapToken,
    },
  });
  if (!res.ok()) {
    throw new Error(
      `Fallback first-run setup failed: ${res.status()} ${await res.text()}`,
    );
  }
  return {
    apiToken,
    password: E2E_CREDENTIALS.password,
    username: E2E_CREDENTIALS.username,
  };
}

async function completeSetupWizard(
  page: Page,
  bootstrapToken: string,
  options: CompleteSetupWizardOptions = {},
): Promise<CompleteSetupWizardResult> {
  const completionTarget = options.completionTarget ?? "sources";
  if (!bootstrapToken) {
    throw new Error(
      "Pulse requires first-run setup but no bootstrap token is available",
    );
  }

  await page.goto("/");
  await waitForAppShell(page);

  const wizard = page.getByRole("main", { name: "Pulse Setup Wizard" });
  await expect(wizard).toBeVisible();

  const completionHeading = wizard.getByRole("heading", {
    name: /choose your first infrastructure source|first monitored system connected/i,
  });
  const addInfrastructureButton = wizard.getByRole("button", {
    name: "Add infrastructure",
    exact: true,
  });
  const installPulseAgentButton = wizard.getByRole("button", {
    name: "Install Pulse Agent",
    exact: true,
  });
  const securityStepHeading = wizard.getByRole("heading", {
    name: /secure pulse|create your admin account/i,
  });
  const unlockButton = wizard.getByRole("button", {
    name: /verify bootstrap token|continue to setup|continue to security/i,
  });
  const finishButton = wizard.getByRole("button", {
    name: /open infrastructure|skip for now/i,
  });
  const bootstrapTokenInput = page.getByPlaceholder(
    "Paste your bootstrap token",
  );

  const enterBootstrapToken = async () => {
    await expect(bootstrapTokenInput).toBeVisible();
    await bootstrapTokenInput.click();
    await bootstrapTokenInput.fill("");
    await bootstrapTokenInput.fill(bootstrapToken);
    if ((await bootstrapTokenInput.inputValue()) !== bootstrapToken) {
      await bootstrapTokenInput.fill("");
      await bootstrapTokenInput.pressSequentially(bootstrapToken, { delay: 1 });
    }
    await expect(bootstrapTokenInput).toHaveValue(bootstrapToken);
  };

  const clickContinueIfReady = async (): Promise<boolean> => {
    if (!(await unlockButton.isVisible({ timeout: 250 }).catch(() => false))) {
      return false;
    }
    if (!(await unlockButton.isEnabled().catch(() => false))) {
      if (
        !(await bootstrapTokenInput
          .isVisible({ timeout: 250 })
          .catch(() => false))
      ) {
        return false;
      }
      if ((await bootstrapTokenInput.inputValue().catch(() => "")) !== bootstrapToken) {
        await enterBootstrapToken();
      }
    }
    if (!(await unlockButton.isEnabled().catch(() => false))) {
      return false;
    }
    try {
      await unlockButton.click({ timeout: 1_000 });
      return true;
    } catch {
      return false;
    }
  };

  await enterBootstrapToken();

  const detectWizardStep = async (): Promise<
    "security" | "completion" | "pending"
  > => {
    if (
      (await completionHeading
        .isVisible({ timeout: 100 })
        .catch(() => false)) ||
      (await addInfrastructureButton
        .isVisible({ timeout: 100 })
        .catch(() => false)) ||
      (await installPulseAgentButton
        .isVisible({ timeout: 100 })
        .catch(() => false))
    ) {
      return "completion";
    }
    if (
      await securityStepHeading.isVisible({ timeout: 100 }).catch(() => false)
    ) {
      return "security";
    }
    return "pending";
  };

  // Prefer the explicit verify action in tests. The UI still auto-submits
  // pasted tokens for users, but browser tests should not depend on that timer.
  let wizardStep = await detectWizardStep();
  if (wizardStep === "pending" && (await clickContinueIfReady())) {
    await expect
      .poll(detectWizardStep, { timeout: 30_000 })
      .not.toBe("pending")
      .catch(() => {});
    wizardStep = await detectWizardStep();
  }
  if (wizardStep === "pending") {
    await expect
      .poll(detectWizardStep, { timeout: 30_000 })
      .not.toBe("pending")
      .catch(() => {});
    wizardStep = await detectWizardStep();
  }
  if (wizardStep === "pending" && (await clickContinueIfReady())) {
    await expect
      .poll(detectWizardStep, { timeout: 30_000 })
      .not.toBe("pending");
    wizardStep = await detectWizardStep();
  }

  const onSecurityStep = wizardStep === "security";
  let onCompleteStep = wizardStep === "completion";

  if (!onSecurityStep && !onCompleteStep) {
    throw new Error("Setup wizard did not reach security or completion step");
  }

  if (onSecurityStep) {
    const customPasswordButton = wizard.getByRole("button", {
      name: "Custom password",
      exact: true,
    });
    await expect(customPasswordButton).toBeVisible();
    await customPasswordButton.click({ timeout: 10_000 });
    await expect(
      wizard.getByPlaceholder("Password (min 12 characters)"),
    ).toBeVisible();

    await wizard
      .locator('input[type="text"]')
      .first()
      .fill(E2E_CREDENTIALS.username);
    await wizard
      .locator('input[type="password"]')
      .nth(0)
      .fill(E2E_CREDENTIALS.password);
    await wizard
      .locator('input[type="password"]')
      .nth(1)
      .fill(E2E_CREDENTIALS.password);

    await wizard.getByRole("button", { name: /create account/i }).click();
    await expect
      .poll(async () => {
        if (
          (await completionHeading
            .isVisible({ timeout: 250 })
            .catch(() => false)) ||
          (await addInfrastructureButton
            .isVisible({ timeout: 250 })
            .catch(() => false)) ||
          (await installPulseAgentButton
            .isVisible({ timeout: 250 })
            .catch(() => false))
        ) {
          return "complete";
        }
        if (
          completionTarget !== "none" &&
          SETUP_COMPLETION_HANDOFFS[completionTarget].urlPattern.test(
            page.url(),
          )
        ) {
          return "handoff";
        }
        return "pending";
      })
      .not.toBe("pending");
    await expect
      .poll(
        async () => {
          const status = await getSecurityStatus(page).catch(() => ({}));
          return status.hasAuthentication === true;
        },
        { timeout: 10_000 },
      )
      .toBe(true);
    onCompleteStep = true;
  }

  if (onCompleteStep && completionTarget !== "none") {
    const completionAction = SETUP_COMPLETION_HANDOFFS[completionTarget];
    if (!completionAction.urlPattern.test(page.url())) {
      const completionButton = wizard.getByRole("button", {
        name: completionAction.buttonName,
        exact: true,
      });
      const completionVisible = await completionButton
        .isVisible({ timeout: 500 })
        .catch(() => false);

      if (completionVisible) {
        let clickedCompletionButton = false;
        try {
          await completionButton.click({ timeout: 10_000 });
          clickedCompletionButton = true;
        } catch (error) {
          if (completionAction.urlPattern.test(page.url())) {
            clickedCompletionButton = true;
          } else if (completionTarget !== "agent") {
            throw error;
          }
        }

        if (clickedCompletionButton) {
          await page
            .waitForURL(completionAction.urlPattern, { timeout: 10_000 })
            .catch(() => {});
        }
      }

      if (
        completionTarget === "sources" &&
        !completionAction.urlPattern.test(page.url()) &&
        /\/settings\/infrastructure$/.test(page.url())
      ) {
        await page
          .getByRole("button", { name: "Add infrastructure", exact: true })
          .first()
          .click({ timeout: 10_000 });
        await expect(page).toHaveURL(completionAction.urlPattern);
      }

      if (
        completionTarget === "sources" &&
        !completionAction.urlPattern.test(page.url())
      ) {
        const addDialog = page.getByRole("dialog", {
          name: "Add infrastructure",
        });
        if (await addDialog.isVisible({ timeout: 500 }).catch(() => false)) {
          await expect(page).toHaveURL(completionAction.urlPattern);
        }
      }

      if (
        completionTarget === "agent" &&
        !completionAction.urlPattern.test(page.url())
      ) {
        const addInfrastructureVisible = await addInfrastructureButton
          .isVisible({ timeout: 500 })
          .catch(() => false);

        if (addInfrastructureVisible) {
          await addInfrastructureButton.click({ timeout: 10_000 });
          await expect(page).toHaveURL(
            SETUP_COMPLETION_HANDOFFS.sources.urlPattern,
          );

          const addDialog = page.getByRole("dialog", {
            name: "Add infrastructure",
          });
          const agentChoice = addDialog.getByRole("button", {
            name: /Install Pulse Agent/i,
          });
          await expect(agentChoice).toBeVisible();
          await agentChoice.click({ timeout: 10_000 });
          await expect(page).toHaveURL(completionAction.urlPattern);
        }
      }
    }

    if (!completionAction.urlPattern.test(page.url())) {
      throw new Error(
        `Setup wizard completion did not hand off through ${completionAction.buttonName}: ${page.url()}`,
      );
    }
  } else if (onCompleteStep) {
    const finishVisible = await finishButton
      .isVisible({ timeout: 500 })
      .catch(() => false);
    if (finishVisible) {
      await finishButton.scrollIntoViewIfNeeded();
      await finishButton.click({ timeout: 10_000 });
    }
  }

  await page.waitForLoadState("domcontentloaded");

  const handoff = await page
    .evaluate((storageKey) => {
      try {
        const raw = window.sessionStorage.getItem(storageKey);
        if (!raw) return {};
        const payload = JSON.parse(raw) as {
          apiToken?: unknown;
          password?: unknown;
          username?: unknown;
        };
        return {
          apiToken:
            typeof payload.apiToken === "string"
              ? payload.apiToken.trim()
              : "",
          password:
            typeof payload.password === "string"
              ? payload.password.trim()
              : "",
          username:
            typeof payload.username === "string"
              ? payload.username.trim()
              : "",
        };
      } catch {
        return {};
      }
    }, SETUP_HANDOFF_STORAGE_KEY)
    .catch(() => ({}));

  return {
    apiToken: handoff.apiToken || undefined,
    password: handoff.password || undefined,
    username: handoff.username || undefined,
  };
}

export async function getSecurityStatus(page: Page): Promise<SecurityStatus> {
  const res = await page.request.get("/api/security/status");
  if (!res.ok()) {
    throw new Error(`Failed to fetch security status: ${res.status()}`);
  }
  return (await res.json()) as SecurityStatus;
}

export async function maybeCompleteSetupWizard(page: Page) {
  const security = await getSecurityStatus(page);
  if (security.hasAuthentication !== false) {
    return;
  }

  const setup = await completeSetupWizard(page, E2E_CREDENTIALS.bootstrapToken);
  rememberSetupCredentials(setup);
}

// Last-resort read of the rotated bootstrap token straight from the test
// container, mirroring the docker-exec channel the entitlement bootstrap
// uses. The on-disk file is encrypted, so this goes through the CLI, which
// prints the decrypted token in a banner. Returns "" when docker (or the
// container) is unavailable.
function readContainerBootstrapToken(): string {
  const container =
    String(process.env.PULSE_E2E_PULSE_CONTAINER || "").trim() ||
    "pulse-test-server";
  try {
    const banner = execFileSync(
      "docker",
      ["exec", container, "/app/pulse", "bootstrap-token"],
      { encoding: "utf8", timeout: 15_000 },
    );
    return banner.match(/[0-9a-f]{48}/)?.[0] ?? "";
  } catch {
    return "";
  }
}

async function resetFirstRunState(
  page: Page,
  security: SecurityStatus,
): Promise<ResetFirstRunResponse> {
  if (browserBaseURLIsImplicitDevRuntime()) {
    throw new Error(
      `Refusing to reset first-run state against the implicitly resolved dev runtime at ${preferredBrowserBaseURL()}. ` +
        "This is almost certainly a developer's live Pulse, not the e2e stack. " +
        "Set PULSE_BASE_URL (or run through pretest/run-tests.sh) to target the intended instance.",
    );
  }
  const resetPath = "/api/security/dev/reset-first-run";
  const reset = (headers?: Record<string, string>) =>
    apiRequest(page, resetPath, {
      method: "POST",
      headers,
    });
  const isAuthFailure = (status: number) => status === 401 || status === 403;

  for (const candidate of primaryAPITokenCandidates()) {
    const tokenRes = await reset({ "X-API-Token": candidate.token });
    if (tokenRes.ok()) {
      return (await tokenRes.json()) as ResetFirstRunResponse;
    }
    if (!isAuthFailure(tokenRes.status())) {
      throw new Error(
        `Failed to reset first-run state: ${tokenRes.status()} ${await tokenRes.text()}`,
      );
    }

    forgetRejectedPrimaryAPIToken(candidate);
  }

  if (security.hasAuthentication !== false) {
    await login(page);
    const sessionRes = await reset();
    if (sessionRes.ok()) {
      return (await sessionRes.json()) as ResetFirstRunResponse;
    }
    throw new Error(
      `Failed to reset first-run state after login fallback: ${sessionRes.status()} ${await sessionRes.text()}`,
    );
  }

  // No authentication is configured: the instance already sits in the
  // post-reset state, but the on-disk bootstrap token may have been rotated
  // by an earlier interrupted reset, so the seeded value cannot be assumed.
  // Recover by completing setup with a valid bootstrap token (seeded value
  // first, container file as fallback), then mint a fresh bootstrap token
  // through an authenticated reset.
  let recovered: CompleteSetupWizardResult | null = null;
  try {
    recovered = await completeFirstRunViaAPI(page, E2E_CREDENTIALS.bootstrapToken);
  } catch {
    const containerToken = readContainerBootstrapToken();
    if (!containerToken) {
      throw new Error(
        "Failed to reset first-run state: no authentication is configured, the seeded bootstrap token was rejected, " +
          "and the container bootstrap token could not be read via docker exec.",
      );
    }
    recovered = await completeFirstRunViaAPI(page, containerToken);
  }
  rememberSetupCredentials(recovered);
  await login(page);
  const recoveredRes = await reset();
  if (recoveredRes.ok()) {
    return (await recoveredRes.json()) as ResetFirstRunResponse;
  }
  throw new Error(
    `Failed to reset first-run state after recovering an unauthenticated instance: ${recoveredRes.status()} ${await recoveredRes.text()}`,
  );
}

export async function ensureFirstRunExperience(
  page: Page,
  options: CompleteSetupWizardOptions = {},
) {
  await waitForPulseReady(page);
  const completionTarget = options.completionTarget ?? "sources";

  let bootstrapToken = E2E_CREDENTIALS.bootstrapToken;
  const security = await getSecurityStatus(page);
  const payload = await resetFirstRunState(page, security);
  bootstrapToken = String(payload.bootstrapToken || "").trim();
  if (bootstrapToken === "") {
    throw new Error(
      "First-run reset response did not include a bootstrap token",
    );
  }

  const firstRunLandingPattern =
    completionTarget === "none"
      ? /\/(settings\/infrastructure|proxmox|nodes|hosts|docker|infrastructure)/
      : SETUP_COMPLETION_HANDOFFS[completionTarget].urlPattern;
  const targetPath =
    completionTarget === "none"
      ? "/settings/infrastructure"
      : SETUP_COMPLETION_HANDOFFS[completionTarget].path;

  const ensureTargetVisible = async () => {
    if (completionTarget !== "sources") {
      return true;
    }
    const addDialog = page.getByRole("dialog", {
      name: "Add infrastructure",
    });
    if (await addDialog.isVisible().catch(() => false)) {
      return true;
    }
    if (/\/settings\/infrastructure$/.test(page.url())) {
      const addButton = page.getByRole("button", {
        name: "Add infrastructure",
        exact: true,
      });
      if (await addButton.isVisible().catch(() => false)) {
        await addButton.click({ timeout: 10_000 });
      }
    }
    return addDialog
      .waitFor({ state: "visible", timeout: 15_000 })
      .then(() => true)
      .catch(() => false);
  };

  const completeViaAPIFallback = async () => {
    const fallbackSetup = await completeFirstRunViaAPI(page, bootstrapToken);
    rememberSetupCredentials(fallbackSetup);
    if (!(await authenticateWithPrimaryAPIToken(page))) {
      await login(page, {
        ...E2E_CREDENTIALS,
        ...fallbackSetup,
      });
    }
    await page.goto(targetPath, { waitUntil: "domcontentloaded" });
    await waitForAppShell(page);
    await expect(page).toHaveURL(firstRunLandingPattern);
    await expect(page.getByRole("main", { name: "Pulse Setup Wizard" })).not.toBeVisible({
      timeout: 10_000,
    });
    if (!(await ensureTargetVisible())) {
      throw new Error(`First-run setup did not render target UI at ${page.url()}`);
    }
  };

  if (completionTarget === "none") {
    const setup = await completeFirstRunViaAPI(page, bootstrapToken);
    rememberSetupCredentials(setup);
    if (!(await authenticateWithPrimaryAPIToken(page))) {
      await login(page, {
        ...E2E_CREDENTIALS,
        ...setup,
      });
    }
    await page.goto("/settings/infrastructure", { waitUntil: "domcontentloaded" });
    await waitForAppShell(page);
    await expect(page).toHaveURL(firstRunLandingPattern);
    await expect(page.getByRole("main", { name: "Pulse Setup Wizard" })).not.toBeVisible({
      timeout: 10_000,
    });
    return;
  }

  for (let setupAttempt = 0; setupAttempt < 2; setupAttempt += 1) {
    let setup: CompleteSetupWizardResult;
    let usedAPIFallback = false;
    try {
      setup = await completeSetupWizard(page, bootstrapToken, {
        completionTarget,
      });
    } catch (error) {
      if (setupAttempt === 1) {
        setup = await completeFirstRunViaAPI(page, bootstrapToken);
        usedAPIFallback = true;
      } else {
        // completeSetupWizard starts every attempt at the root route. Do not
        // issue another navigation here: the app can still be finishing its
        // auth-driven redirect after setup was reset, and competing root
        // navigations leave the shared backend in first-run state when the
        // retry is interrupted.
        await page.waitForLoadState("domcontentloaded").catch(() => {});
        continue;
      }
    }
    rememberSetupCredentials(setup);

    if (usedAPIFallback) {
      if (!(await authenticateWithPrimaryAPIToken(page))) {
        await login(page, {
          ...E2E_CREDENTIALS,
          ...setup,
        });
      }
      const targetPath =
        completionTarget === "none"
          ? "/settings/infrastructure"
          : SETUP_COMPLETION_HANDOFFS[completionTarget].path;
      await page.goto(targetPath, { waitUntil: "domcontentloaded" });
      await waitForAppShell(page);
    }

    if (!firstRunLandingPattern.test(page.url())) {
      if (completionTarget !== "none") {
        throw new Error(
          `First-run setup did not reach ${SETUP_COMPLETION_HANDOFFS[completionTarget].buttonName}: ${page.url()}`,
        );
      }
      await login(page);
    }
    await expect(page).toHaveURL(firstRunLandingPattern);
    const setupWizard = page.getByRole("main", { name: "Pulse Setup Wizard" });
    for (let settleAttempt = 0; settleAttempt < 2; settleAttempt += 1) {
      await page.waitForTimeout(750);
      if (!(await setupWizard.isVisible({ timeout: 500 }).catch(() => false))) {
        break;
      }
      const status = await getSecurityStatus(page).catch(() => ({}));
      if (status.hasAuthentication !== true) {
        break;
      }
      await page.reload({ waitUntil: "domcontentloaded" });
      await waitForAppShell(page);
      await expect(page).toHaveURL(firstRunLandingPattern);
    }
    if (!(await setupWizard.isVisible({ timeout: 500 }).catch(() => false))) {
      if (!(await ensureTargetVisible())) {
        await completeViaAPIFallback();
      }
      break;
    }
    const status = await getSecurityStatus(page).catch(() => ({}));
    if (status.hasAuthentication === true) {
      await expect(setupWizard).not.toBeVisible({ timeout: 10_000 });
      break;
    }
    if (setupAttempt === 1) {
      await completeViaAPIFallback();
      break;
    }
    await page.goto("/");
  }
}

/**
 * Login as admin user
 */
export async function loginAsAdmin(page: Page) {
  await page.goto("/");
  await page.waitForSelector('input[name="username"]', { state: "visible" });
  await page.fill('input[name="username"]', E2E_CREDENTIALS.username);
  await page.fill('input[name="password"]', E2E_CREDENTIALS.password);
  await page.click('button[type="submit"]');

  // Wait for redirect to the authenticated application shell.
  await page.waitForURL(/\/(infrastructure|nodes|proxmox)/);
}

export async function login(page: Page, credentials = E2E_CREDENTIALS) {
  await page.goto("/");
  await waitForAppShell(page);

  const usernameInput = page.locator('input[name="username"]');

  const state = await Promise.race([
    usernameInput
      .waitFor({ state: "visible", timeout: 15_000 })
      .then(() => "login")
      .catch(() => undefined),
    page
      .waitForURL(AUTHENTICATED_URL, { timeout: 15_000 })
      .then(() => "authenticated")
      .catch(() => undefined),
  ]);

  if (state === "authenticated") {
    return;
  }

  if (state !== "login") {
    const url = page.url();
    const preview = ((await page.locator("body").textContent()) || "")
      .replace(/\s+/g, " ")
      .slice(0, 200);
    throw new Error(
      `Login did not render and did not redirect (url=${url}, body="${preview}")`,
    );
  }

  const loginErrorText = page
    .locator(
      "text=/Invalid username or password|Too many requests|Account locked|Failed to connect to server|Server error/i",
    )
    .first();

  const submitAndAwaitOutcome = async (): Promise<string> => {
    const loginResponsePromise = page
      .waitForResponse(
        (response) =>
          new URL(response.url()).pathname === "/api/login" &&
          response.request().method().toUpperCase() === "POST",
        { timeout: 30_000 },
      )
      .catch(() => null);

    await page.fill('input[name="username"]', credentials.username);
    await page.fill('input[name="password"]', credentials.password);
    await page.click('button[type="submit"]');

    const deadline = Date.now() + 30_000;
    while (Date.now() < deadline) {
      if (AUTHENTICATED_URL.test(page.url())) {
        return "authenticated";
      }
      const loginErrorVisible = await loginErrorText
        .isVisible()
        .catch(() => false);
      if (loginErrorVisible) {
        const message = (
          (await loginErrorText.textContent()) || "login_error"
        ).trim();
        const loginResponse = await Promise.race([
          loginResponsePromise,
          page.waitForTimeout(100).then(() => null),
        ]);
        if (loginResponse?.status() === 429) {
          return "error:Too many requests";
        }
        return `error:${message}`;
      }
      await page.waitForTimeout(250);
    }
    return "timeout waiting for authenticated app state after login submission";
  };

  const prepareFreshLoginAttempt = async (): Promise<"login" | "authenticated" | "unknown"> => {
    await waitForPulseReady(page);
    await page.goto("/");
    await waitForAppShell(page);

    const retryState = await Promise.race([
      usernameInput
        .waitFor({ state: "visible", timeout: 15_000 })
        .then(() => "login" as const)
        .catch(() => undefined),
      page
        .waitForURL(AUTHENTICATED_URL, { timeout: 15_000 })
        .then(() => "authenticated" as const)
        .catch(() => undefined),
    ]);

    return retryState ?? "unknown";
  };

  const isRetryableLoginOutcome = (outcome: string): boolean =>
    /too many|failed to connect to server|server error|timeout waiting/i.test(outcome);

  // The backend allows 10 login attempts per minute per IP and counts
  // successful ones, so bursts of session logins (worker fixtures plus
  // per-test session auth) can transiently trip the limiter. CI can also see
  // a short transport miss while the compose service is healthy-but-settling.
  // Back off and retry through a fresh login form.
  const MAX_LOGIN_ATTEMPTS = 3;
  let lastOutcome = "pending";
  for (let attempt = 1; attempt <= MAX_LOGIN_ATTEMPTS; attempt++) {
    lastOutcome = await submitAndAwaitOutcome();
    if (lastOutcome === "authenticated") {
      return;
    }
    if (attempt < MAX_LOGIN_ATTEMPTS && isRetryableLoginOutcome(lastOutcome)) {
      const backoffMs = /too many/i.test(lastOutcome) ? 15_000 : 2_000 * attempt;
      await page.waitForTimeout(backoffMs);
      const retryState = await prepareFreshLoginAttempt();
      if (retryState === "authenticated") {
        return;
      }
      if (retryState !== "login") {
        lastOutcome = `retry login form unavailable after ${lastOutcome}`;
        break;
      }
      continue;
    }
    break;
  }
  throw new Error(`Login failed: ${lastOutcome}`);
}

async function authenticateWithPrimaryAPIToken(page: Page): Promise<boolean> {
  for (const candidate of primaryAPITokenCandidates()) {
    // A URL-token navigation reaches an authenticated route before the app's
    // first API request has validated the token. Reject stale runtime tokens
    // up front so that transient route is never mistaken for a live session.
    const probe = await apiRequest(page, "/api/state", {
      headers: { "X-API-Token": candidate.token },
      timeout: 10_000,
    }).catch(() => null);
    if (!probe?.ok()) {
      if (probe && (probe.status() === 401 || probe.status() === 403)) {
        forgetRejectedPrimaryAPIToken(candidate);
      }
      continue;
    }

    await page.goto(`/?token=${encodeURIComponent(candidate.token)}`, {
      waitUntil: "domcontentloaded",
    });
    await waitForAppShell(page);

    const state = await Promise.race([
      page
        .waitForURL(AUTHENTICATED_URL, { timeout: 10_000 })
        .then(() => "authenticated")
        .catch(() => undefined),
      page
        .locator('input[name="username"]')
        .waitFor({ state: "visible", timeout: 10_000 })
        .then(() => "login")
        .catch(() => undefined),
    ]);

    if (state === "authenticated") {
      return true;
    }

    // The probe above already proved this token is valid. A transient app
    // navigation or websocket delay must not permanently evict it and force
    // later fixtures through the rate-limited password login path. Only a
    // definitive 401/403 from the token probe rejects a candidate.
    await page
      .evaluate(() => {
        window.sessionStorage.removeItem("pulse_auth");
      })
      .catch(() => {});
  }

  return false;
}

export async function ensureAuthenticated(page: Page) {
  await waitForPulseReady(page);
  await maybeCompleteSetupWizard(page);
  if (!(await authenticateWithPrimaryAPIToken(page))) {
    await login(page);
  }
  await expect(page).toHaveURL(AUTHENTICATED_URL);
}

// Deterministic cookie-session auth. Use instead of ensureAuthenticated when
// the test exercises session semantics (page.request cookies, CSRF, session
// revocation): ensureAuthenticated may satisfy itself via the
// sessionStorage-only token path, which leaves page.request unauthenticated.
export async function ensureSessionAuthenticated(page: Page) {
  await waitForPulseReady(page);
  await maybeCompleteSetupWizard(page);
  await login(page);
  await expect(page).toHaveURL(AUTHENTICATED_URL);
}

// Playwright storage states persist cookies and localStorage only. The
// primary-API-token flow authenticates through sessionStorage, so a storage
// state captured after token auth is silently unauthenticated and every test
// that loads it lands on the login screen. Storage states must therefore
// always come from the cookie-backed password login.
const sharedCookieStatePath = (): string =>
  path.resolve(
    helpersDir,
    "..",
    "tmp",
    "playwright-auth",
    "shared-cookie-session.json",
  );

async function storageStateHasLiveSession(
  browser: Browser,
  statePath: string,
): Promise<boolean> {
  const context = await browser.newContext({
    baseURL: preferredBrowserBaseURL(),
    storageState: statePath,
  });
  const page = await context.newPage();
  try {
    const res = await context.request.get("/api/state");
    if (res.status() !== 200) {
      return false;
    }
    await waitForDefaultMockRuntimeReady(page);
    return true;
  } catch {
    return false;
  } finally {
    await context.close();
  }
}

type E2ERuntimeStateResource = {
  name?: string;
  sources?: string[];
  type?: string;
};

type E2ERuntimeState = {
  connectedInfrastructure?: Array<{ name?: string }>;
  resources?: E2ERuntimeStateResource[];
};

type E2EMetricPoint = {
  timestamp?: number;
};

type E2EStorageCharts = {
  pools?: Record<string, { used?: E2EMetricPoint[] }>;
  disks?: Record<string, { temperature?: E2EMetricPoint[] }>;
};

const requiresDefaultMockRuntimeReadiness = (): boolean =>
  ["1", "true", "yes", "on"].includes(
    String(process.env.PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY || "")
      .trim()
      .toLowerCase(),
  );

const resourceHasSource = (
  resource: E2ERuntimeStateResource,
  source: string,
): boolean => resource.sources?.includes(source) === true;

async function waitForDefaultMockRuntimeReady(page: Page): Promise<void> {
  if (!requiresDefaultMockRuntimeReadiness()) {
    return;
  }

  await expect
    .poll(
      async () => {
        const response = await apiRequest(page, "/api/state").catch(() => null);
        if (!response?.ok()) {
          return false;
        }

        const state = (await response.json()) as E2ERuntimeState;
        const resources = Array.isArray(state.resources) ? state.resources : [];
        const infrastructure = Array.isArray(state.connectedInfrastructure)
          ? state.connectedInfrastructure
          : [];

        return (
          resources.some(
            (resource) =>
              resource.name === "nvme-primary" &&
              resource.type === "storage" &&
              resourceHasSource(resource, "vmware"),
          ) &&
          resources.some(
            (resource) =>
              resource.type === "k8s-cluster" &&
              resourceHasSource(resource, "kubernetes"),
          ) &&
          resources.some(
            (resource) =>
              resource.name === "tank" &&
              resourceHasSource(resource, "truenas"),
          ) &&
          resources.some((resource) => resource.type === "docker-host") &&
          resources.some((resource) => resource.type === "pbs") &&
          resources.some((resource) => resource.type === "pmg") &&
          infrastructure.some(
            (entry) => entry.name === "esxi-01.lab.local",
          )
        );
      },
      {
        message:
          "default mock inventory should be complete before browser fixtures start",
        timeout: 120_000,
        intervals: [1_000, 2_000, 5_000],
      },
    )
    .toBe(true);

  const hasDeepSeries = (points: E2EMetricPoint[] | undefined): boolean => {
    const timestamps = (points ?? [])
      .map((point) => Number(point.timestamp))
      .filter(Number.isFinite)
      .sort((left, right) => left - right);
    if (timestamps.length < 2) {
      return false;
    }
    return timestamps[timestamps.length - 1] - timestamps[0] > 5 * 24 * 60 * 60 * 1000;
  };

  // Inventory becomes available before the monitor has finished building its
  // historical mock timeline. Do not advertise the shared browser fixture as
  // ready until the deepest Core E2E chart window is queryable for both pools
  // and disks.
  await expect
    .poll(
      async () => {
        const response = await apiRequest(
          page,
          "/api/storage-charts?range=10080",
          { timeout: 60_000 },
        ).catch(() => null);
        if (!response?.ok()) {
          return false;
        }

        const charts = (await response.json()) as E2EStorageCharts;
        return (
          Object.values(charts.pools ?? {}).some((pool) =>
            hasDeepSeries(pool.used),
          ) &&
          Object.values(charts.disks ?? {}).some((disk) =>
            hasDeepSeries(disk.temperature),
          )
        );
      },
      {
        message:
          "default mock history should cover the seven-day Core E2E chart window before browser fixtures start",
        timeout: 180_000,
        intervals: [1_000, 2_000, 5_000],
      },
    )
    .toBe(true);
}

export async function createAuthenticatedStorageState(
  browser: Browser,
  storageStatePath: string,
): Promise<void> {
  // Reuse one cookie session per run so repeated fixture setups stay far
  // below the backend's per-user session cap and login rate limit.
  const sharedPath = sharedCookieStatePath();
  fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
  if (
    fs.existsSync(sharedPath) &&
    (await storageStateHasLiveSession(browser, sharedPath))
  ) {
    fs.copyFileSync(sharedPath, storageStatePath);
    return;
  }

  const context = await browser.newContext({
    baseURL: preferredBrowserBaseURL(),
  });
  const page = await context.newPage();
  try {
    await ensureSessionAuthenticated(page);
    await waitForDefaultMockRuntimeReady(page);
    await context.storageState({ path: storageStatePath });
  } finally {
    await context.close();
  }

  fs.mkdirSync(path.dirname(sharedPath), { recursive: true });
  const tempSharedPath = `${sharedPath}.${process.pid}.tmp`;
  fs.copyFileSync(storageStatePath, tempSharedPath);
  fs.renameSync(tempSharedPath, sharedPath);
}

export async function logout(page: Page) {
  const logoutButton = page.locator('button[aria-label="Logout"]').first();
  await expect(logoutButton).toBeVisible();
  await logoutButton.click();
  await page.waitForURL(/\/$/, { timeout: 15000 });
  await expect(page.locator('input[name="username"]')).toBeVisible();
}

export async function setMockMode(page: Page, enabled: boolean) {
  await waitForPulseReady(page);
  const send = () =>
    apiRequest(page, "/api/system/mock-mode", {
      method: "POST",
      data: { enabled },
      headers: { "Content-Type": "application/json" },
    });

  // Mock mode toggle can fail transiently when the backend is still
  // processing a previous toggle (e.g. between consecutive suite runs).
  // Retry up to 3 times with a short backoff, catching both HTTP errors
  // and transport-level failures (connection reset, timeout).
  let lastError: Error | null = null;
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      let res = await send();
      if (res.status() === 401) {
        await login(page);
        res = await send();
      }

      if (res.ok()) {
        const result = (await res.json()) as { enabled: boolean };
        if (enabled) {
          await waitForDefaultMockRuntimeReady(page);
        }
        return result;
      }

      lastError = new Error(`HTTP ${res.status()}: ${await res.text()}`);
    } catch (err) {
      lastError = err instanceof Error ? err : new Error(String(err));
    }

    if (attempt < 2) {
      await page.waitForTimeout(2000);
    }
  }

  throw new Error(
    `Failed to update mock mode after 3 attempts: ${lastError?.message}`,
  );
}

export async function getMockMode(page: Page) {
  await waitForPulseReady(page);
  const send = () => apiRequest(page, "/api/system/mock-mode");
  let lastError: Error | null = null;

  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      let res = await send();
      if (res.status() === 401) {
        await login(page);
        res = await send();
      }

      if (res.ok()) {
        return (await res.json()) as { enabled: boolean };
      }

      lastError = new Error(`HTTP ${res.status()}: ${await res.text()}`);
    } catch (error) {
      lastError = error instanceof Error ? error : new Error(String(error));
    }

    if (attempt < 2) {
      await page.waitForTimeout(2_000);
    }
  }

  throw new Error(
    `Failed to read mock mode after 3 attempts: ${lastError?.message}`,
  );
}

/**
 * Navigate to settings page
 */
export async function navigateToSettings(page: Page) {
  await page.goto("/settings");

  // Wait for the settings route to load. The desktop sidebar (aria-label="Settings navigation")
  // is hidden on mobile viewports (lg:flex), so we wait for the URL instead of sidebar visibility.
  await page.waitForURL(/\/settings/, { timeout: 10000 });
}

/**
 * Wait for update banner to appear
 */
export async function waitForUpdateBanner(page: Page, timeout = 30000) {
  const banner = page
    .locator('[data-testid="update-banner"], .update-banner')
    .first();
  await expect(banner).toBeVisible({ timeout });
  return banner;
}

/**
 * Click "Apply Update" button in update banner
 */
export async function clickApplyUpdate(page: Page) {
  const applyButton = page
    .locator("button")
    .filter({ hasText: /apply update/i })
    .first();
  await expect(applyButton).toBeVisible();
  await applyButton.click();
}

/**
 * Wait for update confirmation modal
 */
export async function waitForConfirmationModal(page: Page) {
  const modal = page
    .locator('[role="dialog"], .modal')
    .filter({ hasText: /confirm/i })
    .first();
  await expect(modal).toBeVisible({ timeout: 10000 });
  return modal;
}

/**
 * Confirm update in modal (check acknowledgement and click confirm)
 */
export async function confirmUpdate(page: Page) {
  // Check acknowledgement checkbox if present
  const checkbox = page.locator('input[type="checkbox"]').first();
  if (await checkbox.isVisible({ timeout: 2000 }).catch(() => false)) {
    await checkbox.check();
  }

  // Click confirm button
  const confirmButton = page
    .locator("button")
    .filter({ hasText: /confirm|proceed|continue/i })
    .first();
  await confirmButton.click();
}

/**
 * Wait for update progress modal
 */
export async function waitForProgressModal(page: Page) {
  const modal = page
    .locator('[data-testid="update-progress-modal"], [role="dialog"]')
    .filter({ hasText: /updating|progress|downloading/i })
    .first();
  await expect(modal).toBeVisible({ timeout: 10000 });
  return modal;
}

/**
 * Count visible modals on page
 */
export async function countVisibleModals(page: Page): Promise<number> {
  const modals = page
    .locator('[role="dialog"], .modal')
    .filter({ hasText: /update|progress/i });
  return await modals.count();
}

/**
 * Wait for error message in modal
 */
export async function waitForErrorInModal(page: Page, modal: any) {
  const errorText = modal.locator("text=/error|failed|invalid/i").first();
  await expect(errorText).toBeVisible({ timeout: 30000 });
  return errorText;
}

/**
 * Check that error message is user-friendly (not a raw stack trace or API error)
 */
export async function assertUserFriendlyError(errorText: string) {
  // User-friendly errors should NOT contain:
  expect(errorText).not.toMatch(/stack trace|at Object\.|Error:/i);
  expect(errorText).not.toMatch(/500 Internal Server Error/i);
  expect(errorText).not.toMatch(/\/api\//i); // No API paths

  // User-friendly errors SHOULD be concise
  expect(errorText.length).toBeLessThan(200);
}

/**
 * Dismiss modal (click close button or backdrop)
 */
export async function dismissModal(page: Page) {
  // Try close button first
  const closeButton = page
    .locator('button[aria-label="Close"], button.close, button')
    .filter({ hasText: /close|dismiss/i })
    .first();
  if (await closeButton.isVisible({ timeout: 2000 }).catch(() => false)) {
    await closeButton.click();
    return;
  }

  // Try ESC key
  await page.keyboard.press("Escape");
}

/**
 * Wait for progress to reach a certain percentage
 */
export async function waitForProgress(
  page: Page,
  modal: any,
  minPercent: number,
) {
  await page.waitForFunction(
    ({ modalSelector, min }) => {
      const modal = document.querySelector(modalSelector);
      if (!modal) return false;

      // Check for progress bar or percentage text
      const progressBar = modal.querySelector('[role="progressbar"]');
      if (progressBar) {
        const value = progressBar.getAttribute("aria-valuenow");
        return value && parseInt(value) >= min;
      }

      // Check for percentage text
      const text = modal.textContent || "";
      const match = text.match(/(\d+)%/);
      return match && parseInt(match[1]) >= min;
    },
    { modalSelector: '[role="dialog"]', min: minPercent },
    { timeout: 30000 },
  );
}

/**
 * Restart test environment with specific mock configuration
 */
export async function restartWithMockConfig(config: {
  checksumError?: boolean;
  networkError?: boolean;
  rateLimit?: boolean;
  staleRelease?: boolean;
}) {
  // This would be implemented by CI/test runner to restart containers
  // with new environment variables
  console.log("Mock config:", config);
}

/**
 * Reset test environment to clean state
 */
export async function resetTestEnvironment() {
  // Clear any cached update checks
  // Reset database state
  // Restart services
}

// Shared cookie-less request context for explicit token-auth calls. Kept for
// the worker lifetime because disposing it would invalidate in-flight
// response bodies.
let tokenAuthRequestContext: APIRequestContext | null = null;

async function getTokenAuthRequestContext(): Promise<APIRequestContext> {
  if (!tokenAuthRequestContext) {
    tokenAuthRequestContext = await playwrightRequest.newContext();
  }
  return tokenAuthRequestContext;
}

/**
 * Make API request to Pulse backend
 */
export async function apiRequest(
  page: Page,
  endpoint: string,
  options: any = {},
) {
  const baseURL = preferredPlaywrightRouteBaseURL();

  const method = String(options.method || "GET").toUpperCase();
  const headers = { ...(options.headers || {}) } as Record<string, string>;
  const hasNonSessionAuth =
    (typeof headers.Authorization === "string" &&
      /^(basic|bearer)\s+/i.test(headers.Authorization)) ||
    typeof headers["X-API-Token"] === "string";

  if (hasNonSessionAuth) {
    // Explicit token credentials must be exercised without the page's
    // ambient session cookie: the backend intentionally CSRF-rejects any
    // mutation that arrives with a session cookie and no CSRF token, even
    // when a valid Authorization header is present.
    const context = await getTokenAuthRequestContext();
    return context.fetch(`${baseURL}${endpoint}`, {
      ...options,
      headers,
    });
  }

  const cookies = await page.context().cookies(baseURL);
  const hasSessionCookie = cookies.some(
    (cookie) =>
      cookie.name === "pulse_session" ||
      cookie.name === "__Host-pulse_session",
  );
  if (!hasSessionCookie) {
    const primaryAPIToken = configuredPrimaryAPIToken();
    if (primaryAPIToken) {
      // ensureAuthenticated may intentionally use the browser-only primary
      // token path. Carry the same validated credential into API requests
      // without introducing an ambient cookie that would activate CSRF.
      headers["X-API-Token"] = primaryAPIToken;
      const context = await getTokenAuthRequestContext();
      return context.fetch(`${baseURL}${endpoint}`, {
        ...options,
        headers,
      });
    }
  }

  if (!["GET", "HEAD", "OPTIONS"].includes(method)) {
    const hasCSRFHeader = Object.keys(headers).some(
      (name) => name.toLowerCase() === "x-csrf-token",
    );
    if (!hasCSRFHeader) {
      const csrfCookie = cookies.find(
        (cookie) => cookie.name === "pulse_csrf",
      )?.value;
      if (csrfCookie) {
        headers["X-CSRF-Token"] = csrfCookie;
      }
    }
  }

  const response = await page.request.fetch(`${baseURL}${endpoint}`, {
    ...options,
    headers,
  });
  return response;
}

export async function isMultiTenantEnabled(page: Page): Promise<boolean> {
  const orgsRes = await apiRequest(page, "/api/orgs");
  return orgsRes.ok();
}

const toOrgID = (displayName: string) => {
  const base =
    displayName
      .toLowerCase()
      .replace(/[^a-z0-9-]+/g, "-")
      .replace(/^-+|-+$/g, "")
      .slice(0, 36) || "org";
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`;
  return `${base}-${suffix}`.slice(0, 64);
};

export async function createOrg(
  page: Page,
  displayName: string,
): Promise<{ id: string }> {
  const res = await apiRequest(page, "/api/orgs", {
    method: "POST",
    data: { id: toOrgID(displayName), displayName },
    headers: { "Content-Type": "application/json" },
  });
  if (!res.ok())
    throw new Error(
      `Failed to create org: ${res.status()} ${await res.text()}`,
    );

  const payload = (await res.json()) as { id?: string };
  if (!payload.id) {
    throw new Error("Failed to create org: response missing org id");
  }

  return { id: payload.id };
}

export async function deleteOrg(page: Page, orgId: string): Promise<void> {
  const res = await apiRequest(page, `/api/orgs/${encodeURIComponent(orgId)}`, {
    method: "DELETE",
  });
  if (!res.ok() && res.status() !== 404) {
    throw new Error(
      `Failed to delete org: ${res.status()} ${await res.text()}`,
    );
  }
}

export async function switchOrg(page: Page, orgId: string): Promise<void> {
  await page.evaluate((id) => {
    window.sessionStorage.setItem("pulse_org_id", id);
    window.localStorage.setItem("pulse_org_id", id);
    document.cookie = `pulse_org_id=${encodeURIComponent(id)}; Path=/; SameSite=Lax`;
  }, orgId);
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitForAppShell(page);
  await expect(page.getByRole("combobox", { name: "Organization" })).toHaveValue(
    orgId,
    { timeout: 20_000 },
  );
}

/**
 * Check for updates via API
 */
export async function checkForUpdatesAPI(
  page: Page,
  channel: "stable" | "rc" = "stable",
) {
  const response = await apiRequest(
    page,
    `/api/updates/check?channel=${channel}`,
  );
  return response.json();
}

/**
 * Apply update via API
 */
export async function applyUpdateAPI(page: Page, downloadUrl: string) {
  const response = await apiRequest(page, "/api/updates/apply", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    data: { url: downloadUrl },
  });
  return response;
}

/**
 * Get update status via API
 */
export async function getUpdateStatusAPI(page: Page) {
  const response = await apiRequest(page, "/api/updates/status");
  return response.json();
}

/**
 * Measure time between events
 */
export class Timer {
  private start: number;

  constructor() {
    this.start = Date.now();
  }

  elapsed(): number {
    return Date.now() - this.start;
  }

  reset() {
    this.start = Date.now();
  }
}

/**
 * Poll until condition is met
 */
export async function pollUntil<T>(
  fn: () => Promise<T>,
  condition: (result: T) => boolean,
  options: { timeout?: number; interval?: number } = {},
): Promise<T> {
  const timeout = options.timeout || 30000;
  const interval = options.interval || 1000;
  const start = Date.now();

  while (Date.now() - start < timeout) {
    const result = await fn();
    if (condition(result)) {
      return result;
    }
    await new Promise((resolve) => setTimeout(resolve, interval));
  }

  throw new Error(`Polling timed out after ${timeout}ms`);
}
