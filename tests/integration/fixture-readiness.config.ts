import { defineConfig, devices } from '@playwright/test';
import { preferredBrowserBaseURL } from './tests/runtime-defaults';

// This is admission to the existing suite, not another tier or a replacement
// for its browser assertions. Use the same authenticated readiness helper.
export default defineConfig({
  testDir: './fixture-readiness',
  testMatch: 'default-mock-ready.spec.ts',
  outputDir: 'test-results-readiness',
  workers: 1,
  retries: 0,
  forbidOnly: !!process.env.CI,
  timeout: 360_000,
  globalTimeout: 420_000,
  reporter: [
    ['list'],
    ['html', { outputFolder: 'playwright-report-readiness', open: 'never' }],
    ['junit', { outputFile: 'test-results-readiness/junit.xml' }],
  ],
  use: {
    ...devices['Desktop Chrome'],
    baseURL: preferredBrowserBaseURL(),
    ignoreHTTPSErrors: ['1', 'true', 'yes', 'on'].includes(
      String(process.env.PULSE_E2E_INSECURE_TLS || '').trim().toLowerCase(),
    ),
  },
});
