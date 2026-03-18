import type { JSX } from 'solid-js';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@solidjs/testing-library';
import { NodeModal } from '../NodeModal';

const { getAgentInstallCommandMock, copyToClipboardMock, successMock, errorMock } = vi.hoisted(
  () => ({
    getAgentInstallCommandMock: vi.fn(),
    copyToClipboardMock: vi.fn(),
    successMock: vi.fn(),
    errorMock: vi.fn(),
  }),
);

vi.mock('@/api/nodes', async () => {
  const actual = await vi.importActual<typeof import('@/api/nodes')>('@/api/nodes');
  return {
    ...actual,
    NodesAPI: {
      ...actual.NodesAPI,
      getAgentInstallCommand: getAgentInstallCommandMock,
    },
  };
});

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: copyToClipboardMock,
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: successMock,
    error: errorMock,
  },
}));

vi.mock('@/components/shared/Dialog', () => ({
  Dialog: (props: { isOpen: boolean; children: JSX.Element }) =>
    props.isOpen ? <div>{props.children}</div> : null,
}));

vi.mock('@/stores/license', () => ({
  licenseStatus: () => 'active',
  startProTrial: vi.fn(),
}));

const renderNodeModal = (nodeType: 'pve' | 'pbs') =>
  render(() => (
    <NodeModal
      isOpen
      nodeType={nodeType}
      onClose={vi.fn()}
      onSave={vi.fn()}
    />
  ));

describe('NodeModal proxmox agent install command', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('surfaces canonical install-command validation errors for PVE agent setup', async () => {
    getAgentInstallCommandMock.mockRejectedValueOnce(new Error('Invalid agent install command response'));

    renderNodeModal('pve');

    fireEvent.click(screen.getByTitle('Copy command'));

    await waitFor(() => {
      expect(getAgentInstallCommandMock).toHaveBeenCalledWith({
        type: 'pve',
        enableProxmox: true,
      });
      expect(screen.getByText('Invalid agent install command response')).toBeInTheDocument();
    });

    expect(errorMock).toHaveBeenCalledWith('Invalid agent install command response');
  });

  it('surfaces canonical install-command validation errors for PBS agent setup', async () => {
    getAgentInstallCommandMock.mockRejectedValueOnce(new Error('Invalid agent install command response'));

    renderNodeModal('pbs');

    fireEvent.click(screen.getByTitle('Copy to clipboard'));

    await waitFor(() => {
      expect(getAgentInstallCommandMock).toHaveBeenCalledWith({
        type: 'pbs',
        enableProxmox: true,
      });
      expect(screen.getByText('Invalid agent install command response')).toBeInTheDocument();
    });

    expect(errorMock).toHaveBeenCalledWith('Invalid agent install command response');
  });

  it('shows a concrete copy failure for Proxmox agent setup', async () => {
    getAgentInstallCommandMock.mockResolvedValueOnce({
      command: 'curl -fsSL https://pulse.example/install.sh | bash',
    });
    copyToClipboardMock.mockResolvedValueOnce(false);

    renderNodeModal('pve');

    fireEvent.click(screen.getByTitle('Copy command'));

    await waitFor(() => {
      expect(screen.getByText('Failed to copy to clipboard')).toBeInTheDocument();
    });

    expect(errorMock).toHaveBeenCalledWith('Failed to copy to clipboard');
    expect(successMock).not.toHaveBeenCalled();
  });
});
