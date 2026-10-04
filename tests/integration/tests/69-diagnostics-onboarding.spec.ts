import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base, type Page } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

type DiagnosticsTraffic = {
  reads: number;
  writes: string[];
};

type OverflowAudit = {
  viewportWidth: number;
  pageWidth: number;
  overflowPx: number;
  offenders: Array<{ tag: string; className: string; overflow: number }>;
};

const DIAGNOSTICS_PAYLOAD = {
  version: "6.0.0",
  runtime: "go",
  uptime: 3600,
  nodes: [],
  pbs: [],
  system: {
    os: "linux",
    arch: "amd64",
    goVersion: "go1.25",
    numCPU: 8,
    numGoroutine: 32,
    memoryMB: 128,
  },
  metricsStore: {
    enabled: true,
    status: "healthy",
    dbSize: 4 * 1024 * 1024,
    rawCount: 12,
    minuteCount: 24,
    hourlyCount: 36,
    dailyCount: 48,
    totalPoints: 120,
    bufferSize: 0,
    notes: [],
  },
  commercialFunnel: {
    enabled: true,
    status: "active",
    windowDays: 30,
    summary: {
      pricing_viewed: 3,
      paywall_viewed: 0,
      trial_started: 1,
      upgrade_clicked: 0,
      checkout_clicked: 2,
      checkout_started: 2,
      checkout_completed: 1,
      license_activated: 1,
      license_activation_failed: 0,
      period: {
        from: "2026-03-19T00:00:00Z",
        to: "2026-04-18T00:00:00Z",
      },
    },
    daily: [
      {
        day: "2026-04-17",
        pricing_viewed: 1,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 1,
        checkout_started: 1,
        checkout_completed: 0,
        license_activated: 0,
        license_activation_failed: 0,
      },
    ],
    surfaces: [
      {
        key: "settings_self_hosted_billing_compare_prompt",
        pricing_viewed: 0,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 2,
        checkout_started: 0,
        checkout_completed: 0,
        license_activated: 0,
        license_activation_failed: 0,
      },
    ],
    capabilities: [
      {
        key: "self_hosted_plan",
        pricing_viewed: 3,
        paywall_viewed: 0,
        trial_started: 0,
        upgrade_clicked: 0,
        checkout_clicked: 2,
        checkout_started: 2,
        checkout_completed: 1,
        license_activated: 1,
        license_activation_failed: 0,
      },
    ],
    notes: [
      "Local pricing and activation events show at least one completed conversion.",
    ],
  },
  infrastructureOnboarding: {
    enabled: true,
    status: "warning",
    windowDays: 30,
    summary: {
      opened: 4,
      api_path_selected: 2,
      agent_path_selected: 1,
      probe_detected: 1,
      probe_no_match: 2,
      probe_error: 0,
      catalog_selected: 2,
      credentials_opened: 1,
      period: {
        from: "2026-03-19T00:00:00Z",
        to: "2026-04-18T00:00:00Z",
      },
    },
    daily: [
      {
        day: "2026-04-17",
        opened: 2,
        api_path_selected: 1,
        agent_path_selected: 1,
        probe_detected: 0,
        probe_no_match: 1,
        probe_error: 0,
        catalog_selected: 1,
        credentials_opened: 0,
      },
      {
        day: "2026-04-18",
        opened: 2,
        api_path_selected: 1,
        agent_path_selected: 0,
        probe_detected: 1,
        probe_no_match: 1,
        probe_error: 0,
        catalog_selected: 1,
        credentials_opened: 1,
      },
    ],
    paths: [
      { key: "api", count: 2 },
      { key: "agent", count: 1 },
    ],
    platforms: [{ key: "truenas", catalog_selected: 2, credentials_opened: 1 }],
    notes: [
      "More probed addresses miss than detect a supported API-backed platform.",
    ],
  },
  discovery: {
    enabled: true,
    configuredSubnet: "auto",
    scanInterval: "5m",
    lastResultServers: 3,
  },
  apiTokens: {
    enabled: true,
    tokenCount: 2,
    recommendTokenSetup: false,
    unusedTokenCount: 0,
    notes: [],
  },
  dockerAgents: {
    agentsTotal: 1,
    agentsOnline: 1,
    agentsReportingVersion: 1,
    agentsWithTokenBinding: 1,
    agentsWithoutTokenBinding: 0,
    agentsNeedingAttention: 0,
    notes: [],
  },
  alerts: {
    missingCooldown: false,
    missingGroupingWindow: false,
    notes: [],
  },
  aiChat: {
    enabled: true,
    running: true,
    healthy: true,
    assistantRuntimeConnected: true,
    notes: [],
  },
  errors: [],
  nodeSnapshots: [],
  guestSnapshots: [],
  memorySources: [],
  memorySourceBreakdown: [],
};

