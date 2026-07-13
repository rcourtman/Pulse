import fs from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import { expect, test as base, type Page } from "@playwright/test";
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

type CanonicalResultFixture = {
  execution: { status: string; reasonCode?: string };
  verification: { status: string; evidenceClass: string; reasonCode?: string; evidence?: unknown[] };
  compensation: { support: string; status: string };
};

const assertCanonicalResultFixture = (truth: CanonicalResultFixture): void => {
  if (truth.execution.status !== "succeeded" && !truth.execution.reasonCode) throw new Error("non-success execution fixture requires reasonCode");
  if (truth.verification.status === "inconclusive" && !truth.verification.reasonCode) throw new Error("inconclusive verification fixture requires reasonCode");
  const evidenceCount = truth.verification.evidence?.length ?? 0;
  const conclusive = truth.verification.status === "confirmed" || truth.verification.status === "contradicted";
  if (conclusive && (truth.verification.evidenceClass === "none" || evidenceCount === 0)) throw new Error("conclusive verification fixture requires sourced evidence");
  if (truth.verification.evidenceClass === "none" && evidenceCount !== 0) throw new Error("no-source verification fixture cannot carry evidence");
  if (truth.verification.evidenceClass !== "none" && evidenceCount === 0) throw new Error("declared evidence source fixture requires evidence");
  if (truth.verification.evidenceClass === "independent") throw new Error("APT browser fixtures cannot claim tier-6 independent evidence");
  if (truth.verification.status === "not_attempted" && (truth.verification.evidenceClass !== "none" || evidenceCount !== 0)) throw new Error("not-attempted verification fixture cannot carry evidence");
  if (truth.compensation.support !== "unavailable" || truth.compensation.status !== "not_available") throw new Error("APT fixture cannot claim rollback or compensation");
};

const aptAction = ({ id, capabilityName, state = "completed", summary, execution = "succeeded", verification = "confirmed", evidenceClass, verificationReasonCode, elevated = true, params = {} }: {
  id: string;
  capabilityName: "install_os_updates" | "clean_package_cache";
  state?: string;
  summary?: string;
  execution?: string;
  verification?: string;
  evidenceClass?: string;
  verificationReasonCode?: string;
  elevated?: boolean;
  params?: Record<string, unknown>;
}) => {
  const aptScope = { orgId: "org-1", resourceId: "proxmox:node:pve-1", capabilityName };
  const requiresApproval = state === "pending_approval";
  const executionReasonCode = execution === "inconclusive" ? "possible_partial_effect" : execution === "failed" ? "execution_failed" : execution === "not_run" ? "preflight_refused" : undefined;
  const resolvedEvidenceClass = evidenceClass ?? (verification === "confirmed" || verification === "contradicted" ? "agent_attested" : "none");
  const resolvedVerificationReason = verificationReasonCode ?? (verification === "inconclusive" ? "agent_readback_inconclusive" : undefined);
  const evidenceEnvelope = { version: 1, id: `${id}-evidence`, observerId: "agent:pve-1", observerKind: "unified_agent", observerTrustDomain: "agent:pve-1", executorTrustDomain: "agent:pve-1", method: "typed_read_after_write", subjectId: aptScope.resourceId, observedAt: "2026-07-12T10:01:00Z", receivedAt: "2026-07-12T10:05:00Z", digest: "" };
  const evidence = resolvedEvidenceClass === "none" ? undefined : [{ ...evidenceEnvelope, digest: `sha256:${createHash("sha256").update(JSON.stringify(evidenceEnvelope)).digest("hex")}` }];
  const actionResultV2 = summary ? {
    version: 2 as const,
    execution: { status: execution, ...(executionReasonCode ? { reasonCode: executionReasonCode } : {}), summary },
    verification: { status: verification, evidenceClass: resolvedEvidenceClass, ...(resolvedVerificationReason ? { reasonCode: resolvedVerificationReason } : {}), summary: verification === "confirmed" ? "The executing agent observed the canonical postcondition." : "The canonical postcondition was not confirmed.", ...(evidence ? { evidence } : {}) },
    compensation: { support: "unavailable", status: "not_available", summary: "No rollback is available for this typed workflow." },
  } : undefined;
  if (actionResultV2) assertCanonicalResultFixture(actionResultV2);
  return {
    id, createdAt: "2026-07-12T10:00:00Z", updatedAt: "2026-07-12T10:05:00Z", state, decisionRevision: 0,
    request: { requestId: `${id}-request`, resourceId: aptScope.resourceId, capabilityName, params, reason: capabilityName === "install_os_updates" ? "Resolve the current operating system update finding" : "Relieve package data pressure reported by Patrol", requestedBy: "pulse_patrol" },
    plan: { actionId: id, requestId: `${id}-request`, allowed: true, requiresApproval, approvalPolicy: requiresApproval ? "admin" : "none", approvalRequirement: { ...requirement, floor: requiresApproval ? "admin" : "none" }, rollbackAvailable: false, plannedAt: "2026-07-12T10:00:00Z", expiresAt: "2099-07-12T10:10:00Z", resourceVersion: "resource:sha256:apt", policyVersion: "policy:sha256:apt", planHash: `sha256:${id}`, policyDecision: { version: 1, status: "resolved", decisionId: `policy-decision:${id}`, actionId: id, scope: aptScope, approvalRequirement: { ...requirement, floor: requiresApproval ? "admin" : "none" }, planningAllowed: true, requiresApproval, authorities: [{ kind: "capability_registry", sourceId: `capability-registry:${capabilityName}`, revision: "policy:sha256:apt", status: "consulted", scope: aptScope, approvalFloor: requiresApproval ? "admin" : "none", reasonCodes: [requiresApproval ? "capability_approval_admin" : "capability_approval_none", elevated ? "capability_auto_elevated" : "capability_auto_low_risk"] }] } },
    ...(actionResultV2 ? { result: { success: execution === "succeeded", actionResultV2 } } : {}),
    verificationOutcome: { status: verification === "confirmed" ? "verified" : verification === "contradicted" ? "failed" : verification === "inconclusive" ? "unverified" : "unknown" },
  };
};

