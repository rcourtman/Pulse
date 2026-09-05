// Browser contract proof against the current hot-dev build. Inference is scripted.
// This does not qualify provider reasoning or real infrastructure actions.
import {
  chromium,
  expect,
} from "../tests/integration/node_modules/@playwright/test/index.mjs";
import { mkdir, writeFile } from "node:fs/promises";
const base = process.env.PLAYWRIGHT_BASE_URL || "http://127.0.0.1:5173";
const output = new URL("../tmp/patrol-assistant-journey/", import.meta.url);
await mkdir(output, { recursive: true });
const now = new Date().toISOString();
const resourceId = "pve:vm:101";
const makeItem = (id, title) => ({
  id,
  operationalRecordId: id,
  subjectResourceId: resourceId,
  subjectResourceName: "Database VM",
  subjectResourceType: "vm",
  kind: "disk",
  title,
  plainLanguageSummary: "The database disk is nearly full.",
  severity: "warning",
  state: "open",
  firstObservedAt: now,
  lastObservedAt: now,
  evidenceFreshness: "fresh",
  evidenceCompleteness: "complete",
  impact: "Writes may fail.",
  relatedResources: [],
  availableActions: [],
  verificationState: "not_available",
});
const items = [
  makeItem("issue-a", "Database disk pressure"),
  makeItem("issue-b", "Backup protection missing"),
];
const summary = {
  activeCount: 2,
  openCount: 2,
  acknowledgedCount: 0,
  suppressedCount: 0,
  uncertainCount: 0,
  resolvedCount: 0,
  calm: false,
  coverageState: "current",
  evaluatedAt: now,
};
const detail = (item) => ({
  item,
  operationalRecord: {
    ...item,
    canonicalSpecId: "disk-pressure",
    stateChangedAt: now,
    evidenceIds: ["evidence-1"],
    causeKey: "capacity",
    relatedResourceIds: [],
  },
  timeline: [],
  evidence: [
    {
      id: "evidence-1",
      source: { provider: "proxmox", collector: "metrics" },
      subject: { resourceId },
      observedAt: now,
      ingestedAt: now,
      completeness: "complete",
      confidence: "confirmed",
      permissions: "sufficient",
      reason: { code: "capacity", message: "Disk usage is 95 percent." },
    },
  ],
});
const browser = await chromium.launch({ headless: true });
const context = await browser.newContext();
const page = await context.newPage();
const requests = [];
const mutations = [];
const errors = [];
let failNext = false;
page.on("pageerror", (error) => errors.push(error.message));
const reply = async (route, failure = false) =>
  route.fulfill({
    status: 200,
    contentType: "text/event-stream",
    body: (failure
      ? [
          {
            type: "error",
            data: { message: "Qualification provider unavailable" },
          },
        ]
      : [
          { type: "session", data: { id: "explanation-proof" } },
          {
            type: "content",
            data: "The database disk is 95 percent full. Check the recent growth before choosing a change. No changes were made.",
          },
          { type: "done", data: { session_id: "explanation-proof" } },
        ]
    )
      .map((event) => `data: ${JSON.stringify(event)}\n\n`)
      .join(""),
  });
