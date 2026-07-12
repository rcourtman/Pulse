import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base } from "@playwright/test";
import { createAuthenticatedStorageState } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
type WorkerFixtures = { authStorageStatePath: string };
const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => use(authStorageStatePath),
  authStorageStatePath: [async ({ browser }, use, workerInfo) => {
    const storageStatePath = path.resolve(__dirname, "..", "..", "tmp", "playwright-auth", `actions-inbox-${workerInfo.project.name}.json`);
    fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
    await createAuthenticatedStorageState(browser, storageStatePath);
    try { await use(storageStatePath); } finally { fs.rmSync(storageStatePath, { force: true }); }
  }, { scope: "worker" }],
});

const scope = { orgId: "org-1", resourceId: "docker:container:edge", capabilityName: "restart" };
const requirement = { version: 1, floor: "admin", quorum: 1, disallowRequester: false };
const action = {
  id: "action-1", createdAt: "2026-07-12T00:00:00Z", updatedAt: "2026-07-12T00:01:00Z", state: "pending_approval", decisionRevision: 0,
  request: { requestId: "request-1", resourceId: scope.resourceId, capabilityName: scope.capabilityName, reason: "Recover the edge proxy", requestedBy: "pulse_patrol", actor: { subjectId: "patrol", kind: "service", credentialId: "patrol", orgId: "org-1" } },
  plan: { actionId: "action-1", requestId: "request-1", allowed: true, requiresApproval: true, approvalPolicy: "admin", approvalRequirement: requirement, rollbackAvailable: false, plannedAt: "2026-07-12T00:00:00Z", expiresAt: "2099-07-12T00:10:00Z", resourceVersion: "resource:sha256:one", policyVersion: "policy:sha256:one", planHash: "sha256:plan", policyDecision: { version: 1, status: "resolved", decisionId: "policy-decision:sha256:one", actionId: "action-1", scope, approvalRequirement: requirement, planningAllowed: true, requiresApproval: true, authorities: [{ kind: "capability_registry", sourceId: "capability-registry:restart", revision: "policy:sha256:one", status: "consulted", scope, approvalFloor: "admin", reasonCodes: ["capability_approval_admin", "capability_auto_low_risk"] }, { kind: "resource_operator_policy", sourceId: "resource-operator-policy:docker:container:edge", revision: "resource-policy:sha256:one", status: "consulted", scope, approvalFloor: "admin", reasonCodes: ["resource_capability_allowed", "resource_window_open"] }] } },
  verificationOutcome: { status: "unknown" },
};

test("Actions inbox exposes the canonical decision packet and durable calm history", async ({ page }, testInfo) => {
  await page.route("**/api/actions?*", async (route) => {
    const view = new URL(route.request().url()).searchParams.get("view");
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(view === "pending" ? { view, actions: [action], count: 1 } : { view, actions: [], count: 0 }) });
  });
  await page.route("**/api/actions/action-1", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ audit: action, events: [] }) }));
  await page.goto("/actions", { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("heading", { name: "Actions" })).toBeVisible();
  await page.getByRole("button", { name: /Restart.*docker:container:edge/ }).click();
  await expect(page.getByRole("dialog", { name: "Restart" })).toBeVisible();
  await expect(page.getByText("Capability safety policy")).toBeVisible();
  await expect(page.getByText("Policy for this resource")).toBeVisible();
  await expect(page.getByText("This records planning-time policy evidence. Pulse checks current authority again before execution.")).toBeVisible();
  await testInfo.attach("canonical-action-decision-packet", { body: await page.screenshot(), contentType: "image/png" });
  await page.getByRole("button", { name: "Close action review" }).click();
  await page.getByRole("tab", { name: "History" }).click();
  await expect(page.getByTestId("actions-calm-state")).toContainText("No action history yet");
});

test("Actions inbox gives a recoverable error without presenting stale authority", async ({ page }, testInfo) => {
  await page.route("**/api/actions?*", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: "action store unavailable" }) }));
  await page.goto("/actions", { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("alert")).toContainText("Actions could not be loaded");
  await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
  await expect(page.getByTestId("actions-calm-state")).toHaveCount(0);
  await testInfo.attach("actions-recoverable-error", { body: await page.screenshot(), contentType: "image/png" });
});