const EXPORT_DIAGNOSTICS_PAYLOAD = {
  ...DIAGNOSTICS_PAYLOAD,
  nodes: [
    {
      id: "node-raw-id",
      name: "pve-01",
      host: "10.0.0.5",
      type: "pve",
      authMethod: "token",
      connected: false,
      error: "dial tcp 10.0.0.5:8006: connect: connection refused",
    },
  ],
  discovery: {
    enabled: true,
    configuredSubnet: "10.0.0.0/24",
    activeSubnet: "10.0.1.0/24",
    environmentOverride: "PULSE_DISCOVERY_SUBNET=10.0.2.0/24",
    subnetAllowlist: ["10.0.0.0/24"],
    subnetBlocklist: ["10.0.3.0/24"],
    history: [
      {
        startedAt: "2026-04-17T10:00:00Z",
        completedAt: "2026-04-17T10:00:05Z",
        duration: "5s",
        durationMs: 5000,
        subnet: "10.0.0.0/24",
        serverCount: 3,
        errorCount: 1,
        blocklistLength: 1,
        status: "completed",
      },
    ],
  },
  apiTokens: {
    ...DIAGNOSTICS_PAYLOAD.apiTokens,
    tokens: [
      {
        id: "synthetic-private-token-a",
        name: "synthetic-private-purpose-a",
        hint: "synthetic-private-hint-a",
      },
      {
        id: "synthetic-private-token-b",
        name: "synthetic-private-purpose-b",
        hint: "synthetic-private-hint-b",
      },
    ],
    // Deliberately reverse token order: aliases must follow identity, not position.
    usage: [
      {
        tokenId: "synthetic-private-token-b",
        agentCount: 1,
        agents: ["synthetic-private-agent"],
      },
      {
        tokenId: "synthetic-private-token-a",
        agentCount: 1,
        agents: ["synthetic-private-agent"],
      },
    ],
  },
  dockerAgents: {
    ...DIAGNOSTICS_PAYLOAD.dockerAgents,
    attention: [
      {
        agentId: "synthetic-private-agent-id",
        name: "synthetic-private-agent",
        tokenHint: "synthetic-private-hint",
        issues: [],
      },
    ],
  },
  nodeSnapshots: [
    {
      instance: "synthetic-private-instance",
      node: "synthetic-private-node",
      memorySource: "agent",
      memory: { used: 256 },
      raw: { total: 512 },
    },
  ],
  guestSnapshots: [
    {
      instance: "synthetic-private-instance",
      node: "synthetic-private-node",
      name: "synthetic-private-guest",
      vmid: 9501,
      guestType: "vm",
      memorySource: "agent",
      memory: { used: 128 },
      raw: { hostAgentUsed: 128 },
      notes: [],
    },
  ],
  memorySources: [
    { instance: "synthetic-private-instance", scope: "node", count: 1 },
  ],
  memorySourceBreakdown: [
    {
      instance: "synthetic-private-instance",
      scope: "node",
      source: "agent",
      trust: "host",
      count: 1,
      fallback: false,
      fallbackReasons: [],
    },
  ],
  errors: ["probe failed for 10.0.0.10 after timeout"],
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        "..",
        "..",
        "tmp",
        "playwright-auth",
        `diagnostics-onboarding-${workerInfo.project.name}.json`,
      );
      fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
      await createAuthenticatedStorageState(browser, storageStatePath);
      try {
        await use(storageStatePath);
      } finally {
        fs.rmSync(storageStatePath, { force: true });
      }
    },
    { scope: "worker" },
  ],
});

async function prepareDiagnosticsRoute(
  page: Page,
  payload: typeof DIAGNOSTICS_PAYLOAD | typeof EXPORT_DIAGNOSTICS_PAYLOAD =
    DIAGNOSTICS_PAYLOAD,
): Promise<DiagnosticsTraffic> {
  const traffic: DiagnosticsTraffic = { reads: 0, writes: [] };
  page.on("request", (request) => {
    if (!["GET", "HEAD"].includes(request.method()))
      traffic.writes.push(new URL(request.url()).pathname);
  });
  await page.route("**/api/diagnostics", async (route) => {
    const requestUrl = new URL(route.request().url());
    if (
      route.request().method() !== "GET" ||
      requestUrl.pathname !== "/api/diagnostics"
    ) {
      await route.continue();
      return;
    }

    traffic.reads += 1;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(payload),
    });
  });
  return traffic;
}

