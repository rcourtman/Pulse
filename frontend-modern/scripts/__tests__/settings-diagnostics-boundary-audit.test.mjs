import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

import { afterEach, describe, expect, it } from 'vitest';

import { collectUserDiagnosticsInternalAnalyticsFindings } from '../settings-diagnostics-boundary-audit.mjs';

const tmpRoots = [];

function writeFixture(root, repoRoot, files) {
  const requiredFiles = {
    [path.join(repoRoot, 'internal', 'api', 'diagnostics.go')]:
      'package api\n\ntype DiagnosticsInfo struct {}\n',
    [path.join(root, 'src', 'components', 'Settings', 'DiagnosticsResultsPanel.tsx')]:
      'export function DiagnosticsResultsPanel() { return null; }\n',
    [path.join(root, 'src', 'components', 'Settings', 'diagnosticsModel.ts')]:
      'export interface DiagnosticsInfo { version: string; }\n',
    ...files,
  };

  for (const [filePath, content] of Object.entries(requiredFiles)) {
    fs.mkdirSync(path.dirname(filePath), { recursive: true });
    fs.writeFileSync(filePath, content);
  }
}

function makeFixture(files = {}) {
  const tmpRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'pulse-diagnostics-boundary-'));
  tmpRoots.push(tmpRoot);
  const repoRoot = path.join(tmpRoot, 'repo');
  const root = path.join(repoRoot, 'frontend-modern');
  const fixtureFiles = typeof files === 'function' ? files({ root, repoRoot }) : files;
  writeFixture(root, repoRoot, fixtureFiles);
  return { root, repoRoot };
}

afterEach(() => {
  while (tmpRoots.length > 0) {
    fs.rmSync(tmpRoots.pop(), { force: true, recursive: true });
  }
});

describe('settings diagnostics boundary audit', () => {
  it('reports admin analytics fields on the user diagnostics boundary', () => {
    const { root, repoRoot } = makeFixture(({ root, repoRoot }) => ({
      [path.join(repoRoot, 'internal', 'api', 'diagnostics.go')]:
        'package api\n\ntype CommercialFunnel struct { PricingViews int }\n',
      [path.join(root, 'src', 'components', 'Settings', 'DiagnosticsResultsPanel.tsx')]:
        "export const label = 'Pricing Views';\n",
      [path.join(root, 'src', 'components', 'Settings', 'diagnosticsModel.ts')]:
        'export interface InfrastructureOnboardingDiagnostic { commercialFunnel?: unknown; }\n',
    }));

    const findings = collectUserDiagnosticsInternalAnalyticsFindings({ root, repoRoot });

    expect(findings.map((finding) => finding.rule)).toEqual([
      'canonical-settings/no-internal-analytics-in-diagnostics-api',
      'canonical-settings/no-internal-analytics-in-diagnostics-ui',
      'canonical-settings/no-internal-analytics-diagnostics-types',
      'canonical-settings/no-internal-analytics-diagnostics-types',
    ]);
    expect(findings[0]).toMatchObject({
      file: '../internal/api/diagnostics.go',
      line: 3,
    });
  });

  it('allows the diagnostics model to strip stale internal analytics keys defensively', () => {
    const { root, repoRoot } = makeFixture(({ root }) => ({
      [path.join(root, 'src', 'components', 'Settings', 'diagnosticsModel.ts')]: `
const INTERNAL_ANALYTICS_DIAGNOSTICS_FIELDS = ['commercialFunnel', 'infrastructureOnboarding'];

export function stripInternalAnalyticsDiagnosticsFields(payload) {
  for (const field of INTERNAL_ANALYTICS_DIAGNOSTICS_FIELDS) {
    delete payload[field];
  }
  return payload;
}
`,
    }));

    expect(collectUserDiagnosticsInternalAnalyticsFindings({ root, repoRoot })).toEqual([]);
  });

  it('reports production frontend commercial analytics shims', () => {
    const { root, repoRoot } = makeFixture(({ root }) => ({
      [path.join(root, 'src', 'components', 'Settings', 'CommercialProbe.tsx')]: `
import { trackPaywallViewed } from '@/utils/upgradeMetrics';

export function CommercialProbe() {
  const onboardingMetricsTracker = { recordOpened() {} };
  trackPaywallViewed('rbac', 'settings_roles_panel');
  onboardingMetricsTracker.recordOpened();
  return null;
}
`,
    }));

    const findings = collectUserDiagnosticsInternalAnalyticsFindings({ root, repoRoot });

    expect(findings.map((finding) => finding.rule)).toEqual(
      Array(5).fill('canonical-settings/no-product-commercial-analytics-source'),
    );
  });

  it('reports direct production frontend calls to upgrade-metrics ingestion', () => {
    const { root, repoRoot } = makeFixture(({ root }) => ({
      [path.join(root, 'src', 'components', 'Settings', 'CommercialProbe.tsx')]: `
export function CommercialProbe() {
  void fetch('/api/upgrade-metrics/events');
  return null;
}
`,
      [path.join(root, 'src', 'components', 'Settings', '__tests__', 'CommercialProbe.test.tsx')]:
        "expect(source).toContain('/api/upgrade-metrics/events');\n",
    }));

    const findings = collectUserDiagnosticsInternalAnalyticsFindings({ root, repoRoot });

    expect(findings.map((finding) => finding.rule)).toEqual([
      'canonical-settings/no-product-upgrade-metrics-endpoint',
    ]);
  });
});
