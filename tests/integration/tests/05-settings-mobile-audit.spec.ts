import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base, expect } from "@playwright/test";
import {
  apiRequest,
  createAuthenticatedStorageState,
  createOrg,
} from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
type WorkerFixtures = {
  authStorageStatePath: string;
};

const MULTI_TENANT_ENABLED = process.env.PULSE_MULTI_TENANT_ENABLED === "true";
const HOSTED_BILLING_ADMIN_ENABLED =
  MULTI_TENANT_ENABLED && process.env.PULSE_HOSTED_MODE === "true";

const STANDARD_SETTINGS_ROUTES = [
  "/settings/infrastructure",
  "/settings/monitoring/availability",
  "/settings/pulse-intelligence/provider",
  "/settings/pulse-intelligence/patrol",
  "/settings/pulse-intelligence/assistant",
  "/settings/pulse-intelligence/billing/plan",
  "/settings/system-general",
  "/settings/system-network",
  "/settings/system-updates",
  "/settings/system-recovery",
  "/settings/system-relay",
  "/settings/support/diagnostics",
  "/settings/support/reporting",
  "/settings/support/logs",
  "/settings/security/api",
  "/settings/security-overview",
  "/settings/security-data-handling",
  "/settings/security-auth",
  "/settings/security-sso",
  "/settings/security-roles",
  "/settings/security-users",
  "/settings/security-audit",
  "/settings/security-webhooks",
] as const;

const ORGANIZATION_SETTINGS_ROUTES = [
  "/settings/organization",
  "/settings/organization/access",
  "/settings/organization/sharing",
  "/settings/organization/billing",
] as const;

const HOSTED_SETTINGS_ROUTES = [
  "/settings/organization/billing-admin",
] as const;

const SETTINGS_ROUTES = [
  ...STANDARD_SETTINGS_ROUTES,
  ...(MULTI_TENANT_ENABLED ? ORGANIZATION_SETTINGS_ROUTES : []),
  ...(HOSTED_BILLING_ADMIN_ENABLED ? HOSTED_SETTINGS_ROUTES : []),
] as const;

const SETTINGS_ROUTE_REDIRECTS = [
  {
    route: "/settings/pulse-intelligence/discovery",
    expectedPath: "/settings/pulse-intelligence/assistant",
  },
  {
    route: "/settings/pulse-intelligence/billing/usage",
    expectedPath: "/settings/pulse-intelligence/billing/plan",
  },
  ...(!MULTI_TENANT_ENABLED
    ? ORGANIZATION_SETTINGS_ROUTES.map((route) => ({
        route,
        expectedPath: "/settings/infrastructure",
      }))
    : []),
  ...(!HOSTED_BILLING_ADMIN_ENABLED
    ? [
        {
          route: "/settings/organization/billing-admin",
          expectedPath: "/settings/infrastructure",
        } as const,
      ]
    : []),
] as const;

const MOBILE_VIEWPORTS = [
  { width: 320, height: 568, label: "compact phone" },
  { width: 390, height: 844, label: "modern phone" },
] as const;