async function scrollToBottom(page: Page): Promise<void> {
  const viewportHeight = await page.evaluate(() => window.innerHeight || 800);
  const step = Math.max(240, Math.floor(viewportHeight * 0.75));
  let wheelSupported = true;

  for (let i = 0; i < 20; i += 1) {
    if (wheelSupported) {
      try {
        await page.mouse.wheel(0, step);
      } catch {
        wheelSupported = false;
        await page.evaluate((deltaY) => window.scrollBy(0, deltaY), step);
      }
    } else {
      await page.evaluate((deltaY) => window.scrollBy(0, deltaY), step);
    }
    await page.waitForTimeout(60);
  }
}

async function auditHorizontalOverflow(page: Page): Promise<OverflowAudit> {
  return page.evaluate(() => {
    const viewportWidth = Math.max(
      document.documentElement.clientWidth,
      window.innerWidth || 0,
    );
    const pageWidth = Math.max(
      document.body.scrollWidth,
      document.documentElement.scrollWidth,
      document.body.offsetWidth,
      document.documentElement.offsetWidth,
    );

    const offenders = Array.from(document.querySelectorAll("body *"))
      .map((el) => {
        const rect = el.getBoundingClientRect();
        if (rect.width <= 0 || rect.height <= 0) return null;
        const style = window.getComputedStyle(el);
        if (style.position === "fixed" || style.position === "absolute")
          return null;
        const overflow = rect.right - viewportWidth;
        if (overflow <= 1) return null;
        return {
          tag: el.tagName.toLowerCase(),
          className: (el.getAttribute("class") || "").trim().slice(0, 120),
          overflow: Number(overflow.toFixed(1)),
        };
      })
      .filter(
        (
          entry,
        ): entry is { tag: string; className: string; overflow: number } =>
          Boolean(entry),
      )
      .slice(0, 8);

    return {
      viewportWidth,
      pageWidth,
      overflowPx: Number((pageWidth - viewportWidth).toFixed(1)),
      offenders,
    };
  });
}

async function openDiagnostics(page: Page): Promise<void> {
  await page.goto("/settings/support/diagnostics", {
    waitUntil: "domcontentloaded",
  });
  await page.waitForURL(/\/settings\/support\/diagnostics$/, {
    timeout: 15_000,
  });
  await expect(
    page.getByRole("heading", { name: "Diagnostics & Health" }),
  ).toBeVisible();
}

async function describedBy(
  button: ReturnType<Page["getByRole"]>,
): Promise<string> {
  return button.evaluate((element) =>
    (element.getAttribute("aria-describedby") || "")
      .split(/\s+/)
      .map((id) => document.getElementById(id)?.textContent || "")
      .join(" "),
  );
}

async function downloadJSON(
  page: Page,
  label: string,
  kind: string,
  keyboard = false,
) {
  const button = page.getByRole("button", { name: label, exact: true });
  await expect(button).toBeEnabled();
  // Register the observer before activation. Keyboard and pointer use the same
  // public accessible control, on desktop and both mobile projects.
  const pending = page.waitForEvent("download");
  if (keyboard) {
    await button.focus();
    await expect(button).toBeFocused();
    await button.press("Enter");
  } else {
    await button.click();
  }
  const download = await pending;
  expect(download.suggestedFilename()).toMatch(
    new RegExp(`^pulse-diagnostics-${kind}-\\d{4}-\\d{2}-\\d{2}\\.json$`),
  );
  const file = await download.path();
  expect(file).not.toBeNull();
  return JSON.parse(fs.readFileSync(file!, "utf8"));
}

async function assertControlsInViewport(
  page: Page,
  mobile: boolean,
): Promise<void> {
  const audit = await auditHorizontalOverflow(page);
  expect(
    audit.pageWidth,
    `Diagnostics overflow: ${JSON.stringify(audit)}`,
  ).toBeLessThanOrEqual(audit.viewportWidth + 1);
  for (const button of await page
    .getByRole("button", {
      name: /Run Diagnostics|Full \(private\)|GitHub \(review first\)/,
    })
    .all()) {
    const bounds = await button.boundingBox();
    expect(bounds).not.toBeNull();
    expect(bounds!.x).toBeGreaterThanOrEqual(0);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(
      audit.viewportWidth + 1,
    );
    if (mobile) expect(bounds!.height).toBeGreaterThanOrEqual(44);
  }
}

