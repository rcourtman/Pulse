// Recorded job evidence through the real Docs fetch/renderer/router and shipped Markdown/CSS.
const fs = require("node:fs");
const path = require("node:path");
const http = require("node:http");
const crypto = require("node:crypto");
const assert = require("node:assert/strict");
const { chromium, webkit } = require("playwright");

const root = "/workspace";
const preview = path.join(root, "tmp/doc-command-help/static");
const output = path.join(root, "tmp/backup-job-timing-help");
const hash = (bytes) => crypto.createHash("sha256").update(bytes).digest("hex");
const bindingBytes = fs.readFileSync(path.join(preview, "binding.json"));
const binding = JSON.parse(bindingBytes);
const fragment = "read-backup-job-identity-and-timing";
const result = {
  result: "incomplete",
  source_sha: binding.source_sha,
  binding_sha256: hash(bindingBytes),
  protocol_sha256: hash(fs.readFileSync(__filename)),
  playwright: require("playwright/package.json").version,
  cases: [],
  captures: [],
  errors: [],
  unexpected: [],
  served: [],
  cleanup: {},
};

(async () => {
  let server, browser;
  fs.mkdirSync(output, { recursive: true });
  try {
    assert.equal(
      result.playwright,
      JSON.parse(
        fs.readFileSync(path.join(root, "tests/integration/package-lock.json")),
      ).packages["node_modules/@playwright/test"].version,
    );
    for (const [file, digest] of Object.entries(binding.sources))
      assert.equal(
        hash(fs.readFileSync(path.join(root, "frontend-modern", file))),
        digest,
        file,
      );
    assert.equal(
      binding.sources["browser-tests/backup-job-timing-help.cjs"],
      result.protocol_sha256,
    );
    const assets = new Map();
    for (const [file, digest] of Object.entries(binding.artifacts)) {
      const bytes = fs.readFileSync(path.join(preview, file));
      assert.equal(hash(bytes), digest, file);
      assets.set(file, bytes);
    }
    for (const slug of ["PBS", "TROUBLESHOOTING", "VM_DISK_MONITORING"])
      assert.deepEqual(
        assets.get(`docs/${slug}.md`),
        fs.readFileSync(path.join(root, "docs", `${slug}.md`)),
      );
    server = http.createServer((req, res) => {
      const route = new URL(req.url, "http://fixture.invalid").pathname;
      let file = route.slice(1);
      if (route.startsWith("/docs/") && !route.endsWith(".md"))
        file = "browser-tests/doc-command-help.html";
      if (
        req.method !== "GET" ||
        !assets.has(file) ||
        route.startsWith("/api/")
      ) {
        result.unexpected.push({ method: req.method, route });
        res.writeHead(404);
        res.end();
        return;
      }
      const bytes = assets.get(file);
      result.served.push({ route, file, sha256: hash(bytes) });
      res.writeHead(200, {
        "Content-Type":
          {
            ".md": "text/markdown",
            ".js": "text/javascript",
            ".css": "text/css",
            ".html": "text/html",
          }[path.extname(file)] + "; charset=utf-8",
        "Cache-Control": "no-store",
      });
      res.end(bytes);
    });
    await new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", resolve);
    });
    const origin = `http://127.0.0.1:${server.address().port}`;
    for (const [name, engine, width, dark] of [
      ["desktop", chromium, 1280, false],
      ["narrow", webkit, 320, true],
    ]) {
      browser = await engine.launch(
        engine === chromium
          ? { headless: true, channel: "chromium", args: ["--no-sandbox"] }
          : { headless: true },
      );
      const context = await browser.newContext({
        viewport: { width, height: 1000 },
        isMobile: width < 500,
        hasTouch: width < 500,
        locale: "en-GB",
        colorScheme: dark ? "dark" : "light",
      });
      const page = await context.newPage();
      page.setDefaultTimeout(10000);
      page.on("pageerror", (error) => result.errors.push(String(error)));
      await page.addInitScript((isDark) => {
        document.addEventListener("DOMContentLoaded", () =>
          document.documentElement.classList.toggle("dark", isDark),
        );
      }, dark);
      const activate = async (link) => {
        await link.scrollIntoViewIfNeeded();
        if (width < 500) await link.tap();
        else {
          await link.focus();
          await page.keyboard.press("Enter");
        }
      };
      const capture = async (label, locator) => {
        await locator.evaluate((element) =>
          window.scrollBy(0, element.getBoundingClientRect().top - 50),
        );
        await page.evaluate(
          () =>
            new Promise((resolve) =>
              requestAnimationFrame(() => requestAnimationFrame(resolve)),
            ),
        );
        const geometry = await page.evaluate(() => ({
          viewport: innerWidth,
          page: document.documentElement.scrollWidth,
        }));
        assert.ok(
          geometry.page <= geometry.viewport + 1,
          `${name}: horizontal overflow`,
        );
        const file = `${name}-${label}.png`;
        await page.screenshot({ path: path.join(output, file) });
        result.captures.push({
          file,
          sha256: hash(fs.readFileSync(path.join(output, file))),
          geometry,
        });
      };
      await page.goto(`${origin}/docs/TROUBLESHOOTING#backups-and-storage`);
      const entry = page.getByRole("link", {
        name: "job identity and timing",
        exact: true,
      });
      await entry.waitFor();
      await capture("entry", entry.locator(".."));
      await activate(entry);
      await page.waitForURL(`**/docs/PBS#${fragment}`);
      await page.waitForFunction(
        (id) => document.activeElement?.id === id,
        fragment,
      );
      const heading = page.locator(`#${fragment}`);
      assert.equal(await heading.count(), 1);
      const words = await heading.evaluate((element) => {
        let text = "";
        for (
          let next = element.nextElementSibling;
          next && !/^H[1-4]$/.test(next.tagName);
          next = next.nextElementSibling
        )
          text += " " + next.textContent;
        return text.replace(/\s+/g, " ");
      });
      for (const phrase of [
        "not a complete or permanent native task history",
        "not a globally unique guest identity",
        "date, seconds and timezone",
        "compare the same instant",
        "Unknown · reported:",
        "Unknown is not a live elapsed timer or proof that the job is still running",
        "0s is a recorded interval rounded to seconds, not missing duration",
        "not when the job started, ended or the guest recovered",
        "combine one job’s start with another job’s end",
        "only currently collected evidence",
        "Job IDs can contain private names",
        "does not prove successful backup, guest thaw or filesystem writes",
      ])
        assert.ok(
          words.includes(phrase.replace(/’/g, "'")),
          `${name}: ${phrase}`,
        );
      const table = page
        .getByRole("table")
        .filter({ hasText: "Recorded duration" });
      assert.equal(await table.count(), 1);
      const rows = table.getByRole("row");
      assert.equal(await rows.count(), 6);
      assert.deepEqual(
        await table.getByRole("columnheader").allTextContents(),
        ["Field", "How to compare it"],
      );
      assert.deepEqual(
        await table
          .getByRole("columnheader")
          .evaluateAll((headers) =>
            headers.map((header) => header.getAttribute("scope")),
          ),
        ["col", "col"],
      );
      const fieldChecks = [];
      for (const [index, field] of [
        "Job ID",
        "Started",
        "Reported end",
        "Recorded duration",
        "Last observed",
      ].entries()) {
        const row = rows.nth(index + 1);
        assert.equal(await row.getByRole("cell").first().innerText(), field);
        await row.scrollIntoViewIfNeeded();
        const geometry = await row.evaluate((element) => {
          const rect = element.getBoundingClientRect();
          const container = element.closest("table").parentElement;
          return {
            left: rect.left,
            right: rect.right,
            viewport: innerWidth,
            scroll: container.scrollWidth,
            available: container.clientWidth,
          };
        });
        assert.ok(
          geometry.left >= -1 && geometry.right <= geometry.viewport + 1,
          `${name}: clipped ${field}`,
        );
        assert.ok(
          geometry.scroll <= geometry.available + 1,
          `${name}: hidden horizontal table text`,
        );
        fieldChecks.push({ field, geometry });
      }
      await capture("fields", heading);
      await capture("interval", rows.nth(4));
      const privacy = page
        .locator("article p")
        .filter({ hasText: "Job IDs can contain private names" });
      const safety = page
        .locator("article p")
        .filter({ hasText: "A reported end or duration does not prove" });
      await capture("safety", privacy);
      await activate(
        safety.getByRole("link", { name: "Backup safety", exact: true }),
      );
      await page.waitForURL("**/docs/VM_DISK_MONITORING#backup-safety");
      await page.waitForFunction(
        () => document.activeElement?.id === "backup-safety",
      );
      await page.goBack();
      await page.waitForURL(`**/docs/PBS#${fragment}`);
      // The old uncertainty fragment and its new same-document return both work.
      await page.goto(`${origin}/docs/PBS#latest-backup-job-is-unknown`);
      await page.waitForFunction(
        () => document.activeElement?.id === "latest-backup-job-is-unknown",
      );
      const timing = page.getByRole("link", {
        name: "identity and timing checks",
        exact: true,
      });
      await activate(timing);
      await page.waitForURL(`**#${fragment}`);
      await page.waitForFunction(
        (id) => document.activeElement?.id === id,
        fragment,
      );
      result.cases.push({
        name,
        width,
        height: 1000,
        browser: browser.version(),
        focusedCrossDocumentEntry: true,
        fieldChecks,
        accessibleTableHeaders: true,
        oldUncertaintyFragment: true,
        sameDocumentTimingReturn: true,
        historyReturn: true,
        focusedCrossDocumentSafetyLink: true,
        noPageOverflow: true,
      });
      await context.close();
      await browser.close();
      browser = undefined;
      result.cleanup[name] = true;
    }
    assert.deepEqual(result.errors, []);
    assert.deepEqual(result.unexpected, []);
    result.result = "passed";
  } finally {
    if (browser) await browser.close();
    if (server) await new Promise((resolve) => server.close(resolve));
    result.cleanup.browser = true;
    result.cleanup.server = true;
    fs.writeFileSync(
      path.join(output, "result.json"),
      JSON.stringify(result, null, 2) + "\n",
    );
  }
  console.log(JSON.stringify(result));
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
