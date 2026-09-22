// Offline real-browser regression for issue #2119 (Alerts overview card footer).
//
// Mounts the production footer markup with the class returned by
// getAlertOverviewStartedAtClass() and measures whether the "Started" run and
// the adjacent delivery-status run share a vertical baseline. Run with:
//   pulse-worker-browser scripts/check-alert-card-footer.cjs
const path = require("node:path");
const { chromium } = require("playwright");

const ROOT = path.resolve(process.cwd(), "frontend-modern");

const launchOptions = {
  headless: true,
  channel: "chromium",
  args: ["--no-sandbox"],
};

const measure = () =>
  (() => {
    const started = document.querySelector('[data-testid="started"]');
    const status = document.querySelector('[data-testid="status"]');
    if (!started || !status) return null;
    const textRect = (el) => {
      const range = document.createRange();
      range.selectNodeContents(el);
      const rect = range.getBoundingClientRect();
      return { top: rect.top, bottom: rect.bottom };
    };
    const startedText = textRect(started);
    const statusText = textRect(status);
    return {
      startedClass: started.getAttribute("class"),
      statusClass: status.getAttribute("class"),
      startedText,
      statusText,
      baselineDelta: Math.abs(startedText.bottom - statusText.bottom),
      topDelta: Math.abs(startedText.top - statusText.top),
    };
  })();

(async () => {
  process.chdir(ROOT);
  const { createServer } = await import(
    path.join(ROOT, "node_modules", "vite", "dist", "node", "index.js")
  );
  const server = await createServer({
    root: ROOT,
    configFile: path.join(ROOT, "vite.config.ts"),
    server: { host: "127.0.0.1", port: 5198, strictPort: true },
  });
  let browser;
  const errors = [];
  const httpErrors = [];
  try {
    await server.listen();
    browser = await chromium.launch(launchOptions);
    const page = await browser.newPage({
      viewport: { width: 2010, height: 1072 },
    });
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (message.type() !== "error") return;
      if (message.text().includes("Failed to load resource")) return;
      errors.push(`console: ${message.text()}`);
    });
    page.on("response", (response) => {
      if (response.status() >= 400)
        httpErrors.push(`${response.status()} ${response.url()}`);
    });

    await page.goto(
      "http://127.0.0.1:5198/browser-tests/alert-card-footer.html",
    );
    await page.waitForSelector('[data-testid="started"]');
    await page.waitForSelector('[data-testid="status"]');

    const desktop = await page.evaluate(measure);
    if (!desktop) throw new Error("footer runs not rendered");
    await page.screenshot({
      path: path.join(ROOT, "browser-tests", "alert-card-footer-desktop.png"),
    });

    await page.setViewportSize({ width: 480, height: 800 });
    await page.waitForTimeout(150);
    const narrow = await page.evaluate(measure);
    await page.screenshot({
      path: path.join(ROOT, "browser-tests", "alert-card-footer-narrow.png"),
    });

    if (errors.length > 0)
      throw new Error(`page errors: ${errors.join(" | ")}`);

    const worst = Math.max(
      desktop.baselineDelta,
      desktop.topDelta,
      narrow.baselineDelta,
      narrow.topDelta,
    );
    if (worst > 1) {
      throw new Error(
        `Started and status runs are vertically misaligned: desktop ${JSON.stringify(desktop)} ` +
          `narrow ${JSON.stringify(narrow)}`,
      );
    }

    console.log(
      JSON.stringify(
        { result: "passed", desktop, narrow, httpErrors },
        null,
        2,
      ),
    );
  } finally {
    if (browser) await browser.close();
    await server.close();
  }
})().catch((error) => {
  console.error("FAILED:", error && error.stack ? error.stack : error);
  process.exitCode = 1;
});
