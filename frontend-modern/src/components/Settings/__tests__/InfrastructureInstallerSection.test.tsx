import type { JSX } from 'solid-js';
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
  enableFailureLogs: () => false,
  setEnableFailureLogs: vi.fn(),
  isGeneratingToken: () => false,
};

vi.mock('@solidjs/router', () => ({
  A: (props: JSX.AnchorHTMLAttributes<HTMLAnchorElement>) => <a {...props} />,
}));

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

  it('opens Compose applications and standalone containers from a connected Docker install', () => {
    const lookup = stateOverrides.lookupResult;
    stateOverrides.lookupResult = () => ({
      ...lookupResult,
      agent: { ...lookupResult.agent, connected: true },
    });
    try {
      const view = render(() => <InfrastructureInstallerSection focus="docker" />);
      expect(screen.getByRole('link', { name: 'Open Compose applications' })).toHaveAttribute(
        'href',
        '/docker/applications',
      );
      expect(screen.getByRole('link', { name: 'Open Docker overview' })).toHaveAttribute(
        'href',
        '/docker/overview',
      );
      expect(
        screen.getByText(/open Service checks to review its published ports/),
      ).toBeInTheDocument();
      view.unmount();
    } finally {
      stateOverrides.lookupResult = lookup;
    }
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
    // On a Proxmox node the same choice lets Pulse start a stopped guest, and
    // nothing more: a person who ticks it must not expect stop or restart.
    expect(checkbox).toHaveAccessibleDescription(
      /On a Proxmox node it also lets Pulse start a stopped guest after an admin approves it\. Stopping or restarting a guest still needs the action runner\./,
    );
    expect(checkbox).toHaveAccessibleDescription(
      /prepares a new token and command.*does not reconfigure an installed agent.*Earlier tokens remain valid.*only when no agent still uses them/,
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

  // A flag nobody is shown is a feature nobody has. The choice sits with the
  // copied commands, in view, and says what leaves the host and how to undo it.
  it('offers failure logs beside the install commands, off by default', () => {
    const view = render(() => <InfrastructureInstallerSection />);
    expect(
      screen.getByRole('button', { name: 'Show advanced connection and install options' }),
    ).toBeInTheDocument();
    const checkbox = screen.getByRole('checkbox', {
      name: 'Send the last log lines of a failed container',
    });
    expect(checkbox).not.toBeChecked();
    expect(checkbox).toHaveAccessibleDescription(
      /crashes, restarts in a loop or fails its health check.*last 30 log lines/,
    );
    expect(checkbox).toHaveAccessibleDescription(
      /masked on the host first, but masking can miss some.*leave this off if your logs hold secrets/,
    );
    expect(checkbox).toHaveAccessibleDescription(/rerun the installer with --disable-failure-logs/);

    fireEvent.click(checkbox);
    expect(stateOverrides.setEnableFailureLogs).toHaveBeenCalledWith(true);
    view.unmount();
  });

  // #1775: operators should be able to set Connection URL (Agent -> Pulse) and
  // TLS verification before generating the token, not only after minting.
  it('exposes advanced connection and TLS options before a token is generated', () => {
    const prevUnlocked = stateOverrides.commandsUnlocked;
    const prevRequiresToken = stateOverrides.requiresToken;
    const setCustomAgentUrl = vi.fn();
    const setInsecureMode = vi.fn();
    stateOverrides.commandsUnlocked = () => false;
    stateOverrides.requiresToken = () => true;
    stateOverrides.agentUrl = () => 'https://pulse.example.local:7655';
    stateOverrides.selectedAgentUrl = () => 'https://pulse.example.local:7655';
    stateOverrides.customAgentUrl = () => '';
    stateOverrides.setCustomAgentUrl = setCustomAgentUrl;
    stateOverrides.customCaPath = () => '';
    stateOverrides.setCustomCaPath = vi.fn();
    stateOverrides.insecureMode = () => false;
    stateOverrides.setInsecureMode = setInsecureMode;
    stateOverrides.installProfile = () => 'auto';
    stateOverrides.getSelectedInstallProfile = () => ({ description: 'Auto-detect' });
    stateOverrides.handleInstallProfileChange = vi.fn();
    try {
      const view = render(() => <InfrastructureInstallerSection />);
      const toggle = screen.getByRole('button', {
        name: 'Show advanced connection and install options',
      });
      const generateButton = screen.getByRole('button', { name: 'Generate token' });
      expect(
        toggle.compareDocumentPosition(generateButton) & Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();

      fireEvent.click(toggle);

      const urlInput = screen.getByLabelText('Connection URL (Agent → Pulse)');
      fireEvent.input(urlInput, { target: { value: 'https://10.0.0.5:7655' } });
      expect(setCustomAgentUrl).toHaveBeenCalledWith('https://10.0.0.5:7655');

      const tlsCheckbox = screen.getByRole('checkbox', {
        name: /Skip TLS certificate verification/i,
      });
      fireEvent.click(tlsCheckbox);
      expect(setInsecureMode).toHaveBeenCalledWith(true);
      view.unmount();
    } finally {
      stateOverrides.commandsUnlocked = prevUnlocked;
      stateOverrides.requiresToken = prevRequiresToken;
      delete stateOverrides.agentUrl;
      delete stateOverrides.selectedAgentUrl;
      delete stateOverrides.customAgentUrl;
      delete stateOverrides.setCustomAgentUrl;
      delete stateOverrides.customCaPath;
      delete stateOverrides.setCustomCaPath;
      delete stateOverrides.insecureMode;
      delete stateOverrides.setInsecureMode;
      delete stateOverrides.installProfile;
      delete stateOverrides.getSelectedInstallProfile;
      delete stateOverrides.handleInstallProfileChange;
    }
  });

  it('keeps setup handoff compact without exposing the admin token inline and collapses Proxmox LXC Docker notes', () => {
    const prevHandoff = stateOverrides.setupHandoff;
    const prevTokenSource = stateOverrides.latestTokenSource;
    const prevCommandSections = stateOverrides.commandSections;
    stateOverrides.setupHandoff = () => ({
      username: 'admin',
      apiToken: 'admin-super-secret-token-value',
    });
    stateOverrides.latestTokenSource = () => 'setup_handoff';
    stateOverrides.commandSections = () => [
      {
        platform: 'linux',
        title: 'Linux',
        description: 'Run on Linux',
        snippets: [
          { label: 'Install', command: 'curl -fsSL https://pulse.local/install.sh | bash' },
        ],
      },
    ];
    try {
      const view = render(() => <InfrastructureInstallerSection focus="docker" />);
      expect(screen.getByText(/Admin account ready \(admin\)/)).toBeInTheDocument();
      // Admin API token must not appear inline above the scoped agent install token.
      expect(screen.queryByText('admin-super-secret-token-value')).not.toBeInTheDocument();
      expect(screen.queryByText(/transitional command-capable profile/i)).not.toBeInTheDocument();

      const lxcSummary = screen.getByText('Docker inside Proxmox LXCs');
      const lxcDetails = lxcSummary.closest('details');
      expect(lxcDetails).not.toBeNull();
      expect(lxcDetails?.open).toBe(false);

      const copyButton = screen.getByRole('button', { name: 'Copy Install command' });
      expect(copyButton).not.toHaveClass('absolute');
      expect(copyButton).toHaveClass('shrink-0');
      view.unmount();
    } finally {
      stateOverrides.setupHandoff = prevHandoff;
      stateOverrides.latestTokenSource = prevTokenSource;
      stateOverrides.commandSections = prevCommandSections;
    }
  });
});
