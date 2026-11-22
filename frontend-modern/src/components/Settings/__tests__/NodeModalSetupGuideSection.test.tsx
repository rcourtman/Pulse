import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { NodeModalNodeType, NodeModalSetupMode } from '@/utils/nodeModalPresentation';
import type { NodeModalState } from '../useNodeModalState';
import { NodeModalSetupGuideSection } from '../NodeModalSetupGuideSection';

const renderSetupGuide = (
  nodeType: NodeModalNodeType = 'pve',
  options: {
    isEditingExistingNode?: boolean;
    setupHandoffDisabled?: boolean;
    setupHandoffDisabledReason?: string;
    quickSetupCommandReady?: boolean;
    quickSetupTokenHint?: string;
  } = {},
) => {
  const copyCommand = vi.fn();
  const copyQuickSetupCommand = vi.fn();
  const Harness = () => {
    const [setupMode, setSetupMode] = createSignal<NodeModalSetupMode>('auto');
    const [agentInstallInsecure, setAgentInstallInsecure] = createSignal(false);
    const [agentInstallEnableCommands, setAgentInstallEnableCommands] = createSignal(false);
    const [agentInstallUrl, setAgentInstallUrl] = createSignal('');
    const updateField = vi.fn((field: string, value: string | boolean | number) => {
      if (field === 'setupMode') {
        setSetupMode(value as NodeModalSetupMode);
      }
    });

    const state = {
      agentCommandError: () => null,
      agentInstallCommand: () => '',
      agentInstallEnableCommands,
      agentInstallInsecure,
      agentInstallUrl,
      copyCommand,
      copyProxmoxAgentInstallCommand: vi.fn(),
      copyQuickSetupCommand,
      downloadProxmoxSetupScript: vi.fn(),
      formData: () => ({ setupMode: setupMode(), host: '' }),
      isAdvancedSetupMode: () => setupMode() === 'manual',
      isEditingExistingNode: () => Boolean(options.isEditingExistingNode),
      loadingAgentCommand: () => false,
      quickSetupExpiry: () => null,
      quickSetupExpiryLabel: () => '',
      quickSetupCommandReady: () => Boolean(options.quickSetupCommandReady),
      quickSetupTokenHint: () => options.quickSetupTokenHint ?? '',
      setAgentInstallEnableCommands,
      setAgentInstallInsecure,
      setAgentInstallUrl,
      updateField,
    } as unknown as NodeModalState;

    return (
      <NodeModalSetupGuideSection
        modalProps={
          {
            nodeType,
            setupHandoffDisabled:
              options.setupHandoffDisabled === undefined
                ? undefined
                : () => Boolean(options.setupHandoffDisabled),
            setupHandoffDisabledReason: options.setupHandoffDisabledReason,
          } as any
        }
        state={state}
      />
    );
  };

  render(() => <Harness />);
  return { copyCommand, copyQuickSetupCommand };
};