try {
  await page.goto(base, { waitUntil: "domcontentloaded" });
  await page
    .getByLabel("Username", { exact: true })
    .fill(process.env.PULSE_E2E_USERNAME || "admin");
  await page
    .getByLabel("Password", { exact: true })
    .fill(process.env.PULSE_E2E_PASSWORD || "adminadminadmin");
  await page.getByRole("button", { name: "Sign in to Pulse" }).click();
  await expect(page.getByLabel("Password", { exact: true })).toHaveCount(0);
  await page.route("**/api/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/ai/chat") {
      requests.push(route.request().postDataJSON());
      const failure = failNext;
      failNext = false;
      return reply(route, failure);
    }
    if (path.startsWith("/api/ai/test/"))
      return route.fulfill({
        json: { success: true, message: "Scripted provider readiness" },
      });
    if (route.request().method() !== "GET") {
      mutations.push(path);
      return route.fulfill({
        status: 403,
        json: { error: "Writes disabled in journey proof" },
      });
    }
    if (path === "/api/ai/status")
      return route.fulfill({ json: { running: true } });
    if (path === "/api/ai/settings")
      return route.fulfill({
        json: {
          enabled: true,
          model: "ollama:qualification",
          chat_model: "ollama:qualification",
          control_level: "read_only",
          discovery_enabled: false,
          autonomous_mode: false,
        },
      });
    if (path === "/api/ai/models")
      return route.fulfill({
        json: {
          models: [
            {
              id: "ollama:qualification",
              name: "Qualification",
              provider: "ollama",
            },
          ],
        },
      });
    if (path === "/api/ai/sessions") return route.fulfill({ json: [] });
    if (path === "/api/ai/patrol/attention/summary")
      return route.fulfill({ json: summary });
    if (path === "/api/ai/patrol/attention")
      return route.fulfill({
        json: {
          data: items,
          summary,
          meta: { page: 1, limit: 50, total: 2, totalPages: 1 },
        },
      });
    const item = items.find(
      (item) => path === `/api/ai/patrol/attention/${item.id}`,
    );
    if (item) return route.fulfill({ json: detail(item) });
    return route.continue();
  });
  const results = [];
  for (const width of [1440, 900, 390]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto(`${base}/patrol`, { waitUntil: "domcontentloaded" });
    await page
      .getByRole("button", { name: "Start review", exact: true })
      .click();
    await page.getByText("Evidence and history", { exact: true }).click();
    const explain = page.getByRole("button", {
      name: "Explain with Assistant",
      exact: true,
    });
    await explain.scrollIntoViewIfNeeded();
    await explain.focus();
    await expect(explain).toBeFocused();
    const before = requests.length;
    await explain.press("Enter");
    await expect.poll(() => requests.length).toBe(before + 1);
    expect(requests.at(-1).prompt).toContain("Explain this issue");
    expect(requests.at(-1).handoff_context).toContain(
      "Attention Item: issue-a",
    );
    expect(requests.at(-1).handoff_context).toContain(
      "Disk usage is 95 percent.",
    );
    expect(requests.at(-1).handoff_resources[0].id).toBe(resourceId);
    expect(requests.at(-1).autonomous_mode).toBe(false);
    await expect(
      page.getByText("No changes were made.", { exact: false }),
    ).toBeVisible();
    await expect(
      page.getByText("Database disk pressure", { exact: true }).last(),
    ).toBeVisible();
    await expect(page.getByTestId("assistant-workflow-starters")).toHaveCount(
      0,
    );
    await page.screenshot({
      path: new URL(`explanation-${width}.png`, output).pathname,
    });
    const composer = page.getByPlaceholder("Ask about your infrastructure...");
    await composer.fill("My unfinished question");
    await page.keyboard.press("Escape");
    await page
      .getByRole("button", { name: /Ask Pulse Assistant about Patrol/ })
      .click();
    await expect(composer).toHaveValue("My unfinished question");
    expect(requests.length).toBe(before + 1);
    await page.keyboard.press("Escape");
    results.push({
      width,
      state:
        "selected issue, evidence expansion, keyboard Explain, streamed result, close/reopen, draft preserved",
    });
  }
  // A failed explicit request uses the normal error and retry surface.
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto(`${base}/patrol`, { waitUntil: "domcontentloaded" });
  await page.getByRole("button", { name: "Start review", exact: true }).click();
  failNext = true;
  await page
    .getByRole("button", { name: "Explain with Assistant", exact: true })
    .click();
  await expect(
    page.getByText("Qualification provider unavailable", { exact: false }),
  ).toBeVisible();
  await page.screenshot({
    path: new URL("provider-error.png", output).pathname,
  });
  await page
    .getByRole("button", { name: "Try again", exact: true })
    .first()
    .click();
  await expect(
    page.getByText("No changes were made.", { exact: false }),
  ).toBeVisible();
  expect(requests.at(-1).handoff_context).toContain("Attention Item: issue-a");
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 1000 });
    await page.goto(`${base}/alerts`, { waitUntil: "domcontentloaded" });
    const menu = page.getByRole("button", { name: "More alert actions", exact: true }).first();
    await expect(menu).toBeVisible({ timeout: 20000 });
    await menu.click();
    await expect(page.getByRole("menuitem")).toBeVisible();
    await page.screenshot({ path: new URL(`alert-menu-${width}.png`, output).pathname });
    await page.keyboard.press("Escape");
    await expect(page.getByRole("menuitem")).toHaveCount(0);
    await menu.click();
    await page.getByText("Active Alerts", { exact: true }).click();
    await expect(page.getByRole("menuitem")).toHaveCount(0);
    await menu.click();
    const before = requests.length;
    await page.getByRole("menuitem").click();
    await expect.poll(() => requests.length).toBe(before + 1);
    expect(requests.at(-1).handoff_context).toContain("Source: Pulse Alerts active alert");
    expect(requests.at(-1).handoff_resources.length).toBeGreaterThan(0);
    expect(requests.at(-1).autonomous_mode).toBe(false);
    await expect(page.getByText("No changes were made.", { exact: false })).toBeVisible();
    await expect(page.getByRole("menuitem")).toHaveCount(0);
    await page.screenshot({ path: new URL(`alert-explanation-${width}.png`, output).pathname });
    results.push({ width, state: "alert menu, Escape, outside click, selected alert explanation" });
  }
  // No background inference after a reload or context-only drawer open.
  const beforeReload = requests.length;
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForTimeout(500);
  expect(requests.length).toBe(beforeReload);
  expect(mutations).toEqual([]);
  expect(errors).toEqual([]);
  await writeFile(
    new URL("result.json", output),
    JSON.stringify(
      {
        passed: true,
        base,
        results,
        requestCount: requests.length,
        mutations,
        errors,
      },
      null,
      2,
    ),
  );
  console.log(
    JSON.stringify({ passed: true, results, requestCount: requests.length }),
  );
} catch (error) {
  console.error(await page.locator("body").innerText());
  console.error("Page errors:", errors);
  await page.screenshot({ path: new URL("failure.png", output).pathname });
  throw error;
} finally {
  await browser.close();
}
