import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import webInterfaceUrlFieldSource from '@/components/shared/WebInterfaceUrlField.tsx?raw';
import webInterfaceUrlFieldModelSource from '@/components/shared/webInterfaceUrlFieldModel.ts?raw';
import webInterfaceUrlFieldStateSource from '@/components/shared/useWebInterfaceUrlFieldState.ts?raw';

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

  it('keeps the URL field on shell, runtime, and model owners', () => {
    expect(webInterfaceUrlFieldSource).toContain('useWebInterfaceUrlFieldState');
    expect(webInterfaceUrlFieldSource).not.toContain('GuestMetadataAPI.getMetadata');
    expect(webInterfaceUrlFieldSource).not.toContain('AgentMetadataAPI.updateMetadata');
    expect(webInterfaceUrlFieldSource).not.toContain('validateWebInterfaceCustomUrl');
    expect(webInterfaceUrlFieldSource).not.toContain('createSignal');

    expect(webInterfaceUrlFieldStateSource).toContain('GuestMetadataAPI.getMetadata');
    expect(webInterfaceUrlFieldStateSource).toContain('AgentMetadataAPI.updateMetadata');
    expect(webInterfaceUrlFieldStateSource).toContain('createSignal');
    expect(webInterfaceUrlFieldStateSource).toContain('export function useWebInterfaceUrlFieldState');

    expect(webInterfaceUrlFieldModelSource).toContain('validateWebInterfaceCustomUrl');
    expect(webInterfaceUrlFieldModelSource).toContain('getWebInterfaceSuggestedUrlFallback');
    expect(webInterfaceUrlFieldModelSource).toContain('shouldShowWebInterfaceSuggestedUrl');
  });

  it('renders URL controls for a metadata target', async () => {
    render(() => (
      <WebInterfaceUrlField metadataKind="agent" metadataId="host-1" targetLabel="agent" />
    ));

    expect(await screen.findByText('Web Interface URL')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument();
  });

  it('supports embedded rendering with a custom title', async () => {
    const { container } = render(() => (
      <WebInterfaceUrlField
        metadataKind="agent"
        metadataId="host-1"
        targetLabel="agent"
        title="Web interface"
        embedded
      />
    ));

    expect(await screen.findByText('Web interface')).toBeInTheDocument();
    expect(container.querySelector('.shadow-sm')).toBeNull();
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

    const input = await screen.findByPlaceholderText('https://198.51.100.100:8080');
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

    expect(
      await screen.findByText(
        "Add a URL to quickly access this workload's web interface from the dashboard.",
      ),
    ).toBeInTheDocument();
  });

  it('uses the canonical discovery fallback copy for missing suggested urls', async () => {
    render(() => (
      <WebInterfaceUrlField
        metadataKind="guest"
        metadataId="guest-1"
        customUrl=""
        suggestedUrlDiagnostic="No management interface could be inferred."
      />
    ));

    expect(await screen.findByText('No suggested URL found')).toBeInTheDocument();
    expect(screen.getByText('No management interface could be inferred.')).toBeInTheDocument();
  });
});
