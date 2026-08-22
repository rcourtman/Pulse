/**
 * Fail-closed API and rendered-UI readiness diagnostic for release rehearsals.
 */

import fs from "node:fs/promises";
import path from "node:path";

import { test, expect, type Locator, type Page } from "@playwright/test";
import { waitForAppShell, waitForPulseReady } from "./helpers";

type HealthPayload = {
  status?: unknown;
  dependencies?: {
    monitor?: unknown;
    scheduler?: unknown;
    websocket?: unknown;
  };
};

type SecurityStatusPayload = {
  hasAuthentication?: unknown;
  hideLocalLogin?: unknown;
  ssoProviders?: Array<{
    name?: unknown;
    displayName?: unknown;
  }>;
};

const truthy = (value: string | undefined) => {
  if (!value) return false;
  return ["1", "true", "yes", "on"].includes(value.trim().toLowerCase());
};

const compact = (value: string, limit = 1000): string =>
  value.replace(/\s+/g, " ").trim().slice(0, limit);

const errorMessage = (error: unknown): string =>
  error instanceof Error ? error.message : String(error);

const effectiveRenderedOpacity = (locator: Locator): Promise<number> =>
  locator.evaluate((element) => {
    let opacity = 1;
    for (
      let current: Element | null = element;
      current;
      current = current.parentElement
    ) {
      const style = window.getComputedStyle(current);
      if (style.display === "none" || style.visibility !== "visible") {
        return 0;
      }
      opacity *= Number.parseFloat(style.opacity || "1");
    }
    return opacity;
  });

const gotoWithHealthRetry = async (
  page: Page,
  url: string,
  attempts = 3,
): Promise<void> => {
  let lastError: unknown = null;
  for (let attempt = 1; attempt <= attempts; attempt++) {
    try {
      await waitForPulseReady(page, 30_000);
      await page.goto(url, { waitUntil: "domcontentloaded" });
      return;
    } catch (error) {
      lastError = error;
      if (attempt === attempts) {
        break;
      }
      console.log(
        `Navigation attempt ${attempt}/${attempts} failed: ${errorMessage(error)}; retrying...`,
      );
      await page.waitForTimeout(1_000);
    }
  }
  throw new Error(
    `Rendered UI readiness failed: could not navigate to ${url} after ${attempts} attempts: ${errorMessage(lastError)}`,
  );
};