const routeActionFixtures = async (page: Page, pending: ReturnType<typeof aptAction>[], settled: ReturnType<typeof aptAction>[]) => {
  const all = [...pending, ...settled];
  await page.route("**/api/actions?*", async (route) => {
    const view = new URL(route.request().url()).searchParams.get("view");
    const actions = view === "pending" ? pending : settled;
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ view, actions, count: actions.length }) });
  });
  await page.route("**/api/actions/*", async (route) => {
    const id = decodeURIComponent(new URL(route.request().url()).pathname.split("/").pop() || "");
    const audit = all.find((candidate) => candidate.id === id);
    await route.fulfill({ status: audit ? 200 : 404, contentType: "application/json", body: JSON.stringify(audit ? { audit, events: [], attempt: { id: `${id}-attempt`, actionId: id, state: "receipt_recorded", createdAt: audit.createdAt, updatedAt: audit.updatedAt, dispatchCount: 1 }, receipt: { attemptId: `${id}-attempt`, actionId: id, transportRequestId: `${id}-transport`, receivedAt: "2026-07-12T10:05:00Z" } } : { error: "not found" }) });
  });
};

test("Actions inbox exposes the canonical decision packet and durable calm history", async ({ page }, testInfo) => {
  await page.route("**/api/actions?*", async (route) => {
    const view = new URL(route.request().url()).searchParams.get("view");
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(view === "pending" ? { view, actions: [action], count: 1 } : { view, actions: [], count: 0 }) });
  });
  await page.route("**/api/actions/action-1", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ audit: action, events: [] }) }));
  await page.goto("/actions", { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("heading", { name: "Actions" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Open", exact: true })).toHaveAttribute("aria-selected", "true");
  const openActions = page.getByRole("list", { name: "Open actions" });
  await expect(openActions).toBeVisible();
  await expect(openActions.getByText("Approval required")).toBeVisible();
  await expect(openActions.getByText("edge", { exact: true })).toBeVisible();
  await expect(openActions.getByText("Docker container", { exact: true })).toBeVisible();
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

test("APT plans expose exact empty authority and elevated versus low-risk posture", async ({ page }, testInfo) => {
  const update = aptAction({ id: "apt-update-pending", capabilityName: "install_os_updates", state: "pending_approval", elevated: true });
  const cleanup = aptAction({ id: "apt-cleanup-planned", capabilityName: "clean_package_cache", state: "planned", elevated: false });
  await routeActionFixtures(page, [update, cleanup], []);
  await page.goto("/actions", { waitUntil: "domcontentloaded" });

  await page.getByRole("button", { name: /Install operating system updates.*proxmox:node:pve-1/ }).click();
  const updateDialog = page.getByRole("dialog", { name: "Install operating system updates" });
  await expect(updateDialog.getByText("Elevated change")).toBeVisible();
  await expect(updateDialog.getByText(/accepts no command, path, package selection, removal choice, or reboot request/)).toBeVisible();
  await expect(updateDialog.getByText(/cannot remove packages or reboot the host/)).toBeVisible();
  await expect(updateDialog.getByRole("button", { name: "Approve" })).toBeVisible();
  await expect(updateDialog.getByRole("button", { name: /reboot/i })).toHaveCount(0);
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: /Clear downloaded package data.*proxmox:node:pve-1/ }).click();
  const cleanupDialog = page.getByRole("dialog", { name: "Clear downloaded package data" });
  await expect(cleanupDialog.getByText("Low-risk automation eligible")).toBeVisible();
  await expect(cleanupDialog.getByText(/clear only downloaded package data/)).toBeVisible();
  await expect(cleanupDialog.getByRole("button", { name: "Run action" })).toBeVisible();
  await testInfo.attach("apt-empty-authority-and-policy-posture", { body: await page.screenshot(), contentType: "image/png" });
});

test("APT history keeps update execution verification recovery and delayed receipt truth separate", async ({ page }, testInfo) => {
  const confirmed = aptAction({ id: "apt-update-confirmed", capabilityName: "install_os_updates", summary: "APT package updates: phase=complete; 6 pending before, 0 pending after; package manager health: healthy; recovery required: false; reboot required: true" });
  const partial = aptAction({ id: "apt-update-partial", capabilityName: "install_os_updates", summary: "APT package updates: phase=install; 6 pending before, 3 pending after; package manager health: unhealthy; recovery required: true; reboot required: false", execution: "inconclusive", verification: "contradicted" });
  const delayed = aptAction({ id: "apt-update-delayed", capabilityName: "install_os_updates", summary: "APT package updates: phase=verify; 4 pending before, 4 pending after; package manager health: unknown; recovery required: false; reboot required: false", execution: "inconclusive", verification: "inconclusive", evidenceClass: "none", verificationReasonCode: "package_manager_health_unknown" });
  await routeActionFixtures(page, [], [confirmed, partial, delayed]);
  await page.goto("/actions", { waitUntil: "domcontentloaded" });
  await page.getByRole("tab", { name: "History" }).click();

  await page.getByRole("button", { name: /Install operating system updates.*proxmox:node:pve-1/ }).first().click();
  let dialog = page.getByRole("dialog", { name: "Install operating system updates" });
  await expect(dialog.getByText("Confirmed by executing agent")).toBeVisible();
  await expect(dialog.getByText("Source: Executing agent")).toBeVisible();
  await expect(dialog.getByText("Yes — fact only; no reboot was authorized")).toBeVisible();
  await expect(dialog.getByTestId("action-execution-truth")).toBeVisible();
  await expect(dialog.getByTestId("action-verification-truth")).toBeVisible();
  await expect(dialog.getByTestId("action-compensation-truth")).toBeVisible();
  await expect(dialog.getByTestId("action-delivery-truth")).toHaveCount(1);
  await expect(dialog.getByText("Agent observation")).toBeVisible();
  await expect(dialog.getByText("Receipt recorded by Pulse")).toBeVisible();
  await expect(dialog.getByText("Legacy check passed (source unclassified)")).toHaveCount(0);
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: /Install operating system updates.*proxmox:node:pve-1/ }).nth(1).click();
  dialog = page.getByRole("dialog", { name: "Install operating system updates" });
  await expect(dialog.getByTestId("action-execution-truth")).toContainText("Inconclusive");
  await expect(dialog.getByTestId("action-verification-truth")).toContainText("Outcome contradicted");
  await expect(dialog.getByText("Install updates")).toBeVisible();
  await expect(dialog.getByText("Known unhealthy")).toBeVisible();
  await expect(dialog.getByText("Do not retry. Repair the host update system")).toBeVisible();
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: /Install operating system updates.*proxmox:node:pve-1/ }).nth(2).click();
  dialog = page.getByRole("dialog", { name: "Install operating system updates" });
  await expect(dialog.getByText("Unknown", { exact: true })).toBeVisible();
  await expect(dialog.getByText("Do not retry automatically. Run a fresh host scan")).toBeVisible();
  await testInfo.attach("apt-update-truth-and-receipt-recovery", { body: await page.screenshot(), contentType: "image/png" });
});

