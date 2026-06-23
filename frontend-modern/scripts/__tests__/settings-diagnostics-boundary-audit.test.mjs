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
    [path.join(repoRoot, 'internal', 'api', 'router_routes_licensing.go')]:
      'package api\n\nfunc (r *Router) registerOrgLicenseRoutesGroup() {}\n',
    [path.join(repoRoot, 'internal', 'api', 'system_settings.go')]:
      'package api\n\ntype SystemSettingsHandler struct {}\n',
    [path.join(repoRoot, 'internal', 'config', 'config.go')]:
      'package config\n\ntype Config struct {}\n',
    [path.join(repoRoot, 'internal', 'config', 'persistence.go')]:
      'package config\n\ntype SystemSettings struct {}\n',
    [path.join(repoRoot, 'pkg', 'server', 'server.go')]: 'package server\n\nfunc Run() {}\n',
    [path.join(root, 'src', 'components', 'Settings', 'DiagnosticsResultsPanel.tsx')]:
      'export function DiagnosticsResultsPanel() { return null; }\n',
    [path.join(root, 'src', 'components', 'Settings', 'diagnosticsModel.ts')]:
      'export interface DiagnosticsInfo { version: string; }\n',
    [path.join(root, 'src', 'stores', 'systemSettings.ts')]:
      'export function updateSystemSettingsFromResponse() {}\n',
    [path.join(root, 'src', 'types', 'config.ts')]:
      'export interface SystemConfig { autoUpdateEnabled: boolean; }\n',
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

  it('reports legacy MCP vocabulary on native Assistant diagnostics', () => {
    const { root, repoRoot } = makeFixture(({ root, repoRoot }) => ({
      [path.join(repoRoot, 'internal', 'api', 'diagnostics.go')]: `
package api

type AIChatDiagnostic struct {
  MCPConnected bool
}
`,
      [path.join(root, 'src', 'components', 'Settings', 'DiagnosticsResultsPanel.tsx')]:
        "export const label = 'MCP Connection';\n",
      [path.join(root, 'src', 'components', 'Settings', 'diagnosticsModel.ts')]: `
export interface AIChatDiagnostic {
  mcpConnected: boolean;
}
`,
    }));

    const findings = collectUserDiagnosticsInternalAnalyticsFindings({ root, repoRoot });

    expect(findings.map((finding) => finding.rule)).toEqual([
      'canonical-settings/no-mcp-vocabulary-in-assistant-diagnostics-api',
      'canonical-settings/no-mcp-vocabulary-in-assistant-diagnostics-ui',
      'canonical-settings/no-mcp-vocabulary-in-assistant-diagnostics-types',
    ]);
  });

  it('reports commercial analytics routes, stores, and settings in product runtime files', () => {
    const { root, repoRoot } = makeFixture(({ root, repoRoot }) => ({
      [path.join(repoRoot, 'internal', 'api', 'router_routes_licensing.go')]: `
package api

func (r *Router) registerOrgLicenseRoutesGroup() {
  r.mux.HandleFunc("POST /api/upgrade-metrics/events", NewConversionHandlers().HandleRecordEvent)
  SetEnforcementConversionRecorder(nil, nil)
}
`,
      [path.join(repoRoot, 'pkg', 'server', 'server.go')]: `
package server

func Run() {
  _ = "upgrade_metrics.db"
  _ = NewConversionStore
}
`,
      [path.join(repoRoot, 'internal', 'config', 'config.go')]: `
package config

type Config struct {
  DisableLocalUpgradeMetrics bool
}
`,
      [path.join(root, 'src', 'types', 'config.ts')]: `
export interface SystemConfig {
  disableLocalUpgradeMetrics?: boolean;
}
`,
    }));

    const findings = collectUserDiagnosticsInternalAnalyticsFindings({ root, repoRoot });

    expect(findings.map((finding) => finding.rule)).toEqual([
      'canonical-settings/no-product-commercial-analytics-routes',
      'canonical-settings/no-product-commercial-analytics-routes',
      'canonical-settings/no-product-commercial-analytics-routes',
      'canonical-settings/no-product-commercial-analytics-store',
      'canonical-settings/no-product-commercial-analytics-store',
      'canonical-settings/no-product-commercial-analytics-setting',
      'canonical-settings/no-product-commercial-analytics-setting',
    ]);
  });

  it('reports retired commercial analytics packages in compiled product licensing code', () => {
    const { root, repoRoot } = makeFixture(({ repoRoot }) => ({
      [path.join(repoRoot, 'pkg', 'licensing', 'conversion_events.go')]: 'package licensing\n',
      [path.join(repoRoot, 'pkg', 'licensing', 'metering', 'event.go')]: 'package metering\n',
      [path.join(repoRoot, 'internal', 'license', 'conversion', 'events.go')]:
        'package conversion\n',
      [path.join(repoRoot, 'internal', 'license', 'metering', 'event.go')]: 'package metering\n',
    }));

    const findings = collectUserDiagnosticsInternalAnalyticsFindings({ root, repoRoot });

    expect(findings.map((finding) => finding.rule)).toEqual(
      Array(4).fill('canonical-settings/no-product-commercial-analytics-package'),
    );
    expect(findings.map((finding) => finding.file)).toEqual([
      '../pkg/licensing/conversion_events.go',
      '../pkg/licensing/metering/event.go',
      '../internal/license/conversion/events.go',
      '../internal/license/metering/event.go',
    ]);
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
