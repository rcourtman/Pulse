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

const recentPatrolStartedAt = new Date(Date.now() - 102_000).toISOString();
const recentPatrolCompletedAt = new Date(Date.now() - 60_000).toISOString();

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
    started_at: recentPatrolStartedAt,
    completed_at: recentPatrolCompletedAt,
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
  last_patrol_at: recentPatrolCompletedAt,
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
        impact:
          "Sustained CPU pressure can slow hosted workloads on this Proxmox host.",
        evidence:
          "CPU stayed above the configured warning threshold during the scheduled Patrol check.",
        recommendation:
          "Review the host load and move or stop noisy workloads before approving a fix.",
        detected_at: recentPatrolStartedAt,
        last_seen_at: recentPatrolCompletedAt,
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

const buildAttentionItem = (
  options: Required<PatrolWorkbenchFixtureOptions>,
) => ({
  id: "attention-high-cpu",
  operationalRecordId: "attention-high-cpu",
  subjectResourceId: monitoredProxmoxResource.id,
  subjectResourceName: monitoredProxmoxResource.displayName,
  subjectResourceType: monitoredProxmoxResource.type,
  kind: "patrol_finding",
  title: "High CPU pressure on pve-main",
  plainLanguageSummary:
    "CPU stayed above the configured warning threshold during the scheduled Patrol check.",
  severity: "warning",
  state: "open",
  firstObservedAt: recentPatrolStartedAt,
  lastObservedAt: recentPatrolCompletedAt,
  evidenceFreshness: "fresh",
  evidenceCompleteness: "complete",
  impact:
    "Sustained CPU pressure can slow hosted workloads on this Proxmox host.",
  relatedResources: [],
  recommendedNextStep:
    "Review the host load and move or stop noisy workloads before approving a fix.",
  availableActions:
    options.pendingApprovalCount > 0
      ? [
          {
            actionId: "action-finding-active-work",
            targetResourceId: monitoredProxmoxResource.id,
            capability: "agent.restart",
            kind: "command",
            label: "Restart Pulse Agent",
            mode: "execute",
            risk: "medium",
            approval: "required",
            eligibility: "eligible",
            reasons: [],
            evidenceIds: ["evidence-high-cpu"],
            expectedPostcondition: "The agent reconnects and reports current CPU evidence.",
            verificationPolicy: "Confirm a fresh agent heartbeat after restart.",
            requiresApproval: true,
          },
        ]
      : [],
  verificationState: "not_available",
});

