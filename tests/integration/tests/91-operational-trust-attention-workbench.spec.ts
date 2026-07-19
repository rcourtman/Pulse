import { expect, test, type Page, type Route } from "@playwright/test";

type AttentionMode = "active" | "calm" | "failed";

test.beforeEach(async ({ page }) => {
  const pageErrors: string[] = [];
  const consoleErrors: string[] = [];
  await page.routeWebSocket("**/ws?*", () => {});
  page.on("pageerror", (error) => pageErrors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });
  await page.addInitScript(() => {
    window.sessionStorage.setItem(
      "pulse_auth",
      JSON.stringify({
        type: "token",
        value: "attention-workbench-test-token",
      }),
    );
    window.sessionStorage.setItem("pulse_auth_user", "operator");
  });
  await page.route("**/*", async (route) => {
    if (!new URL(route.request().url()).pathname.startsWith("/api/")) {
      await route.continue();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: "{}",
    });
  });
  await page.route("**/api/state", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
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
          startTime: evaluatedAt,
          uptime: 3600,
          pollingCycles: 1,
          webSocketClients: 0,
          version: "6.1.0-test",
        },
        activeAlerts: [],
        recentlyResolved: [],
        pveTagColors: {},
        pveTagStyles: {},
        lastUpdate: Date.parse(evaluatedAt),
        resources: [],
      }),
    });
  });
  await page.route("**/api/version", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        version: "6.1.0-test",
        buildTime: evaluatedAt,
        gitCommit: "test",
        isDevelopment: true,
        isSourceBuild: true,
      }),
    });
  });
  await page.route("**/api/security/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        detailLevel: "authenticated",
        hasAuthentication: true,
        requiresAuth: true,
        authUsername: "operator",
        sessionCapabilities: {
          demoMode: false,
          assistantEnabled: true,
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
  await page.goto("/", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(100);
  expect(pageErrors, "The Patrol shell raised a browser error").toEqual([]);
  expect(consoleErrors, "The Patrol shell logged a browser error").toEqual([]);
  const shellText = (await page.locator("body").innerText()).trim();
  expect(
    shellText,
    `The authenticated shell was blank at ${page.url()}`,
  ).not.toBe("");
  await expect(
    page
      .getByRole("tab", { name: /Patrol/ })
      .or(page.getByRole("button", { name: "Patrol", exact: true })),
  ).toBeVisible();
});

const now = new Date();
const firstObservedAt = new Date(now.getTime() - 42 * 60_000).toISOString();
const lastObservedAt = new Date(now.getTime() - 2 * 60_000).toISOString();
const evaluatedAt = now.toISOString();
const attentionID = "node-1::metric-threshold:cpu";

const protectionPosture = {
  subjectResourceId: "node-1",
  state: "attention",
  lastAttemptAt: lastObservedAt,
  lastSuccessfulPointAt: firstObservedAt,
  lastVerifiedAt: firstObservedAt,
  freshness: "current",
  verification: "stale",
  coverage: "complete",
  providerStates: [
    {
      provider: "pbs",
      source: "recovery-points",
      scope: "primary",
      jobState: "success",
      historyCompleteness: "complete",
      permissions: "sufficient",
      lastAttemptAt: lastObservedAt,
      lastSuccessAt: firstObservedAt,
      lastVerifiedAt: firstObservedAt,
      evidenceIds: ["backup-evidence-1"],
      verificationExpected: true,
    },
  ],
  repositoryResourceIds: ["pbs:repository:primary"],
  evidenceIds: ["backup-evidence-1"],
  explanation:
    "A current backup exists, but its verification is outside the verification window.",
  evaluatedAt,
};

const openAttentionItem = {
  id: attentionID,
  operationalRecordId: attentionID,
  subjectResourceId: "node-1",
  subjectResourceName: "pve-main",
  subjectResourceType: "agent",
  title: "CPU pressure on pve-main",
  plainLanguageSummary:
    "CPU has remained above the configured threshold for two collection cycles.",
  severity: "critical",
  state: "open",
  firstObservedAt,
  lastObservedAt,
  evidenceFreshness: "fresh",
  evidenceCompleteness: "complete",
  impact: "Workloads on this node may respond slowly.",
  protectionPosture,
  relatedResources: [{ resourceId: "vm-101" }],
  recommendedNextStep:
    "Open the node and verify which workload is consuming CPU before making changes.",
  availableActions: [],
  verificationState: "not_available",
};

const uncertainAttentionItem = {
  ...openAttentionItem,
  id: "node-2::connectivity",
  operationalRecordId: "node-2::connectivity",
  subjectResourceId: "node-2",
  subjectResourceName: "pve-edge",
  title: "Connection state unknown for pve-edge",
  plainLanguageSummary:
    "Pulse does not have recent enough evidence to report this node as healthy.",
  severity: "warning",
  state: "unknown",
  evidenceFreshness: "unknown",
  evidenceCompleteness: "partial",
  protectionPosture: undefined,
  relatedResources: [],
  recommendedNextStep:
    "Check the provider connection and collect current evidence before deciding whether the node is offline.",
};

const acknowledgedAttentionItem = {
  ...openAttentionItem,
  id: "node-3::memory",
  operationalRecordId: "node-3::memory",
  subjectResourceId: "node-3",
  subjectResourceName: "pve-lab",
  title: "Memory pressure acknowledged on pve-lab",
  state: "acknowledged",
};

const suppressedAttentionItem = {
  ...openAttentionItem,
  id: "node-4::maintenance",
  operationalRecordId: "node-4::maintenance",
  subjectResourceId: "node-4",
  subjectResourceName: "pve-maintenance",
  title: "Maintenance alert suppressed on pve-maintenance",
  state: "suppressed",
};

const resolvedAttentionItem = {
  ...openAttentionItem,
  id: "node-5::storage",
  operationalRecordId: "node-5::storage",
  subjectResourceId: "node-5",
  subjectResourceName: "pve-recovered",
  title: "Storage pressure resolved on pve-recovered",
  state: "resolved",
};

const activeSummary = {
  activeCount: 2,
  openCount: 1,
  acknowledgedCount: 1,
  suppressedCount: 1,
  uncertainCount: 1,
  resolvedCount: 1,
  calm: false,
  coverageState: "current",
  evaluatedAt,
};

const detail = {
  item: openAttentionItem,
  operationalRecord: {
    id: attentionID,
    canonicalSpecId: "metric-threshold:cpu",
    subjectResourceId: "node-1",
    state: "open",
    severity: "critical",
    firstObservedAt,
    lastObservedAt,
    stateChangedAt: firstObservedAt,
    evidenceIds: ["metric-evidence-1"],
    causeKey: attentionID,
    relatedResourceIds: ["vm-101"],
    impactSummary: openAttentionItem.impact,
    recommendedNextStep: openAttentionItem.recommendedNextStep,
  },
  timeline: [
    {
      id: "transition-1",
      operationalRecordId: attentionID,
      from: "observing",
      to: "open",
      at: firstObservedAt,
      cause: "detector_decision",
      causeKey: attentionID,
      evidenceIds: ["metric-evidence-1"],
      reason:
        "The threshold remained breached for the required confirmation window.",
    },
  ],
  evidence: [
    {
      id: "metric-evidence-1",
      source: {
        provider: "proxmox",
        collector: "node-metrics",
        instance: "homelab",
      },
      subject: { resourceId: "node-1" },
      observedAt: lastObservedAt,
      ingestedAt: lastObservedAt,
      completeness: "complete",
      confidence: "confirmed",
      permissions: "sufficient",
      reason: {
        code: "threshold_breach",
        message: "Two consecutive samples were above 90%.",
      },
    },
  ],
};

async function mockAttention(
  page: Page,
  initialMode: AttentionMode,
): Promise<{ setMode: (mode: AttentionMode) => void }> {
  let mode = initialMode;
  let primaryState: "open" | "acknowledged" | "suppressed" = "open";
  let suppressionReason = "";
  const primaryItem = () => ({
    ...openAttentionItem,
    state: primaryState,
  });
  const primaryDetail = () => ({
    ...detail,
    item: primaryItem(),
    operationalRecord: {
      ...detail.operationalRecord,
      state: primaryState,
      ...(primaryState === "acknowledged"
        ? {
            acknowledgement: {
              at: evaluatedAt,
              by: "operator",
            },
          }
        : {}),
      ...(primaryState === "suppressed"
        ? {
            suppression: {
              at: evaluatedAt,
              by: "operator",
              reason: suppressionReason,
              expiresAt: new Date(now.getTime() + 60 * 60_000).toISOString(),
            },
          }
        : {}),
    },
    timeline:
      primaryState === "open"
        ? detail.timeline
        : [
            ...detail.timeline,
            {
              id: `transition-${primaryState}`,
              operationalRecordId: attentionID,
              from: "open",
              to: primaryState,
              at: evaluatedAt,
              cause:
                primaryState === "acknowledged"
                  ? "acknowledgement"
                  : "suppression",
              causeKey: attentionID,
              evidenceIds: ["metric-evidence-1"],
              reason:
                primaryState === "suppressed"
                  ? suppressionReason
                  : "Operator acknowledged the issue.",
            },
          ],
  });
  const currentSummary = () => ({
    ...activeSummary,
    activeCount: primaryState === "suppressed" ? 1 : 2,
    openCount: primaryState === "open" ? 1 : 0,
    acknowledgedCount: primaryState === "acknowledged" ? 2 : 1,
    suppressedCount: primaryState === "suppressed" ? 2 : 1,
  });
  await page.route("**/api/resources**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname !== "/api/resources") {
      await route.continue();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: [],
        meta: { page: 1, limit: 100, total: 0, totalPages: 0 },
        links: { next: null },
      }),
    });
  });
  await page.route("**/api/replication/jobs", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: "[]",
    });
  });
  await page.route("**/api/ai/patrol/status", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        runtime_state: "active",
        running: false,
        enabled: true,
        last_patrol_at: lastObservedAt,
        next_patrol_at: new Date(now.getTime() + 6 * 60 * 60_000).toISOString(),
        last_duration_ms: 42_000,
        resources_checked: 2,
        findings_count: 0,
        error_count: 0,
        healthy: true,
        interval_ms: 21_600_000,
        fixed_count: 0,
        blocked_reason: "",
        blocked_at: "",
        license_required: false,
        license_status: "active",
        summary: { critical: 0, warning: 0, watch: 0, info: 0 },
      }),
    });
  });
  await page.route("**/api/ai/patrol/runs*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: "[]",
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
      body: "[]",
    });
  });
  await page.route("**/api/ai/unified/findings*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ findings: [], count: 0, active_count: 0 }),
    });
  });
  await page.route("**/api/ai/intelligence", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        timestamp: evaluatedAt,
        overall_health: {
          score: 100,
          grade: "A",
          trend: "stable",
          factors: [],
          prediction: "No legacy Patrol findings are waiting.",
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
        total_successes: 1,
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
  await page.route("**/api/ai/patrol/attention**", async (route: Route) => {
    if (mode === "failed") {
      await route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({
          error: {
            code: "operational_lifecycle_unavailable",
            message: "Current lifecycle evidence is unavailable.",
          },
        }),
      });
      return;
    }

    const url = new URL(route.request().url());
    if (route.request().method() === "POST") {
      if (url.pathname.endsWith("/acknowledge")) {
        primaryState = "acknowledged";
      } else if (url.pathname.endsWith("/unacknowledge")) {
        primaryState = "open";
      } else if (url.pathname.endsWith("/suppress")) {
        const body = route.request().postDataJSON() as {
          reason?: string;
        };
        suppressionReason = body.reason ?? "";
        primaryState = "suppressed";
      } else if (url.pathname.endsWith("/unsuppress")) {
        primaryState = "open";
        suppressionReason = "";
      }
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ success: true }),
      });
      return;
    }
    if (url.pathname.endsWith("/summary")) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          mode === "calm"
            ? {
                activeCount: 0,
                openCount: 0,
                acknowledgedCount: 0,
                suppressedCount: 0,
                uncertainCount: 0,
                resolvedCount: 1,
                calm: true,
                coverageState: "current",
                evaluatedAt,
              }
            : currentSummary(),
        ),
      });
      return;
    }

    if (!url.pathname.endsWith("/attention")) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(primaryDetail()),
      });
      return;
    }

    const filter = url.searchParams.get("filter") ?? "active";
    const data =
      mode === "calm"
        ? []
        : filter === "stale_unknown"
          ? [uncertainAttentionItem]
          : filter === "open"
            ? [openAttentionItem]
            : filter === "acknowledged"
              ? [acknowledgedAttentionItem]
              : filter === "suppressed"
                ? [suppressedAttentionItem]
                : filter === "resolved"
                  ? [resolvedAttentionItem]
                  : primaryState === "suppressed"
                    ? [uncertainAttentionItem]
                    : [primaryItem(), uncertainAttentionItem];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data,
        summary:
          mode === "calm"
            ? {
                activeCount: 0,
                openCount: 0,
                acknowledgedCount: 0,
                suppressedCount: 0,
                uncertainCount: 0,
                resolvedCount: 1,
                calm: true,
                coverageState: "current",
                evaluatedAt,
              }
            : currentSummary(),
        meta: {
          page: 1,
          limit: 50,
          total: data.length,
          totalPages: data.length > 0 ? 1 : 0,
        },
      }),
    });
  });
  return {
    setMode: (nextMode) => {
      mode = nextMode;
    },
  };
}