test.describe("Release readiness diagnostic", () => {
  // A readiness failure must remain red. CI's general retry policy is useful
  // for the wider suite but would let a flaky release preflight pass.
  test.describe.configure({ retries: 0 });

  test.skip(
    !truthy(process.env.PULSE_E2E_DIAGNOSTIC),
    "Set PULSE_E2E_DIAGNOSTIC=1 to enable release readiness diagnostics",
  );

  test("API and browser shell are genuinely ready", async ({
    page,
  }, testInfo) => {
    test.setTimeout(120_000);

    const consoleErrors: string[] = [];
    const pageErrors: string[] = [];
    const requestFailures: string[] = [];

    page.on("console", (message) => {
      const text = `${message.type().toUpperCase()}: ${message.text()}`;
      console.log(`BROWSER ${text}`);
      if (message.type() === "error") {
        consoleErrors.push(text);
      }
    });
    page.on("pageerror", (error) => {
      const text = errorMessage(error);
      pageErrors.push(text);
      console.log(`PAGE ERROR: ${text}`);
    });
    page.on("requestfailed", (request) => {
      const text = `${request.method()} ${request.url()}: ${request.failure()?.errorText ?? "unknown failure"}`;
      requestFailures.push(text);
      console.log(`REQUEST FAILED: ${text}`);
    });

    let renderedSurface = "unknown";
    let healthBody = "";
    let securityBody = "";
    let readinessFailed = false;
    try {
      await waitForPulseReady(page, 30_000).catch((error) => {
        throw new Error(
          `API readiness failed: GET /api/health did not become reachable within 30s: ${errorMessage(error)}`,
        );
      });

      const healthResponse = await page.request
        .get("/api/health")
        .catch((error) => {
          throw new Error(
            `API readiness failed: GET /api/health could not complete: ${errorMessage(error)}`,
          );
        });
      healthBody = await healthResponse.text();
      expect(
        healthResponse.status(),
        `API readiness failed: GET /api/health returned HTTP ${healthResponse.status()}; body=${compact(healthBody)}`,
      ).toBe(200);

      let healthPayload: HealthPayload;
      try {
        healthPayload = JSON.parse(healthBody) as HealthPayload;
      } catch (error) {
        throw new Error(
          `API readiness failed: GET /api/health returned invalid JSON: ${errorMessage(error)}; body=${compact(healthBody)}`,
        );
      }
      const dependencies = healthPayload.dependencies ?? {};
      expect(
        healthPayload.status,
        `API readiness failed: health status was ${JSON.stringify(healthPayload.status)}; body=${compact(healthBody)}`,
      ).toBe("healthy");
      expect(
        dependencies.monitor,
        `API readiness failed: dependencies.monitor was not ready; body=${compact(healthBody)}`,
      ).toBe(true);
      expect(
        dependencies.scheduler,
        `API readiness failed: dependencies.scheduler was not ready; body=${compact(healthBody)}`,
      ).toBe(true);
      expect(
        dependencies.websocket,
        `API readiness failed: dependencies.websocket was not ready; body=${compact(healthBody)}`,
      ).toBe(true);
      console.log(`API readiness passed: ${compact(healthBody)}`);

      await gotoWithHealthRetry(page, "/");
      await waitForAppShell(page, 20_000).catch((error) => {
        throw new Error(
          `Rendered UI readiness failed: Pulse app shell did not render within 20s: ${errorMessage(error)}`,
        );
      });

      const root = page.locator("#root");
      await expect(
        root,
        `Rendered UI readiness failed: #root was not visible at ${page.url()}`,
      ).toBeVisible();
      await expect
        .poll(() => root.evaluate((element) => element.children.length), {
          message: `Rendered UI readiness failed: #root stayed empty at ${page.url()}`,
          timeout: 20_000,
        })
        .toBeGreaterThan(0);

      const securityProbe = await page.evaluate(async () => {
        try {
          const response = await fetch("/api/security/status", {
            cache: "no-store",
            credentials: "same-origin",
          });
          return {
            body: await response.text(),
            ok: response.ok,
            status: response.status,
          };
        } catch (error) {
          return {
            body: "",
            error: error instanceof Error ? error.message : String(error),
            ok: false,
            status: 0,
          };
        }
      });
      securityBody = securityProbe.body;
      expect(
        securityProbe.ok,
        `API readiness failed: browser GET /api/security/status returned HTTP ${securityProbe.status}` +
          `${securityProbe.error ? ` error=${securityProbe.error}` : ""}; body=${compact(securityBody)}`,
      ).toBe(true);

      let securityPayload: SecurityStatusPayload;
      try {
        securityPayload = JSON.parse(securityBody) as SecurityStatusPayload;
      } catch (error) {
        throw new Error(
          `API readiness failed: browser GET /api/security/status returned invalid JSON: ${errorMessage(error)}; body=${compact(securityBody)}`,
        );
      }
      expect(
        typeof securityPayload.hasAuthentication,
        `API readiness failed: /api/security/status hasAuthentication was not boolean; body=${compact(securityBody)}`,
      ).toBe("boolean");

      const welcomeHeading = page.getByRole("heading", {
        name: "Welcome to Pulse",
      });
      await expect(
        welcomeHeading,
        `Rendered UI readiness failed: the Pulse welcome heading was not visible at ${page.url()}`,
      ).toBeVisible();

      let renderedSurfaceControl: Locator;
      if (securityPayload.hasAuthentication === false) {
        renderedSurface = "first-run setup wizard";
        await expect(
          page.getByRole("main", { name: "Pulse Setup Wizard" }),
          `Rendered UI readiness failed: security status requires first-run setup, but the setup wizard was not visible; body=${compact(securityBody)}`,
        ).toBeVisible();
        await expect(
          page.getByPlaceholder("Paste your bootstrap token"),
          `Rendered UI readiness failed: the setup wizard did not render its bootstrap-token control; body=${compact(securityBody)}`,
        ).toBeVisible();
        renderedSurfaceControl = page.getByPlaceholder(
          "Paste your bootstrap token",
        );
      } else if (securityPayload.hideLocalLogin !== true) {
        renderedSurface = "local login";
        await expect(
          page.getByLabel("Username"),
          `Rendered UI readiness failed: authentication is configured, but the username control was not visible; body=${compact(securityBody)}`,
        ).toBeVisible();
        await expect(page.getByLabel("Password")).toBeVisible();
        await expect(
          page.getByRole("button", { name: "Sign in to Pulse" }),
        ).toBeVisible();
        renderedSurfaceControl = page.getByRole("button", {
          name: "Sign in to Pulse",
        });
      } else {
        renderedSurface = "SSO login";
        const providers = (securityPayload.ssoProviders ?? [])
          .map((provider) =>
            String(provider.displayName || provider.name || "").trim(),
          )
          .filter(Boolean);
        expect(
          providers.length,
          `Rendered UI readiness failed: local login is hidden but /api/security/status advertised no SSO provider; body=${compact(securityBody)}`,
        ).toBeGreaterThan(0);
        await expect(
          page.getByRole("button", { name: `Continue with ${providers[0]}` }),
          `Rendered UI readiness failed: SSO provider ${JSON.stringify(providers[0])} was not rendered; body=${compact(securityBody)}`,
        ).toBeVisible();
        renderedSurfaceControl = page.getByRole("button", {
          name: `Continue with ${providers[0]}`,
        });
      }

      await expect
        .poll(() => effectiveRenderedOpacity(renderedSurfaceControl), {
          message: `Rendered UI readiness failed: ${renderedSurface} control or an ancestor stayed transparent`,
          timeout: 10_000,
        })
        .toBeGreaterThan(0.99);
      const renderedControlBounds = await renderedSurfaceControl.boundingBox();
      expect(
        (renderedControlBounds?.width ?? 0) *
          (renderedControlBounds?.height ?? 0),
        `Rendered UI readiness failed: ${renderedSurface} control had no rendered area`,
      ).toBeGreaterThan(0);

      expect(
        pageErrors,
        `Rendered UI readiness failed: uncaught page errors were observed: ${JSON.stringify(pageErrors)}`,
      ).toEqual([]);
      console.log(
        `Rendered UI readiness passed: surface=${renderedSurface}; url=${page.url()}; consoleErrors=${consoleErrors.length}; requestFailures=${requestFailures.length}`,
      );
    } catch (error) {
      readinessFailed = true;
      const appState = await page
        .evaluate(() => {
          const root = document.getElementById("root");
          return {
            bodyText: document.body?.innerText?.slice(0, 800) ?? "",
            rootChildren: root?.children.length ?? 0,
            rootText: root?.textContent?.slice(0, 800) ?? "",
            url: window.location.href,
          };
        })
        .catch((stateError) => ({
          stateError: errorMessage(stateError),
          url: page.url(),
        }));
      throw new Error(
        `${errorMessage(error)}; appState=${JSON.stringify(appState)}; healthBody=${compact(healthBody)}; ` +
          `securityBody=${compact(securityBody)}; pageErrors=${JSON.stringify(pageErrors)}; ` +
          `consoleErrors=${JSON.stringify(consoleErrors.slice(-10))}; requestFailures=${JSON.stringify(requestFailures.slice(-10))}`,
      );
    } finally {
      try {
        const evidenceDirectory = path.resolve("diagnostic-evidence");
        await fs.mkdir(evidenceDirectory, { recursive: true });
        const evidencePath = path.join(
          evidenceDirectory,
          "release-dry-run-rendered-readiness-chromium.png",
        );
        await page.screenshot({ fullPage: true, path: evidencePath });
        await testInfo.attach("release-dry-run-rendered-readiness-chromium", {
          path: evidencePath,
          contentType: "image/png",
        });
      } catch (attachmentError) {
        const evidenceError = `DIAGNOSTIC EVIDENCE ERROR: could not retain rendered UI screenshot: ${errorMessage(attachmentError)}`;
        console.error(evidenceError);
        if (!readinessFailed) {
          throw new Error(evidenceError);
        }
      }
    }
  });
});
