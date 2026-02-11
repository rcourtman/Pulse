import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiFetchMock } = vi.hoisted(() => ({
  apiFetchMock: vi.fn(() => Promise.resolve(new Response('{}', { status: 200 }))),
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
}));

import {
  trackAgentFirstConnected,
  trackAgentInstallCommandCopied,
  trackAgentInstallProfileSelected,
  trackAgentInstallTokenGenerated,
} from '@/utils/conversionEvents';

function getPayloadForCall(index: number) {
  const [, options] = apiFetchMock.mock.calls[index] as unknown as [string, RequestInit];
  return JSON.parse(String(options.body));
}

describe('conversionEvents unified agent telemetry wrappers', () => {
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
});
