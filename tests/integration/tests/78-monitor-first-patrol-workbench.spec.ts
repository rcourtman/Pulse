import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base, type Page } from "@playwright/test";

import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

type PatrolWorkbenchFixtureOptions = {
  findingCount?: number;
  pendingApprovalCount?: number;
  runErrorCount?: number;
  resourcesChecked?: number;
  runStatus?: "healthy" | "issues_found" | "error";
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
        `monitor-first-patrol-workbench-${workerInfo.project.name}.json`,
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

const monitoredProxmoxResource = {
  id: "node:pve-main",
  type: "agent",
  name: "pve-main",
  displayName: "pve-main",
  status: "online",
  lastSeen: "2026-06-30T08:00:00Z",
  platformType: "proxmox-pve",
  sourceType: "agent",
  sources: ["agent", "proxmox"],
  platformScopes: ["proxmox-pve"],
  metrics: {
    cpu: { value: 12, unit: "%" },
    memory: {
      used: 4_294_967_296,
      total: 17_179_869_184,
      percent: 25,
      unit: "bytes",
    },
    disk: {
      used: 68_719_476_736,
      total: 274_877_906_944,
      percent: 25,
      unit: "bytes",
    },
  },
  proxmox: {
    nodeName: "pve-main",
    clusterName: "homelab",
    temperature: 42,
  },
  agent: {
    agentId: "agent-pve-main",
    agentVersion: "6.0.0",
    hostname: "pve-main",
    osName: "Proxmox VE",
    uptimeSeconds: 86_400,
  },
};

const buildRuntimeState = () => ({
  connectedInfrastructure: [],
  metrics: [],
  performance: {
    apiCallDuration: {},
    lastPollDuration: 0,
    pollingStartTime: "",
    totalApiCalls: 0,
    failedApiCalls: 0,
    cacheHits: 0,
    cacheMisses: 0,
  },
  connectionHealth: {},
  stats: {
    startTime: "2026-06-30T08:00:00Z",
    uptime: 3600,
    pollingCycles: 1,
    webSocketClients: 1,
    version: "6.0.0-rc.7",
  },
  activeAlerts: [],
  recentlyResolved: [],
  pveTagColors: {},
  pveTagStyles: {},
  lastUpdate: Date.parse("2026-06-30T08:00:00Z"),
  resources: [monitoredProxmoxResource],
});

const buildRunHistory = (options: Required<PatrolWorkbenchFixtureOptions>) => [
  {
    id: options.findingCount > 0 ? "run-active-work" : "run-calm-day",
    started_at: "2026-06-30T08:05:00Z",
    completed_at: "2026-06-30T08:05:42Z",
    duration_ms: 42_000,
    type: "full",
    trigger_reason: "scheduled",
    resources_checked: options.resourcesChecked,
    nodes_checked: 1,
    guests_checked: 0,
    docker_checked: 0,
    storage_checked: 0,
    hosts_checked: 1,
    pbs_checked: 0,
    pmg_checked: 0,
    kubernetes_checked: 0,
    new_findings: options.findingCount,
    existing_findings: 0,
    rejected_findings: 0,
    resolved_findings: 0,
    auto_fix_count: 0,
    findings_summary:
      options.findingCount > 0 ? "1 active finding" : "No active findings",
    finding_ids: options.findingCount > 0 ? ["finding-active-work"] : [],
    error_count: options.runErrorCount,
    status: options.runStatus,
    triage_flags: 0,
    tool_call_count: 1,
  },
];

const buildPatrolStatus = (
  options: Required<PatrolWorkbenchFixtureOptions>,
) => ({
  runtime_state: "active",
  running: false,
  enabled: true,
  last_patrol_at: "2026-06-30T08:05:42Z",
  next_patrol_at: "2099-06-30T14:05:42Z",
  last_duration_ms: 42_000,
  resources_checked: options.resourcesChecked,
  findings_count: options.findingCount,
  error_count: options.runErrorCount,
  healthy: options.findingCount === 0 && options.runErrorCount === 0,
  interval_ms: 21_600_000,
  fixed_count: 0,
  blocked_reason: "",
  blocked_at: "",
  license_required: false,
  license_status: "active",
  summary: {
    critical: 0,
    warning: options.findingCount,
    watch: 0,
    info: 0,
  },
});

const buildIntelligenceSummary = (
  options: Required<PatrolWorkbenchFixtureOptions>,
) => ({
  timestamp: "2026-06-30T08:06:00Z",
  overall_health: {
    score: options.findingCount > 0 ? 74 : 100,
    grade: options.findingCount > 0 ? "C" : "A",
    trend: "stable",
    factors: [],
    prediction:
      options.findingCount > 0
        ? "Patrol found one current issue for review."
        : "Patrol checked monitored resources and found no current issues.",
  },
  findings_count: {
    critical: 0,
    warning: options.findingCount,
    watch: 0,
    info: 0,
    total: options.findingCount,
  },
  predictions_count: 0,
  recent_changes_count: 0,
  recent_changes: [],
  learning: {
    resources_with_knowledge: 1,
    total_notes: 0,
    resources_with_baselines: 1,
    patterns_detected: 0,
    correlations_learned: 0,
    incidents_tracked: 0,
  },
});

const buildPatrolFindingsResponse = (
  options: Required<PatrolWorkbenchFixtureOptions>,
) => {
  if (options.findingCount === 0) {
    return { findings: [], count: 0, active_count: 0 };
  }

  return {
    findings: [
      {
        id: "finding-active-work",
        severity: "warning",
        category: "reliability",
        resource_id: monitoredProxmoxResource.id,
        resource_name: monitoredProxmoxResource.displayName,
        resource_type: monitoredProxmoxResource.type,
        title: "High CPU pressure on pve-main",
        description:
          "Patrol detected sustained CPU pressure on the monitored Proxmox host.",
        detected_at: "2026-06-30T08:05:00Z",
        last_seen_at: "2026-06-30T08:05:42Z",
        auto_resolved: false,
        times_raised: 1,
        suppressed: false,
        investigation_attempts: 0,
        investigation_status:
          options.pendingApprovalCount > 0 ? "needs_attention" : undefined,
        investigation_outcome:
          options.pendingApprovalCount > 0 ? "fix_queued" : undefined,
      },
    ],
    count: 1,
    active_count: 1,
  };
};

const buildPatrolFindings = (
  options: Required<PatrolWorkbenchFixtureOptions>,
) => buildPatrolFindingsResponse(options).findings;

async function mockMonitorFirstPatrolWorkbench(
  page: Page,
  options: PatrolWorkbenchFixtureOptions = {},
): Promise<void> {
  const resolved = {
    findingCount: options.findingCount ?? 0,
    resourcesChecked: options.resourcesChecked ?? 1,
    pendingApprovalCount: options.pendingApprovalCount ?? 0,
    runErrorCount:
      options.runErrorCount ?? (options.runStatus === "error" ? 1 : 0),
    runStatus:
      options.runStatus ?? (options.findingCount ? "issues_found" : "healthy"),
  } satisfies Required<PatrolWorkbenchFixtureOptions>;

  await page.route("**/api/security/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        hasAuthentication: true,
        hasHTTPS: true,
        apiTokenConfigured: true,
        exportProtected: true,
        hideLocalLogin: false,
        publicAccess: false,
        requiresAuth: true,
        ssoEnabled: false,
        ssoProviders: [],
        tokenScopes: ["*"],
        sessionCapabilities: {
          assistantEnabled: true,
          demoMode: false,
        },
        presentationPolicy: {
          demoMode: false,
          readOnly: false,
          hideCommercial: false,
          hideUpgrade: false,
        },
      }),
    });
  });

  await page.route("**/api/state", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(buildRuntimeState()),
    });
  });

  await page.route("**/api/resources**", async (route) => {
    const requestUrl = new URL(route.request().url());
    if (requestUrl.pathname !== "/api/resources") {
      await route.continue();
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: [monitoredProxmoxResource],
        meta: {
          page: 1,
          limit: 100,
          total: 1,
          totalPages: 1,
        },
        links: {
          next: null,
        },
      }),
    });
  });

  await page.route("**/api/replication/jobs", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });

  await page.route("**/api/ai/patrol/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(buildPatrolStatus(resolved)),
    });
  });

  await page.route("**/api/ai/patrol/runs*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(buildRunHistory(resolved)),
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

  await page.route("**/api/ai/patrol/findings*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(buildPatrolFindings(resolved)),
    });
  });

  await page.route("**/api/ai/unified/findings*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(buildPatrolFindingsResponse(resolved)),
    });
  });

  await page.route("**/api/ai/intelligence", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(buildIntelligenceSummary(resolved)),
    });
  });

  await page.route("**/api/ai/intelligence/correlations*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ correlations: [], count: 0 }),
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
        total_successes: 4,
        total_failures: 0,
      }),
    });
  });

  await page.route("**/api/ai/approvals", async (route) => {
    const approvals =
      resolved.pendingApprovalCount > 0
        ? [
            {
              id: "approval-finding-active-work",
              toolId: "investigation_fix",
              command: "systemctl restart pulse-agent",
              targetType: "agent",
              targetId: "finding-active-work",
              targetName: "pve-main",
              context:
                "Restart Pulse Agent on pve-main to clear the sustained pressure finding.",
              riskLevel: "medium",
              status: "pending",
              requestedAt: "2026-06-30T08:06:00Z",
              expiresAt: "2099-06-30T08:11:00Z",
            },
          ]
        : [];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ approvals }),
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
        patrol_anomaly_triggers_enabled: false,
        patrol_event_triggers_enabled: true,
        patrol_auto_fix: false,
        auto_fix_model: "",
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
}

