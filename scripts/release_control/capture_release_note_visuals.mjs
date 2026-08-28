#!/usr/bin/env node

import fs from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { chromium } from '@playwright/test';

const [, , planPath, beforeBaseURL, afterBaseURL, outputDirectory] = process.argv;
if (!planPath || !beforeBaseURL || !afterBaseURL || !outputDirectory) {
  throw new Error(
    'usage: capture_release_note_visuals.mjs PLAN BEFORE_BASE_URL AFTER_BASE_URL OUTPUT_DIRECTORY',
  );
}

const plan = JSON.parse(await fs.readFile(planPath, 'utf8'));
await fs.mkdir(outputDirectory, { recursive: true });

const username = process.env.PULSE_RELEASE_VISUAL_USERNAME || 'admin';
const password = process.env.PULSE_RELEASE_VISUAL_PASSWORD || 'adminadminadmin';

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
    case 'role':
      locator = page.getByRole(descriptor.role, {
        name: descriptor.name,
        exact: descriptor.exact,
      });
      break;
    case 'text':
      locator = page.getByText(descriptor.value, { exact: descriptor.exact });
      break;
    case 'label':
      locator = page.getByLabel(descriptor.value, { exact: descriptor.exact });
      break;
    case 'testid':
      locator = page.getByTestId(descriptor.value);
      break;
    default:
      throw new Error(`unsupported locator kind: ${descriptor.kind}`);
  }
  return locator.nth(descriptor.nth || 0);
}

async function authenticate(page, baseURL) {
  await page.goto(new URL('/', baseURL).toString(), { waitUntil: 'domcontentloaded' });
  const usernameInput = page.locator('input[name="username"]');
  if (await usernameInput.isVisible({ timeout: 15_000 }).catch(() => false)) {
    await usernameInput.fill(username);
    await page.locator('input[name="password"]').fill(password);
    await page.locator('button[type="submit"]').click();
  }
  await page
    .locator('input[name="username"]')
    .waitFor({ state: 'hidden', timeout: 30_000 });
}

async function captureState(browser, baseURL, capture, state, suffix) {
  const context = await browser.newContext({
    viewport: capture.viewport,
    colorScheme: 'dark',
    reducedMotion: 'reduce',
    locale: 'en-GB',
    timezoneId: 'UTC',
  });
  const page = await context.newPage();
  try {
    await authenticate(page, baseURL);
    const response = await page.goto(sameOriginURL(state.route, baseURL), {
      waitUntil: 'domcontentloaded',
    });
    if (!response || !response.ok()) {
      throw new Error(`capture route did not load successfully: ${state.route}`);
    }
    for (const step of state.steps || []) {
      const locator = locatorFor(page, step.locator);
      if (step.action === 'click') {
        await locator.click({ timeout: 15_000 });
      } else {
        await locator.waitFor({ state: 'visible', timeout: 15_000 });
      }
    }
    await locatorFor(page, state.ready).waitFor({ state: 'visible', timeout: 20_000 });
    await page.waitForTimeout(750);
    const outputPath = path.join(
      outputDirectory,
      `release-note-${capture.id}-${suffix}.png`,
    );
    await page.screenshot({
      path: outputPath,
      fullPage: false,
      animations: 'disabled',
      caret: 'hide',
    });
  } finally {
    await context.close();
  }
}

const browser = await chromium.launch({ headless: true });
try {
  for (const capture of plan.captures) {
    if (capture.before) {
      await captureState(browser, beforeBaseURL, capture, capture.before, 'before');
    }
    await captureState(browser, afterBaseURL, capture, capture.after, 'now');
  }
} finally {
  await browser.close();
}
