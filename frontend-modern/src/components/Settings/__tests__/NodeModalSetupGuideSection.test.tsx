import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { NodeModalNodeType, NodeModalSetupMode } from '@/utils/nodeModalPresentation';
import type { NodeModalState } from '../useNodeModalState';
import { NodeModalSetupGuideSection } from '../NodeModalSetupGuideSection';

const renderSetupGuide = (nodeType: NodeModalNodeType = 'pve') => {
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
      copyCommand: vi.fn(),
      copyProxmoxAgentInstallCommand: vi.fn(),
      copyQuickSetupCommand: vi.fn(),
      downloadProxmoxSetupScript: vi.fn(),
      formData: () => ({ setupMode: setupMode(), host: '' }),
      isAdvancedSetupMode: () => setupMode() === 'manual',
      loadingAgentCommand: () => false,
      quickSetupExpiry: () => null,
      quickSetupExpiryLabel: () => '',
      quickSetupPreviewCommand: () => '',
      quickSetupTokenHint: () => '',
      updateField,
    } as unknown as NodeModalState;

    return <NodeModalSetupGuideSection modalProps={{ nodeType } as any} state={state} />;
  };

  render(() => <Harness />);
};

describe('NodeModalSetupGuideSection', () => {
  afterEach(() => cleanup());

  it('keeps Proxmox API inventory framed as the recommended least-privilege path', () => {
    renderSetupGuide('pve');

    expect(screen.getByText('Source strategy')).toBeInTheDocument();
    expect(screen.getByText('API inventory')).toBeInTheDocument();
    expect(screen.getByText(/Recommended least-privilege path/i)).toBeInTheDocument();
    expect(screen.getByText(/Recommended API inventory path/i)).toBeInTheDocument();
    // Tab renamed from 'API Inventory' (internal term) to 'Connect via API'.
    expect(screen.getByRole('button', { name: /Connect via API/i })).toBeInTheDocument();
    expect(screen.queryByText('Manual API token')).not.toBeInTheDocument();
  });

  it('keeps the root agent path explicit as optional full host telemetry', () => {
    renderSetupGuide('pve');

    fireEvent.click(screen.getByRole('button', { name: /^Host Telemetry Agent$/i }));

    expect(screen.getByText('Host telemetry agent')).toBeInTheDocument();
    expect(screen.getAllByText(/Optional full host telemetry/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/Pulse Agent root service/i).length).toBeGreaterThan(0);
    expect(screen.getByText(/temperatures, SMART, ZFS, Ceph, and mdadm/i)).toBeInTheDocument();
  });

  it('labels manual setup as the advanced token escape hatch', () => {
    renderSetupGuide('pve');

    fireEvent.click(screen.getByRole('button', { name: /^Manual Token Setup$/i }));
    expect(screen.getByText('Manual API token')).toBeInTheDocument();
    expect(screen.getByText(/Advanced manual token setup/i)).toBeInTheDocument();
    expect(screen.getByText(/Advanced escape hatch: use this only when/i)).toBeInTheDocument();
  });

  it('applies the same API-first strategy language to PBS', () => {
    renderSetupGuide('pbs');

    expect(screen.getByText('API inventory')).toBeInTheDocument();
    expect(screen.getByText(/creates the Proxmox Backup Server API token/i)).toBeInTheDocument();
    expect(screen.getAllByText(/without installing a root agent/i).length).toBeGreaterThan(0);
  });
});