test.describe("Diagnostics safe collection and local exports", () => {
  test.setTimeout(180_000);

  test.afterEach(async ({ page }, testInfo) => {
    if (testInfo.status === "passed") {
      await page.screenshot({
        path: testInfo.outputPath("diagnostics-completed-state.png"),
        fullPage: true,
      });
    }
  });

  test("keeps internal analytics absent and connects safety guidance to keyboard-accessible controls", async ({
    page,
  }, testInfo) => {
    const traffic = await prepareDiagnosticsRoute(page);
    await openDiagnostics(page);
    expect(traffic.reads).toBe(0);
    await expect(
      page.getByRole("button", { name: "Full (private)", exact: true }),
    ).toHaveCount(0);
    // Both the toolbar and empty-state run actions need the same warning.
    const actions = page.getByRole("button", {
      name: "Run Diagnostics",
      exact: true,
    });
    await expect(actions).toHaveCount(2);
    for (const button of await actions.all()) {
      expect(await describedBy(button)).toMatch(
        /Do not run it during a backup, freeze\/thaw/,
      );
    }
    await expect(
      page.getByRole("link", { name: "Safe diagnostics and sharing" }),
    ).toHaveAttribute("href", /#collect-diagnostics-safely$/);
    await actions.first().focus();
    await expect(actions.first()).toBeFocused();
    await actions.first().press("Enter");
    await expect(
      page.getByText("Metrics Store", { exact: false }).first(),
    ).toBeVisible();
    expect(traffic.reads).toBe(1);
    for (const label of [
      "Commercial Funnel",
      "Infrastructure Onboarding",
      "Credentials Opened",
    ]) {
      await expect(page.getByText(label, { exact: true })).toHaveCount(0);
    }
    for (const label of ["Full (private)", "GitHub (review first)"]) {
      expect(
        await describedBy(
          page.getByRole("button", { name: label, exact: true }),
        ),
      ).toMatch(
        /Nothing is uploaded.*Keep the full file private.*Review even a sanitised file/,
      );
    }
    await assertControlsInViewport(
      page,
      testInfo.project.name.startsWith("mobile-"),
    );
  });

  test("downloads the displayed private and review-first JSON without another live request or upload", async ({
    page,
  }, testInfo) => {
    const traffic = await prepareDiagnosticsRoute(
      page,
      EXPORT_DIAGNOSTICS_PAYLOAD,
    );
    await openDiagnostics(page);
    await page
      .getByRole("button", { name: "Run Diagnostics", exact: true })
      .first()
      .click();
    await expect(
      page.getByText("Metrics Store", { exact: false }).first(),
    ).toBeVisible();
    const writesBeforeExport = [...traffic.writes];

    const full = await downloadJSON(page, "Full (private)", "full", true);
    const {
      infrastructureOnboarding: _onboarding,
      commercialFunnel: _funnel,
      ...expectedFull
    } = EXPORT_DIAGNOSTICS_PAYLOAD;
    expect(full).toEqual(expectedFull);
    expect(traffic.reads).toBe(1);
    expect(traffic.writes).toEqual(writesBeforeExport);
    await expect(
      page.getByRole("heading", {
        name: "Full diagnostics downloaded — keep this file private",
        exact: true,
      }),
    ).toBeVisible();

    const sanitised = await downloadJSON(
      page,
      "GitHub (review first)",
      "sanitized",
    );
    expect(sanitised.nodes[0]).toMatchObject({
      id: "node-1",
      name: "node-1",
      host: "node-1",
    });
    expect(sanitised.discovery).toMatchObject({
      configuredSubnet: "[REDACTED_SUBNET]",
      activeSubnet: "[REDACTED_SUBNET]",
      environmentOverride: "[REDACTED]",
      subnetAllowlist: ["[REDACTED_SUBNET]"],
      subnetBlocklist: ["[REDACTED_SUBNET]"],
      history: [
        expect.objectContaining({
          subnet: "[REDACTED_SUBNET]",
          serverCount: 3,
        }),
      ],
    });
    expect(sanitised.errors).toEqual([
      "probe failed for [REDACTED_IP] after timeout",
    ]);
    expect(sanitised.apiTokens.tokens[0]).toMatchObject({
      id: "token-1",
      name: "token-1",
      hint: "[REDACTED]",
    });
    expect(sanitised.apiTokens.usage[0].tokenId).toBe(
      sanitised.apiTokens.tokens[1].id,
    );
    expect(sanitised.apiTokens.usage[1].tokenId).toBe(
      sanitised.apiTokens.tokens[0].id,
    );
    expect(sanitised.apiTokens.usage[0].agents).toEqual(
      sanitised.apiTokens.usage[1].agents,
    );
    expect(sanitised.apiTokens.usage[0].agents[0]).toBe(
      sanitised.dockerAgents.attention[0].name,
    );
    expect(sanitised.guestSnapshots[0]).toMatchObject({
      instance: sanitised.nodeSnapshots[0].instance,
      node: sanitised.nodeSnapshots[0].node,
    });
    expect(sanitised.guestSnapshots[0].vmid).toBeUndefined();
    expect(sanitised.guestSnapshots[0].memory).toEqual(
      EXPORT_DIAGNOSTICS_PAYLOAD.guestSnapshots[0].memory,
    );
    expect(sanitised.memorySources[0].instance).toBe(
      sanitised.nodeSnapshots[0].instance,
    );
    expect(sanitised.memorySourceBreakdown[0].instance).toBe(
      sanitised.nodeSnapshots[0].instance,
    );
    for (const payload of [full, sanitised]) {
      expect(payload.infrastructureOnboarding).toBeUndefined();
      expect(payload.commercialFunnel).toBeUndefined();
      expect(payload.metricsStore).toEqual(
        EXPORT_DIAGNOSTICS_PAYLOAD.metricsStore,
      );
    }
    expect(JSON.stringify(sanitised)).not.toContain("synthetic-private-");
    expect(JSON.stringify(sanitised)).not.toMatch(/10\.0\.[0-3]\./);
    expect(traffic.reads).toBe(1);
    expect(traffic.writes).toEqual(writesBeforeExport);
    await expect(
      page.getByRole("heading", {
        name: "Sanitised diagnostics downloaded — review before sharing",
        exact: true,
      }),
    ).toBeVisible();
    // A full export after sanitisation must still be the original displayed
    // data: redaction may not mutate it.
    expect(await downloadJSON(page, "Full (private)", "full")).toEqual(
      expectedFull,
    );
    expect(traffic.reads).toBe(1);
    expect(traffic.writes).toEqual(writesBeforeExport);
    await assertControlsInViewport(
      page,
      testInfo.project.name.startsWith("mobile-"),
    );
    await page.screenshot({
      path: testInfo.outputPath("diagnostics-safe-export.png"),
      fullPage: true,
    });
  });

  test("disables exports during a refresh and retains the previous result after a failed live check", async ({
    page,
  }, testInfo) => {
    let reads = 0;
    let release: (() => void) | undefined;
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.route("**/api/diagnostics", async (route) => {
      reads += 1;
      if (reads === 1) return route.fulfill({ json: DIAGNOSTICS_PAYLOAD });
      await held;
      return route.fulfill({
        status: 400,
        json: { error: "Synthetic live check failed" },
      });
    });
    await openDiagnostics(page);
    const run = page
      .getByRole("button", { name: "Run Diagnostics", exact: true })
      .first();
    await run.click();
    const full = page.getByRole("button", {
      name: "Full (private)",
      exact: true,
    });
    const github = page.getByRole("button", {
      name: "GitHub (review first)",
      exact: true,
    });
    await expect(full).toBeEnabled();
    await run.focus();
    await run.press("Space");
    await expect(
      page.getByRole("button", { name: "Running...", exact: true }),
    ).toBeDisabled();
    await expect(full).toBeDisabled();
    await expect(github).toBeDisabled();
    expect(reads).toBe(2);
    await page.screenshot({
      path: testInfo.outputPath("diagnostics-refresh-disabled.png"),
      fullPage: true,
    });
    release!();
    await expect(
      page.getByRole("heading", {
        name: "Synthetic live check failed",
        exact: true,
      }),
    ).toBeVisible();
    await expect(full).toBeEnabled();
    await expect(github).toBeEnabled();
    const {
      infrastructureOnboarding: _onboarding,
      commercialFunnel: _funnel,
      ...expectedFull
    } = DIAGNOSTICS_PAYLOAD;
    expect(await downloadJSON(page, "Full (private)", "full")).toEqual(
      expectedFull,
    );
    expect(reads).toBe(2);
    await scrollToBottom(page);
    await assertControlsInViewport(
      page,
      testInfo.project.name.startsWith("mobile-"),
    );
  });
});
