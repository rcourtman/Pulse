// Isolated real-browser component qualification; no installed backend or delivery claim.
import { createServer } from "../frontend-modern/node_modules/vite/dist/node/index.js";
import solid from "../frontend-modern/node_modules/vite-plugin-solid/dist/esm/index.mjs";
import { chromium } from "@playwright/test";
import { resolve } from "node:path";
import { mkdirSync } from "node:fs";
import assert from "node:assert/strict";
const root = resolve("frontend-modern");
process.chdir(root);
import { fixture } from './incident-browser-fixture.mjs';
import { historyFixture } from './history-clear-browser-fixture.mjs';

const server = await createServer({
  root,
  configFile: false,
  optimizeDeps: {
    noDiscovery: true,
    entries: [],
    esbuildOptions: { target: "esnext" },
  },
  esbuild: { target: "esnext" },
  plugins: [
    solid(),
    {
      name: "incident-fixture",
      configureServer(s) {
        s.middlewares.use((req, res, next) => {
          if ((req.url === "/qualification" || req.url === "/history-qualification")) {
            res.setHeader("Content-Type", "text/html");
            res.end(
              '<div id="root"></div><script type="module" src="' + (req.url === '/history-qualification' ? '/history-fixture.tsx' : '/incident-fixture.tsx') + '"></script>',
            );
          } else next();
        });
      },
      resolveId(id) {
        if (id === "/incident-fixture.tsx" || id === "/history-fixture.tsx") return id;
      },
      load(id) {
        if (id === "/incident-fixture.tsx") return fixture;
        if (id === "/history-fixture.tsx") return historyFixture;
      },
    },
  ],
  resolve: { alias: { "@": resolve(root, "src") } },
  server: { host: "127.0.0.1", port: 5198, strictPort: true },
});
let browser;
try {
  await server.listen();
  browser = await chromium.launch({ headless: true });
  const output =
    process.env.PULSE_BROWSER_OUTPUT || "/tmp/pulse-incident-ownership";
  mkdirSync(output, { recursive: true });
  let cases = 0;
  for (const width of [1440, 390]) {
    for (const scenario of [
      "reset",
      "reverse-success",
      "obsolete-failure",
      "failure-after-success",
      "dispose-success",
      "dispose-failure",
      "current-failure",
      "retry",
    ]) {
      const page = await browser.newPage({ viewport: { width, height: 900 } });
      await page.goto("http://127.0.0.1:5198/qualification");
      await page.getByRole("button", { name: "Open row", exact: true }).click();
      await page.waitForFunction(() => window.count?.() === 1);
      if (scenario === "reset") {
        await page.getByRole("button", { name: "Reset", exact: true }).click();
        await page.evaluate(() => window.finish(0, "empty"));
      } else if (scenario.startsWith("dispose")) {
        await page
          .getByRole("button", { name: "Unmount", exact: true })
          .click();
        await page.evaluate(
          (s) => window.finish(0, s === "dispose-failure" ? "error" : "empty"),
          scenario,
        );
      } else if (scenario === "current-failure" || scenario === "retry") {
        await page.evaluate(() => window.finish(0, "error"));
        await page.waitForFunction(() => window.snapshot().error.host === true);
        if (scenario === "retry") {
          await page.getByRole("button", { name: "Overlap refresh", exact: true }).click();
          await page.waitForFunction(() => window.count() === 2);
          assert.equal((await page.evaluate(() => window.snapshot())).error.host, false);
          await page.evaluate(() => window.finish(1, "Latest incident"));
        }
      } else {
        await page
          .getByRole("button", { name: "Overlap refresh", exact: true })
          .click();
        await page.waitForFunction(() => window.count() === 2);
        if (scenario === "obsolete-failure") {
          await page.evaluate(() => window.finish(0, "error"));
          await page.evaluate(
            () => new Promise((r) => requestAnimationFrame(r)),
          );
          assert.equal(
            (await page.evaluate(() => window.snapshot())).loading.host,
            true,
          );
          assert.equal(
            (await page.evaluate(() => window.snapshot())).error.host,
            false,
          );
          assert.equal(await page.getByTestId("errors").textContent(), "0");
          await page.evaluate(() => window.finish(1, "Latest incident"));
        } else {
          await page.evaluate(() => window.finish(1, "Latest incident"));
          await page.getByText("Latest incident", { exact: true }).waitFor();
          await page.evaluate(
            (s) =>
              window.finish(
                0,
                s === "failure-after-success" ? "error" : "Obsolete incident",
              ),
            scenario,
          );
        }
      }
      await page.evaluate(
        () =>
          new Promise((r) =>
            requestAnimationFrame(() => requestAnimationFrame(r)),
          ),
      );
      const snapshot = await page.evaluate(() => window.snapshot());
      assert.equal(
        await page.getByTestId("errors").textContent(),
        ["current-failure", "retry"].includes(scenario) ? "1" : "0",
      );
      if (scenario === "reset") {
        assert.deepEqual(snapshot, { incidents: {}, loading: {}, error: {} });
      } else if (scenario.startsWith("dispose")) {
        assert.deepEqual(snapshot, {
          incidents: {},
          loading: { host: true },
          error: { host: false },
        });
        assert.equal(await page.locator("section").count(), 0);
      } else if (scenario === "current-failure") {
        assert.equal(snapshot.loading.host, false);
        assert.equal(snapshot.error.host, true);
      } else {
        assert.equal(snapshot.incidents.host[0].id, "Latest incident");
        assert.equal(snapshot.loading.host, false);
        assert.equal(snapshot.error.host, false);
        await page.getByText("Latest incident", { exact: true }).waitFor();
        assert.equal(
          await page.getByText("Obsolete incident", { exact: true }).count(),
          0,
        );
      }
      assert.equal(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
        true,
      );
      await page.screenshot({
        path: output + "/" + width + "-" + scenario + ".png",
      });
      await page.close();
      cases++;
    }
  }
  for (const width of [1440, 390]) {
    const page = await browser.newPage({ viewport: { width, height: 900 } });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    page.on('dialog', dialog => dialog.accept());
    await page.goto('http://127.0.0.1:5198/history-qualification');
    await page.waitForFunction(() => window.snapshot?.().rows === 1);
    await page.getByRole('button', {name:'Refresh range',exact:true}).click();
    await page.waitForFunction(() => window.snapshot().loading && !!window.finishHistory);
    await page.getByRole('button', {name: /Clear All History/i}).click();
    await page.waitForFunction(() => window.snapshot().rows === 0 && !window.snapshot().loading);
    await page.evaluate(() => window.finishHistory());
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(resolve)));
    assert.deepEqual(await page.evaluate(() => window.snapshot()), {rows:0,loading:false});
    assert.deepEqual(errors, []);
    await page.screenshot({path: output + '/' + width + '-history-clear.png'});
    await page.close();
    cases++;
  }
  console.log(
    JSON.stringify({
      result: "passed",
      cases,
      viewports: [1440, 390],
      scope:
        "Real Chromium, real hook and incident panel, scripted API and fixture lifecycle controls; not installed application or delivery acceptance",
    }),
  );
} finally {
  await browser?.close();
  await server.close();
}