describe('NodeModalSetupGuideSection', () => {
  afterEach(() => cleanup());

  it('keeps Proxmox API inventory framed as the recommended least-privilege path', () => {
    renderSetupGuide('pve');

    expect(screen.getByText('Source strategy')).toBeInTheDocument();
    expect(screen.getByText('API inventory')).toBeInTheDocument();
    expect(screen.getByText(/Recommended least-privilege path/i)).toBeInTheDocument();
    expect(screen.getByText(/Recommended API inventory path/i)).toBeInTheDocument();
    expect(screen.getByText(/Docker inside Proxmox LXCs\?/i)).toBeInTheDocument();
    expect(screen.getByText(/API inventory alone does not run/i)).toBeInTheDocument();
    // Tab renamed from 'API Inventory' (internal term) to 'Connect via API'.
    expect(screen.getByRole('button', { name: /Connect via API/i })).toBeInTheDocument();
    expect(screen.queryByText('Manual API token')).not.toBeInTheDocument();
  });

  it('does not present the tokenless setup preview as a runnable command', () => {
    renderSetupGuide('pve', {
      quickSetupCommandReady: true,
      quickSetupTokenHint: 'set…123',
    });

    expect(screen.getByText('Credential-free command ready')).toBeInTheDocument();
    expect(
      screen.getByText(/Use Copy command to place the runnable command on your clipboard/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/one-time setup token is shown separately/i)).toBeInTheDocument();
    expect(screen.queryByText(/curl -fsSL/i)).not.toBeInTheDocument();
    expect(screen.getByText('set…123')).toBeInTheDocument();
  });

  it('keeps the root agent path explicit as optional full host telemetry', () => {
    renderSetupGuide('pve');

    fireEvent.click(screen.getByRole('button', { name: /^Host Telemetry Agent$/i }));

    expect(screen.getByText('Host telemetry agent')).toBeInTheDocument();
    expect(screen.getAllByText(/Optional full host telemetry/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/Pulse Agent root service/i).length).toBeGreaterThan(0);
    expect(screen.getByText(/temperatures, SMART, ZFS, Ceph, and mdadm/i)).toBeInTheDocument();
    expect(screen.getByText(/Docker inside Proxmox LXCs:/i)).toBeInTheDocument();
    expect(
      screen.getAllByText(/does not grant remote\s+command authority/i).length,
    ).toBeGreaterThan(0);
    const commandAccess = screen.getByRole('checkbox', {
      name: /Allow Pulse to run commands on this host/i,
    });
    expect(commandAccess).not.toBeChecked();
    const connectionUrl = screen.getByLabelText('Connection URL (Agent → Pulse)');
    expect(connectionUrl).toHaveValue('');
    const tlsOverride = screen.getByRole('checkbox', {
      name: /Skip TLS certificate verification when downloading the installer/i,
    });
    expect(tlsOverride).not.toBeChecked();

    // All three pre-generation controls appear before the command copy button.
    const copyButton = screen.getByTitle('Copy command');
    expect(
      commandAccess.compareDocumentPosition(copyButton) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      connectionUrl.compareDocumentPosition(copyButton) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      tlsOverride.compareDocumentPosition(copyButton) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    fireEvent.input(connectionUrl, { target: { value: 'https://pulse.lan:7655' } });
    expect(connectionUrl).toHaveValue('https://pulse.lan:7655');

    fireEvent.click(commandAccess);
    expect(commandAccess).toBeChecked();
    expect(screen.getByText(/command-capable Proxmox VE credential/i)).toBeInTheDocument();
    expect(screen.getByText(/Mints a command-capable Pulse credential/i)).toBeInTheDocument();

    fireEvent.click(tlsOverride);
    expect(tlsOverride).toBeChecked();
    expect(screen.getByText('Discover Docker in LXC guests')).toBeInTheDocument();
    expect(
      screen.getByText('PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS=101,102'),
    ).toBeInTheDocument();
    expect(screen.getByText(/bounded/i)).toHaveTextContent(/pct exec/);
  });

  it('offers the same pre-generation controls on the PBS host telemetry agent flow', () => {
    renderSetupGuide('pbs');

    fireEvent.click(screen.getByRole('button', { name: /^Host Telemetry Agent$/i }));

    const commandAccess = screen.getByRole('checkbox', {
      name: /Allow Pulse to run commands on this host/i,
    });
    const connectionUrl = screen.getByLabelText('Connection URL (Agent → Pulse)');
    const tlsOverride = screen.getByRole('checkbox', {
      name: /Skip TLS certificate verification when downloading the installer/i,
    });
    const copyButton = screen.getByTitle('Copy to clipboard');

    expect(commandAccess).not.toBeChecked();
    expect(tlsOverride).not.toBeChecked();
    expect(
      commandAccess.compareDocumentPosition(copyButton) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      connectionUrl.compareDocumentPosition(copyButton) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      tlsOverride.compareDocumentPosition(copyButton) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    fireEvent.click(commandAccess);
    expect(commandAccess).toBeChecked();
    expect(
      screen.getByText(/command-capable Proxmox Backup Server credential/i),
    ).toBeInTheDocument();
  });

  it('shows the scoped-token path before the PVE whole-cluster example', () => {
    renderSetupGuide('pve');

    fireEvent.click(screen.getByRole('button', { name: /^Manual Token Setup$/i }));
    expect(screen.getByText('Manual API token')).toBeInTheDocument();
    const guidance = screen.getByText(/Match user and token permissions/i);
    expect(guidance).toHaveTextContent('Keep privilege separation on');
    expect(guidance).toHaveTextContent(/intended resources only/i);
    const example = screen.getByText('New whole-cluster token example');
    expect(
      guidance.compareDocumentPosition(example) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.getByText(/Skip for existing or scoped tokens/i)).toBeInTheDocument();
    const help = screen.getByRole('link', { name: /Proxmox permissions help/i });
    expect(help).toHaveAttribute('href', '/docs/TROUBLESHOOTING#check-permissions-proxmox');
    expect(help).toHaveAttribute('target', '_blank');
    expect(help).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('copies a separated PVE token recipe with paired monitoring ACLs but no store-wide admin grant', () => {
    const { copyCommand } = renderSetupGuide('pve');
    fireEvent.click(screen.getByRole('button', { name: /^Manual Token Setup$/i }));

    fireEvent.click(screen.getByRole('button', { name: 'Copy token creation command' }));
    expect(copyCommand).toHaveBeenLastCalledWith(
      'pveum user token add pulse-monitor@pve pulse-token --privsep 1',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Copy monitoring ACL commands' }));
    const aclCommands: string = copyCommand.mock.lastCall?.[0];
    for (const role of ['PVEAuditor', 'PulseMonitor']) {
      expect(aclCommands).toContain(`pveum aclmod / -user pulse-monitor@pve -role ${role}\n`);
      expect(aclCommands).toContain(
        `pveum aclmod / -token 'pulse-monitor@pve!pulse-token' -role ${role}\n`,
      );
    }
    expect(aclCommands).not.toContain('PVEDatastoreAdmin');
    expect(screen.getAllByTitle('Copy command')).toHaveLength(3);
    const backups = screen.getByText('Optional backup listings:').parentElement;
    expect(backups).toHaveTextContent('PVEDatastoreAdmin also allows writes');
    expect(backups).toHaveTextContent(
      /limit both the user and token grants to the specific \/storage\/<datastore>/,
    );
    expect(backups).toHaveTextContent('A timeout does not mean access was denied');
  });

  it('surfaces non-destructive repair guidance for existing Proxmox API sources', () => {
    renderSetupGuide('pve', { isEditingExistingNode: true });

    expect(screen.getByText('Existing source repair:')).toBeInTheDocument();
    expect(screen.getByText(/choose Audit\/Repair/i)).toBeInTheDocument();
    expect(screen.getByText(/without rotating the current API token/i)).toBeInTheDocument();
    expect(screen.getByText(/Choose Install\/Configure only when/i)).toBeInTheDocument();
  });

  it('applies the same API-first strategy language to PBS', () => {
    renderSetupGuide('pbs');

    expect(screen.getByText('API inventory')).toBeInTheDocument();
    expect(screen.getByText(/creates the Proxmox Backup Server API token/i)).toBeInTheDocument();
    expect(screen.getAllByText(/without installing a root agent/i).length).toBeGreaterThan(0);
  });

  it('blocks guided setup handoff actions until an import plan is approved', () => {
    const reason = 'Approve the import plan before generating setup commands.';
    const { copyQuickSetupCommand } = renderSetupGuide('pve', {
      setupHandoffDisabled: true,
      setupHandoffDisabledReason: reason,
    });

    expect(screen.getByText(reason)).toBeInTheDocument();
    const copyButton = screen.getAllByTitle(reason)[0];
    expect(copyButton).toBeDisabled();

    fireEvent.click(copyButton);

    expect(copyQuickSetupCommand).not.toHaveBeenCalled();
  });

  it.each([
    ['pve' as const, /pveum user add pulse-monitor@pve/i, 3],
    ['pbs' as const, /proxmox-backup-manager user create pulse-monitor@pbs/i, 4],
  ])(
    'blocks %s manual token command copies until an import plan is approved',
    (nodeType, command, controls) => {
      const reason = 'Approve the import plan before generating setup commands.';
      const { copyCommand } = renderSetupGuide(nodeType, {
        setupHandoffDisabled: true,
        setupHandoffDisabledReason: reason,
      });

      fireEvent.click(screen.getByRole('button', { name: /^Manual Token Setup$/i }));

      expect(screen.getByText(command)).toBeInTheDocument();
      const disabledCopyButtons = screen.getAllByTitle(reason);
      expect(disabledCopyButtons.length).toBe(controls);
      for (const button of disabledCopyButtons) {
        expect(button).toBeDisabled();
      }

      fireEvent.click(disabledCopyButtons[0]);

      expect(copyCommand).not.toHaveBeenCalled();
    },
  );
});