const prepareOrganizationAuditFixture = async (
  page: import("@playwright/test").Page,
  route: string,
): Promise<string[]> => {
  if (!(ORGANIZATION_SETTINGS_ROUTES as readonly string[]).includes(route))
    return [];

  await page.goto("/settings/infrastructure", {
    waitUntil: "domcontentloaded",
  });
  const createdOrgIDs: string[] = [];
  let targetOrgID = "";

  if (route === "/settings/organization/sharing") {
    const target = await createOrg(page, "Mobile Audit Target");
    targetOrgID = target.id;
    createdOrgIDs.push(target.id);
  }

  const source = await createOrg(page, "Mobile Audit Organization");
  createdOrgIDs.push(source.id);
  await page.reload({ waitUntil: "domcontentloaded" });
  const organizationSwitcher = page.getByRole("combobox", {
    name: "Organization",
  });
  await expect(organizationSwitcher).toBeVisible({ timeout: 20000 });
  await organizationSwitcher.selectOption(source.id);
  await expect(organizationSwitcher).toHaveValue(source.id, {
    timeout: 20000,
  });

  if (route === "/settings/organization/access") {
    const invitation = await apiRequest(
      page,
      `/api/orgs/${encodeURIComponent(source.id)}/members`,
      {
        method: "POST",
        data: { userId: "mobile-audit-viewer", role: "viewer" },
        headers: {
          "Content-Type": "application/json",
          "X-Org-ID": source.id,
          "X-Pulse-Org-ID": source.id,
        },
      },
    );
    expect(
      invitation.ok(),
      `mobile access fixture invitation: ${invitation.status()} ${await invitation.text()}`,
    ).toBeTruthy();
  }

  if (route === "/settings/organization/sharing") {
    const share = await apiRequest(
      page,
      `/api/orgs/${encodeURIComponent(source.id)}/shares`,
      {
        method: "POST",
        data: {
          targetOrgId: targetOrgID,
          resourceType: "view",
          resourceId: "mobile-audit-view",
          resourceName: "Mobile Audit Shared View",
          accessRole: "viewer",
        },
        headers: {
          "Content-Type": "application/json",
          "X-Org-ID": source.id,
          "X-Pulse-Org-ID": source.id,
        },
      },
    );
    expect(
      share.ok(),
      `mobile sharing fixture: ${share.status()} ${await share.text()}`,
    ).toBeTruthy();
  }

  return createdOrgIDs;
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
        `settings-mobile-audit-${workerInfo.project.name}.json`,
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

type OverflowAudit = {
  viewportWidth: number;
  pageWidth: number;
  overflowPx: number;
  offenders: Array<{ tag: string; className: string; overflow: number }>;
};

const auditHorizontalOverflow = async (
  page: import("@playwright/test").Page,
): Promise<OverflowAudit> =>
  page.evaluate(() => {
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

const scrollToBottom = async (
  page: import("@playwright/test").Page,
): Promise<void> => {
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
};

test.describe("Settings mobile optimization audit", () => {
  test.setTimeout(180_000);

  for (const route of SETTINGS_ROUTES) {
    test(`no horizontal overflow after full scroll on ${route}`, async ({
      page,
    }) => {
      const createdOrgIDs = await prepareOrganizationAuditFixture(page, route);
      try {
        for (const viewport of MOBILE_VIEWPORTS) {
          await page.setViewportSize(viewport);
          await page.goto(route, { waitUntil: "domcontentloaded" });
          await page.waitForURL((url) => url.pathname === route, {
            timeout: 15000,
          });
          expect(
            new URL(page.url()).pathname,
            `${route} must remain the surface being audited rather than silently redirecting`,
          ).toBe(route);
          await expect(page.locator("#root")).toBeVisible();
          await page.waitForTimeout(1500);

          const loadFailure = page
            .getByRole("alert")
            .filter({ hasText: /^Unable to load .* settings\.$/ });
          await expect(
            loadFailure,
            `${route} must render its real settings state instead of an API-error shell`,
          ).toHaveCount(0);

          if (
            route === "/settings/organization/billing-admin" &&
            viewport.width === MOBILE_VIEWPORTS[0].width
          ) {
            const organizationToggle = page
              .locator("tbody button")
              .filter({ hasText: "default" });
            await expect(organizationToggle).toHaveCount(1);
            await organizationToggle.click();
            await expect(
              page.getByText("Billing state JSON", { exact: true }),
            ).toBeVisible();
          }

          await scrollToBottom(page);
          const atBottom = await page.evaluate(() => {
            const scrollTop = window.scrollY;
            const maxScrollTop = Math.max(
              0,
              document.documentElement.scrollHeight - window.innerHeight,
            );
            return scrollTop >= maxScrollTop - 3;
          });
          expect(
            atBottom,
            `Expected to reach the bottom while auditing ${route} on a ${viewport.label}`,
          ).toBeTruthy();

          const audit = await auditHorizontalOverflow(page);
          expect(
            audit.pageWidth,
            `Mobile overflow on ${route} at ${viewport.width}px (viewport=${audit.viewportWidth}, page=${audit.pageWidth}, offenders=${JSON.stringify(audit.offenders)})`,
          ).toBeLessThanOrEqual(audit.viewportWidth + 1);
        }
      } finally {
        if (createdOrgIDs.length > 0) {
          const organizationSwitcher = page.getByRole("combobox", {
            name: "Organization",
          });
          if (await organizationSwitcher.isVisible().catch(() => false)) {
            await organizationSwitcher.selectOption("default");
            await expect(organizationSwitcher).toHaveValue("default", {
              timeout: 20000,
            });
          }
        }
        for (const orgID of [...createdOrgIDs].reverse()) {
          const response = await apiRequest(
            page,
            `/api/orgs/${encodeURIComponent(orgID)}`,
            {
              method: "DELETE",
              headers: {
                "X-Org-ID": "default",
                "X-Pulse-Org-ID": "default",
              },
            },
          );
          const responseBody =
            response.status() === 204 ? "" : await response.text();
          expect(
            [204, 404].includes(response.status()),
            `cleanup organization ${orgID}: ${response.status()} ${responseBody}`,
          ).toBeTruthy();
        }
      }
    });
  }

  for (const { route, expectedPath } of SETTINGS_ROUTE_REDIRECTS) {
    test(`resolves unavailable or compatibility route ${route} explicitly`, async ({
      page,
    }) => {
      await page.goto(route, { waitUntil: "domcontentloaded" });
      await page.waitForURL((url) => url.pathname === expectedPath, {
        timeout: 15000,
      });
      expect(new URL(page.url()).pathname).toBe(expectedPath);
    });
  }

  test("mobile Settings navigation is viewport-bounded and independently scrollable", async ({
    page,
  }) => {
    await page.setViewportSize(MOBILE_VIEWPORTS[0]);
    await page.goto("/settings/system-general", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(
      (url) => url.pathname === "/settings/system-general",
      {
        timeout: 15000,
      },
    );

    await page
      .getByRole("main")
      .getByRole("button", { name: "Settings", exact: true })
      .click();
    const navigation = page.getByLabel("Settings navigation", { exact: true });
    await expect(navigation).toBeVisible();

    const geometry = await navigation.evaluate((element) => {
      const style = window.getComputedStyle(element);
      return {
        clientHeight: element.clientHeight,
        scrollHeight: element.scrollHeight,
        overflowY: style.overflowY,
        viewportHeight: window.innerHeight,
      };
    });

    expect(["auto", "scroll"]).toContain(geometry.overflowY);
    expect(geometry.clientHeight).toBeLessThan(geometry.viewportHeight);
    expect(geometry.scrollHeight).toBeGreaterThan(geometry.clientHeight);
  });
});
