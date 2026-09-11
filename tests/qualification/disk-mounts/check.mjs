import { firefox, chromium } from "@playwright/test";
import { createServer } from "../../../frontend-modern/node_modules/vite/dist/node/index.js";
import assert from "node:assert/strict";
process.chdir("frontend-modern");
const server = await createServer({
  root: ".",
  configFile: "vite.config.ts",
  server: { port: 18791 },
});
await server.listen();
try {
  for (const engine of [firefox, chromium]) {
    const browser = await engine.launch();
    try {
      for (const width of [600, 1200])
      for (const theme of ["light", "dark"])
        for (const count of [2, 24]) {
          const page = await browser.newPage({
            viewport: { width, height: 500 },
          });
          await page.goto(
            `http://127.0.0.1:18791/qualification/disks/?theme=${theme}&count=${count}`,
          );
          const last = page.getByTitle(`/mnt/disk-${count - 1}`, {
            exact: true,
          });
          await last.waitFor();
          const geometry = await last.evaluate((el) => {
            const list = el.parentElement.parentElement.parentElement;
            const style = getComputedStyle(list);
            return {
              height: list.clientHeight,
              scrollHeight: list.scrollHeight,
              overflow: style.overflowY,
              maxHeight: style.maxHeight,
            };
          });
          console.log(
            JSON.stringify({
              browser: browser.version(),
              theme,
              width,
              count,
              ...geometry,
            }),
          );
          assert.equal(
            geometry.height,
            geometry.scrollHeight,
            "mounts must not be clipped inside a nested scroller",
          );
          await page.getByRole("button", { name: "Before disks" }).focus();
          await page.keyboard.press("End");
          await page.waitForTimeout(300);
          await page.keyboard.press("Tab");
          assert.equal(
            await page
              .getByRole("button", { name: "After disks" })
              .evaluate((el) => el === document.activeElement),
            true,
          );
          const box = await last.boundingBox();
          assert.ok(
            box && box.y >= 0 && box.y + box.height <= 500,
            "last mount reachable through outer scrolling",
          );
          if (process.env.DISK_SCREENSHOT_DIR && count === 24) {
            await page.screenshot({path: `${process.env.DISK_SCREENSHOT_DIR}/${engine.name()}-${theme}-${width}.png`, fullPage:true});
          }
          await page.close();
        }
    } finally {
      await browser.close();
    }
  }
} finally {
  await server.close();
}
