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
    const updateField = vi.fn((field: string, value: string | boolean | number) => {
      if (field === 'setupMode') {
        setSetupMode(value as NodeModalSetupMode);
      }
    });

    const state = {
      agentCommandError: () => null,
      agentInstallCommand: () => '',
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

    expect(screen.getByText('Credentialed command ready')).toBeInTheDocument();
    expect(
      screen.getByText(/Use Copy command to place the runnable command on your clipboard/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/one-time setup token is intentionally not shown/i),
    ).toBeInTheDocument();
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
    expect(screen.getAllByText(/Pulse command execution/i).length).toBeGreaterThan(0);
    expect(
      screen.getByText('PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('PULSE_PROXMOX_GUEST_DOCKER_INVENTORY_VMIDS=101,102'),
    ).toBeInTheDocument();
    expect(screen.getByText(/bounded/i)).toHaveTextContent(/pct exec/);
  });

  it('labels manual setup as the advanced token escape hatch', () => {
    renderSetupGuide('pve');

    fireEvent.click(screen.getByRole('button', { name: /^Manual Token Setup$/i }));
    expect(screen.getByText('Manual API token')).toBeInTheDocument();
    expect(screen.getByText(/Advanced manual token setup/i)).toBeInTheDocument();
    expect(screen.getByText(/Advanced escape hatch: use this only when/i)).toBeInTheDocument();
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
    ['pve' as const, /pveum user add pulse-monitor@pve/i],
    ['pbs' as const, /proxmox-backup-manager user create pulse-monitor@pbs/i],
  ])(
    'blocks %s manual token command copies until an import plan is approved',
    (nodeType, command) => {
      const reason = 'Approve the import plan before generating setup commands.';
      const { copyCommand } = renderSetupGuide(nodeType, {
        setupHandoffDisabled: true,
        setupHandoffDisabledReason: reason,
      });

      fireEvent.click(screen.getByRole('button', { name: /^Manual Token Setup$/i }));

      expect(screen.getByText(command)).toBeInTheDocument();
      const disabledCopyButtons = screen.getAllByTitle(reason);
      expect(disabledCopyButtons.length).toBeGreaterThanOrEqual(4);
      for (const button of disabledCopyButtons) {
        expect(button).toBeDisabled();
      }

      fireEvent.click(disabledCopyButtons[0]);

      expect(copyCommand).not.toHaveBeenCalled();
    },
  );
});
