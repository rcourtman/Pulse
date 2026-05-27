import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect } from "@playwright/test";
import { ensureAuthenticated, apiRequest } from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const CRAWL_PAGES = [
  // Platform-shaped top-level pages
  { name: "Proxmox", url: "/proxmox" },
  { name: "Proxmox PVE", url: "/proxmox/pve" },
  { name: "Proxmox PBS", url: "/proxmox/pbs" },
  { name: "Proxmox Mail Gateway", url: "/proxmox/mail" },
  { name: "Proxmox Backups", url: "/proxmox/backups" },
  { name: "Proxmox Storage", url: "/proxmox/storage" },
  { name: "Proxmox Replication", url: "/proxmox/replication" },
  { name: "Proxmox Ceph", url: "/proxmox/ceph" },
  { name: "Docker", url: "/docker" },
  { name: "Kubernetes", url: "/kubernetes" },
  { name: "Kubernetes Nodes", url: "/kubernetes/nodes" },
  { name: "Kubernetes Deployments", url: "/kubernetes/deployments" },
  { name: "Kubernetes Config", url: "/kubernetes/config" },
  { name: "TrueNAS", url: "/truenas" },
  { name: "VMware", url: "/vmware" },
  { name: "Standalone (Machines)", url: "/standalone" },

  // Alerts, Patrol
  { name: "Alerts Overview", url: "/alerts/overview" },
  { name: "Alerts History", url: "/alerts/history" },
  { name: "Alerts Rules", url: "/alerts/rules" },
  { name: "Patrol", url: "/patrol" },

  // Settings (Representative Tabs)
  { name: "Settings General", url: "/settings/system-general" },
  { name: "Settings Network", url: "/settings/system-network" },
  { name: "Settings Updates", url: "/settings/system-updates" },
  { name: "Settings Recovery", url: "/settings/system-recovery" },
  { name: "Settings AI", url: "/settings/system-ai" },
  { name: "Settings Infrastructure", url: "/settings/infrastructure" },
  { name: "Settings Security Overview", url: "/settings/security-overview" },
  { name: "Settings API Access", url: "/settings/api" },
  { name: "Settings Authentication", url: "/settings/security-auth" },
  { name: "Settings SSO", url: "/settings/security-sso" },
  { name: "Settings Roles", url: "/settings/security-roles" },
  { name: "Settings Users", url: "/settings/security-users" },
  { name: "Settings Audit Log", url: "/settings/security-audit" },
  { name: "Settings Webhooks", url: "/settings/security-webhooks" },
  { name: "Settings Remote Access", url: "/settings/system-relay" },
];

