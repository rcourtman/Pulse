import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test } from "@playwright/test";
import { ensureAuthenticated } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type LabArtifact = {
  run_id: string;
  proposal: { proposal_id: string; finding_id: string; investigation_id: string; resource_id: string; capability_name: string; params: Record<string, unknown>; evidence_ids: string[] };
  investigation: { id: string; finding_id: string };
  action: {
    id: string; createdAt: string; updatedAt: string; state: string; decisionRevision: number;
    request: { requestId: string; resourceId: string; capabilityName: string; params?: Record<string, unknown> | null; reason: string; requestedBy: string };
    plan: { actionId: string; requestId: string; rollbackAvailable: boolean; [key: string]: unknown };
    origin: { surface: string; findingId: string; investigationId: string; proposalId: string };
    result: { success: boolean; actionResultV2: { version: number; execution: { status: string; summary?: string }; verification: { status: string; evidenceClass: string; summary?: string; evidence: Array<{ id: string; observerId: string; observerKind: string; observerTrustDomain: string; executorTrustDomain: string; method: string; subjectId: string; observedAt: string; receivedAt: string; digest: string }> }; compensation: { support: string; status: string; summary?: string } } };
    verificationOutcome: { status: string };
  };
  attempt: { id: string; actionId: string; state: string; createdAt: string; updatedAt: string; dispatchCount: number; operationKind: string; operationVersion: number; requestDigest: string; agentId: string };
  receipt: { attemptId: string; actionId: string; transportRequestId: string; receivedAt: string };
  finding: { id: string; outcome: string; resolved_at: string };
  notification: { kind: string; outcome: string; action_type: string; action_id: string; category: string; severity: string };
  before: { container_id: string; state: string; running: boolean; started_at: string; observed_at: string };
  after: { container_id: string; state: string; running: boolean; started_at: string; observed_at: string };
  evidence: { id: string; observerTrustDomain: string; executorTrustDomain: string; digest: string };
};

const loadArtifact = (): { artifact: LabArtifact; artifactDir: string } => {
  const configured = String(process.env.PULSE_INTELLIGENCE_LAB_ARTIFACT || "").trim();
  if (!configured) throw new Error("PULSE_INTELLIGENCE_LAB_ARTIFACT is required");
  const artifactPath = path.resolve(configured);
  const repoRoot = path.resolve(__dirname, "..", "..", "..");
  const allowedRoot = path.join(repoRoot, "tmp", "intelligence-lab") + path.sep;
  if (!artifactPath.startsWith(allowedRoot) || path.basename(artifactPath) !== "canonical-journey.json") throw new Error("artifact path must select an ignored canonical-journey.json");
  const artifact = JSON.parse(fs.readFileSync(artifactPath, "utf8")) as LabArtifact;
  return { artifact, artifactDir: path.dirname(artifactPath) };
};

const assertExactArtifactBindings = (artifact: LabArtifact): void => {
  const { proposal, investigation, action, attempt, receipt, finding, evidence } = artifact;
  if (Object.keys(proposal.params).length !== 0 || Object.keys(action.request.params ?? {}).length !== 0) throw new Error("real proposal and action must carry zero parameter authority");
  if (proposal.proposal_id !== action.origin.proposalId || proposal.finding_id !== finding.id || proposal.finding_id !== investigation.finding_id || proposal.investigation_id !== investigation.id || proposal.investigation_id !== action.origin.investigationId || proposal.finding_id !== action.origin.findingId) throw new Error("proposal/finding/investigation correlation mismatch");
  if (proposal.resource_id !== action.request.resourceId || proposal.capability_name !== action.request.capabilityName) throw new Error("proposal/action target mismatch");
  if (action.id !== action.plan.actionId || action.id !== attempt.actionId || action.id !== receipt.actionId || attempt.id !== receipt.attemptId || attempt.id !== receipt.transportRequestId) throw new Error("action/attempt/receipt identity mismatch");
  if (!attempt.requestDigest.startsWith("sha256:") || attempt.dispatchCount !== 1 || attempt.state !== "receipt_recorded") throw new Error("durable attempt binding is incomplete or duplicated");
  const canonicalEvidence = action.result.actionResultV2.verification.evidence;
  if (canonicalEvidence.length !== 1 || canonicalEvidence[0].id !== evidence.id || canonicalEvidence[0].digest !== evidence.digest || canonicalEvidence[0].observerTrustDomain === canonicalEvidence[0].executorTrustDomain) throw new Error("independent evidence identity mismatch");
  if (finding.outcome !== "fix_verified" || !finding.resolved_at) throw new Error("finding did not retain a resolved verified outcome");
  if (artifact.notification.kind !== "fix_completed" || artifact.notification.outcome !== "confirmed") throw new Error("notification outcome does not match canonical verification");
  if (artifact.before.container_id !== artifact.after.container_id || new Date(artifact.after.started_at) <= new Date(artifact.before.started_at)) throw new Error("daemon before/after restart facts are not monotonic");
};

test("real Colima restart artifact renders one independently verified durable action", async ({ page }) => {
  const { artifact, artifactDir } = loadArtifact();
  assertExactArtifactBindings(artifact);
  await page.setViewportSize({ width: 1440, height: 1600 });
  await ensureAuthenticated(page);
  await page.route("**/api/actions?*", async (route) => {
    const view = new URL(route.request().url()).searchParams.get("view");
    const actions = view === "settled" ? [artifact.action] : [];
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ view, actions, count: actions.length }) });
  });
  await page.route(`**/api/actions/${encodeURIComponent(artifact.action.id)}`, (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ audit: artifact.action, events: [], attempt: artifact.attempt, receipt: artifact.receipt }),
  }));

  await page.goto("/actions", { waitUntil: "domcontentloaded" });
  await page.getByRole("tab", { name: "History" }).click();
  const actionButton = page.getByRole("button", { name: new RegExp(`Restart.*${artifact.action.request.resourceId.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}`) });
  await expect(actionButton).toHaveCount(1);
  await actionButton.click();
  const dialog = page.getByRole("dialog", { name: "Restart" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByTestId("action-execution-truth")).toContainText("Succeeded");
  await expect(dialog.getByTestId("action-verification-truth")).toContainText("Confirmed by independent observer");
  await expect(dialog.getByTestId("action-verification-truth")).toContainText("Source: Independent observer");
  await expect(dialog.getByTestId("action-verification-truth")).toContainText("Direct daemon observation");
  await expect(dialog.getByTestId("action-delivery-truth")).toHaveCount(1);
  await expect(dialog.getByText("One agent receipt is recorded for this action.")).toBeVisible();
  await expect(dialog.getByTestId("action-compensation-truth")).toContainText("Not Available");
  await expect(dialog.getByTestId("action-compensation-truth")).toContainText("Support: Unavailable");
  await expect(dialog.getByTestId("action-compensation-truth")).toContainText("Container restart is non-rollbackable");
  await expect(dialog.getByText("Operator-selected parameters", { exact: true })).toHaveCount(0);
  await expect(dialog.getByText("Legacy check passed (source unclassified)")).toHaveCount(0);
  await expect(dialog.getByTestId("action-verification-truth")).toHaveCount(1);
  await dialog.getByTestId("action-execution-truth").scrollIntoViewIfNeeded();
  await page.screenshot({ path: path.join(artifactDir, "actions-history-current-build.png"), fullPage: true });
});