test("starts from the normal monitor shell and reaches the canonical attention queue", async ({
  page,
}, testInfo) => {
  await mockAttention(page, "active");
  await page.goto("/", { waitUntil: "domcontentloaded" });

  const patrolNavigation = testInfo.project.name.startsWith("mobile-")
    ? page
        .getByRole("tablist", { name: "Mobile navigation" })
        .getByRole("button", { name: /Patrol/ })
    : page
        .getByRole("tab", { name: /Patrol/ })
        .or(page.getByRole("link", { name: /Patrol/ }));
  await patrolNavigation.click();

  await expect(page).toHaveURL(/\/patrol/);
  await expect(
    page.getByRole("region", { name: "Needs attention" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Open CPU pressure on pve-main" }),
  ).toBeVisible();
});

test("makes active operational work primary and preserves the evidence boundary", async ({
  page,
}) => {
  await mockAttention(page, "active");
  await page.goto("/patrol", { waitUntil: "domcontentloaded" });

  await expect(
    page.getByRole("tab", { name: "Patrol: 2 active attention items" }).or(
      page.getByRole("button", {
        name: "Patrol: 2 active attention items",
      }),
    ),
  ).toBeVisible();
  const queue = page.getByRole("region", { name: "Needs attention" });
  await expect(queue.getByLabel("2 active attention items")).toBeVisible();
  await expect(
    queue.getByText(
      "CPU has remained above the configured threshold for two collection cycles.",
    ),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Explain with Assistant" }),
  ).toHaveCount(0);

  await queue
    .getByRole("button", { name: "Acknowledged 1", exact: true })
    .click();
  await expect(
    queue.getByText("Memory pressure acknowledged on pve-lab"),
  ).toBeVisible();
  await queue
    .getByRole("button", { name: "Suppressed 1", exact: true })
    .click();
  await expect(
    queue.getByText("Maintenance alert suppressed on pve-maintenance"),
  ).toBeVisible();
  await queue
    .getByRole("button", { name: "Stale or unknown 1", exact: true })
    .click();
  await expect(
    queue.getByText("Connection state unknown for pve-edge"),
  ).toBeVisible();
  await queue
    .getByRole("button", { name: "Recent resolved 1", exact: true })
    .click();
  await expect(
    queue.getByText("Storage pressure resolved on pve-recovered"),
  ).toBeVisible();
  await queue.getByRole("button", { name: "Active 2", exact: true }).click();

  const itemButton = queue.getByRole("button", {
    name: "Open CPU pressure on pve-main",
  });
  await itemButton.focus();
  await page.keyboard.press("Enter");

  const detailPanel = page.getByRole("complementary", {
    name: "CPU pressure on pve-main",
  });
  await expect(detailPanel).toBeVisible();
  await expect(
    detailPanel.getByText("Workloads on this node may respond slowly."),
  ).toBeVisible();
  await expect(
    detailPanel.getByText(
      "Open the node and verify which workload is consuming CPU before making changes.",
    ),
  ).toBeVisible();
  await expect(
    detailPanel.getByText("Two consecutive samples were above 90%."),
  ).toBeVisible();
  await expect(
    detailPanel.getByText(
      "A current backup exists, but its verification is outside the verification window.",
    ),
  ).toBeVisible();
  await expect(detailPanel.getByText("Observing to Open")).toBeVisible();
  await expect(page).toHaveURL(
    new RegExp(`attention=${encodeURIComponent(attentionID)}`),
  );

  await detailPanel.getByRole("button", { name: "Acknowledge" }).click();
  await expect(
    detailPanel.getByText(/Acknowledged by operator/i),
  ).toBeVisible();
  await expect(
    detailPanel.getByRole("button", { name: "Return to open" }),
  ).toBeVisible();

  await detailPanel
    .getByRole("button", { name: "Suppress temporarily" })
    .click();
  await detailPanel
    .getByRole("textbox", {
      name: "Why is this safe to hide from active attention?",
    })
    .fill("Planned host maintenance");
  await detailPanel
    .getByRole("combobox", {
      name: "Return it to active attention after",
    })
    .selectOption(String(60 * 60 * 1000));
  await detailPanel
    .getByRole("button", { name: "Suppress temporarily" })
    .click();
  await expect(
    detailPanel.getByText(/Suppressed by operator: Planned host maintenance/i),
  ).toBeVisible();
  await detailPanel
    .getByRole("button", { name: "Return to active attention" })
    .click();
  await expect(
    detailPanel.getByRole("button", { name: "Acknowledge" }),
  ).toBeVisible();

  await detailPanel
    .getByRole("button", { name: "Close attention detail" })
    .click();
  await expect(itemButton).toBeFocused();
  await expect(page).not.toHaveURL(/attention=/);
});

test("puts the selected detail in view on a phone without page overflow", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await mockAttention(page, "active");
  await page.goto("/patrol", { waitUntil: "domcontentloaded" });

  const itemButton = page.getByRole("button", {
    name: "Open CPU pressure on pve-main",
  });
  await itemButton.click();

  const detailPanel = page.getByRole("complementary", {
    name: "CPU pressure on pve-main",
  });
  await expect(detailPanel).toBeInViewport();
  await expect(
    detailPanel.getByText(
      "Open the node and verify which workload is consuming CPU before making changes.",
    ),
  ).toBeVisible();
  const overflows = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth,
  );
  expect(overflows).toBeFalsy();

  await page.reload({ waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("complementary", { name: "CPU pressure on pve-main" }),
  ).toBeInViewport();
});

test("shows calm only with current coverage and never converts failure into health", async ({
  page,
}) => {
  const fixture = await mockAttention(page, "calm");
  await page.goto("/patrol", { waitUntil: "domcontentloaded" });

  await expect(page.getByText("Nothing needs your attention")).toBeVisible();
  await expect(
    page.getByText(
      "The current operational lifecycle evaluation has no active items.",
      {
        exact: false,
      },
    ),
  ).toBeVisible();

  fixture.setMode("failed");
  await page.getByRole("button", { name: "Refresh Patrol attention" }).click();

  await expect(page.getByText("Patrol attention is unavailable")).toBeVisible();
  await expect(
    page.getByText(
      "Pulse has not inferred a calm or healthy state from this failure.",
    ),
  ).toBeVisible();
  await expect(page.getByText("Nothing needs your attention")).toHaveCount(0);
});

type GovernedActionVerification = "confirmed" | "contradicted";

const governedActionAttentionID = "docker-health-operational-record";
const governedActionID = "act-operational-trust-restart";
const governedResourceID = "docker:host-1/container-api";

const governedActionOffer = (actionId?: string) => ({
  ...(actionId ? { actionId } : {}),
  targetResourceId: governedResourceID,
  capability: "restart",
  kind: "container_restart",
  label: "Restart this container",
  mode: actionId ? "execute" : "plan",
  risk: "low",
  approval: actionId ? "granted" : "required",
  eligibility: "eligible",
  reasons: ["fresh_confirmed_unhealthy_container", "declared_live_capability"],
  evidenceIds: ["docker-health-evidence"],
  expectedPostcondition:
    "The same container is observed running after the restart.",
  verificationPolicy:
    "Pulse requires a fresh container readback and records whether it is agent-attested or independently observed.",
  requiresApproval: true,
});

const governedActionAttentionItem = (
  actionState: "unplanned" | "pending_approval" | "approved" | "completed",
  verification: GovernedActionVerification,
) => ({
  id: governedActionAttentionID,
  operationalRecordId: governedActionAttentionID,
  subjectResourceId: governedResourceID,
  subjectResourceName: "API container",
  subjectResourceType: "app-container",
  kind: "docker-container-health",
  title: "API container is unhealthy",
  plainLanguageSummary:
    "Docker reported that the API container is unhealthy from a current health check.",
  severity: "critical",
  state: "open",
  firstObservedAt,
  lastObservedAt,
  evidenceFreshness: "fresh",
  evidenceCompleteness: "complete",
  impact: "Requests handled by this container may fail.",
  relatedResources: [],
  recommendedNextStep:
    "Review the bounded restart and its current policy before approving it.",
  availableActions: [
    governedActionOffer(
      actionState === "unplanned" ? undefined : governedActionID,
    ),
  ],
  verificationState:
    actionState === "unplanned"
      ? "not_available"
      : actionState === "completed"
        ? verification === "confirmed"
          ? "succeeded"
          : "failed"
        : "pending",
});

const governedActionAudit = (
  actionState: "pending_approval" | "approved" | "completed",
  verification: GovernedActionVerification,
) => {
  const scope = {
    orgId: "default",
    resourceId: governedResourceID,
    capabilityName: "restart",
  };
  const evidence = {
    version: 1,
    id: "restart-readback",
    observerId: "docker-agent-1",
    observerKind: "unified_agent",
    observerTrustDomain: "agent:docker-agent-1",
    executorTrustDomain: "agent:docker-agent-1",
    method: "typed_container_read_after_write",
    subjectId: governedResourceID,
    observedAt: lastObservedAt,
    receivedAt: evaluatedAt,
    summary:
      verification === "confirmed"
        ? "The same container was observed running."
        : "The container was still not running after the provider returned success.",
    digest: `sha256:${"a".repeat(64)}`,
  };
  return {
    id: governedActionID,
    createdAt: firstObservedAt,
    updatedAt: evaluatedAt,
    state: actionState,
    decisionRevision: actionState === "pending_approval" ? 0 : 1,
    request: {
      requestId: `operational-trust:${governedActionAttentionID}:restart`,
      resourceId: governedResourceID,
      capabilityName: "restart",
      params: {},
      reason:
        "Restart API container after fresh confirmed evidence reported an unhealthy container.",
      requestedBy: "operator",
    },
    resource: {
      id: governedResourceID,
      name: "API container",
      type: "app-container",
    },
    plan: {
      actionId: governedActionID,
      requestId: `operational-trust:${governedActionAttentionID}:restart`,
      allowed: true,
      requiresApproval: true,
      approvalPolicy: "admin",
      approvalRequirement: {
        version: 1,
        floor: "admin",
        quorum: 1,
        disallowRequester: false,
      },
      predictedBlastRadius: [governedResourceID],
      rollbackAvailable: false,
      plannedAt: firstObservedAt,
      expiresAt: "2099-07-19T12:00:00Z",
      resourceVersion: "resource:sha256:docker-health",
      policyVersion: "policy:sha256:docker-restart",
      planHash: `sha256:${"b".repeat(64)}`,
      policyDecision: {
        version: 1,
        status: "resolved",
        decisionId: "policy-decision:docker-restart",
        actionId: governedActionID,
        scope,
        approvalRequirement: {
          version: 1,
          floor: "admin",
          quorum: 1,
          disallowRequester: false,
        },
        planningAllowed: true,
        requiresApproval: true,
        authorities: [
          {
            kind: "capability_registry",
            sourceId: "capability-registry:restart",
            revision: "policy:sha256:docker-restart",
            status: "consulted",
            scope,
            approvalFloor: "admin",
            reasonCodes: [
              "capability_approval_admin",
              "capability_auto_low_risk",
            ],
          },
        ],
      },
      preflight: {
        target: governedResourceID,
        currentState: "warning",
        intendedChange: "Restart this Docker container.",
        dryRunAvailable: false,
        safetyChecks: [
          "The container still declares restart.",
          "The reporting agent remains connected.",
        ],
        verificationSteps: [
          "Read the same container after the restart and compare its running state.",
        ],
        generatedAt: firstObservedAt,
      },
    },
    origin: {
      surface: "operational_trust_attention",
      operationalRecordId: governedActionAttentionID,
      evidenceIds: ["docker-health-evidence"],
    },
    approvals:
      actionState === "pending_approval"
        ? []
        : [
            {
              actor: "operator",
              method: "session",
              timestamp: evaluatedAt,
              outcome: "approved",
            },
          ],
    ...(actionState === "completed"
      ? {
          result: {
            success: true,
            actionResultV2: {
              version: 2,
              execution: {
                status: "succeeded",
                summary: "The provider accepted and completed the restart.",
              },
              verification: {
                status: verification,
                evidenceClass: "agent_attested",
                ...(verification === "contradicted"
                  ? { reasonCode: "postcondition_contradicted" }
                  : {}),
                summary: evidence.summary,
                evidence: [evidence],
              },
              compensation: {
                support: "unavailable",
                status: "not_available",
                summary: "Container restart is not rollbackable.",
              },
            },
          },
          verificationOutcome: {
            status: verification === "confirmed" ? "verified" : "failed",
            evidenceSummary: evidence.summary,
          },
        }
      : { verificationOutcome: { status: "unknown" } }),
  };
};

async function mockGovernedAttentionAction(
  page: Page,
  verification: GovernedActionVerification,
) {
  await mockAttention(page, "active");
  let actionState: "unplanned" | "pending_approval" | "approved" | "completed" =
    "unplanned";
  let executeCalls = 0;

  await page.route("**/api/ai/patrol/attention**", async (route) => {
    const url = new URL(route.request().url());
    if (route.request().method() === "POST" && url.pathname.endsWith("/plan")) {
      actionState = "pending_approval";
      const audit = governedActionAudit(actionState, verification);
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(audit.plan),
      });
      return;
    }
    const item = governedActionAttentionItem(actionState, verification);
    if (url.pathname.endsWith("/summary")) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          activeCount: 1,
          openCount: 1,
          acknowledgedCount: 0,
          suppressedCount: 0,
          uncertainCount: 0,
          resolvedCount: 0,
          calm: false,
          coverageState: "current",
          evaluatedAt,
        }),
      });
      return;
    }
    if (url.pathname !== "/api/ai/patrol/attention") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          item,
          operationalRecord: {
            id: governedActionAttentionID,
            canonicalSpecId: "docker-container-health",
            subjectResourceId: governedResourceID,
            state: "open",
            severity: "critical",
            firstObservedAt,
            lastObservedAt,
            stateChangedAt: firstObservedAt,
            evidenceIds: ["docker-health-evidence"],
            causeKey: "docker-container-health",
            relatedResourceIds: [],
            impactSummary: item.impact,
            recommendedNextStep: item.recommendedNextStep,
          },
          timeline: [],
          evidence: [
            {
              id: "docker-health-evidence",
              source: {
                provider: "docker",
                collector: "docker-container-health",
              },
              subject: { resourceId: governedResourceID },
              observedAt: lastObservedAt,
              ingestedAt: lastObservedAt,
              validUntil: new Date(Date.now() + 5 * 60_000).toISOString(),
              completeness: "complete",
              confidence: "confirmed",
              permissions: "sufficient",
            },
          ],
        }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: [item],
        summary: {
          activeCount: 1,
          openCount: 1,
          acknowledgedCount: 0,
          suppressedCount: 0,
          uncertainCount: 0,
          resolvedCount: 0,
          calm: false,
          coverageState: "current",
          evaluatedAt,
        },
        meta: { page: 1, limit: 50, total: 1, totalPages: 1 },
      }),
    });
  });

  await page.route("**/api/actions/**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/decision")) {
      actionState = "approved";
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          actionId: governedActionID,
          state: actionState,
          approval: { outcome: "approved" },
          audit: governedActionAudit(actionState, verification),
        }),
      });
      return;
    }
    if (url.pathname.endsWith("/execute")) {
      executeCalls += 1;
      actionState = "completed";
      const audit = governedActionAudit(actionState, verification);
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          actionId: governedActionID,
          state: actionState,
          result: audit.result,
          audit,
        }),
      });
      return;
    }
    const audit = governedActionAudit(
      actionState === "unplanned" ? "pending_approval" : actionState,
      verification,
    );
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        audit,
        events: [],
        ...(actionState === "completed"
          ? {
              attempt: {
                id: `${governedActionID}.dispatch.1`,
                actionId: governedActionID,
                state: "receipt_recorded",
                createdAt: firstObservedAt,
                updatedAt: evaluatedAt,
                dispatchCount: 1,
              },
              receipt: {
                attemptId: `${governedActionID}.dispatch.1`,
                actionId: governedActionID,
                transportRequestId: `${governedActionID}.dispatch.1`,
                receivedAt: evaluatedAt,
              },
            }
          : {}),
      }),
    });
  });
  return {
    executeCalls: () => executeCalls,
  };
}

