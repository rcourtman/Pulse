import { createSignal } from 'solid-js';
import { fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
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
  getSelectedInstallProfile: () => ({ description: 'Automatic' }),
  lookupResult: () => lookupResult,
};

vi.mock('../useInfrastructureOperationsState', () => ({
  useInfrastructureOperationsContext: () =>
    new Proxy(stateOverrides, {
      get: (target, key: string) => (key in target ? target[key] : () => undefined),
    }),
}));

describe('InfrastructureInstallerSection', () => {
  beforeEach(() => {
    stateOverrides.selectedAgentUrl = () => 'https://pulse.example.invalid';
    stateOverrides.insecureMode = () => false;
    stateOverrides.customCaPath = () => '';
    stateOverrides.commandSections = () => [];
  });

  it('discloses HTTP transport before opening advanced options, even with skip verification off', () => {
    stateOverrides.selectedAgentUrl = () => 'http://pulse.example.invalid:7655';
    render(() => <InfrastructureInstallerSection />);
    const policy = screen.getByRole('status', { name: 'Installation transport policy' });
    expect(policy).toHaveTextContent('HTTP does not protect this installation');
    expect(policy).toHaveTextContent('credentials');
    expect(policy).toHaveTextContent('not encrypted or authenticated by TLS');
    expect(screen.queryByRole('checkbox', { name: /Skip TLS/ })).not.toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Show advanced connection and install options' }),
    ).toBeVisible();
  });

  it('does not describe an explicit TLS bypass as limited to self-signed certificates or rescued by a CA', () => {
    stateOverrides.insecureMode = () => true;
    stateOverrides.customCaPath = () => '/etc/pulse/trust/ca.pem';
    render(() => <InfrastructureInstallerSection />);
    const policy = screen.getByRole('status', { name: 'Installation transport policy' });
    expect(policy).toHaveTextContent('Certificate identity checks are disabled');
    expect(policy).toHaveTextContent('certificate and hostname');
    expect(policy).toHaveTextContent('not just for self-signed');
    expect(policy).toHaveTextContent('override takes precedence');
    expect(policy).toHaveTextContent('/etc/pulse/trust/ca.pem');
    expect(policy).toHaveTextContent('not a connection test');
  });

  it('keeps command descriptions bound to the current policy across URL, CA and bypass edits', async () => {
    const [url, setUrl] = createSignal('http://pulse.example.invalid:7655');
    const [ca, setCa] = createSignal('');
    const [insecure, setInsecure] = createSignal(false);
    Object.assign(stateOverrides, {
      selectedAgentUrl: url,
      insecureMode: insecure,
      customCaPath: ca,
      setInsecureMode: setInsecure,
      commandSections: () => [
        {
          platform: 'linux',
          title: 'Install on Linux',
          description: 'Fixture',
          snippets: [{ label: 'Install', command: 'fixture only' }],
        },
      ],
    });
    render(() => <InfrastructureInstallerSection />);
    const policy = screen.getByRole('status', { name: 'Installation transport policy' });
    expect(screen.getByRole('button', { name: 'Copy Install command' })).toHaveAttribute(
      'aria-describedby',
      policy.id,
    );
    setUrl('https://pulse.example.invalid');
    expect(policy).toHaveTextContent('HTTPS certificate verification requested');
    expect(policy).not.toHaveTextContent('HTTP does not protect');
    setCa('/etc/pulse/trusted-ca.pem');
    expect(policy).toHaveTextContent('HTTPS with a custom CA requested');
    expect(policy).toHaveTextContent('already-trusted channel');
    await fireEvent.click(
      screen.getByRole('button', { name: 'Show advanced connection and install options' }),
    );
    const checkbox = screen.getByRole('checkbox', {
      name: 'Skip TLS certificate verification (unsafe)',
    });
    expect(checkbox).not.toBeChecked();
    expect(checkbox).toHaveAttribute('aria-describedby', policy.id);
    await fireEvent.click(checkbox);
    expect(policy).toHaveTextContent('override takes precedence');
    await fireEvent.click(
      screen.getByRole('button', { name: 'Hide advanced connection and install options' }),
    );
    expect(policy).toBeVisible();
    expect(within(policy).getByText('Certificate identity checks are disabled')).toBeVisible();
    setInsecure(false);
    setCa('');
    expect(policy).toHaveTextContent('HTTPS certificate verification requested');
    expect(policy).not.toHaveTextContent('override takes precedence');
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it.each(['pulse.example.invalid', 'ftp://pulse.example.invalid'])(
    'does not imply verified HTTPS for %s',
    (url) => {
      stateOverrides.selectedAgentUrl = () => url;
      render(() => <InfrastructureInstallerSection />);
      const policy = screen.getByRole('status', { name: 'Installation transport policy' });
      expect(policy).toHaveTextContent('Check the Pulse URL before installing');
      expect(policy).not.toHaveTextContent('HTTPS certificate verification requested');
    },
  );

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
