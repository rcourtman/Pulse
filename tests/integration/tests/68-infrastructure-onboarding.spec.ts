import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base, type Page } from "@playwright/test";
import {
  createAuthenticatedStorageState,
  ensureAuthenticated,
} from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

type OverflowAudit = {
  viewportWidth: number;
  pageWidth: number;
  overflowPx: number;
  offenders: Array<{ tag: string; className: string; overflow: number }>;
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
        `infrastructure-onboarding-${workerInfo.project.name}.json`,
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

async function stubConnectionsList(page: Page): Promise<void> {
  await page.route("**/api/connections", async (route) => {
    const requestUrl = new URL(route.request().url());
    if (
      route.request().method() !== "GET" ||
      requestUrl.pathname !== "/api/connections"
    ) {
      await route.continue();
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ connections: [] }),
    });
  });
}

async function prepareOnboardingPage(page: Page): Promise<void> {
  await stubConnectionsList(page);
  await ensureAuthenticated(page);
}

async function scrollToBottom(page: Page): Promise<void> {
  const viewportHeight = await page.evaluate(() => window.innerHeight || 800);
  const step = Math.max(240, Math.floor(viewportHeight * 0.75));
  let wheelSupported = true;

  for (let i = 0; i < 20; i += 1) {
    if (wheelSupported) {
      try {
        await page.mouse.wheel(0, step);
      } catch {
        wheelSupported = false;
        await page.evaluate((deltaY) => window.scrollBy(0, deltaY), step);
      }
    } else {
      await page.evaluate((deltaY) => window.scrollBy(0, deltaY), step);
    }
    await page.waitForTimeout(60);
  }
}

async function auditHorizontalOverflow(page: Page): Promise<OverflowAudit> {
  return page.evaluate(() => {
    const viewportWidth = Math.max(
      document.documentElement.clientWidth,
      window.innerWidth || 0,
    );
    const pageWidth = Math.max(
      document.body.scrollWidth,
      document.documentElement.scrollWidth,
      document.body.offsetWidth,
      document.documentElement.offsetWidth,
    );

    const offenders = Array.from(document.querySelectorAll("body *"))
      .map((el) => {
        const rect = el.getBoundingClientRect();
        if (rect.width <= 0 || rect.height <= 0) return null;
        const style = window.getComputedStyle(el);
        if (style.position === "fixed" || style.position === "absolute")
          return null;
        const overflow = rect.right - viewportWidth;
        if (overflow <= 1) return null;
        return {
          tag: el.tagName.toLowerCase(),
          className: (el.getAttribute("class") || "").trim().slice(0, 120),
          overflow: Number(overflow.toFixed(1)),
        };
      })
      .filter(
        (
          entry,
        ): entry is { tag: string; className: string; overflow: number } =>
          Boolean(entry),
      )
      .slice(0, 8);

    return {
      viewportWidth,
      pageWidth,
      overflowPx: Number((pageWidth - viewportWidth).toFixed(1)),
      offenders,
    };
  });
}

