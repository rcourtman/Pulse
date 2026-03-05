import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: {
    getMetadata: vi.fn(async () => ({ id: 'guest-1', customUrl: '' })),
    updateMetadata: vi.fn(async () => ({ id: 'guest-1', customUrl: '' })),
  },
}));

vi.mock('@/api/agentMetadata', () => ({
  AgentMetadataAPI: {
    getMetadata: vi.fn(async () => ({ id: 'host-1', customUrl: '' })),
    updateMetadata: vi.fn(async () => ({ id: 'host-1', customUrl: '' })),
  },
}));

import { AgentMetadataAPI } from '@/api/agentMetadata';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';

describe('WebInterfaceUrlField', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('renders URL controls for a metadata target', async () => {
    render(() => (
      <WebInterfaceUrlField metadataKind="agent" metadataId="host-1" targetLabel="agent" />
    ));

    expect(await screen.findByText('Web Interface URL')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
  });

  it('saves a host URL through metadata API', async () => {
    render(() => (
      <WebInterfaceUrlField
        metadataKind="agent"
        metadataId="host-1"
        targetLabel="agent"
        customUrl=""
      />
    ));

    const input = await screen.findByPlaceholderText('https://192.168.1.100:8080');
    fireEvent.input(input, { target: { value: 'https://pve1.local:8006' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => {
      expect(AgentMetadataAPI.updateMetadata).toHaveBeenCalledWith('host-1', {
        customUrl: 'https://pve1.local:8006',
      });
    });
  });

  it('defaults guest metadata labels to workload wording', async () => {
    render(() => <WebInterfaceUrlField metadataKind="guest" metadataId="guest-1" customUrl="" />);

    expect(await screen.findByText("Add a URL to quickly access this workload's web interface from the dashboard.")).toBeInTheDocument();
  });
});