test.describe("Pulse Visual Crawl & Inspection", () => {
  test.setTimeout(300_000); // 5 minutes

  test("crawls all pages to inspect layout, headings, inputs, tables, and colors", async ({ page }, testInfo) => {
    // 1. Authenticate
    await ensureAuthenticated(page);

    const results: any[] = [];
    const screenshotDir = path.resolve(__dirname, "..", "..", "tmp", "visual-crawl-screenshots");
    fs.mkdirSync(screenshotDir, { recursive: true });

    // Set desktop screen size
    await page.setViewportSize({ width: 1440, height: 900 });

    for (const surface of CRAWL_PAGES) {
      console.log(`\n--- Inspecting Page: ${surface.name} (${surface.url}) ---`);

      // Navigate
      await page.goto(surface.url, { waitUntil: "domcontentloaded" });

      // Wait for page load indicators or give it a brief time to settle
      await page.waitForTimeout(1000);

      // Take a screenshot
      const safeName = surface.name.replace(/[^a-zA-Z0-9]/g, "_").toLowerCase();
      const screenshotPath = path.join(screenshotDir, `${safeName}.png`);
      await page.screenshot({ path: screenshotPath });
      console.log(`[Screenshot Saved] -> ${screenshotPath}`);

      // Extract DOM analysis
      const analysis = await page.evaluate((info) => {
        // A. Heading Analysis
        const h1Elements = Array.from(document.querySelectorAll("h1"));
        const headings = h1Elements.map(el => {
          const rect = el.getBoundingClientRect();
          return {
            text: el.textContent?.replace(/\s+/g, ' ').trim() || "",
            tag: el.tagName.toLowerCase(),
            class: el.getAttribute("class") || "",
            rect: { x: Math.round(rect.x), y: Math.round(rect.y), width: Math.round(rect.width), height: Math.round(rect.height) }
          };
        });

        // B. Search / Filters Analysis
        const inputs = Array.from(document.querySelectorAll("input, select, button")).map(el => {
          const tagName = el.tagName.toLowerCase();
          const type = el.getAttribute("type") || "";
          const placeholder = el.getAttribute("placeholder") || "";
          const ariaLabel = el.getAttribute("aria-label") || "";
          const text = el.textContent?.trim() || "";
          const className = el.getAttribute("class") || "";

          // Match inputs/selects/buttons that act as search or filters
          const isFilterSearch =
            tagName === "input" && (type === "search" || placeholder.toLowerCase().includes("search") || placeholder.toLowerCase().includes("filter") || ariaLabel.toLowerCase().includes("search") || ariaLabel.toLowerCase().includes("filter"))
            || tagName === "select"
            || (tagName === "button" && (ariaLabel.toLowerCase().includes("filter") || text.toLowerCase().includes("filter")));

          if (!isFilterSearch) return null;
          const rect = el.getBoundingClientRect();
          return {
            tag: tagName,
            type,
            placeholder,
            ariaLabel,
            text: text.slice(0, 60),
            class: className,
            rect: { x: Math.round(rect.x), y: Math.round(rect.y), width: Math.round(rect.width), height: Math.round(rect.height) }
          };
        }).filter(Boolean);

        // C. Table Analysis
        const tables = Array.from(document.querySelectorAll("table")).map(t => {
          const headerCells = Array.from(t.querySelectorAll("thead th, thead td")).map(c => c.textContent?.trim() || "");
          const bodyRowCount = t.querySelectorAll("tbody tr").length;
          const rect = t.getBoundingClientRect();
          return {
            headers: headerCells,
            rowCount: bodyRowCount,
            rect: { x: Math.round(rect.x), y: Math.round(rect.y), width: Math.round(rect.width), height: Math.round(rect.height) }
          };
        });

        // D. Color Class Violation Analysis
        // We look for elements having hardcoded raw color classes like "bg-gray-", "text-slate-", "border-zinc-", "bg-white", "bg-black" etc.
        const allElements = Array.from(document.querySelectorAll("*"));
        const rawColorViolations: string[] = [];

        allElements.forEach(el => {
          const classes = el.getAttribute("class") || "";
          // Look for raw Tailwind grey/slate/blue colors or hardcoded white/black background/text combinations
          const classList = classes.split(/\s+/);
          classList.forEach(cls => {
            const isRawColor = /^(bg|text|border)-(gray|slate|zinc|neutral|stone|slate|white|black|red|green|blue|yellow)-\d+$/.test(cls)
              || /^(bg|text|border)-(white|black)$/.test(cls);

            // Exclude common safe things or icons if necessary
            if (isRawColor) {
              const tag = el.tagName.toLowerCase();
              const textContent = el.textContent?.trim().slice(0, 30) || "";
              rawColorViolations.push(`${tag}[text="${textContent}"]: class="${cls}"`);
            }
          });
        });

        return {
          headings,
          inputs,
          tables,
          rawColorViolations: rawColorViolations.slice(0, 15) // Limit output
        };
      }, surface);

      console.log(`Headings detected:`, analysis.headings);
      console.log(`Search / filter elements:`, analysis.inputs);
      console.log(`Tables:`, analysis.tables);
      if (analysis.rawColorViolations.length > 0) {
        console.log(`⚠️ Raw Color Class Violations:`, analysis.rawColorViolations);
      }

      results.push({
        name: surface.name,
        url: surface.url,
        analysis,
        screenshotPath
      });
    }

    // Write final analysis summary report to workspace JSON for review
    const reportsPath = path.resolve(__dirname, "..", "..", "tmp", "visual-crawl-report.json");
    fs.writeFileSync(reportsPath, JSON.stringify(results, null, 2), "utf8");
    console.log(`\n✅ Visual Crawl completed. Detailed report written to ${reportsPath}`);
  });
});
