import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { NodesAPI } from '@/api/nodes';
import { ResourceAPI } from '@/api/resources';
import { SecurityAPI } from '@/api/security';
import { copyToClipboard } from '@/utils/clipboard';
import {
  EnableRemoteCommandsDialog,
  clearInFlightEnableTokensForTests,
} from '../EnableRemoteCommandsDialog';
import type { EnableRemoteCommandsTarget } from '../enableRemoteCommandsModel';

vi.mock('@/api/nodes', () => ({
  NodesAPI: {
    createHostAgentInstallToken: vi.fn(),
  },
}));

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    getStatus: vi.fn(),
  },
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getResource: vi.fn(),
  },
}));

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: vi.fn().mockResolvedValue(true),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  },
}));

const unraidTarget: EnableRemoteCommandsTarget = {
  hostResourceId: 'agent:tower-1',
  hostName: 'Tower',
  agentId: 'tower-1',
  hostname: 'tower',
  platform: 'linux',
  hostProfile: 'unraid',
  commandsEnabled: false,
  containerName: 'gluetun',
  actionKind: 'update',
};

describe('EnableRemoteCommandsDialog', () => {
  beforeEach(() => {
    vi.mocked(SecurityAPI.getStatus).mockResolvedValue({
      requiresAuth: true,
      apiTokenConfigured: true,
      agentUrl: 'https://pulse.local',
    } as never);
    vi.mocked(NodesAPI.createHostAgentInstallToken).mockResolvedValue({
      token: 'pulse_test_exec_token_12345',
      record: { id: 'tok-1', name: 'Remote commands (Tower)' } as never,
    });
    vi.mocked(ResourceAPI.getResource).mockResolvedValue({
      id: 'agent:tower-1',
      agent: { commandsEnabled: false },
    } as never);
  });

  afterEach(() => {
    cleanup();
    clearInFlightEnableTokensForTests();
    vi.clearAllMocks();
  });

  it('keeps the shell boilerplate collapsed by default and reveals command and token on toggle', async () => {
    render(() => (
      <EnableRemoteCommandsDialog
        isOpen={true}
        target={unraidTarget}
        commandsEnabled={false}
        onClose={vi.fn()}
      />
    ));

    expect(
      await screen.findByRole('button', { name: 'Generate command for Tower' }),
    ).toBeInTheDocument();
    expect(NodesAPI.createHostAgentInstallToken).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Generate command for Tower' }));

    await waitFor(() =>
      expect(NodesAPI.createHostAgentInstallToken).toHaveBeenCalledWith({
        enableCommands: true,
        name: 'Remote commands (Tower)',
      }),
    );

    expect(
      await screen.findByText('1. Run the enable command in the Unraid terminal or SSH on Tower'),
    ).toBeInTheDocument();

    // Raw shell command stays collapsed until View command is clicked.
    expect(screen.queryByTestId('enable-remote-commands-script')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Copy command' }));
    await waitFor(() => expect(copyToClipboard).toHaveBeenCalledTimes(1));
    const copiedCommand = vi.mocked(copyToClipboard).mock.calls[0][0];
    expect(copiedCommand).toContain('--update --enable-commands');
    expect(copiedCommand).toContain('--agent-id');
    expect(copiedCommand).toContain('tower-1');
    expect(copiedCommand).toContain('--hostname');
    expect(copiedCommand).toContain('tower');
    expect(copiedCommand).not.toContain('pulse_test_exec_token_12345');

    fireEvent.click(screen.getByRole('button', { name: 'View command' }));
    expect(screen.getByTestId('enable-remote-commands-script')).toHaveTextContent(
      '--update --enable-commands',
    );

    // Token stays hidden until revealed.
    expect(screen.queryByTestId('enable-remote-commands-token')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Show token' }));
    expect(screen.getByTestId('enable-remote-commands-token')).toHaveTextContent(
      'pulse_test_exec_token_12345',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Copy token' }));
    await waitFor(() =>
      expect(copyToClipboard).toHaveBeenCalledWith('pulse_test_exec_token_12345'),
    );
    expect(screen.getByTestId('enable-remote-commands-waiting')).toBeInTheDocument();
  });

  it('updates the generated command live when self-signed HTTPS options are enabled', async () => {
    render(() => (
      <EnableRemoteCommandsDialog
        isOpen={true}
        target={unraidTarget}
        commandsEnabled={false}
        onClose={vi.fn()}
      />
    ));

    fireEvent.click(await screen.findByRole('button', { name: 'Generate command for Tower' }));
    await screen.findByTestId('enable-remote-commands-tls-options');

    fireEvent.click(screen.getByRole('button', { name: 'View command' }));
    expect(screen.getByTestId('enable-remote-commands-script').textContent).not.toContain(
      '--insecure',
    );

    fireEvent.click(screen.getByLabelText('Skip TLS certificate verification (--insecure)'));
    expect(screen.getByTestId('enable-remote-commands-script').textContent).toContain('--insecure');
    expect(screen.getByTestId('enable-remote-commands-script').textContent).toContain('-kfsSL');

    fireEvent.input(screen.getByLabelText('Custom CA certificate path (optional)'), {
      target: { value: '/etc/ssl/certs/homelab.crt' },
    });
    expect(screen.getByTestId('enable-remote-commands-script').textContent).toContain('--cacert');
    expect(screen.getByTestId('enable-remote-commands-script').textContent).toContain(
      '/etc/ssl/certs/homelab.crt',
    );
  });

  it('shows the command immediately when tokens are not required and hides HTTPS options on HTTP', async () => {
    vi.mocked(SecurityAPI.getStatus).mockResolvedValue({
      requiresAuth: false,
      apiTokenConfigured: false,
      agentUrl: 'http://pulse.local:7655',
    } as never);

    render(() => (
      <EnableRemoteCommandsDialog
        isOpen={true}
        target={unraidTarget}
        commandsEnabled={false}
        onClose={vi.fn()}
      />
    ));

    expect(await screen.findByTestId('enable-remote-commands-ready')).toBeInTheDocument();
    expect(NodesAPI.createHostAgentInstallToken).not.toHaveBeenCalled();
    expect(screen.queryByRole('button', { name: 'Copy token' })).not.toBeInTheDocument();
    expect(screen.queryByTestId('enable-remote-commands-tls-options')).not.toBeInTheDocument();
  });

  it('flips to ready, fires onEnabled, and continues the original action when commandsEnabled becomes true', async () => {
    const [enabled, setEnabled] = createSignal(false);
    const onClose = vi.fn();
    const onContinue = vi.fn();
    const onEnabled = vi.fn();

    render(() => (
      <EnableRemoteCommandsDialog
        isOpen={true}
        target={unraidTarget}
        commandsEnabled={enabled()}
        onClose={onClose}
        onContinue={onContinue}
        onEnabled={onEnabled}
      />
    ));

    await screen.findByRole('button', { name: 'Generate command for Tower' });
    setEnabled(true);

    expect(await screen.findByTestId('enable-remote-commands-complete')).toBeInTheDocument();
    expect(onEnabled).toHaveBeenCalledTimes(1);
    const continueBtn = screen.getByRole('button', { name: 'Continue to update gluetun' });
    fireEvent.click(continueBtn);

    expect(onContinue).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('generates a PowerShell command for Windows hosts', async () => {
    const windowsTarget: EnableRemoteCommandsTarget = {
      ...unraidTarget,
      hostName: 'WinBox',
      platform: 'windows',
      hostProfile: undefined,
    };

    render(() => (
      <EnableRemoteCommandsDialog
        isOpen={true}
        target={windowsTarget}
        commandsEnabled={false}
        onClose={vi.fn()}
      />
    ));

    fireEvent.click(await screen.findByRole('button', { name: 'Generate command for WinBox' }));

    expect(
      await screen.findByText('Run the enable command in an elevated PowerShell window on WinBox'),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'View command' }));
    const code = screen.getByTestId('enable-remote-commands-script');
    expect(code.textContent).toContain('$env:PULSE_ENABLE_COMMANDS="true"');
    expect(code.textContent).toContain('$env:PULSE_AGENT_ID="tower-1"');
  });

  it('directs least-privilege hosts to Agent Doctor instead of generating --enable-commands', async () => {
    const safeTarget: EnableRemoteCommandsTarget = {
      ...unraidTarget,
      collectionMode: 'typed-helper-summary',
    };

    render(() => (
      <EnableRemoteCommandsDialog
        isOpen={true}
        target={safeTarget}
        commandsEnabled={false}
        onClose={vi.fn()}
      />
    ));

    expect(
      await screen.findByText('Tower uses the least-privilege agent profile.'),
    ).toBeInTheDocument();
    const link = screen.getByRole('link', { name: 'Open Agent Doctor' });
    expect(link).toHaveAttribute(
      'href',
      '/settings/infrastructure/agent-doctor?agents=agent%3Atower-1',
    );
    expect(screen.queryByRole('button', { name: /Generate command/i })).not.toBeInTheDocument();
  });

  it('reuses an in-flight token if closed and reopened for the same host, and clears it once enabled', async () => {
    const [open, setOpen] = createSignal(true);
    const [enabled, setEnabled] = createSignal(false);

    render(() => (
      <EnableRemoteCommandsDialog
        isOpen={open()}
        target={unraidTarget}
        commandsEnabled={enabled()}
        onClose={() => setOpen(false)}
      />
    ));

    fireEvent.click(await screen.findByRole('button', { name: 'Generate command for Tower' }));
    await screen.findByTestId('enable-remote-commands-ready');
    expect(NodesAPI.createHostAgentInstallToken).toHaveBeenCalledTimes(1);

    // Close and reopen: command stays ready with the same token without minting a second one.
    setOpen(false);
    setOpen(true);
    expect(await screen.findByTestId('enable-remote-commands-ready')).toBeInTheDocument();
    expect(NodesAPI.createHostAgentInstallToken).toHaveBeenCalledTimes(1);

    // Once commands turn on, the cached token is purged.
    setEnabled(true);
    await screen.findByTestId('enable-remote-commands-complete');
    setOpen(false);
    setEnabled(false);
    setOpen(true);
    expect(
      await screen.findByRole('button', { name: 'Generate command for Tower' }),
    ).toBeInTheDocument();
  });

  it('hydrates missing host profile and platform when opened with partial metadata', async () => {
    vi.mocked(ResourceAPI.getResource).mockResolvedValueOnce({
      id: 'agent:tower-1',
      type: 'agent',
      name: 'tower',
      displayName: 'Tower',
      agent: {
        agentId: 'tower-1',
        hostname: 'tower',
        platform: 'linux',
        hostProfile: 'unraid',
        commandsEnabled: false,
      },
    } as never);

    render(() => (
      <EnableRemoteCommandsDialog
        isOpen={true}
        target={{
          hostResourceId: 'agent:tower-1',
          hostName: 'this host',
          agentId: 'tower-1',
          containerName: 'gluetun',
          actionKind: 'update',
        }}
        onClose={vi.fn()}
      />
    ));

    expect(
      await screen.findByRole('heading', { name: 'Enable remote commands on Tower' }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Generate command for Tower' }));
    expect(
      await screen.findByText('1. Run the enable command in the Unraid terminal or SSH on Tower'),
    ).toBeInTheDocument();
  });
});
