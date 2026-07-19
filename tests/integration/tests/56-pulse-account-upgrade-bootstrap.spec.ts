import { spawn, type ChildProcess } from "node:child_process";
import { createServer } from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { expect, test } from "@playwright/test";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const PORTAL_FRONTEND_DIR = path.resolve(
  __dirname,
  "../../../internal/cloudcp/portal/frontend",
);

async function reserveLoopbackPort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = createServer();
    server.unref();
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (!address || typeof address === "string") {
        server.close();
        reject(new Error("Could not reserve a loopback port"));
        return;
      }
      server.close((error) => {
        if (error) {
          reject(error);
          return;
        }
        resolve(address.port);
      });
    });
  });
}

async function waitForPortalPreview(
  url: string,
  timeoutMs: number,
  process: ChildProcess,
  readOutput: () => string,
) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (process.exitCode !== null || process.signalCode !== null) {
      throw new Error(
        `Pulse Account preview exited before becoming ready at ${url}:\n${readOutput()}`,
      );
    }
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
  throw new Error(
    `Timed out waiting for Pulse Account preview at ${url}:\n${readOutput()}`,
  );
}

async function stopPortalPreview(process: ChildProcess | null) {
  if (
    !process ||
    process.killed ||
    process.exitCode !== null ||
    process.signalCode !== null
  ) {
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
  let previewProcess: ChildProcess | null = null;
  let portalPreviewURL = "";

  test.beforeAll(async ({}, testInfo) => {
    if (testInfo.project.name.startsWith("mobile-")) {
      return;
    }

    const portalPreviewPort = String(await reserveLoopbackPort());
    portalPreviewURL = `http://127.0.0.1:${portalPreviewPort}`;
    let previewOutput = "";
    const appendOutput = (chunk: Buffer) => {
      previewOutput = `${previewOutput}${chunk.toString("utf8")}`.slice(-8000);
    };
    const processHandle = spawn(process.execPath, ["dev.mjs"], {
      cwd: PORTAL_FRONTEND_DIR,
      env: {
        ...process.env,
        PULSE_PORTAL_PREVIEW_PORT: portalPreviewPort,
      },
      stdio: ["ignore", "pipe", "pipe"],
    });
    previewProcess = processHandle;
    processHandle.stdout?.on("data", appendOutput);
    processHandle.stderr?.on("data", appendOutput);
    await waitForPortalPreview(
      `${portalPreviewURL}/healthz`,
      60_000,
      processHandle,
      () => previewOutput,
    );
  });

  test.afterAll(async () => {
    await stopPortalPreview(previewProcess);
  });

  test("accepts the canonical portal handoff bootstrap", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    await page.goto(
      `${portalPreviewURL}/?scenario=managed&portal_handoff_id=cph_preview_upgrade`,
      { waitUntil: "domcontentloaded" },
    );

    await expect(
      page.getByText(
        "Pulse Account will return completed checkout directly to the Plans page in Pulse.",
      ),
    ).toBeVisible();
    await expect(
      page.getByText(
        "Community keeps core monitoring free. Relay gets your Pulse web UI securely reachable from anywhere. Pro adds Patrol control, alert investigation, verified fixes, and 90-day history.",
      ),
    ).toBeVisible();
    await expect(
      page.getByText(
        "Pulse Account keeps checkout tied to the Pulse instance that opened it, so completed Relay or Pro purchases return to the right Plans page automatically.",
      ),
    ).toBeVisible();
    await expect(
      page.getByText(
        "Open this upgrade from the Plans page in Pulse so Pulse Account can verify the secure plan upgrade handoff before checkout.",
      ),
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Buy Annual" }).first(),
    ).toBeEnabled();

    const checkoutRequestPromise = page.waitForRequest((request) => {
      return (
        request.method() === "POST" &&
        request
          .url()
          .includes("/__portal_preview/commercial/v1/checkout/session")
      );
    });
    const buyAnnual = page.getByRole("button", { name: "Buy Annual" }).first();
    await buyAnnual.click();

    const checkoutRequest = await checkoutRequestPromise;
    const checkoutPayload = checkoutRequest.postDataJSON() as Record<
      string,
      string
    >;
    expect(checkoutPayload.portal_handoff_id).toBe("cph_preview_upgrade");
    expect("checkout_intent_id" in checkoutPayload).toBe(false);
    await expect(page).toHaveURL(/preview_toast=/);
  });

  test("blocks the retired checkout-intent bootstrap path", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    await page.goto(
      `${portalPreviewURL}/?scenario=managed&service=upgrade&checkout_intent_id=cki_legacy`,
      { waitUntil: "domcontentloaded" },
    );

    await expect(
      page.getByText(
        "Open this upgrade from the Plans page in Pulse so Pulse Account can verify the secure plan upgrade handoff before checkout.",
      ),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Buy Annual" }).first(),
    ).toBeDisabled();
  });

  test("blocks a completed secure portal handoff from reopening checkout", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only billing continuity",
    );

    await page.goto(
      `${portalPreviewURL}/?scenario=managed&portal_handoff_id=cph_preview_completed`,
      { waitUntil: "domcontentloaded" },
    );

    await expect(
      page.getByText(
        "This secure upgrade handoff already completed. Return to the Plans page in Pulse to review the live plan state.",
      ),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Buy Annual" }).first(),
    ).toBeDisabled();
  });
});
