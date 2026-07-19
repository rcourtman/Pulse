import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect, type Page, type Route } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
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
        `operational-trust-availability-facet-${workerInfo.project.name}.json`,
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

const DOCKER_HOST_ID = "docker-host:operational-trust";
const DOCKER_HOST_NAME = "Operational Trust Docker Host";
const ATTACHED_TCP_TARGET_ID = "ops-api";
const ATTACHED_HTTPS_TARGET_ID = "ops-web";
const CANONICAL_AVAILABILITY_SPEC_ID =
  "alertspec:provider-incident:22f0f1f19599cd71";
const AVAILABILITY_EVIDENCE_ID = "evidence_2825048c9470f82ba5490f8f8496813a";
const ATTENTION_ID = `${DOCKER_HOST_ID}::${CANONICAL_AVAILABILITY_SPEC_ID}`;

type RouteResource = Record<string, unknown>;

const resourceResponse = (resources: RouteResource[]) => ({
  data: resources,
  meta: {
    page: 1,
    limit: 100,
    total: resources.length,
    totalPages: resources.length > 0 ? 1 : 0,
  },
  links: { next: null },
});

async function routeResources(page: Page, resources: RouteResource[]) {
  // Keep the fixture authoritative: an empty mocked socket makes the
  // websocket-first resource hook fall back to the routed REST snapshot
  // without accepting a live backend frame.
  await page.routeWebSocket("**/ws", () => {});
  await page.route("**/api/resources**", async (route) => {
    const requestUrl = new URL(route.request().url());
    if (requestUrl.pathname !== "/api/resources") {
      await route.continue();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(resourceResponse(resources)),
    });
  });
}

const evidenceEnvelope = ({
  id,
  resourceId,
  targetId,
  observedAt,
  validUntil,
  completeness = "complete",
  confidence = "confirmed",
}: {
  id: string;
  resourceId: string;
  targetId: string;
  observedAt: string;
  validUntil?: string;
  completeness?: "complete" | "partial" | "unavailable";
  confidence?: "confirmed" | "inferred" | "unknown";
}) => ({
  id,
  source: {
    provider: "availability",
    collector: "availability-poller",
  },
  subject: {
    resourceId,
    providerRef: targetId,
    providerScope: "availability-target",
  },
  observedAt,
  ingestedAt: observedAt,
  validUntil,
  completeness,
  confidence,
  permissions: "sufficient",
  payloadRef: {
    kind: "availability-target",
    id: targetId,
  },
  correlation: {
    rule: "explicit-linked-resource",
    matchedFields: {
      linkedResourceId: resourceId,
    },
    candidateCount: 1,
  },
});

const availabilityCheck = ({
  targetId,
  address,
  protocol,
  observedAt,
  validUntil,
  port,
  path: targetPath,
  latencyMillis,
}: {
  targetId: string;
  address: string;
  protocol: string;
  observedAt: string;
  validUntil: string;
  port?: number;
  path?: string;
  latencyMillis: number;
}) => ({
  targetId,
  linkedResourceId: DOCKER_HOST_ID,
  name: targetId,
  targetKind: "service",
  address,
  protocol,
  port,
  path: targetPath,
  enabled: true,
  available: true,
  lastChecked: observedAt,
  lastSuccess: observedAt,
  latencyMillis,
  consecutiveFailures: 0,
  failureThreshold: 2,
  pollIntervalSeconds: 60,
  timeoutMillis: 5_000,
  correlationState: "attached",
  correlationRule: "explicit-linked-resource",
  correlationCandidates: 1,
  evidence: evidenceEnvelope({
    id: `evidence_${targetId}`,
    resourceId: DOCKER_HOST_ID,
    targetId,
    observedAt,
    validUntil,
  }),
});