test.describe("Infrastructure onboarding", () => {
  test.setTimeout(180_000);

  test("desktop landing hides empty platform sections before any onboarding flow opens", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only infrastructure manager coverage",
    );

    await prepareOnboardingPage(page);
    await page.route("**/api/discover", async (route) => {
      const requestUrl = new URL(route.request().url());
      if (
        route.request().method() !== "GET" ||
        requestUrl.pathname !== "/api/discover"
      ) {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          servers: [
            {
              ip: "192.168.0.2",
              port: 8006,
              type: "pve",
              version: "8.2.2",
            },
          ],
          errors: [],
          cached: true,
          updated: 0,
          age: 120,
        }),
      });
    });

    await page.goto("/settings/infrastructure", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, {
      timeout: 15_000,
    });

    await expect(
      page.getByText("Connected systems", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Run discovery/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Add infrastructure/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Discovery settings/i }),
    ).toBeVisible();
    await expect(page.getByText("VMware vCenter", { exact: true })).toHaveCount(
      0,
    );
    await expect(page.getByText("TrueNAS SCALE", { exact: true })).toHaveCount(
      0,
    );
    await expect(page.getByText("Proxmox VE", { exact: true })).toBeVisible();
    await expect(
      page.getByText("Standalone hosts", { exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: /Add TrueNAS SCALE/i }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: /Detect API platform/i }),
    ).toHaveCount(0);
    await expect(
      page.getByText("Monitored systems", { exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByText("Connection types", { exact: true }),
    ).toHaveCount(0);
  });

  test("desktop discovery settings open as an infrastructure-owned dialog", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only infrastructure manager coverage",
    );

    await prepareOnboardingPage(page);

    await page.goto("/settings/infrastructure", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, {
      timeout: 15_000,
    });

    await page.getByRole("button", { name: /Discovery settings/i }).click();

    const dialog = page.getByRole("dialog", { name: "Discovery settings" });
    await expect(dialog).toBeVisible();
    await expect(
      dialog.getByText(
        "Configure the saved network scope and background scan behavior for infrastructure source discovery.",
      ),
    ).toBeVisible();
    await expect(dialog.getByText("Automatic scanning")).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "Close discovery settings dialog" }),
    ).toBeVisible();
    await expect(page).toHaveURL(/\/settings\/infrastructure(?:\?.*)?$/);
  });

  test("desktop picker add opens the matching modal", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only manager add-path instrumentation coverage",
    );

    await prepareOnboardingPage(page);

    await page.goto("/settings/infrastructure", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, {
      timeout: 15_000,
    });

    await page
      .getByRole("button", { name: "Add infrastructure", exact: true })
      .first()
      .click();
    await page.waitForURL(/\/settings\/infrastructure\?add=pick$/, {
      timeout: 15_000,
    });
    await page
      .getByRole("dialog")
      .getByRole("button", { name: /TrueNAS SCALE/i })
      .click();
    await page.waitForURL(/\/settings\/infrastructure\?add=truenas$/, {
      timeout: 15_000,
    });

    await expect(
      page.getByText("Connected systems", { exact: true }),
    ).toBeVisible();
    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(
      page
        .getByRole("dialog")
        .getByRole("heading", { name: "Add TrueNAS SCALE", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", {
        name: "Close add infrastructure dialog",
        exact: true,
      }),
    ).toBeVisible();
  });

  test("desktop explicit discovery scan surfaces a candidate row and opens a prefilled review dialog", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only discovery-candidate review coverage",
    );

    await prepareOnboardingPage(page);

    await page.route("**/api/discover", async (route) => {
      const requestUrl = new URL(route.request().url());
      if (requestUrl.pathname !== "/api/discover") {
        await route.continue();
        return;
      }

      if (route.request().method() === "GET") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            servers: [],
            errors: [],
            cached: true,
            updated: 0,
            age: 0,
          }),
        });
        return;
      }

      if (route.request().method() === "POST") {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            servers: [
              {
                ip: "10.0.0.55",
                port: 8006,
                type: "pve",
                version: "8.2.2",
                hostname: "discovered-pve.lab",
              },
            ],
            errors: [],
            cached: false,
            scanning: false,
            timestamp: 1_700_000_000_000,
          }),
        });
        return;
      }

      await route.continue();
    });

    await page.goto("/settings/infrastructure", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, {
      timeout: 15_000,
    });

    await page.getByRole("button", { name: /Run discovery/i }).click();

    await expect(
      page.getByText("discovered-pve.lab", { exact: true }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: /^Review$/i })).toBeVisible();

    await page.getByRole("button", { name: /^Review$/i }).click();
    await page.waitForURL(/\/settings\/infrastructure\?add=pve$/, {
      timeout: 15_000,
    });

    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(
      page
        .getByRole("dialog")
        .getByRole("heading", { name: "Add Proxmox VE", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByPlaceholder("https://proxmox.example.com:8006"),
    ).toHaveValue("https://discovered-pve.lab:8006");
  });

  test("desktop detect utility offers no-match agent fallback from the add-infrastructure picker", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop-only onboarding probe instrumentation coverage",
    );

    await prepareOnboardingPage(page);

    await page.route("**/api/connections/probe", async (route) => {
      const requestUrl = new URL(route.request().url());
      if (
        route.request().method() !== "POST" ||
        requestUrl.pathname !== "/api/connections/probe"
      ) {
        await route.continue();
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          candidates: [],
          probedMs: 184,
        }),
      });
    });

    await page.goto("/settings/infrastructure", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, {
      timeout: 15_000,
    });
    await page
      .getByRole("button", { name: "Add infrastructure", exact: true })
      .first()
      .click();
    await page.waitForURL(/\/settings\/infrastructure\?add=pick$/, {
      timeout: 15_000,
    });
    await page.getByRole("button", { name: /Detect API platform/i }).click();
    await page.waitForURL(/\/settings\/infrastructure\?add=detect$/, {
      timeout: 15_000,
    });

    await page.getByLabel("API endpoint").fill("baremetal.lab");
    await page
      .getByRole("button", { name: "Probe API endpoint", exact: true })
      .click();

    await expect(
      page.getByText(
        "No supported API-backed platform detected at that address.",
        { exact: true },
      ),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "install Pulse Agent instead", exact: true })
      .click();
    await page.waitForURL(/\/settings\/infrastructure\?add=linux-host$/, {
      timeout: 15_000,
    });
    await expect(
      page.getByRole("heading", { level: 2, name: "Install on a host" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Probe API endpoint/i }),
    ).toHaveCount(0);
  });

  test("mobile landing and grouped platform tables stay inside the viewport", async ({
    page,
  }, testInfo) => {
    test.skip(
      !testInfo.project.name.startsWith("mobile-"),
      "Mobile-only infrastructure manager overflow coverage",
    );

    await prepareOnboardingPage(page);

    await page.goto("/settings/infrastructure", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/infrastructure(?:\?.*)?$/, {
      timeout: 15_000,
    });
    await expect(
      page.getByText("Connected systems", { exact: true }),
    ).toBeVisible();
    await expect(page.getByText("Pulse Agent", { exact: true })).toBeVisible();
    await expect(
      page.getByRole("button", { name: /Add Pulse Agent/i }),
    ).toBeVisible();

    await scrollToBottom(page);
    const audit = await auditHorizontalOverflow(page);

    expect(
      audit.pageWidth,
      `Mobile onboarding overflow (viewport=${audit.viewportWidth}, page=${audit.pageWidth}, offenders=${JSON.stringify(audit.offenders)})`,
    ).toBeLessThanOrEqual(audit.viewportWidth + 1);
  });
});
