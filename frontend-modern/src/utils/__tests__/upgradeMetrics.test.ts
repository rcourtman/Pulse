import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiFetchMock } = vi.hoisted(() => ({
  apiFetchMock: vi.fn(() => Promise.resolve(new Response('{}', { status: 200 }))),
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
}));

import {
  trackCheckoutClicked,
  trackPricingViewed,
  trackAgentFirstConnected,
  trackAgentInstallCommandCopied,
  trackAgentInstallProfileSelected,
  trackAgentInstallTokenGenerated,
  trackUpgradeMetricEvent,
} from '@/utils/upgradeMetrics';

function getPayloadForCall(index: number) {
  const [, options] = apiFetchMock.mock.calls[index] as unknown as [string, RequestInit];
  return JSON.parse(String(options.body));
}

describe('upgradeMetrics local-only UX metrics wrappers', () => {
  beforeEach(() => {
    apiFetchMock.mockClear();
  });

  it('sends unified agent onboarding events with expected types', () => {
    trackAgentInstallTokenGenerated('settings_unified_agents', 'manual');
    trackAgentInstallCommandCopied('settings_unified_agents', 'linux:auto:install');
    trackAgentInstallProfileSelected('settings_unified_agents', 'proxmox-pbs');
    trackAgentFirstConnected('setup_wizard_complete', 'first_agent');

    expect(apiFetchMock).toHaveBeenCalledTimes(4);
    expect(getPayloadForCall(0).type).toBe('agent_install_token_generated');
    expect(getPayloadForCall(1).type).toBe('agent_install_command_copied');
    expect(getPayloadForCall(2).type).toBe('agent_install_profile_selected');
    expect(getPayloadForCall(3).type).toBe('agent_first_connected');

    expect(getPayloadForCall(0).surface).toBe('settings_unified_agents');
    expect(getPayloadForCall(2).capability).toBe('proxmox-pbs');
  });

  it('deduplicates repeated identical events within one minute', () => {
    trackAgentInstallCommandCopied('settings_unified_agents', 'linux:auto:install:dedupe');
    trackAgentInstallCommandCopied('settings_unified_agents', 'linux:auto:install:dedupe');

    expect(apiFetchMock).toHaveBeenCalledTimes(1);
  });

  it('honors caller-supplied idempotency keys for distinct same-minute events', () => {
    trackUpgradeMetricEvent({
      type: 'agent_install_command_copied',
      surface: 'settings_unified_agents',
      capability: 'linux:auto:install:custom',
      idempotencyKey: 'custom-1',
    });
    trackUpgradeMetricEvent({
      type: 'agent_install_command_copied',
      surface: 'settings_unified_agents',
      capability: 'linux:auto:install:custom',
      idempotencyKey: 'custom-2',
    });

    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    expect(getPayloadForCall(0).idempotency_key).toBe('custom-1');
    expect(getPayloadForCall(1).idempotency_key).toBe('custom-2');
  });

  it('sends canonical pricing and checkout funnel events for self-hosted billing surfaces', () => {
    trackPricingViewed('settings_self_hosted_billing_plan', 'self_hosted_plan');
    trackCheckoutClicked('settings_self_hosted_billing_compare_prompt', 'self_hosted_plan');

    expect(apiFetchMock).toHaveBeenCalledTimes(2);
    expect(getPayloadForCall(0).type).toBe('pricing_viewed');
    expect(getPayloadForCall(0).surface).toBe('settings_self_hosted_billing_plan');
    expect(getPayloadForCall(0).capability).toBe('self_hosted_plan');
    expect(getPayloadForCall(1).type).toBe('checkout_clicked');
    expect(getPayloadForCall(1).surface).toBe('settings_self_hosted_billing_compare_prompt');
    expect(getPayloadForCall(1).capability).toBe('self_hosted_plan');
  });
});