test.describe("Operational trust availability resource facet", () => {
  test.setTimeout(180_000);

  test("renders attached checks once on the Docker host and exposes both observations in detail", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop table and drawer proof",
    );

    const now = Date.now();
    const firstObservedAt = new Date(now - 2 * 60_000).toISOString();
    const secondObservedAt = new Date(now - 45_000).toISOString();
    const validUntil = new Date(now + 5 * 60_000).toISOString();
    const tcpCheck = availabilityCheck({
      targetId: ATTACHED_TCP_TARGET_ID,
      address: "192.0.2.18",
      protocol: "tcp",
      port: 8007,
      observedAt: firstObservedAt,
      validUntil,
      latencyMillis: 12,
    });
    const httpsCheck = availabilityCheck({
      targetId: ATTACHED_HTTPS_TARGET_ID,
      address: "ops.example.test",
      protocol: "https",
      path: "/health",
      observedAt: secondObservedAt,
      validUntil,
      latencyMillis: 23,
    });

    await routeResources(page, [
      {
        id: DOCKER_HOST_ID,
        type: "docker-host",
        name: DOCKER_HOST_NAME,
        status: "online",
        lastSeen: secondObservedAt,
        sources: ["docker", "availability"],
        docker: {
          hostname: "ops-docker.example.test",
          runtime: "docker",
          runtimeVersion: "27.5.1",
          containerCount: 4,
          hostSourceId: "ops-docker",
          uptimeSeconds: 86_400,
        },
        availability: tcpCheck,
        availabilityChecks: [tcpCheck, httpsCheck],
      },
    ]);

    await page.goto("/docker/overview", { waitUntil: "domcontentloaded" });

    const dockerPage = page.getByTestId("docker-page");
    await expect(dockerPage).toBeVisible({ timeout: 30_000 });
    const hostRows = dockerPage.locator(
      `[data-docker-host-row="${DOCKER_HOST_ID}"]`,
    );
    await expect(hostRows).toHaveCount(1);
    const hostRow = hostRows.first();
    await expect(hostRow).toContainText(DOCKER_HOST_NAME);
    await expect(
      hostRow.getByTitle(/TCP availability probe.*192\.0\.2\.18:8007/i).last(),
    ).toBeVisible();

    await hostRow.click();

    const drawer = dockerPage.getByTestId("docker-host-drawer");
    await expect(drawer).toBeVisible();
    const cards = drawer.getByTestId("availability-probe-status");
    await expect(cards).toHaveCount(2);

    const tcpCard = cards.filter({ hasText: "192.0.2.18:8007" });
    await expect(tcpCard).toHaveCount(1);
    await expect(tcpCard).toContainText("TCP 8007");
    await expect(tcpCard).toContainText("Up");
    await expect(tcpCard).toContainText("12ms");
    await expect(tcpCard).toContainText("fresh");
    await expect(tcpCard).toContainText("Checked");
    await expect(tcpCard.getByText(/(?:ago|now)/i)).toBeVisible();

    const httpsCard = cards.filter({ hasText: "ops.example.test" });
    await expect(httpsCard).toHaveCount(1);
    await expect(httpsCard).toContainText("HTTPS /health");
    await expect(httpsCard).toContainText("Up");
    await expect(httpsCard).toContainText("23ms");
    await expect(httpsCard).toContainText("fresh");
    await expect(httpsCard).toContainText("Checked");
  });

  test("keeps attached checks out of standalone inventory and does not infer health from stale or missing observations", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop availability table proof",
    );

    const now = Date.now();
    const staleObservedAt = new Date(now - 15 * 60_000).toISOString();
    const staleValidUntil = new Date(now - 10 * 60_000).toISOString();
    const freshObservedAt = new Date(now - 30_000).toISOString();
    const freshValidUntil = new Date(now + 2 * 60_000).toISOString();

    await routeResources(page, [
      {
        id: DOCKER_HOST_ID,
        type: "docker-host",
        name: DOCKER_HOST_NAME,
        status: "online",
        lastSeen: freshObservedAt,
        sources: ["docker", "availability"],
        availability: {
          targetId: ATTACHED_TCP_TARGET_ID,
          linkedResourceId: DOCKER_HOST_ID,
          address: "192.0.2.18",
          protocol: "tcp",
          port: 8007,
          enabled: true,
          available: true,
          lastChecked: freshObservedAt,
          latencyMillis: 12,
          pollIntervalSeconds: 60,
          correlationState: "attached",
          evidence: evidenceEnvelope({
            id: "evidence_attached_tcp",
            resourceId: DOCKER_HOST_ID,
            targetId: ATTACHED_TCP_TARGET_ID,
            observedAt: freshObservedAt,
            validUntil: freshValidUntil,
          }),
        },
      },
      {
        id: "network-endpoint:standalone-switch",
        type: "network-endpoint",
        name: "Standalone lab switch",
        status: "online",
        lastSeen: freshObservedAt,
        sources: ["availability"],
        availability: {
          targetId: "standalone-switch",
          address: "192.0.2.40",
          protocol: "icmp",
          enabled: true,
          available: true,
          lastChecked: freshObservedAt,
          lastSuccess: freshObservedAt,
          latencyMillis: 4,
          pollIntervalSeconds: 60,
          correlationState: "standalone",
          evidence: evidenceEnvelope({
            id: "evidence_standalone_switch",
            resourceId: "network-endpoint:standalone-switch",
            targetId: "standalone-switch",
            observedAt: freshObservedAt,
            validUntil: freshValidUntil,
          }),
        },
      },
      {
        id: "network-endpoint:stale-success",
        type: "network-endpoint",
        name: "Stale successful service",
        status: "online",
        lastSeen: staleObservedAt,
        sources: ["availability"],
        availability: {
          targetId: "stale-success",
          address: "stale.example.test",
          protocol: "https",
          path: "/ready",
          enabled: true,
          available: true,
          lastChecked: staleObservedAt,
          lastSuccess: staleObservedAt,
          latencyMillis: 17,
          pollIntervalSeconds: 60,
          correlationState: "standalone",
          evidence: evidenceEnvelope({
            id: "evidence_stale_success",
            resourceId: "network-endpoint:stale-success",
            targetId: "stale-success",
            observedAt: staleObservedAt,
            validUntil: staleValidUntil,
          }),
        },
      },
      {
        id: "network-endpoint:not-observed",
        type: "network-endpoint",
        name: "Unobserved new service",
        status: "unknown",
        lastSeen: staleObservedAt,
        sources: ["availability"],
        availability: {
          targetId: "not-observed",
          address: "new.example.test",
          protocol: "tcp",
          port: 443,
          enabled: true,
          pollIntervalSeconds: 60,
          correlationState: "standalone",
          evidence: evidenceEnvelope({
            id: "evidence_not_observed",
            resourceId: "network-endpoint:not-observed",
            targetId: "not-observed",
            observedAt: staleObservedAt,
            completeness: "partial",
            confidence: "unknown",
          }),
        },
      },
    ]);

    await page.goto("/standalone/availability", {
      waitUntil: "domcontentloaded",
    });

    const standalonePage = page.getByTestId("standalone-page");
    await expect(standalonePage).toBeVisible({ timeout: 30_000 });
    await expect(standalonePage.getByText(DOCKER_HOST_NAME)).toHaveCount(0);
    await expect(
      standalonePage.getByText("Standalone lab switch"),
    ).toBeVisible();

    const staleRow = standalonePage.locator(
      '[data-availability-check-row="network-endpoint:stale-success"]',
    );
    await expect(staleRow).toBeVisible();
    await expect(staleRow.getByTitle("Stale", { exact: true })).toBeVisible();
    await expect(staleRow.getByText("17 ms", { exact: true })).toHaveAttribute(
      "title",
      /stale/i,
    );
    await expect(staleRow).not.toContainText("Healthy");

    const unobservedRow = standalonePage.locator(
      '[data-availability-check-row="network-endpoint:not-observed"]',
    );
    await expect(unobservedRow).toBeVisible();
    await expect(
      unobservedRow.getByText("not checked", { exact: true }),
    ).toBeVisible();
    await expect(unobservedRow).not.toContainText("Healthy");

    const posture = standalonePage.getByTestId("standalone-posture-summary");
    await expect(posture).toContainText("1 healthy");
    await expect(posture).toContainText("2 need attention");
    await expect(posture).not.toContainText("All 3 checks reporting normally");
  });

  test("routes an attached availability failure into Patrol with canonical lifecycle evidence", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop Patrol workbench proof",
    );

    const observedAt = "2026-07-19T02:00:00Z";
    const ingestedAt = "2026-07-19T02:00:01Z";
    const evaluatedAt = "2026-07-19T02:00:02Z";
    const item = {
      id: ATTENTION_ID,
      operationalRecordId: ATTENTION_ID,
      subjectResourceId: DOCKER_HOST_ID,
      subjectResourceName: DOCKER_HOST_NAME,
      subjectResourceType: "docker-host",
      title: `Availability check failed for ${DOCKER_HOST_NAME}`,
      plainLanguageSummary:
        "The attached TCP availability check failed twice and reached its alert threshold.",
      severity: "critical",
      state: "open",
      firstObservedAt: observedAt,
      lastObservedAt: observedAt,
      evidenceFreshness: "fresh",
      evidenceCompleteness: "complete",
      impact:
        "The Docker API may be unreachable even if older host telemetry remains visible.",
      relatedResources: [],
      recommendedNextStep:
        "Verify TCP connectivity to 192.0.2.18:8007 before changing the Docker host.",
      availableActions: [],
      verificationState: "not_available",
    };
    const summary = {
      activeCount: 1,
      openCount: 1,
      acknowledgedCount: 0,
      suppressedCount: 0,
      uncertainCount: 0,
      resolvedCount: 0,
      calm: false,
      coverageState: "current",
      evaluatedAt,
    };
    const availabilityEvidence = {
      id: AVAILABILITY_EVIDENCE_ID,
      source: {
        provider: "availability",
        collector: "availability-poller",
      },
      subject: {
        resourceId: DOCKER_HOST_ID,
        providerRef: ATTACHED_TCP_TARGET_ID,
        providerScope: "availability-target",
      },
      observedAt,
      ingestedAt,
      validUntil: "2026-07-19T02:02:00Z",
      completeness: "complete",
      confidence: "confirmed",
      permissions: "sufficient",
      reason: {
        code: "availability_unreachable",
        message: "TCP probe to 192.0.2.18:8007 failed twice.",
      },
      payloadRef: {
        kind: "availability-target",
        id: ATTACHED_TCP_TARGET_ID,
      },
    };
    const detail = {
      item,
      operationalRecord: {
        id: ATTENTION_ID,
        canonicalSpecId: CANONICAL_AVAILABILITY_SPEC_ID,
        subjectResourceId: DOCKER_HOST_ID,
        state: "open",
        severity: "critical",
        firstObservedAt: observedAt,
        lastObservedAt: observedAt,
        stateChangedAt: observedAt,
        evidenceIds: [AVAILABILITY_EVIDENCE_ID],
        causeKey: ATTENTION_ID,
        relatedResourceIds: [],
        impactSummary: item.impact,
        recommendedNextStep: item.recommendedNextStep,
      },
      timeline: [
        {
          id: "transition_availability_open",
          operationalRecordId: ATTENTION_ID,
          from: "observing",
          to: "open",
          at: observedAt,
          cause: "detector_decision",
          causeKey: ATTENTION_ID,
          evidenceIds: [AVAILABILITY_EVIDENCE_ID],
          reason:
            "The attached availability check reached its failure threshold.",
        },
      ],
      evidence: [availabilityEvidence],
    };

    await routeResources(page, []);
    await routePatrolSupport(page, { item, summary, detail });

    await page.goto("/patrol", { waitUntil: "domcontentloaded" });

    const queue = page.getByRole("region", { name: "Needs attention" });
    await expect(queue).toBeVisible({ timeout: 30_000 });
    await expect(
      page.getByRole("tab", { name: "Patrol: 1 active attention item" }),
    ).toBeVisible();
    await expect(queue.getByText(item.plainLanguageSummary)).toBeVisible();

    const detailResponsePromise = page.waitForResponse((response) => {
      const requestUrl = new URL(response.url());
      return (
        requestUrl.pathname.startsWith("/api/ai/patrol/attention/") &&
        !requestUrl.pathname.endsWith("/summary")
      );
    });
    await queue.getByRole("button", { name: `Open ${item.title}` }).click();
    const detailResponse = await detailResponsePromise;
    const routedDetail = (await detailResponse.json()) as typeof detail;

    expect(routedDetail.operationalRecord.canonicalSpecId).toBe(
      CANONICAL_AVAILABILITY_SPEC_ID,
    );
    expect(routedDetail.operationalRecord.evidenceIds).toEqual([
      AVAILABILITY_EVIDENCE_ID,
    ]);
    expect(routedDetail.evidence).toEqual([availabilityEvidence]);

    const detailPanel = page.getByRole("complementary", { name: item.title });
    await expect(detailPanel).toBeVisible();
    await expect(
      detailPanel.getByText("Availability", { exact: true }),
    ).toBeVisible();
    await expect(
      detailPanel.getByText(/availability-poller · observed/i),
    ).toBeVisible();
    await expect(
      detailPanel.getByText("TCP probe to 192.0.2.18:8007 failed twice."),
    ).toBeVisible();
    await expect(detailPanel.getByText("Observing to Open")).toBeVisible();
  });
});