for (const verification of [
  "confirmed",
  "contradicted",
] as GovernedActionVerification[]) {
  test(`runs the governed restart and represents ${verification} verification honestly`, async ({
    page,
  }) => {
    if (verification === "contradicted") {
      await page.setViewportSize({ width: 390, height: 844 });
      await page.emulateMedia({ reducedMotion: "reduce" });
    }
    const fixture = await mockGovernedAttentionAction(page, verification);
    await page.goto("/patrol", { waitUntil: "domcontentloaded" });

    const itemButton = page.getByRole("button", {
      name: "Open API container is unhealthy",
    });
    await itemButton.focus();
    await page.keyboard.press("Enter");
    const detailPanel = page.getByRole("complementary", {
      name: "API container is unhealthy",
    });
    await expect(detailPanel).toBeVisible();
    await expect(
      detailPanel.getByText(
        "The same container is observed running after the restart.",
      ),
    ).toBeVisible();
    await expect(
      detailPanel.getByText(
        /explicit review and approval before Pulse sends anything/i,
      ),
    ).toBeVisible();

    const reviewTrigger = detailPanel.getByRole("button", {
      name: "Review and approve",
    });
    await reviewTrigger.click();
    const dialog = page.getByRole("dialog", { name: "Restart" });
    await expect(dialog).toBeVisible();
    await expect(
      dialog.getByText(
        "Restart API container after fresh confirmed evidence reported an unhealthy container.",
      ),
    ).toBeVisible();
    await dialog.getByText("Policy evidence", { exact: true }).click();
    await expect(
      dialog.getByText("Eligible for low-risk automation"),
    ).toBeVisible();

    await dialog.getByRole("button", { name: "Approve" }).click();
    await expect(
      dialog.getByRole("button", { name: "Run action" }),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Run action" }).click();
    await expect(
      dialog.getByRole("button", { name: "Run action" }),
    ).toHaveCount(0);
    await dialog.getByRole("button", { name: "Close", exact: true }).click();
    await expect(
      detailPanel.getByRole("button", { name: "Review action" }),
    ).toBeFocused();
    expect(fixture.executeCalls()).toBe(1);
    await expect(
      detailPanel.getByText(/recorded the action result below/i),
    ).toBeVisible();
    await expect(
      detailPanel.getByText(
        /explicit review and approval before Pulse sends anything/i,
      ),
    ).toHaveCount(0);

    if (verification === "confirmed") {
      await expect(
        detailPanel.getByText(/restart postcondition was confirmed/i),
      ).toBeVisible();
      await expect(
        detailPanel.getByText(/issue stays open until fresh health evidence/i),
      ).toBeVisible();
    } else {
      await expect(
        detailPanel.getByText(/restart did not satisfy its postcondition/i),
      ).toBeVisible();
      await expect(detailPanel.getByText(/issue remains open/i)).toBeVisible();
      const overflows = await page.evaluate(
        () =>
          document.documentElement.scrollWidth >
          document.documentElement.clientWidth,
      );
      expect(overflows).toBeFalsy();
    }
  });
}
