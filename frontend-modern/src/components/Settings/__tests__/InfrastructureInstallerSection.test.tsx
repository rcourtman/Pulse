import { fireEvent, render, screen } from '@solidjs/testing-library';
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
  enableCommands: () => false,
  setEnableCommands: vi.fn(),
  isGeneratingToken: () => false,
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

  // #1728: the only way to grant the command access container updates need sat
  // behind "Show advanced connection and install options" under a "legacy"
  // name, so users who wanted updates either missed it or were told not to use
  // it. It is a token-scope choice, so it belongs in the token step.
  it('offers command access in the token step, outside the advanced options', () => {
    const view = render(() => <InfrastructureInstallerSection />);
    const checkbox = screen.getByRole('checkbox', {
      name: 'Allow Pulse to run commands on this host',
    });
    expect(checkbox).not.toBeChecked();
    expect(checkbox).toHaveAccessibleDescription(
      /update, start, stop or restart Docker and Podman containers.*run commands as root/,
    );
    expect(
      screen.getByText(
        'Monitoring only. Pulse cannot run commands or change containers on this host.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/legacy combined/i)).not.toBeInTheDocument();

    fireEvent.click(checkbox);
    expect(stateOverrides.setEnableCommands).toHaveBeenCalledWith(true);
    view.unmount();
  });
});
