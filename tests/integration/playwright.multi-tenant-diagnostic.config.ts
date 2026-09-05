import { defineConfig } from '@playwright/test';
import base from './playwright.config';

// Deliberately separate from stable/probation CI. A diagnostic result cannot
// promote this quarantined spec or be substituted for a stable-tier verdict.
if (process.env.PULSE_E2E_TIER?.trim()) {
  throw new Error('Multi-tenant diagnostics cannot run as an E2E tier; unset PULSE_E2E_TIER');
}

export default defineConfig({
  ...base,
  testMatch: ['**/03-multi-tenant.spec.ts'],
  outputDir: process.env.PULSE_E2E_RESULTS_DIR || 'test-results/multi-tenant-diagnostic',
  reporter: [
    ['list'],
    ['html', {
      outputFolder: process.env.PULSE_E2E_REPORT_DIR || 'playwright-report/multi-tenant-diagnostic',
      open: 'never',
    }],
  ],
  projects: base.projects!
    .filter(project => project.name === 'chromium')
    .map(project => ({ ...project, testIgnore: [] })),
});
