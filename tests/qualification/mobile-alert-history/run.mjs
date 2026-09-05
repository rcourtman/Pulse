// Run from the repository root with pulse-heavy-run -- node <this file>.
import assert from "node:assert/strict";
import { chromium, webkit } from "@playwright/test";
import { createServer } from "../../../frontend-modern/node_modules/vite/dist/node/index.js";
import { fileURLToPath } from "node:url";
const root = fileURLToPath(
  new URL("../../../frontend-modern/", import.meta.url),
);
process.chdir(root); // Tailwind resolves its configuration from the frontend directory.
const server = await createServer({
  root,
  configFile: `${root}/vite.config.ts`,
  server: { host: "127.0.0.1", port: 0, strictPort: false },
});
try {
  await server.listen();
  const address = server.httpServer.address();
  for (const [name, engine] of Object.entries({ chromium, webkit })) {
    const browser = await engine.launch();
    try {
      for (const action of ["Timeline", "Resource"]) {
        for (const removeRow of [false, true]) {
          const page = await browser.newPage({
            viewport: { width: 390, height: 844 },
            isMobile: true,
            hasTouch: true,
          });
          const errors = [];
          page.on("pageerror", (error) => errors.push(error.message));
          await page.goto(
            `http://127.0.0.1:${address.port}/tests/browser/mobile-alert-history.html`,
          );
          const trigger = page.getByRole("button", {
            name: action,
            exact: true,
          });
          await trigger.waitFor();
          await trigger.focus();
          await page.keyboard.press("Enter");
          const dialog = page.getByRole("dialog");
          await dialog.waitFor();
          await dialog
            .getByText("Backup destination unavailable.", { exact: true })
            .first()
            .waitFor();
          assert.equal(
            await dialog.evaluate(
              (el) =>
                getComputedStyle(el.closest("[data-dialog-layer]")).position,
            ),
            "fixed",
            "Production Tailwind styles must be loaded",
          );
          assert.match(
            await dialog.getAttribute("aria-label"),
            /pve-production-01/,
          );
          for (let i = 0; i < 8; i++) {
            await page.keyboard.press("Tab");
            assert.ok(
              await dialog.evaluate((el) =>
                el.contains(document.activeElement),
              ),
              "Tab must stay in dialog",
            );
          }
          if (removeRow) {
            await page.evaluate(() => window.removeHistoryRows());
            assert.equal(await page.locator("article").count(), 0);
          }
          await page.keyboard.press("Escape");
          await dialog.waitFor({ state: "detached" });
          await page.evaluate(
            () =>
              new Promise((resolve) =>
                requestAnimationFrame(() => requestAnimationFrame(resolve)),
              ),
          );
          const target = removeRow
            ? page.getByTestId("alert-history-mobile-list")
            : trigger;
          assert.ok(
            await target.evaluate((el) => el === document.activeElement),
            "Focus must return to surviving trigger or list",
          );
          assert.ok(
            await page.evaluate(
              () => document.documentElement.scrollWidth <= innerWidth + 1,
            ),
            "No horizontal overflow",
          );
          assert.deepEqual(errors, []);
          console.log(
            `PASS ${name}: ${action}, row ${removeRow ? "removed" : "retained"}, focus trap/Escape/return/overflow`,
          );
          await page.close();
        }
      }
    } finally {
      await browser.close();
    }
  }
} finally {
  await server.close();
}
