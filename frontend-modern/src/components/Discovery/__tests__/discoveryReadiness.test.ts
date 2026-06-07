import { describe, it, expect } from 'vitest';
import { computeDiscoveryReadiness } from '../discoveryReadiness';

const base = {
  discoveryEnabled: true,
  aiProviderConfigured: true,
  commandsEnabled: true as boolean | undefined,
  hasConnectedAgent: true,
};

describe('computeDiscoveryReadiness', () => {
  it('is ready when every prerequisite is met', () => {
    expect(computeDiscoveryReadiness(base)).toEqual({ status: 'ready', ready: true });
  });

  it('reports disabled first, even when other prerequisites are unmet', () => {
    expect(
      computeDiscoveryReadiness({ ...base, discoveryEnabled: false, aiProviderConfigured: false }),
    ).toEqual({ status: 'disabled', ready: false });
  });

  it('reports a missing AI provider before a command-disabled agent', () => {
    expect(
      computeDiscoveryReadiness({
        ...base,
        aiProviderConfigured: false,
        commandsEnabled: false,
      }),
    ).toEqual({ status: 'needs_ai_provider', ready: false });
  });

  it('reports commands disabled when the host agent has them off', () => {
    expect(computeDiscoveryReadiness({ ...base, commandsEnabled: false })).toEqual({
      status: 'needs_commands',
      ready: false,
    });
  });

  it('reports a disconnected agent when commands are on but nothing is connected', () => {
    expect(
      computeDiscoveryReadiness({ ...base, commandsEnabled: true, hasConnectedAgent: false }),
    ).toEqual({ status: 'needs_connected_agent', ready: false });
  });

  it('does not block on commands when their state is unknown', () => {
    expect(
      computeDiscoveryReadiness({ ...base, commandsEnabled: undefined, hasConnectedAgent: false }),
    ).toEqual({ status: 'ready', ready: true });
  });
});
