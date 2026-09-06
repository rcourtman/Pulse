// Run with pulse-heavy-run -- node scripts/check-assistant-identity.mjs
import { chromium, expect } from "../node_modules/@playwright/test/index.mjs";
import { createServer } from "../frontend-modern/node_modules/vite/dist/node/index.js";
import solid from "../frontend-modern/node_modules/vite-plugin-solid/dist/esm/index.mjs";
import { resolve } from "node:path";
import { mkdir, readFile, writeFile, readdir, rm } from "node:fs/promises";
import { createHash } from "node:crypto";
const root = resolve(import.meta.dirname, "../frontend-modern");
const output = resolve(
  process.env.ASSISTANT_IDENTITY_OUTPUT || `${root}/../tmp/assistant-identity`,
);
await mkdir(output, { recursive: true });
await rm(`${output}/receipt.json`, { force: true });
process.chdir(root);
const server = await createServer({
  configFile: false,
  root,
  optimizeDeps: { esbuildOptions: { target: "esnext" } },
  plugins: [solid()],
  resolve: { alias: { "@": `${root}/src` } },
  server: { host: "127.0.0.1", port: 0 },
});
let browser;
const results = [];
try {
  await server.listen();
  const base = server.resolvedUrls.local[0];
  browser = await chromium.launch({ headless: true });
  for (const width of [1440, 900, 390]) {
    const context = await browser.newContext({
      viewport: { width, height: 1000 },
      serviceWorkers: "block",
    });
    const page = await context.newPage();
    const errors = [],
      blocked = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await context.route("**/*", (route) => {
      const url = new URL(route.request().url());
      if (
        url.origin === new URL(base).origin &&
        !url.pathname.startsWith("/api/")
      )
        return route.continue();
      blocked.push(url.pathname);
      return route.abort();
    });
    await page.goto(`${base}qualification/assistant-identity/`);
    await page.waitForFunction(() => !!window.identityFixture);
    const call = (method, arg) =>
      page.evaluate(
        ([method, arg]) => window.identityFixture[method](arg),
        [method, arg],
      );
    const fire = (type, data) => call("fire", { type, data });
    await call("start");
    for (const id of ["a", "b"])
      await fire("tool_start", {
        id,
        name: "pulse_query",
        input: JSON.stringify({ action: "search", query: `fixture-${id}` }),
      });
    expect((await call("snapshot")).pendingTools.map((t) => t.id)).toEqual([
      "a",
      "b",
    ]);
    await fire("tool_progress", {
      id: "a",
      name: "pulse_query",
      message: "Reading A",
    });
    await fire("tool_end", {
      id: "a",
      name: "pulse_query",
      output: "identity evidence A",
      success: true,
    });
    expect((await call("snapshot")).pendingTools.map((t) => t.id)).toEqual([
      "b",
    ]);
    await fire("tool_end", {
      id: "b",
      name: "pulse_query",
      output: "identity evidence B",
      success: false,
    });
    expect((await call("snapshot")).toolCalls.map((t) => t.output)).toEqual([
      "identity evidence A",
      "identity evidence B",
    ]);
    for (const id of ["c", "d"]) {
      await fire("tool_start", {
        id,
        name: "pulse_control",
        input: JSON.stringify({ resource_id: id }),
      });
      await fire("approval_needed", {
        tool_id: id,
        tool_name: "pulse_control",
        approval_id: `approval-${id}`,
        command: `synthetic-${id}`,
      });
    }
    await expect(
      page.getByText("Approval Required", { exact: true }),
    ).toHaveCount(2);
    await fire("tool_end", {
      id: "c",
      name: "pulse_control",
      output: "synthetic c completed",
      success: true,
    });
    await expect(
      page.getByText("Approval Required", { exact: true }),
    ).toHaveCount(1);
    expect(
      (await call("snapshot")).pendingApprovals.map((a) => a.toolId),
    ).toEqual(["d"]);
    await expect(page.getByText("synthetic-d", { exact: true })).toBeVisible();
    await page.screenshot({
      path: `${output}/approvals-${width}.png`,
      fullPage: true,
    });
    await fire("done", {});
    while (await page.locator('[aria-expanded="false"]').count())
      await page.locator('[aria-expanded="false"]').first().click();
    await expect(
      page.getByText("identity evidence A", { exact: true }).last(),
    ).toBeVisible();
    await expect(
      page.getByText("identity evidence B", { exact: true }).last(),
    ).toBeVisible();
    for (let stage = 0; stage < 4; stage++) {
      await call("shared", stage);
      // Allow Solid's render effects to run between immutable snapshots.
      await page.evaluate(() => new Promise(requestAnimationFrame));
    }
    expect(await call("evidence")).toEqual([
      {
        name: "pulse_query",
        input: "client",
        output: "client evidence",
        success: true,
      },
      {
        name: "pulse_alerts",
        input: "alerts",
        output: "alert evidence",
        success: true,
      },
    ]);
    const expand = page.locator('[role="button"][aria-expanded="false"]');
    while (await expand.count()) await expand.first().click();
    await expect(
      page.getByText("client evidence", { exact: true }).last(),
    ).toBeVisible();
    await expect(
      page.getByText("alert evidence", { exact: true }).last(),
    ).toBeVisible();
    await expect(
      page.getByText("client", { exact: true }).last(),
    ).toBeVisible();
    await expect(
      page.getByText("alerts", { exact: true }).last(),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await page.screenshot({
      path: `${output}/expanded-${width}.png`,
      fullPage: true,
    });
    expect(errors).toEqual([]);
    expect(blocked).toEqual([]);
    results.push({ width, passed: true });
    await context.close();
  }
  const hashes = {};
  for (const file of await readdir(output)) {
    if (file.endsWith(".png"))
      hashes[`artifacts/${file}`] = createHash("sha256")
        .update(await readFile(`${output}/${file}`))
        .digest("hex");
  }
  hashes["../package-lock.json"] = createHash("sha256")
    .update(await readFile(`${root}/../package-lock.json`))
    .digest("hex");
  async function hashTree(dir) {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const path = `${dir}/${entry.name}`;
      if (entry.isDirectory()) await hashTree(path);
      else
        hashes[path.slice(root.length + 1)] = createHash("sha256")
          .update(await readFile(path))
          .digest("hex");
    }
  }
  await hashTree(`${root}/src`);
  await hashTree(`${root}/qualification/assistant-identity`);
  for (const path of [
    "package-lock.json",
    "tailwind.config.js",
    "postcss.config.js",
  ])
    hashes[path] = createHash("sha256")
      .update(await readFile(`${root}/${path}`))
      .digest("hex");
  hashes["../scripts/check-assistant-identity.mjs"] = createHash("sha256")
    .update(await readFile(import.meta.filename))
    .digest("hex");
  await writeFile(
    `${output}/receipt.json`,
    JSON.stringify(
      {
        recordedAt: new Date().toISOString(),
        scope:
          "Synthetic reducer and renderer only; no provider, backend or real actions qualified",
        browser: browser.version(),
        results,
        hashes,
      },
      null,
      2,
    ),
  );
  console.log(JSON.stringify({ output, results }));
} finally {
  await browser?.close();
  await server.close();
}