test("APT cleanup history shows measured bytes and irreversible rescan recovery", async ({ page }, testInfo) => {
  const confirmed = aptAction({ id: "apt-cleanup-confirmed", capabilityName: "clean_package_cache", elevated: false, summary: "APT package cache: phase=complete; 104857600 bytes before, 52428800 bytes after, 52428800 bytes reclaimed; rollback available: false; rescan required: false" });
  const failed = aptAction({ id: "apt-cleanup-failed", capabilityName: "clean_package_cache", elevated: false, summary: "APT package cache: phase=clean; 104857600 bytes before, 52428800 bytes after, 52428800 bytes reclaimed; rollback available: false; rescan required: true", execution: "failed", verification: "inconclusive" });
  await routeActionFixtures(page, [], [confirmed, failed]);
  await page.goto("/actions", { waitUntil: "domcontentloaded" });
  await page.getByRole("tab", { name: "History" }).click();
  await page.getByRole("button", { name: /Clear downloaded package data.*proxmox:node:pve-1/ }).first().click();
  let dialog = page.getByRole("dialog", { name: "Clear downloaded package data" });
  await expect(dialog.getByText("100 MB")).toBeVisible();
  await expect(dialog.getByText("50.0 MB")).toHaveCount(2);
  await expect(dialog.getByText("Unavailable — cleanup is irreversible")).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: /Clear downloaded package data.*proxmox:node:pve-1/ }).nth(1).click();
  dialog = page.getByRole("dialog", { name: "Clear downloaded package data" });
  await expect(dialog.getByTestId("action-execution-truth")).toContainText("Failed");
  await expect(dialog.getByTestId("action-verification-truth")).toContainText("Outcome inconclusive");
  await expect(dialog.getByText("Fresh rescan required")).toBeVisible();
  await expect(dialog.getByText("Do not retry automatically. Run a fresh scan")).toBeVisible();
  await testInfo.attach("apt-cleanup-measurement-and-recovery", { body: await page.screenshot(), contentType: "image/png" });
});

test("APT action review remains keyboard reachable and actionable at a phone viewport", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const partial = aptAction({ id: "apt-phone-partial", capabilityName: "install_os_updates", summary: "APT package updates: phase=install; 6 pending before, 3 pending after; package manager health: unhealthy; recovery required: true; reboot required: false", execution: "inconclusive", verification: "contradicted" });
  await routeActionFixtures(page, [], [partial]);
  await page.goto("/actions", { waitUntil: "domcontentloaded" });
  await page.getByRole("tab", { name: "History" }).click();
  await page.getByRole("button", { name: /Install operating system updates/ }).focus();
  await page.keyboard.press("Enter");
  const dialog = page.getByRole("dialog", { name: "Install operating system updates" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("What to do next")).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Close action review" })).toBeVisible();
  const box = await dialog.boundingBox();
  expect(box?.x ?? -1).toBeGreaterThanOrEqual(0);
  expect((box?.x ?? 0) + (box?.width ?? 999)).toBeLessThanOrEqual(390);
  await testInfo.attach("apt-phone-action-review", { body: await page.screenshot(), contentType: "image/png" });
});
