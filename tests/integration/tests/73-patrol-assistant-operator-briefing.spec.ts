import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base } from "@playwright/test";

import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const SCREENSHOT_PATH = "/tmp/patrol-assistant-operator-briefing.png";

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
        `patrol-assistant-operator-briefing-${workerInfo.project.name}.json`,
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

test.describe("Patrol Assistant operator briefing", () => {
  test.setTimeout(180_000);

  test("shows attention and operator-decision context in the Assistant drawer", async ({
    page,
  }) => {
    const approvalRequestedAt = new Date(Date.now() - 60_000).toISOString();
    const approvalExpiresAt = new Date(Date.now() + 10 * 60_000).toISOString();
    let includePendingApproval = true;
    let includeUnifiedInvestigationRecord = true;
    let includeInvestigationProposedFix = false;
    const handoffSession = {
      id: "session-operator-briefing",
      title: "High CPU follow-up",
      created_at: "2026-05-06T12:00:00Z",
      updated_at: "2026-05-06T12:08:00Z",
      message_count: 2,
      handoff_summary: {
        kind: "patrol_finding",
        finding_id: "finding-operator-briefing",
        has_model_context: true,
        resource_count: 1,
        primary_resource: {
          id: "host:web-server",
          name: "web-server",
          type: "host",
          node: "pve-1",
        },
        action_count: 1,
        requires_approval: true,
        last_known_approval_status: "pending",
        last_known_action_state: "awaiting_approval",
        last_known_action_risk: "high",
        updated_at: "2026-05-06T12:08:00Z",
      },
    };
    const contextOnlyHandoffSession = {
      id: "session-context-only",
      title: "Context-only Patrol follow-up",
      created_at: "2026-05-06T12:02:00Z",
      updated_at: "2026-05-06T12:09:00Z",
      message_count: 1,
      handoff_summary: {
        kind: "patrol_finding",
        finding_id: "finding-context-only",
        has_model_context: true,
        resource_count: 1,
        primary_resource: {
          id: "host:web-server",
          name: "web-server",
          type: "host",
          node: "pve-1",
        },
        action_count: 0,
        requires_approval: false,
        updated_at: "2026-05-06T12:09:00Z",
      },
    };
    const patrolRunHandoffSession = {
      id: "session-patrol-run",
      title: "Runtime failure follow-up",
      created_at: "2026-05-06T12:04:00Z",
      updated_at: "2026-05-06T12:10:00Z",
      message_count: 1,
      handoff_summary: {
        kind: "patrol_run",
        run_id: "run-runtime-error",
        run_type: "Scoped run",
        run_status: "error",
        runtime_failure: true,
        has_model_context: true,
        resource_count: 1,
        primary_resource: {
          id: "vm-100",
          type: "vm",
        },
        action_count: 0,
        requires_approval: false,
        updated_at: "2026-05-06T12:10:00Z",
      },
    };

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
          data: [
            {
              id: "host:web-server",
              type: "host",
              name: "web-server",
              displayName: "web-server",
              status: "online",
              lastSeen: "2026-05-06T12:10:00Z",
              canonicalIdentity: {
                displayName: "web-server",
                hostname: "web-server",
              },
            },
          ],
          meta: {
            page: 1,
            limit: 100,
            total: 1,
            totalPages: 1,
          },
        }),
      });
    });

    await page.route("**/api/ai/status", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ running: true, engine: "test" }),
      });
    });

    await page.route("**/api/ai/sessions/*/messages", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "msg-operator-briefing",
            role: "assistant",
            content: "Previous governed finding context.",
            timestamp: "2026-05-06T12:08:00Z",
          },
        ]),
      });
    });

    await page.route("**/api/ai/sessions", async (route) => {
      if (route.request().method() !== "GET") {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          handoffSession,
          contextOnlyHandoffSession,
          patrolRunHandoffSession,
        ]),
      });
    });

    await page.route("**/api/ai/models", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          models: [{ id: "openai:gpt-4o-mini", name: "GPT-4o mini" }],
        }),
      });
    });

    await page.route("**/api/settings/ai", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          model: "openai:gpt-4o-mini",
          chat_model: "",
          control_level: "read_only",
          discovery_enabled: true,
          patrol_enabled: true,
          patrol_interval_minutes: 360,
          patrol_model: "",
          alert_triggered_analysis: true,
          patrol_alert_triggers_enabled: true,
          patrol_anomaly_triggers_enabled: true,
          patrol_event_triggers_enabled: true,
          patrol_auto_fix: false,
          auto_fix_model: "",
        }),
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
          last_patrol_at: "2026-05-06T12:00:00Z",
          last_activity_at: "2026-05-06T12:06:00Z",
          next_patrol_at: "2026-05-06T18:00:00Z",
          last_duration_ms: 180000,
          resources_checked: 12,
          findings_count: 1,
          error_count: 0,
          healthy: false,
          interval_ms: 21600000,
          fixed_count: 0,
          blocked_reason: "",
          blocked_at: "",
          license_required: false,
          license_status: "active",
          summary: {
            critical: 1,
            warning: 0,
            watch: 0,
            info: 0,
          },
        }),
      });
    });

    await page.route("**/api/ai/patrol/runs*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "run-operator-briefing",
            started_at: "2026-05-06T12:00:00Z",
            completed_at: "2026-05-06T12:03:00Z",
            duration_ms: 180000,
            type: "full",
            trigger_reason: "scheduled",
            resources_checked: 12,
            findings_summary: "1 finding",
            finding_ids: ["finding-operator-briefing"],
            error_count: 0,
            status: "warning",
            triage_flags: 0,
            tool_call_count: 0,
          },
        ]),
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

    await page.route("**/api/ai/unified/findings*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          findings: [
            {
              id: "finding-operator-briefing",
              source: "ai-patrol",
              severity: "critical",
              category: "performance",
              resource_id: "host:web-server",
              resource_name: "web-server",
              resource_type: "host",
              title: "High CPU usage",
              description: "CPU stayed above 95%.",
              detected_at: "2026-05-06T12:00:00Z",
              last_seen_at: "2026-05-06T12:06:00Z",
              status: "active",
              times_raised: 4,
              regression_count: 2,
              last_regression_at: "2026-05-06T12:06:00Z",
              loop_state: "awaiting_approval",
              remediation_id: "remediation-1",
              investigation_status: "completed",
              investigation_outcome: "fix_queued",
              investigation_attempts: 1,
              ...(includeUnifiedInvestigationRecord
                ? {
                    investigation_record: {
                      id: "record-1",
                      finding_id: "finding-operator-briefing",
                      subject: {
                        resource_id: "host:web-server",
                        resource_name: "web-server",
                        resource_type: "host",
                      },
                      trigger: {
                        detected_at: "2026-05-06T12:00:00Z",
                        title: "High CPU usage",
                      },
                      status: "completed",
                      outcome: "fix_queued",
                      confidence: "high",
                      conclusion: "Backup job saturated CPU.",
                      recommended_action:
                        "Approve a controlled restart after the backup completes.",
                      evidence: [
                        {
                          kind: "metrics",
                          summary: "CPU stayed above 95% for 10 minutes",
                        },
                      ],
                      proposed_fix: {
                        id: "fix-1",
                        description: "Restart the workload service",
                        commands: ["systemctl restart workload.service"],
                        risk_level: "medium",
                        destructive: true,
                      },
                      verification: ["CPU returned below 50%"],
                      tools_used: [],
                      started_at: "2026-05-06T12:00:00Z",
                      approval_id: "approval-1",
                    },
                  }
                : {}),
            },
          ],
          count: 1,
          active_count: 1,
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

    await page.route("**/api/ai/intelligence", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          timestamp: "2026-05-06T12:06:00Z",
          overall_health: {
            score: 62,
            grade: "D",
            trend: "degrading",
            factors: [],
            prediction: "web-server needs operator attention.",
          },
          findings_count: {
            critical: 1,
            warning: 0,
            watch: 0,
            info: 0,
            total: 1,
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

    await page.route("**/api/ai/circuit/status", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          state: "closed",
          can_patrol: true,
          consecutive_failures: 0,
          total_successes: 42,
          total_failures: 0,
        }),
      });
    });

    await page.route("**/api/ai/approvals", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          approvals: includePendingApproval
            ? [
                {
                  id: "approval-1",
                  toolId: "investigation_fix",
                  command: "systemctl restart workload.service",
                  targetType: "host",
                  targetId: "finding-operator-briefing",
                  targetName: "web-server",
                  context: "Restart the workload service",
                  riskLevel: "high",
                  status: "pending",
                  requestedAt: approvalRequestedAt,
                  expiresAt: approvalExpiresAt,
                },
              ]
            : [],
        }),
      });
    });

    await page.route("**/api/ai/findings/*/investigation", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          includeInvestigationProposedFix
            ? {
                id: "session-operator-briefing",
                finding_id: "finding-operator-briefing",
                session_id: "session-operator-briefing",
                status: "completed",
                started_at: "2026-05-06T12:00:00Z",
                turn_count: 1,
                outcome: "fix_queued",
                proposed_fix: {
                  id: "fix-expired-1",
                  description: "Restart the workload service",
                  commands: ["systemctl restart workload.service"],
                  risk_level: "high",
                  destructive: true,
                  target_host: "web-server",
                  rationale:
                    "Workload service stayed wedged after backup pressure.",
                },
              }
            : null,
        ),
      });
    });

    await page.route("**/api/ai/remediation/plans", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ plans: [] }),
      });
    });

    await page.goto("/patrol", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "Findings" })).toBeVisible();

    await page.getByTestId("patrol-assessment-assistant-button").click();
    const assessmentAssistantContext = page.getByLabel("Assistant context");
    await expect(assessmentAssistantContext).toBeVisible();
    await expect(assessmentAssistantContext).toContainText(
      "Patrol assessment attached",
    );
    await expect(assessmentAssistantContext).toContainText(
      "live approval pending",
    );
    await expect(assessmentAssistantContext).toContainText("High risk");
    await expect(
      assessmentAssistantContext.getByRole("button", {
        name: "Review pending approvals and safest next step",
      }),
    ).toBeVisible();
    await expect(
      assessmentAssistantContext.getByText(
        "systemctl restart workload.service",
      ),
    ).toHaveCount(0);
    const dockedAssistantClose = page.getByTitle("Collapse Pulse Assistant");
    if ((await dockedAssistantClose.count()) > 0) {
      await dockedAssistantClose.first().click();
    } else {
      await page.getByLabel("Close Pulse Assistant").first().click();
    }

    await page.getByText("High CPU usage").click();
    const finding = page.locator("#finding-finding-operator-briefing");
    await finding
      .getByRole("button", { name: "Discuss with Assistant" })
      .first()
      .click();

    const assistantContext = page.getByLabel("Assistant context");
    await expect(assistantContext).toBeVisible();
    await expect(assistantContext).toContainText("Operator briefing attached");
    await expect(assistantContext).toContainText(
      "Attention: active critical finding; regressed 2 times; last regression 2026-05-06T12:06:00Z; loop awaiting approval; approval approval-1; live approval pending; destructive proposed fix; fix queued for governed review",
    );
    await expect(assistantContext).toContainText(
      `Decision: review live governed approval approval-1 before execution; approval pending; target web-server; expires ${approvalExpiresAt}; requested ${approvalRequestedAt}; proposed fix fix-1; risk high; destructive true`,
    );
    await expect(assistantContext).toContainText(
      "Command details stay in approval context; destructive actions require governed approval.",
    );
    await expect(
      assistantContext.getByRole("button", {
        name: "Review approval risk and next step",
      }),
    ).toBeVisible();
    await expect(
      assistantContext.getByRole("button", {
        name: "Explain Patrol evidence and confidence",
      }),
    ).toBeVisible();
    await expect(
      assistantContext.getByRole("button", {
        name: "Summarize remediation without command text",
      }),
    ).toBeVisible();
    await expect(
      assistantContext.getByText("systemctl restart workload.service"),
    ).toHaveCount(0);

    await assistantContext
      .getByRole("button", { name: "Review approval risk and next step" })
      .click();
    await expect(
      page.getByPlaceholder("Ask about your infrastructure..."),
    ).toHaveValue("Review approval risk and next step");

    await page.screenshot({ path: SCREENSHOT_PATH, fullPage: true });

    await page.getByTitle("Pulse Assistant sessions").click();
    await expect(page.getByText("High CPU follow-up")).toBeVisible();
    await expect(page.getByText("Pulse Patrol").last()).toBeVisible();
    await expect(page.getByText("Approval required").last()).toBeVisible();
    await expect(page.getByText(/approval pending/).last()).toBeVisible();
    await page.getByText("High CPU follow-up").click();

    const reloadedAssistantContext = page.getByLabel("Assistant context");
    await expect(reloadedAssistantContext).toBeVisible();
    await expect(reloadedAssistantContext).toContainText(
      "Patrol finding on web-server",
    );
    await expect(reloadedAssistantContext).toContainText(
      "Finding finding-operator-briefing",
    );
    await expect(reloadedAssistantContext).toContainText(
      "Last known state: approval pending",
    );
    await expect(
      reloadedAssistantContext.getByText("systemctl restart workload.service"),
    ).toHaveCount(0);

    await page.getByTitle("Pulse Assistant sessions").click();
    await expect(page.getByText("Context-only Patrol follow-up")).toBeVisible();
    await expect(page.getByText("Context attached").last()).toBeVisible();
    await page.getByText("Context-only Patrol follow-up").click();

    const contextOnlyAssistantContext = page.getByLabel("Assistant context");
    await expect(contextOnlyAssistantContext).toBeVisible();
    await expect(contextOnlyAssistantContext).toContainText(
      "Patrol finding on web-server",
    );
    await expect(contextOnlyAssistantContext).toContainText(
      "Finding finding-context-only",
    );
    await expect(
      contextOnlyAssistantContext.getByText(
        "systemctl restart workload.service",
      ),
    ).toHaveCount(0);

    await page.getByTitle("Pulse Assistant sessions").click();
    await expect(page.getByText("Runtime failure follow-up")).toBeVisible();
    await expect(page.getByText("Runtime issue").last()).toBeVisible();
    await expect(page.getByText(/run error/).last()).toBeVisible();
    await page.getByText("Runtime failure follow-up").click();

    const runAssistantContext = page.getByLabel("Assistant context");
    await expect(runAssistantContext).toBeVisible();
    await expect(runAssistantContext).toContainText(
      "Patrol run run-runtime-error",
    );
    await expect(runAssistantContext).toContainText("Run run-runtime-error");
    await expect(runAssistantContext).toContainText("Run type: Scoped run");
    await expect(runAssistantContext).toContainText("Run state: run error");
    await expect(runAssistantContext).toContainText(
      "Review Patrol runtime issue",
    );
    await expect(runAssistantContext.getByText("tool_choice")).toHaveCount(0);
    await expect(
      runAssistantContext.getByText("No endpoints found"),
    ).toHaveCount(0);

    includePendingApproval = false;
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "Findings" })).toBeVisible();

    await page.getByText("High CPU usage").click();
    const queuedFinding = page.locator("#finding-finding-operator-briefing");
    await expect(queuedFinding.getByText("details unavailable")).toBeVisible();
    await queuedFinding
      .getByRole("button", { name: "Discuss with Assistant" })
      .last()
      .click();

    const queuedAssistantContext = page.getByLabel("Assistant context");
    await expect(queuedAssistantContext).toBeVisible();
    await expect(queuedAssistantContext).toContainText(
      "Operator briefing attached",
    );
    await expect(queuedAssistantContext).toContainText("Fix Queued");
    await expect(queuedAssistantContext).toContainText(
      "Attention: active finding; loop fix queued; fix queued for governed review",
    );
    await expect(queuedAssistantContext).toContainText(
      "Decision: Recover or regenerate the governed approval before execution; do not execute from chat context.",
    );
    await expect(
      queuedAssistantContext.getByRole("button", {
        name: "List approval prerequisites before action",
      }),
    ).toBeVisible();
    await expect(
      queuedAssistantContext.getByText("systemctl restart workload.service"),
    ).toHaveCount(0);

    includeUnifiedInvestigationRecord = false;
    includeInvestigationProposedFix = true;
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "Findings" })).toBeVisible();

    await page.getByText("High CPU usage").click();
    const expiredFinding = page.locator("#finding-finding-operator-briefing");
    await expect(expiredFinding.getByText("approval expired")).toBeVisible();
    await expiredFinding
      .getByRole("button", { name: "Discuss with Assistant" })
      .first()
      .click();

    const hydratedFindingAssistantContext =
      page.getByLabel("Assistant context");
    await expect(hydratedFindingAssistantContext).toBeVisible();
    await expect(hydratedFindingAssistantContext).toContainText(
      "Operator briefing attached",
    );
    await expect(hydratedFindingAssistantContext).toContainText("Fix Queued");
    await expect(hydratedFindingAssistantContext).toContainText(
      "Proposed fix: Restart the workload service; target web-server; high risk; 1 command recorded for approval context; destructive proposed fix; rationale Workload service stayed wedged after backup pressure.",
    );
    await expect(
      hydratedFindingAssistantContext.getByRole("button", {
        name: "Summarize remediation without command text",
      }),
    ).toBeVisible();
    await expect(
      hydratedFindingAssistantContext.getByText(
        "systemctl restart workload.service",
      ),
    ).toHaveCount(0);

    await expiredFinding
      .getByRole("button", { name: "Fix with Assistant" })
      .last()
      .click();

    const expiredAssistantContext = page.getByLabel("Assistant context");
    await expect(expiredAssistantContext).toBeVisible();
    await expect(expiredAssistantContext).toContainText(
      "Operator briefing attached",
    );
    await expect(expiredAssistantContext).toContainText("Fix Queued");
    await expect(expiredAssistantContext).toContainText(
      "Proposed fix: Restart the workload service; target web-server; high risk; 1 command recorded for approval context; destructive proposed fix; rationale Workload service stayed wedged after backup pressure.",
    );
    await expect(expiredAssistantContext).toContainText(
      "Command details stay in approval context; destructive actions require governed approval.",
    );
    await expect(
      expiredAssistantContext.getByRole("button", {
        name: "Summarize remediation without command text",
      }),
    ).toBeVisible();
    await expect(
      expiredAssistantContext.getByText("systemctl restart workload.service"),
    ).toHaveCount(0);
  });
});
