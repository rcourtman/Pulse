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
    const storageStatePath = path.resolve(__dirname, "..", "..", "tmp", "playwright-auth", `autopilot-ack-${workerInfo.project.name}.json`);
    fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
    await createAuthenticatedStorageState(browser, storageStatePath);
    try { await use(storageStatePath); } finally { fs.rmSync(storageStatePath, { force: true }); }
  }, { scope: "worker" }],
});

const acceptedLimits = { policyAllowlistRequired: true, emergencyStopHonored: true, approvalFloorsHonored: true, verificationReconciledWhenSupported: true, evidenceClassDisclosed: true, inconclusiveOutcomeAllowed: true, executionSuccessIsNotOutcomeTruth: true };
const requiredStatus = { code: "acknowledgement_required", active: false, currentVersion: 1, acceptedScope: ["policy_authorized_actions", "capability_allowlisted_only", "outcome_truth_not_inferred"], acceptedLimits };

test("Autopilot records a versioned server acknowledgement before effective full mode and supports revocation", async ({ page }, testInfo) => {
  let active = false;
  await page.route("**/api/license/runtime-capabilities", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ capabilities: ["ai_patrol", "ai_autofix"], limits: [], hosted_mode: false, max_history_days: 365, runtime: { build: "enterprise", label: "Pulse Pro runtime" }, blocked_capabilities: [] }) }));
  await page.route("**/api/ai/patrol/autonomy/acknowledgements", async (route) => {
    const body = route.request().postDataJSON();
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ created: true, acknowledgement: { ...requiredStatus, code: "active", active: true, acknowledgementVersion: 1, acknowledgementId: body.acknowledgement_id, acceptedAt: "2026-07-12T00:00:00Z" } }) });
  });
  await page.route("**/api/ai/patrol/autonomy/acknowledgements/*", async (route) => {
    active = false;
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ revoked: true, created: true, acknowledgement_id: "ack-1" }) });
  });
  await page.route("**/api/ai/patrol/autonomy", async (route) => {
    if (route.request().method() === "PUT") { active = route.request().postDataJSON().autonomy_level === "full"; }
    const status = active ? { ...requiredStatus, code: "active", active: true, acknowledgementVersion: 1, acknowledgementId: "ack-1", acceptedAt: "2026-07-12T00:00:00Z" } : requiredStatus;
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, settings: { autonomy_level: active ? "full" : "monitor", requested_autonomy_level: active ? "full" : "monitor", effective_autonomy_level: active ? "full" : "monitor", full_mode_unlocked: active, autopilot_acknowledgement: status, investigation_budget: 15, investigation_timeout_sec: 300 }, autonomy_level: active ? "full" : "monitor", requested_autonomy_level: active ? "full" : "monitor", effective_autonomy_level: active ? "full" : "monitor", full_mode_unlocked: active, autopilot_acknowledgement: status, investigation_budget: 15, investigation_timeout_sec: 300 }) });
  });
  await page.goto("/patrol", { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "Autopilot" }).click();
  await expect(page.getByRole("dialog", { name: "Activate Autopilot" })).toBeVisible();
  const activate = page.getByRole("button", { name: "Record acknowledgement and activate" });
  await expect(activate).toBeDisabled();
  await testInfo.attach("autopilot-versioned-acknowledgement", { body: await page.screenshot(), contentType: "image/png" });
  await page.getByRole("checkbox").check();
  const acknowledgementRequest = page.waitForRequest((request) => request.method() === "POST" && request.url().endsWith("/api/ai/patrol/autonomy/acknowledgements"));
  const activationRequest = page.waitForRequest((request) => request.method() === "PUT" && request.url().endsWith("/api/ai/patrol/autonomy"));
  await activate.click();
  const [acknowledgement, activation] = await Promise.all([acknowledgementRequest, activationRequest]);
  expect(acknowledgement.postDataJSON().acknowledgement_id).toBeTruthy();
  expect(activation.postDataJSON()).toMatchObject({ autonomy_level: "full", acknowledgement_id: expect.any(String) });
  await expect(page.getByText("Autopilot acknowledgement v1")).toBeVisible();
  await testInfo.attach("autopilot-effective-full-mode", { body: await page.screenshot(), contentType: "image/png" });
  const revocationRequest = page.waitForRequest((request) => request.method() === "DELETE" && request.url().includes("/api/ai/patrol/autonomy/acknowledgements/"));
  await page.getByRole("button", { name: "Revoke Autopilot" }).click();
  await revocationRequest;
});