const buildAttentionSummary = (
  options: Required<PatrolWorkbenchFixtureOptions>,
) => ({
  activeCount: options.findingCount,
  openCount: options.findingCount,
  acknowledgedCount: 0,
  suppressedCount: 0,
  uncertainCount: 0,
  resolvedCount: 0,
  calm: options.findingCount === 0 && options.runErrorCount === 0,
  coverageState: "current",
  evaluatedAt: recentPatrolCompletedAt,
});

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
        requested_autonomy_level: "monitor",
        effective_autonomy_level: "monitor",
        full_mode_unlocked: false,
        autopilot_acknowledgement: {
          code: "not_requested",
          active: false,
          currentVersion: 1,
          acceptedScope: [],
          acceptedLimits: {
            policyAllowlistRequired: true,
            emergencyStopHonored: true,
            approvalFloorsHonored: true,
            verificationReconciledWhenSupported: true,
            evidenceClassDisclosed: true,
            inconclusiveOutcomeAllowed: true,
            executionSuccessIsNotOutcomeTruth: true,
          },
        },
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
              plan: {
                actionId: "action-finding-active-work",
                planHash: "sha256:patrol-review",
              },
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

  await page.route("**/api/ai/patrol/attention**", async (route) => {
    const requestUrl = new URL(route.request().url());
    const item = buildAttentionItem(resolved);
    const summary = buildAttentionSummary(resolved);

    if (requestUrl.pathname.endsWith("/summary")) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(summary),
      });
      return;
    }

    if (!requestUrl.pathname.endsWith("/attention")) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          item,
          operationalRecord: {
            id: item.operationalRecordId,
            canonicalSpecId: "patrol-finding",
            subjectResourceId: item.subjectResourceId,
            state: item.state,
            severity: item.severity,
            firstObservedAt: item.firstObservedAt,
            lastObservedAt: item.lastObservedAt,
            stateChangedAt: item.firstObservedAt,
            evidenceIds: ["evidence-high-cpu"],
            causeKey: item.id,
            relatedResourceIds: [],
            impactSummary: item.impact,
            recommendedNextStep: item.recommendedNextStep,
          },
          timeline: [],
          evidence: [
            {
              id: "evidence-high-cpu",
              source: {
                provider: "pulse-agent",
                collector: "node-metrics",
                instance: "pve-main",
              },
              subject: { resourceId: item.subjectResourceId },
              observedAt: item.lastObservedAt,
              ingestedAt: item.lastObservedAt,
              completeness: "complete",
              confidence: "confirmed",
              permissions: "sufficient",
              reason: {
                code: "threshold_breach",
                message: "CPU remained above the configured threshold.",
              },
            },
          ],
        }),
      });
      return;
    }

    const data = resolved.findingCount > 0 ? [item] : [];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data,
        summary,
        meta: {
          page: 1,
          limit: 50,
          total: data.length,
          totalPages: data.length > 0 ? 1 : 0,
        },
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
      desktopNav.getByRole("tab", {
        name: "Patrol: 1 active attention item",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Ask Pulse Assistant about Proxmox" }),
    ).toBeVisible();
    await expect(page.getByRole("heading", { name: "Open work" })).toHaveCount(
      0,
    );
    await expect(
      page.getByRole("list", { name: "Proxmox Patrol coverage" }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("heading", { name: /^Pulse Assistant$/ }),
    ).toHaveCount(0);
  });

  test("calm-day Patrol empty queue stays plain after infrastructure launch", async ({
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
    await expect(
      page.getByRole("list", { name: "Proxmox Patrol coverage" }),
    ).toHaveCount(0);
    await expect(page.getByText("Patrol checked 1 resource")).toHaveCount(0);
    await expect(page.getByText("No Patrol work waiting")).toHaveCount(0);
    await expect(page.getByText("Next check scheduled")).toHaveCount(0);

    await page.getByRole("tab", { name: "Patrol" }).click();
    await expect(page).toHaveURL(/\/patrol$/);
    await expect(
      page.getByRole("heading", { level: 1, name: "Patrol" }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { level: 2, name: "Needs attention" }),
    ).toBeVisible();
    await expect(page.getByText("Nothing needs your attention")).toBeVisible();
    await expect(
      page.getByRole("list", { name: "Patrol protection posture" }),
    ).toHaveCount(0);
    await expect(page.getByText("Protection current")).toHaveCount(0);
    await expect(page.getByText("No recurring issues")).toHaveCount(0);
    await expect(page.getByText("No verification waiting")).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Ask Pulse Assistant about Patrol" }),
    ).toBeVisible();
    await expect(
      page.getByRole("list", { name: "Patrol attention items" }),
    ).toHaveCount(0);
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
    await expect(
      page.getByRole("list", { name: "Proxmox Patrol coverage" }),
    ).toHaveCount(0);
    await expect(page.getByText("What Pulse checked")).toHaveCount(0);

    await page.getByRole("tab", { name: /Patrol/ }).click();
    await expect(page).toHaveURL(/\/patrol$/);
    await expect(
      page.getByRole("heading", { level: 2, name: "Needs attention" }),
    ).toBeVisible();
    await expect(
      page.getByRole("list", { name: "Patrol attention items" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", {
        name: "Open High CPU pressure on pve-main",
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("list", { name: "Patrol protection posture" }),
    ).toHaveCount(0);
    await page
      .getByRole("button", {
        name: "Open High CPU pressure on pve-main",
      })
      .click();
    await expect(
      page.getByRole("heading", { name: "High CPU pressure on pve-main" }),
    ).toBeVisible();
    await expect(
      page.getByText(
        "CPU stayed above the configured warning threshold during the scheduled Patrol check.",
      ),
    ).toBeVisible();
    await expect(
      page.getByText(
        "Sustained CPU pressure can slow hosted workloads on this Proxmox host.",
      ),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Review action" })).toBeVisible();
    await expect(
      page.getByRole("heading", { name: /^Pulse Assistant$/ }),
    ).toHaveCount(0);
  });

  test("Patrol presents bounded APT findings without package-manager internals", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Phone action review is covered by the Actions journey",
    );
    await mockMonitorFirstPatrolWorkbench(page, {
      findingCount: 1,
      runStatus: "issues_found",
    });
    const updateFinding = {
      id: "apt-updates-active",
      key: "apt-host-updates",
      alertIdentifier: `${monitoredProxmoxResource.id}::apt/updates`,
      severity: "warning",
      category: "maintenance",
      resource_id: monitoredProxmoxResource.id,
      resource_name: monitoredProxmoxResource.displayName,
      resource_type: monitoredProxmoxResource.type,
      title: "Operating system updates need review",
      description: "Patrol found a bounded host-maintenance action for review.",
      impact:
        "Unapplied updates can leave the host behind its intended maintenance posture.",
      evidence:
        "pending_updates=6 inventory=sha256:must-not-render checked_at=2026-07-12T10:00:00Z received_at=2026-07-12T10:05:00Z reboot_required=true",
      recommendation:
        "Review the governed update action. A reboot, if required later, is a separate action.",
      detected_at: "2026-07-12T10:00:00Z",
      last_seen_at: "2026-07-12T10:05:00Z",
      auto_resolved: false,
      times_raised: 1,
      suppressed: false,
      investigation_attempts: 1,
      investigation_status: "completed",
      investigation_outcome: "verification_failed",
    };
    const cleanupFinding = {
      id: "apt-cleanup-active",
      key: "apt-package-cache-pressure",
      alertIdentifier: `${monitoredProxmoxResource.id}::apt/cache`,
      severity: "warning",
      category: "storage",
      resource_id: monitoredProxmoxResource.id,
      resource_name: monitoredProxmoxResource.displayName,
      resource_type: monitoredProxmoxResource.type,
      title: "Downloaded package data is using needed space",
      description:
        "Patrol measured reclaimable downloaded package data on the pressured filesystem.",
      impact: "The host may run short of operational disk space.",
      evidence:
        "reclaimable_bytes=104857600 filesystem_usage=91.5 fingerprint=sha256:must-not-render checked_at=2026-07-12T10:00:00Z received_at=2026-07-12T10:05:00Z",
      recommendation:
        "Review the governed cleanup action and rescan after any inconclusive attempt.",
      detected_at: "2026-07-12T10:00:00Z",
      last_seen_at: "2026-07-12T10:05:00Z",
      auto_resolved: false,
      times_raised: 1,
      suppressed: false,
      investigation_attempts: 1,
      investigation_status: "completed",
      investigation_outcome: "fix_failed",
    };
    const findings = [updateFinding];
    await page.route("**/api/ai/patrol/findings*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(findings),
      }),
    );
    await page.route("**/api/ai/unified/findings*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ findings, count: 2, active_count: 2 }),
      }),
    );
    await page.goto("/patrol", { waitUntil: "domcontentloaded" });
    await expect(
      page.getByRole("heading", { level: 1, name: "Patrol" }),
    ).toBeVisible();
    await page
      .getByText("Patrol checks, investigations, and run history", {
        exact: true,
      })
      .click();
    await expect(
      page.getByText("Operating system updates need review").first(),
    ).toBeVisible();
    await page
      .getByRole("button", {
        name: "Review issue for Operating system updates need review",
      })
      .click();
    const updateReview = page.getByRole("complementary", {
      name: "Review Operating system updates need review",
    });
    await expect(
      updateReview.getByText(
        "6 operating system updates were pending when the agent checked",
        { exact: false },
      ),
    ).toBeVisible();
    await expect(
      updateReview.getByText("reboot required: Yes", { exact: false }),
    ).toBeVisible();
    await expect(page.getByText("sha256:must-not-render")).toHaveCount(0);
    await expect(page.getByText("inventory=", { exact: false })).toHaveCount(0);
    await page.unroute("**/api/ai/patrol/findings*");
    await page.unroute("**/api/ai/unified/findings*");
    await page.route("**/api/ai/patrol/findings*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([cleanupFinding]),
      }),
    );
    await page.route("**/api/ai/unified/findings*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          findings: [cleanupFinding],
          count: 1,
          active_count: 1,
        }),
      }),
    );
    await page.reload({ waitUntil: "domcontentloaded" });
    await page
      .getByText("Patrol checks, investigations, and run history", {
        exact: true,
      })
      .click();
    const cleanupTitle = page
      .getByText("Downloaded package data is using needed space")
      .first();
    await expect(cleanupTitle).toBeVisible();
    await cleanupTitle.locator("xpath=ancestor::*[@role='button'][1]").click();
    await expect(
      page
        .getByText("100 MB of downloaded package data was reclaimable", {
          exact: false,
        })
        .first(),
    ).toBeVisible();
    await expect(
      page.getByText("91.5% full", { exact: false }).first(),
    ).toBeVisible();
    await expect(page.getByText("fingerprint=", { exact: false })).toHaveCount(
      0,
    );
    await testInfo.attach("apt-bounded-patrol-findings", {
      body: await page.screenshot(),
      contentType: "image/png",
    });

    const resolvedFinding = {
      ...updateFinding,
      id: "apt-updates-resolved",
      title: "Operating system updates confirmed complete",
      auto_resolved: true,
      resolved_at: "2026-07-12T10:06:00Z",
      investigation_outcome: "fix_verified",
      evidence:
        "pending_updates=0 inventory=sha256:must-not-render checked_at=2026-07-12T10:01:00Z received_at=2026-07-12T10:06:00Z reboot_required=false",
    };
    await page.unroute("**/api/ai/patrol/findings*");
    await page.unroute("**/api/ai/unified/findings*");
    await page.route("**/api/ai/patrol/findings*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([resolvedFinding]),
      }),
    );
    await page.route("**/api/ai/unified/findings*", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          findings: [resolvedFinding],
          count: 1,
          active_count: 0,
        }),
      }),
    );
    await page.reload({ waitUntil: "domcontentloaded" });
    await page
      .getByText("Patrol checks, investigations, and run history", {
        exact: true,
      })
      .click();
    await page.getByRole("button", { name: "Resolved" }).click();
    const resolvedTitle = page
      .getByText("Operating system updates confirmed complete")
      .first();
    await expect(resolvedTitle).toBeVisible();
    await resolvedTitle.locator("xpath=ancestor::*[@role='button'][1]").click();
    await expect(
      page
        .getByText("0 operating system updates were pending", { exact: false })
        .first(),
    ).toBeVisible();
    await expect(
      page.getByText("Operating system updates need review"),
    ).toHaveCount(0);
    await testInfo.attach("apt-confirmed-postcondition-resolved", {
      body: await page.screenshot(),
      contentType: "image/png",
    });
  });
});
