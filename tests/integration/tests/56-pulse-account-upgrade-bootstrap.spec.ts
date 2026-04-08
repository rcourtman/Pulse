import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";

import { expect, test } from "@playwright/test";

const PORTAL_PREVIEW_PORT = "8876";
const PORTAL_PREVIEW_URL = `http://127.0.0.1:${PORTAL_PREVIEW_PORT}`;
const PORTAL_FRONTEND_DIR =
  "/Volumes/Development/pulse/repos/pulse/internal/cloudcp/portal/frontend";

async function waitForPortalPreview(url: string, timeoutMs: number) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url);
      if (response.ok) {
        return;
      }
    } catch {
      // Keep polling until the preview is ready.
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  throw new Error(`Timed out waiting for Pulse Account preview at ${url}`);
}

async function stopPortalPreview(process: ChildProcessWithoutNullStreams | null) {
  if (!process || process.killed) {
    return;
  }
  await new Promise<void>((resolve) => {
    const timer = setTimeout(() => {
      process.kill("SIGKILL");
    }, 2000);
    process.once("exit", () => {
      clearTimeout(timer);
      resolve();
    });
    process.kill("SIGTERM");
  });
}

test.describe.configure({ mode: "serial" });

test.describe("Pulse Account upgrade bootstrap", () => {
  let previewProcess: ChildProcessWithoutNullStreams | null = null;

  test.beforeAll(async () => {
    previewProcess = spawn(process.execPath, ["dev.mjs"], {
      cwd: PORTAL_FRONTEND_DIR,
      env: {
        ...process.env,
        PULSE_PORTAL_PREVIEW_PORT: PORTAL_PREVIEW_PORT,
      },
      stdio: ["ignore", "pipe", "pipe"],
    });
    previewProcess.stdout.on("data", () => {});
    previewProcess.stderr.on("data", () => {});
    await waitForPortalPreview(`${PORTAL_PREVIEW_URL}/healthz`, 30000);
  });

  test.afterAll(async () => {
    await stopPortalPreview(previewProcess);
  });

  test("accepts the canonical portal handoff bootstrap", async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    await page.goto(
      `${PORTAL_PREVIEW_URL}/?scenario=managed&portal_handoff_id=cph_preview_upgrade`,
      { waitUntil: "domcontentloaded" },
    );

    await expect(
      page.getByText(
        "Pulse Account will return completed checkout directly to Pulse Pro billing.",
      ),
    ).toBeVisible();
    await expect(
      page.getByText(
        "Open this upgrade from Pulse Pro billing so Pulse Account can verify the secure upgrade handoff before checkout.",
      ),
    ).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Buy Annual" }).first()).toBeEnabled();

    const checkoutRequestPromise = page.waitForRequest((request) => {
      return (
        request.method() === "POST" &&
        request.url().includes("/__portal_preview/commercial/v1/checkout/session")
      );
    });
    const buyAnnual = page.getByRole("button", { name: "Buy Annual" }).first();
    await buyAnnual.click();

    const checkoutRequest = await checkoutRequestPromise;
    const checkoutPayload = checkoutRequest.postDataJSON() as Record<string, string>;
    expect(checkoutPayload.portal_handoff_id).toBe("cph_preview_upgrade");
    expect("checkout_intent_id" in checkoutPayload).toBe(false);
    await expect(page).toHaveURL(/preview_toast=/);
  });

  test("blocks the retired checkout-intent bootstrap path", async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    await page.goto(
      `${PORTAL_PREVIEW_URL}/?scenario=managed&service=upgrade&checkout_intent_id=cki_legacy`,
      { waitUntil: "domcontentloaded" },
    );

    await expect(
      page.getByText(
        "Open this upgrade from Pulse Pro billing so Pulse Account can verify the secure upgrade handoff before checkout.",
      ),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Buy Annual" }).first()).toBeDisabled();
  });

  test("blocks a completed secure portal handoff from reopening checkout", async ({ page }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    await page.goto(
      `${PORTAL_PREVIEW_URL}/?scenario=managed&portal_handoff_id=cph_preview_completed`,
      { waitUntil: "domcontentloaded" },
    );

    await expect(
      page.getByText(
        "This secure upgrade handoff already completed. Return to Pulse Pro billing to review the live plan state.",
      ),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Buy Annual" }).first()).toBeDisabled();
  });
});
