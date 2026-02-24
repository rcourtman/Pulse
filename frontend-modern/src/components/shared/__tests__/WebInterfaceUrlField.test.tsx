import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/api/guestMetadata', () => ({
  GuestMetadataAPI: {
    getMetadata: vi.fn(async () => ({ id: 'guest-1', customUrl: '' })),
    updateMetadata: vi.fn(async () => ({ id: 'guest-1', customUrl: '' })),
  },
}));

vi.mock('@/api/hostMetadata', () => ({
  HostMetadataAPI: {
    getMetadata: vi.fn(async () => ({ id: 'host-1', customUrl: '' })),
    updateMetadata: vi.fn(async () => ({ id: 'host-1', customUrl: '' })),
  },
}));

import { HostMetadataAPI } from '@/api/hostMetadata';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';

describe('WebInterfaceUrlField', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('renders URL controls for a metadata target', async () => {
    render(() => (
      <WebInterfaceUrlField metadataKind="host" metadataId="host-1" targetLabel="host" />
    ));

    expect(await screen.findByText('Web Interface URL')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
  });

  it('saves a host URL through metadata API', async () => {
    render(() => (
      <WebInterfaceUrlField
        metadataKind="host"
        metadataId="host-1"
        targetLabel="host"
        customUrl=""
      />
    ));

    const input = await screen.findByPlaceholderText('https://192.168.1.100:8080');
    fireEvent.input(input, { target: { value: 'https://pve1.local:8006' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => {
      expect(HostMetadataAPI.updateMetadata).toHaveBeenCalledWith('host-1', {
        customUrl: 'https://pve1.local:8006',
      });
    });
  });
});
