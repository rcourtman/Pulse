import { describe, expect, it } from 'vitest';

import {
  trackAgentFirstConnected,
  trackAgentInstallCommandCopied,
  trackAgentInstallProfileSelected,
  trackAgentInstallTokenGenerated,
  trackCheckoutClicked,
  trackPaywallViewed,
  trackPricingViewed,
  trackUpgradeClicked,
  trackUpgradeMetricEvent,
} from '@/utils/upgradeMetrics';
import upgradeMetricsSource from '@/utils/upgradeMetrics.ts?raw';

describe('upgradeMetrics customer frontend boundary', () => {
  it('does not carry browser-side ingestion plumbing for maintainer analytics', () => {
    expect(upgradeMetricsSource).not.toContain('/api/upgrade-metrics/events');
    expect(upgradeMetricsSource).not.toContain('@/utils/apiClient');
    expect(upgradeMetricsSource).not.toContain('apiFetch(');
    expect(upgradeMetricsSource).not.toContain('fetch(');
    expect(upgradeMetricsSource).not.toContain('sendBeacon');
  });

  it('keeps compatibility wrappers callable without emitting product analytics', () => {
    trackPaywallViewed('rbac', 'settings_roles_panel');
    trackPricingViewed('settings_self_hosted_billing_plan', 'self_hosted_plan');
    trackUpgradeClicked('settings_reporting_panel', 'reporting');
    trackCheckoutClicked('settings_self_hosted_billing_compare_prompt', 'self_hosted_plan');
    trackAgentInstallTokenGenerated('settings_unified_agents', 'manual');
    trackAgentInstallCommandCopied('settings_unified_agents', 'linux:auto:install');
    trackAgentInstallProfileSelected('settings_unified_agents', 'proxmox-pbs');
    trackAgentFirstConnected('setup_wizard_complete', 'first_agent');
    trackUpgradeMetricEvent({
      type: 'agent_install_command_copied',
      surface: 'settings_unified_agents',
      capability: 'linux:auto:install:custom',
      idempotencyKey: 'custom-1',
    });
  });
});
