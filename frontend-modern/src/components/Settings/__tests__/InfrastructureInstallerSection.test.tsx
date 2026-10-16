import { render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { AgentLookupResponse } from '@/types/api';
import { InfrastructureInstallerSection } from '../InfrastructureInstallerSection';

const START = Date.parse('2026-10-16T22:00:00Z');

const lookupResult: AgentLookupResponse = {
  success: true,
  agent: {
    id: 'agent-1',
    hostname: 'host-1',
    status: 'offline',
    connected: false,
    lastSeen: START - 60_000,
  },
};

// Only the fields the lookup result path reads are meaningful; every other
// state accessor reads as unset so the rest of the panel stays collapsed.
const stateOverrides: Record<string, unknown> = {
  commandsUnlocked: () => true,
  requiresToken: () => false,
  commandSections: () => [],
  getInstallProfileFlags: () => [],
  lookupResult: () => lookupResult,
};

vi.mock('../useInfrastructureOperationsState', () => ({
  useInfrastructureOperationsContext: () =>
    new Proxy(stateOverrides, {
      get: (target, key: string) => (key in target ? target[key] : () => undefined),
    }),
}));

describe('InfrastructureInstallerSection', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('ages the lookup Last seen label on the shared clock', async () => {
    vi.useFakeTimers({ now: START });
    const view = render(() => <InfrastructureInstallerSection />);
    const label = screen.getByText(/^Last seen/);
    expect(label).toHaveTextContent('Last seen 1 min ago');

    await vi.advanceTimersByTimeAsync(2 * 3_600_000);

    expect(screen.getByText(/^Last seen/)).toBe(label);
    expect(label).toHaveTextContent('Last seen 2 hours ago');
    view.unmount();
  });
});
