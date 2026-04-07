import { Browser, Page, Request, expect } from "@playwright/test";
import { preferredBrowserBaseURL, readRuntimeState } from "./runtime-defaults";

const runtimePrimaryAPIToken = (): string => {
  const parsed = readRuntimeState();
  return typeof parsed?.primaryAPIToken === "string"
    ? parsed.primaryAPIToken.trim()
    : "";
};

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
  primaryApiToken:
    process.env.PULSE_E2E_PRIMARY_API_TOKEN || runtimePrimaryAPIToken(),
  username: process.env.PULSE_E2E_USERNAME || ADMIN_CREDENTIALS.username,
  password: process.env.PULSE_E2E_PASSWORD || ADMIN_CREDENTIALS.password,
};

async function waitForAppShell(page: Page, timeoutMs = 20_000) {
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

type SetupCompletionTarget = "install" | "platforms" | "none";

type CompleteSetupWizardOptions = {
  completionTarget?: SetupCompletionTarget;
};

const SETUP_COMPLETION_HANDOFFS: Record<
  Exclude<SetupCompletionTarget, "none">,
  { buttonName: string; urlPattern: RegExp }
> = {
  install: {
    buttonName: "Open Infrastructure Install",
    urlPattern: /\/settings\/infrastructure\/install/,
  },
  platforms: {
    buttonName: "Open Platform connections",
    urlPattern: /\/settings\/infrastructure\/platforms/,
  },
};

async function completeSetupWizard(
  page: Page,
  bootstrapToken: string,
  options: CompleteSetupWizardOptions = {},
) {
  const completionTarget = options.completionTarget ?? "install";
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
    name: /install your first monitored host|first monitored host connected/i,
  });
  const openInstallWorkspaceButton = wizard.getByRole("button", {
    name: "Open Infrastructure Install",
    exact: true,
  });
  const openPlatformConnectionsButton = wizard.getByRole("button", {
    name: "Open Platform connections",
    exact: true,
  });
  const secureDashboardHeading = wizard.getByText("Secure Your Dashboard");
  const continueButton = wizard.getByRole("button", {
    name: /verify bootstrap token|continue to setup|continue/i,
  });
  const finishButton = wizard.getByRole("button", {
    name: /go to dashboard|skip for now/i,
  });
  const bootstrapTokenInput = page.getByPlaceholder(
    "Paste your bootstrap token",
  );

  await bootstrapTokenInput.click();
  await bootstrapTokenInput.fill("");
  await bootstrapTokenInput.pressSequentially(bootstrapToken, { delay: 10 });

  const detectWizardStep = async (): Promise<
    "security" | "completion" | "pending"
  > => {
    if (
      (await completionHeading
        .isVisible({ timeout: 100 })
        .catch(() => false)) ||
      (await openInstallWorkspaceButton
        .isVisible({ timeout: 100 })
        .catch(() => false)) ||
      (await openPlatformConnectionsButton
        .isVisible({ timeout: 100 })
        .catch(() => false))
    ) {
      return "completion";
    }
    if (
      await secureDashboardHeading
        .isVisible({ timeout: 100 })
        .catch(() => false)
    ) {
      return "security";
    }
    return "pending";
  };

  // The welcome step now prefers auto-submit once the pasted bootstrap token
  // is long enough. Only fall back to the explicit verify button if the step
  // stays put after that auto-submit window.
  let wizardStep = await detectWizardStep();
  if (wizardStep === "pending") {
    await expect
      .poll(detectWizardStep, { timeout: 10_000 })
      .not.toBe("pending")
      .catch(() => {});
    wizardStep = await detectWizardStep();
  }
  if (
    wizardStep === "pending" &&
    (await continueButton.isVisible({ timeout: 250 }).catch(() => false)) &&
    (await continueButton.isEnabled().catch(() => false))
  ) {
    await continueButton.click({ timeout: 1_000 }).catch(() => {});
    await expect
      .poll(detectWizardStep, { timeout: 10_000 })
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
      name: /custom password/i,
    });
    if (
      await customPasswordButton.isVisible({ timeout: 4000 }).catch(() => false)
    ) {
      let clickedCustomPassword = false;
      for (let attempt = 0; attempt < 3; attempt++) {
        try {
          await customPasswordButton.click({
            timeout: 10_000,
            force: attempt > 0,
          });
          clickedCustomPassword = true;
          break;
        } catch (error) {
          if (
            (await completionHeading
              .isVisible({ timeout: 250 })
              .catch(() => false)) ||
            (await openInstallWorkspaceButton
              .isVisible({ timeout: 250 })
              .catch(() => false)) ||
            (await openPlatformConnectionsButton
              .isVisible({ timeout: 250 })
              .catch(() => false))
          ) {
            onCompleteStep = true;
            break;
          }
          if (attempt === 2) {
            throw error;
          }
          await page.waitForTimeout(200);
        }
      }

      if (!onCompleteStep && clickedCustomPassword) {
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
              (await openInstallWorkspaceButton
                .isVisible({ timeout: 250 })
                .catch(() => false)) ||
              (await openPlatformConnectionsButton
                .isVisible({ timeout: 250 })
                .catch(() => false))
            ) {
              return "complete";
            }
            if (
              completionTarget === "install" &&
              SETUP_COMPLETION_HANDOFFS.install.urlPattern.test(page.url())
            ) {
              return "handoff";
            }
            return "pending";
          })
          .not.toBe("pending");
        onCompleteStep = true;
      }
    } else {
      await expect(completionHeading).toBeVisible();
      onCompleteStep = true;
    }
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
        await completionButton.scrollIntoViewIfNeeded();
        await completionButton.click({ timeout: 10_000 });
        await expect(page).toHaveURL(completionAction.urlPattern);
      }
    }

    if (!completionAction.urlPattern.test(page.url())) {
      throw new Error(
        `Setup wizard completion did not hand off to ${completionAction.buttonName}: ${page.url()}`,
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

  await completeSetupWizard(page, E2E_CREDENTIALS.bootstrapToken);
}

export async function ensureFirstRunExperience(
  page: Page,
  options: CompleteSetupWizardOptions = {},
) {
  await page.addInitScript(() => {
    localStorage.setItem("pulse_whats_new_v2_shown", "true");
  });
  await waitForPulseReady(page);
  const completionTarget = options.completionTarget ?? "install";

  let bootstrapToken = E2E_CREDENTIALS.bootstrapToken;
  const security = await getSecurityStatus(page);
  let resetRes = await apiRequest(page, "/api/security/dev/reset-first-run", {
    method: "POST",
    headers: E2E_CREDENTIALS.primaryApiToken
      ? { "X-API-Token": E2E_CREDENTIALS.primaryApiToken }
      : undefined,
  });
  if (
    resetRes.status() === 401 &&
    !E2E_CREDENTIALS.primaryApiToken &&
    security.hasAuthentication !== false
  ) {
    await login(page);
    resetRes = await apiRequest(page, "/api/security/dev/reset-first-run", {
      method: "POST",
    });
  }
  if (!resetRes.ok()) {
    throw new Error(
      `Failed to reset first-run state: ${resetRes.status()} ${await resetRes.text()}`,
    );
  }

  const payload = (await resetRes.json()) as ResetFirstRunResponse;
  bootstrapToken = String(payload.bootstrapToken || "").trim();
  if (bootstrapToken === "") {
    throw new Error(
      "First-run reset response did not include a bootstrap token",
    );
  }

  await completeSetupWizard(page, bootstrapToken, { completionTarget });
  const firstRunLandingPattern =
    completionTarget === "platforms"
      ? SETUP_COMPLETION_HANDOFFS.platforms.urlPattern
      : /\/(settings\/infrastructure\/install|proxmox|dashboard|nodes|hosts|docker|infrastructure)/;
  if (!firstRunLandingPattern.test(page.url())) {
    if (completionTarget === "platforms") {
      throw new Error(
        `First-run setup did not reach Platform connections: ${page.url()}`,
      );
    }
    await login(page);
  }
  await expect(page).toHaveURL(firstRunLandingPattern);
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

  // Wait for redirect to dashboard
  await page.waitForURL(/\/(dashboard|nodes|proxmox)/);
}

export async function login(page: Page, credentials = E2E_CREDENTIALS) {
  await page.goto("/");
  await waitForAppShell(page);

  const authenticatedURL =
    /\/(proxmox|dashboard|nodes|hosts|docker|infrastructure)/;
  const usernameInput = page.locator('input[name="username"]');

  const state = await Promise.race([
    usernameInput
      .waitFor({ state: "visible", timeout: 15_000 })
      .then(() => "login")
      .catch(() => undefined),
    page
      .waitForURL(authenticatedURL, { timeout: 15_000 })
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

  await page.fill('input[name="username"]', credentials.username);
  await page.fill('input[name="password"]', credentials.password);
  await page.click('button[type="submit"]');

  await expect
    .poll(
      async () => {
        const url = page.url();
        if (authenticatedURL.test(url)) {
          return "authenticated";
        }

        const loginErrorVisible = await loginErrorText
          .isVisible()
          .catch(() => false);
        if (loginErrorVisible) {
          const message = (
            (await loginErrorText.textContent()) || "login_error"
          ).trim();
          return `error:${message}`;
        }

        const stillShowingLogin = await usernameInput
          .isVisible()
          .catch(() => false);
        if (stillShowingLogin) {
          return "login";
        }

        return "pending";
      },
      {
        timeout: 30_000,
        message:
          "Timed out waiting for authenticated app state after login submission",
      },
    )
    .toBe("authenticated");
}

/**
 * Dismiss the WhatsNew modal that appears on first visit by marking it as seen
 * in localStorage. This prevents the "fixed inset-0 z-50" overlay from blocking
 * clicks (logout button, row clicks, etc.) in tests.
 */
export async function dismissWhatsNewModal(page: Page): Promise<void> {
  await page.evaluate(() => {
    localStorage.setItem("pulse_whats_new_v2_shown", "true");
  });
}

export async function ensureAuthenticated(page: Page) {
  // Pre-set the WhatsNew modal localStorage key via an init script that runs before
  // any page script on every navigation. This prevents the "fixed inset-0 z-50"
  // overlay from appearing and blocking clicks (logout, row taps, etc.) in tests.
  await page.addInitScript(() => {
    localStorage.setItem("pulse_whats_new_v2_shown", "true");
  });
  await waitForPulseReady(page);
  await maybeCompleteSetupWizard(page);
  await login(page);
  await expect(page).toHaveURL(
    /\/(proxmox|dashboard|nodes|hosts|docker|infrastructure)/,
  );
}

export async function createAuthenticatedStorageState(
  browser: Browser,
  storageStatePath: string,
): Promise<void> {
  const context = await browser.newContext({
    baseURL: preferredBrowserBaseURL(),
  });
  const page = await context.newPage();
  try {
    await ensureAuthenticated(page);
    await context.storageState({ path: storageStatePath });
  } finally {
    await context.close();
  }
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
        return (await res.json()) as { enabled: boolean };
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

/**
 * Make API request to Pulse backend
 */
export async function apiRequest(
  page: Page,
  endpoint: string,
  options: any = {},
) {
  const baseURL = preferredBrowserBaseURL().replace(/\/+$/, "");

  const method = String(options.method || "GET").toUpperCase();
  const headers = { ...(options.headers || {}) } as Record<string, string>;
  const hasNonSessionAuth =
    (typeof headers.Authorization === "string" &&
      /^(basic|bearer)\s+/i.test(headers.Authorization)) ||
    typeof headers["X-API-Token"] === "string";

  if (!hasNonSessionAuth && !["GET", "HEAD", "OPTIONS"].includes(method)) {
    const hasCSRFHeader = Object.keys(headers).some(
      (name) => name.toLowerCase() === "x-csrf-token",
    );
    if (!hasCSRFHeader) {
      const cookies = await page.context().cookies(baseURL);
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
  await page.reload();
  await page.waitForLoadState("networkidle");
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