test.describe("Monitor-first Patrol workbench browser contract", () => {
  test.setTimeout(180_000);

  test("legacy infrastructure launch opens the monitor lens before Patrol or Assistant chrome", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop workbench routing proof",
    );

    await mockMonitorFirstPatrolWorkbench(page, {
      findingCount: 1,
      runStatus: "issues_found",
    });
    await page.goto("/infrastructure", { waitUntil: "domcontentloaded" });

    await expect(page).toHaveURL(/\/proxmox\/overview$/, { timeout: 30_000 });
    await expect(page.getByTestId("proxmox-page")).toBeVisible({
      timeout: 30_000,
    });

    const desktopNav = page.getByRole("tablist", {
      name: "Primary navigation",
    });
    await expect(
      desktopNav.getByRole("tab", { name: "Proxmox" }),
    ).toBeVisible();
    await expect(
      desktopNav.getByRole("tab", { name: "Patrol: 1 open work item" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Ask Pulse Assistant about Proxmox" }),
    ).toBeVisible();
    await expect(page.getByRole("heading", { name: "Open work" })).toHaveCount(
      0,
    );
    await expect(
      page.getByRole("heading", { name: /^Pulse Assistant$/ }),
    ).toHaveCount(0);
  });

  test("calm-day Patrol posture stays inside Patrol context after infrastructure launch", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop workbench routing proof",
    );

    await mockMonitorFirstPatrolWorkbench(page, {
      findingCount: 0,
      runStatus: "healthy",
    });
    await page.goto("/infrastructure", { waitUntil: "domcontentloaded" });

    await expect(page).toHaveURL(/\/proxmox\/overview$/, { timeout: 30_000 });
    await expect(page.getByTestId("proxmox-page")).toBeVisible({
      timeout: 30_000,
    });
    await expect(
      page.getByRole("heading", { name: "No current issues" }),
    ).toHaveCount(0);
    await expect(page.getByText("Protection current")).toHaveCount(0);
    await expect(
      page.getByRole("list", { name: "Patrol protection posture" }),
    ).toHaveCount(0);

    await page.getByRole("tab", { name: "Patrol" }).click();
    await expect(page).toHaveURL(/\/patrol$/);
    await expect(
      page.getByRole("heading", { level: 1, name: "Patrol" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 2, name: "Open work" }),
    ).toBeVisible();
    await expect(page.getByText("No current issues").first()).toBeVisible();
    await expect(page.getByText("Checked 1 resource.")).toBeVisible();
    const protectionPosture = page.getByRole("list", {
      name: "Patrol protection posture",
    });
    await expect(protectionPosture).toBeVisible();
    await expect(protectionPosture.getByText("Protection current")).toBeVisible();
    await expect(protectionPosture.getByText("Checked 1 resource")).toBeVisible();
    await expect(protectionPosture.getByText("Schedule active")).toBeVisible();
    await expect(protectionPosture.getByText("No recurring issues")).toBeVisible();
    await expect(protectionPosture.getByText("No verification waiting")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Ask Pulse Assistant about Patrol" }),
    ).toBeVisible();
    await expect(page.getByText(/Nothing needs attention/i)).toHaveCount(0);
  });

  test("Patrol groups approvals and failed checks inside the Patrol workbench", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop workbench routing proof",
    );

    await mockMonitorFirstPatrolWorkbench(page, {
      findingCount: 1,
      pendingApprovalCount: 1,
      resourcesChecked: 4,
      runErrorCount: 1,
      runStatus: "error",
    });
    await page.goto("/infrastructure", { waitUntil: "domcontentloaded" });

    await expect(page).toHaveURL(/\/proxmox\/overview$/, { timeout: 30_000 });
    await expect(page.getByText("Latest check needs review")).toHaveCount(0);
    await expect(page.getByText("1 approval waiting")).toHaveCount(0);
    await expect(page.getByText("Protection current")).toHaveCount(0);

    await page.getByRole("tab", { name: /Patrol/ }).click();
    await expect(page).toHaveURL(/\/patrol$/);
    await expect(
      page.getByRole("heading", { level: 2, name: "Open work" }),
    ).toBeVisible();
    const workGroups = page.getByRole("list", { name: "Patrol work groups" });
    await expect(workGroups).toBeVisible();
    await expect(workGroups.getByText("1 approval waiting")).toBeVisible();
    await expect(
      workGroups.getByText("Latest check needs review"),
    ).toBeVisible();
    await expect(
      workGroups.getByText(
        "Patrol checked 4 resources but ended with runtime issues.",
      ),
    ).toBeVisible();
    await expect(
      page.getByRole("list", { name: "Patrol protection posture" }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("heading", { name: /^Pulse Assistant$/ }),
    ).toHaveCount(0);
  });
});
