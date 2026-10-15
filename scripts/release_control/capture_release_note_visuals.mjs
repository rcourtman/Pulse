#!/usr/bin/env node

import fs from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { pathToFileURL } from "node:url";

const username = process.env.PULSE_RELEASE_VISUAL_USERNAME || "admin";
const password = process.env.PULSE_RELEASE_VISUAL_PASSWORD || "adminadminadmin";
const execFileAsync = promisify(execFile);
const imageAttachment =
  /^https:\/\/github\.com\/user-attachments\/assets\/[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i;
const repositoryImage =
  /^https:\/\/raw\.githubusercontent\.com\/rcourtman\/Pulse\/([0-9a-f]{40})\/(?:docs\/images|frontend-modern\/browser-verification\/screenshots)\/[a-zA-Z0-9_-]+\.(?:png|jpg|jpeg|webp)$/;
const pullRequest =
  /^https:\/\/github\.com\/rcourtman\/Pulse\/pull\/([1-9][0-9]*)$/;
const maxImageBytes = 10 * 1024 * 1024;

function allowedImageDestination(value) {
  const url = new URL(value);
  return (
    url.protocol === "https:" &&
    !url.username &&
    !url.password &&
    !url.port &&
    (url.hostname === "github.com" ||
      url.hostname.endsWith(".githubusercontent.com") ||
      /^github-production-user-asset-[a-z0-9-]+\.s3\.amazonaws\.com$/.test(
        url.hostname,
      ))
  );
}

export async function downloadPrImage(imageURL, fetchImage = fetch) {
  if (!imageAttachment.test(imageURL) && !repositoryImage.test(imageURL))
    throw new Error("Unsupported PR image attachment");
  let destination = imageURL;
  for (let redirect = 0; redirect <= 5; redirect += 1) {
    if (!allowedImageDestination(destination))
      throw new Error("PR image redirect left GitHub asset hosting");
    const response = await fetchImage(destination, {
      redirect: "manual",
      headers: { "User-Agent": "Mozilla/5.0" },
      signal: AbortSignal.timeout(30_000),
      // Never send repository credentials to image hosting or redirects.
    });
    if ([301, 302, 303, 307, 308].includes(response.status)) {
      const location = response.headers.get("location");
      await response.body?.cancel();
      if (!location) throw new Error("PR image redirect has no destination");
      destination = new URL(location, destination).href;
      continue;
    }
    if (!response.ok)
      throw new Error(`PR image download failed (${response.status})`);
    const type = response.headers.get("content-type")?.split(";")[0].trim();
    if (!["image/png", "image/jpeg", "image/webp"].includes(type)) {
      await response.body?.cancel();
      throw new Error("PR screenshot must be a PNG, JPEG or WebP image");
    }
    if (Number(response.headers.get("content-length")) > maxImageBytes) {
      await response.body?.cancel();
      throw new Error("PR screenshot exceeds 10 MiB");
    }
    const chunks = [];
    let size = 0;
    for await (const chunk of response.body) {
      size += chunk.length;
      if (size > maxImageBytes) throw new Error("PR screenshot exceeds 10 MiB");
      chunks.push(chunk);
    }
    if (size === 0) throw new Error("PR screenshot is empty");
    return { type, bytes: Buffer.concat(chunks) };
  }
  throw new Error("PR image exceeded the redirect limit");
}

export async function verifyPrImage(state, comparisonTag, dependencies = {}) {
  const match = state.pull_request_url?.match(pullRequest);
  if (
    !match ||
    (!imageAttachment.test(state.image_url) &&
      !repositoryImage.test(state.image_url))
  )
    throw new Error("Invalid PR screenshot source");
  const apiFetch = dependencies.fetch ?? fetch;
  const git =
    dependencies.git ??
    (async (...args) => (await execFileAsync("git", args)).stdout.trim());
  const api = async (resource) => {
    const token = process.env.GH_TOKEN || process.env.GITHUB_TOKEN;
    const response = await apiFetch(
      `https://api.github.com/repos/rcourtman/Pulse/${resource}`,
      {
        headers: {
          Accept: "application/vnd.github+json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        redirect: "error",
        signal: AbortSignal.timeout(30_000),
      },
    );
    if (!response.ok)
      throw new Error(`Cannot verify screenshot PR (${response.status})`);
    return response.json();
  };
  const pr = await api(`pulls/${match[1]}`);
  if (!pr.merged || !/^[0-9a-f]{40}$/.test(pr.merge_commit_sha ?? "")) {
    throw new Error("Screenshot PR must already be merged");
  }
  // Prove that the source PR belongs to this release range, not a later or
  // previously released PR. The model still judges the image's actual content.
  await git("merge-base", "--is-ancestor", pr.merge_commit_sha, "HEAD");
  const repositorySource = state.image_url.match(repositoryImage);
  if (repositorySource) {
    if (!/^[0-9a-f]{40}$/.test(pr.head?.sha ?? ""))
      throw new Error("Screenshot PR has no source revision");
    await git("merge-base", "--is-ancestor", repositorySource[1], pr.head.sha);
  }
  const range = await git("rev-list", `${comparisonTag}..HEAD`);
  if (!range.split("\n").includes(pr.merge_commit_sha))
    throw new Error("Screenshot PR is outside the release range");
  if ((pr.body ?? "").includes(state.image_url)) return;
  for (let page = 1; page <= 10; page += 1) {
    const comments = await api(
      `issues/${match[1]}/comments?per_page=100&page=${page}`,
    );
    if (
      comments.some((comment) => (comment.body ?? "").includes(state.image_url))
    )
      return;
    if (comments.length < 100) break;
  }
  throw new Error("Screenshot attachment is absent from the source PR");
}

function sameOriginURL(route, baseURL) {
  const base = new URL(baseURL);
  const destination = new URL(route, base);
  if (destination.origin !== base.origin) {
    throw new Error(`capture route escaped the application origin: ${route}`);
  }
  return destination.toString();
}

function locatorFor(page, descriptor) {
  let locator;
  switch (descriptor.kind) {
    case "role":
      locator = page.getByRole(descriptor.role, {
        name: descriptor.name,
        exact: descriptor.exact,
      });
      break;
    case "text":
      locator = page.getByText(descriptor.value, { exact: descriptor.exact });
      break;
    case "label":
      locator = page.getByLabel(descriptor.value, { exact: descriptor.exact });
      break;
    case "testid":
      locator = page.getByTestId(descriptor.value);
      break;
    default:
      throw new Error(`unsupported locator kind: ${descriptor.kind}`);
  }
  return locator.filter({ visible: true }).nth(descriptor.nth || 0);
}

async function authenticate(page, baseURL) {
  await page.goto(new URL("/", baseURL).toString(), {
    waitUntil: "domcontentloaded",
  });
  const usernameInput = page.locator('input[name="username"]');
  await page.waitForFunction(
    () => {
      const login = document.querySelector('input[name="username"]');
      const main = document.getElementById("main");
      return login?.getClientRects().length || main?.getClientRects().length;
    },
    undefined,
    { timeout: 30_000 },
  );
  if (await usernameInput.isVisible()) {
    await usernameInput.fill(username);
    await page.locator('input[name="password"]').fill(password);
    await page.locator('button[type="submit"]').click();
  }
  await page.locator("#main").waitFor({ state: "visible", timeout: 30_000 });
}

export async function captureState(
  browser,
  baseURL,
  capture,
  state,
  suffix,
  outputDirectory,
  comparisonTag,
) {
  const context = await browser.newContext({
    viewport: capture.viewport,
    colorScheme: "dark",
    reducedMotion: "reduce",
    locale: "en-GB",
    timezoneId: "UTC",
  });
  const page = await context.newPage();
  try {
    const outputPath = path.join(
      outputDirectory,
      `release-note-${capture.id}-${suffix}.png`,
    );
    if (state.image_url) {
      await verifyPrImage(state, comparisonTag);
      const image = await downloadPrImage(state.image_url);
      // Decode without scripts or network access and normalize to the PNG
      // sidecar contract used by fresh captures and release verification.
      await page.route("**/*", (route) => route.abort());
      await page.setContent(
        `<html><body style="margin:0"><img alt="Release screenshot" src="data:${image.type};base64,${image.bytes.toString("base64")}"></body></html>`,
      );
      const picture = page.getByRole("img", { name: "Release screenshot" });
      const dimensions = await picture.evaluate(async (element) => {
        await element.decode();
        return { width: element.naturalWidth, height: element.naturalHeight };
      });
      if (
        !dimensions.width ||
        !dimensions.height ||
        dimensions.width > 8192 ||
        dimensions.height > 8192 ||
        dimensions.width * dimensions.height > 20_000_000
      ) {
        throw new Error("PR screenshot dimensions exceed the supported bounds");
      }
      await picture.screenshot({ path: outputPath, animations: "disabled" });
      if ((await fs.stat(outputPath)).size > maxImageBytes) {
        await fs.rm(outputPath);
        throw new Error("Normalized PR screenshot exceeds 10 MiB");
      }
      return;
    }
    await authenticate(page, baseURL);
    const response = await page.goto(sameOriginURL(state.route, baseURL), {
      waitUntil: "domcontentloaded",
    });
    if (!response || !response.ok()) {
      throw new Error(
        `capture route did not load successfully: ${state.route}`,
      );
    }
    for (const step of state.steps || []) {
      const locator = locatorFor(page, step.locator);
      if (step.action === "click") {
        await locator.click({ timeout: 15_000 });
      } else {
        await locator.waitFor({ state: "visible", timeout: 15_000 });
      }
    }
    await locatorFor(page, state.ready).waitFor({
      state: "visible",
      timeout: 20_000,
    });
    await page.waitForTimeout(750);
    await page.screenshot({
      path: outputPath,
      fullPage: false,
      animations: "disabled",
      caret: "hide",
    });
  } finally {
    await context.close();
  }
}

async function main() {
  const [
    ,
    ,
    planPath,
    beforeBaseURL,
    afterBaseURL,
    outputDirectory,
    comparisonTag,
  ] = process.argv;
  if (
    !planPath ||
    !beforeBaseURL ||
    !afterBaseURL ||
    !outputDirectory ||
    !comparisonTag
  ) {
    throw new Error(
      "usage: capture_release_note_visuals.mjs PLAN BEFORE_BASE_URL AFTER_BASE_URL OUTPUT_DIRECTORY COMPARISON_TAG",
    );
  }
  const plan = JSON.parse(await fs.readFile(planPath, "utf8"));
  await fs.mkdir(outputDirectory, { recursive: true });
  const { chromium } = await import("@playwright/test");
  const browser = await chromium.launch({ headless: true });
  try {
    for (const capture of plan.captures) {
      if (capture.before) {
        await captureState(
          browser,
          beforeBaseURL,
          capture,
          capture.before,
          "before",
          outputDirectory,
          comparisonTag,
        );
      }
      await captureState(
        browser,
        afterBaseURL,
        capture,
        capture.after,
        "now",
        outputDirectory,
        comparisonTag,
      );
    }
  } finally {
    await browser.close();
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href)
  await main();