async function routePatrolSupport(
  page: Page,
  fixture: {
    item: Record<string, unknown>;
    summary: Record<string, unknown>;
    detail: Record<string, unknown>;
  },
) {
  await page.route("**/api/replication/jobs", async (route) => {
    await fulfillJSON(route, []);
  });
  await page.route("**/api/ai/patrol/status", async (route) => {
    await fulfillJSON(route, {
      runtime_state: "active",
      running: false,
      enabled: true,
      last_patrol_at: fixture.item.lastObservedAt,
      next_patrol_at: "2026-07-19T08:00:00Z",
      last_duration_ms: 1_200,
      resources_checked: 1,
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
    });
  });
  await page.route("**/api/ai/patrol/runs*", async (route) => {
    await fulfillJSON(route, []);
  });
  await page.route("**/api/ai/patrol/autonomy", async (route) => {
    await fulfillJSON(route, {
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
    });
  });
  await page.route("**/api/ai/patrol/findings*", async (route) => {
    await fulfillJSON(route, []);
  });
  await page.route("**/api/ai/unified/findings*", async (route) => {
    await fulfillJSON(route, { findings: [], count: 0, active_count: 0 });
  });
  await page.route("**/api/ai/intelligence", async (route) => {
    await fulfillJSON(route, {
      timestamp: fixture.summary.evaluatedAt,
      overall_health: {
        score: 100,
        grade: "A",
        trend: "stable",
        factors: [],
        prediction: "Operational lifecycle attention is routed separately.",
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
    });
  });
  await page.route("**/api/ai/intelligence/correlations*", async (route) => {
    await fulfillJSON(route, { correlations: [], count: 0 });
  });
  await page.route("**/api/ai/circuit/status", async (route) => {
    await fulfillJSON(route, {
      state: "closed",
      can_patrol: true,
      consecutive_failures: 0,
      total_successes: 1,
      total_failures: 0,
    });
  });
  await page.route("**/api/ai/approvals", async (route) => {
    await fulfillJSON(route, { approvals: [] });
  });
  await page.route("**/api/settings/ai", async (route) => {
    await fulfillJSON(route, {
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
    });
  });
  await page.route("**/api/ai/models", async (route) => {
    await fulfillJSON(route, { models: [] });
  });
  await page.route("**/api/ai/patrol/attention**", async (route) => {
    const requestUrl = new URL(route.request().url());
    if (requestUrl.pathname.endsWith("/summary")) {
      await fulfillJSON(route, fixture.summary);
      return;
    }
    if (requestUrl.pathname !== "/api/ai/patrol/attention") {
      await fulfillJSON(route, fixture.detail);
      return;
    }
    await fulfillJSON(route, {
      data: [fixture.item],
      summary: fixture.summary,
      meta: {
        page: 1,
        limit: 50,
        total: 1,
        totalPages: 1,
      },
    });
  });
}

async function fulfillJSON(route: Route, body: unknown) {
  await route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}
