import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { NodeModalNodeType, NodeModalSetupMode } from '@/utils/nodeModalPresentation';
import type { NodeModalState } from '../useNodeModalState';
import { NodeModalSetupGuideSection } from '../NodeModalSetupGuideSection';

const renderSetupGuide = (nodeType: NodeModalNodeType = 'pve') => {
  const Harness = () => {
    const [setupMode, setSetupMode] = createSignal<NodeModalSetupMode>('agent');
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
      isAdvancedSetupMode: () => setupMode() === 'auto' || setupMode() === 'manual',
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

  it('keeps Proxmox Agent install framed as the recommended API plus Agent path', () => {
    renderSetupGuide('pve');

    expect(screen.getByText('Source strategy')).toBeInTheDocument();
    expect(screen.getByText('API + Agent')).toBeInTheDocument();
    expect(screen.getByText(/No token fields are needed in this form/i)).toBeInTheDocument();
    expect(screen.getByText(/No token fields are needed here/i)).toBeInTheDocument();
    expect(screen.queryByText('Manual API token')).not.toBeInTheDocument();
  });

  it('labels Proxmox direct and manual setup as advanced escape hatches', () => {
    renderSetupGuide('pve');

    fireEvent.click(screen.getByRole('button', { name: /^Advanced$/i }));
    expect(screen.getByText('API inventory')).toBeInTheDocument();
    expect(screen.getByText(/Advanced API inventory path/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /^Manual Token Setup$/i }));
    expect(screen.getByText('Manual API token')).toBeInTheDocument();
    expect(screen.getByText(/Advanced manual token setup/i)).toBeInTheDocument();
    expect(screen.getByText(/Advanced escape hatch: use this only when/i)).toBeInTheDocument();
  });

  it('applies the same API plus Agent strategy language to PBS', () => {
    renderSetupGuide('pbs');

    expect(screen.getByText('API + Agent')).toBeInTheDocument();
    expect(screen.getByText(/creates the Proxmox Backup Server API token/i)).toBeInTheDocument();
    expect(screen.getByText(/No token fields are needed here/i)).toBeInTheDocument();
  });
});
