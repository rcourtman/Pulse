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

vi.mock('@/api/dockerMetadata', () => ({
  DockerMetadataAPI: {
    getMetadata: vi.fn(async () => ({ id: 'docker-host-1:container:container-1', customUrl: '' })),
    updateMetadata: vi.fn(async () => ({
      id: 'docker-host-1:container:container-1',
      customUrl: '',
    })),
  },
}));

vi.mock('@/utils/clipboard', () => ({
  copyToClipboard: vi.fn(async () => true),
}));

import { AgentMetadataAPI } from '@/api/agentMetadata';
import { DockerMetadataAPI } from '@/api/dockerMetadata';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';
import { copyToClipboard } from '@/utils/clipboard';
import { getDiscoveryProvenanceTitle } from '@/utils/discoveryPresentation';
import { RESOURCE_METADATA_CHANGED_EVENT } from '@/utils/resourceMetadataEvents';

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
    expect(webInterfaceUrlFieldStateSource).toContain('DockerMetadataAPI.updateMetadata');
    expect(webInterfaceUrlFieldStateSource).toContain('createSignal');
    expect(webInterfaceUrlFieldStateSource).toContain('copyToClipboard');
    expect(webInterfaceUrlFieldStateSource).toContain(
      'export function useWebInterfaceUrlFieldState',
    );

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

  it('saves a Docker container URL through Docker metadata API', async () => {
    render(() => (
      <WebInterfaceUrlField
        metadataKind="docker"
        metadataId="docker-host-1:container:container-1"
        targetLabel="container"
        customUrl=""
      />
    ));

    const input = await screen.findByPlaceholderText('https://198.51.100.100:8080');
    fireEvent.input(input, { target: { value: 'https://app.internal:9443' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => {
      expect(DockerMetadataAPI.updateMetadata).toHaveBeenCalledWith(
        'docker-host-1:container:container-1',
        {
          customUrl: 'https://app.internal:9443',
        },
      );
    });
  });

  it('broadcasts saved URL changes for same-tab metadata consumers', async () => {
    const handler = vi.fn();
    window.addEventListener(RESOURCE_METADATA_CHANGED_EVENT, handler);

    try {
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
        expect(handler).toHaveBeenCalled();
      });
      const event = handler.mock.calls[0][0] as CustomEvent;
      expect(event.detail).toEqual({
        metadataKind: 'agent',
        metadataId: 'host-1',
        customUrl: 'https://pve1.local:8006',
      });
    } finally {
      window.removeEventListener(RESOURCE_METADATA_CHANGED_EVENT, handler);
    }
  });

  it('defaults guest metadata labels to workload wording', async () => {
    render(() => <WebInterfaceUrlField metadataKind="guest" metadataId="guest-1" customUrl="" />);

    expect(
      await screen.findByText(
        "Add a URL to quickly access this workload's web interface from Pulse.",
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

    expect(await screen.findByText('No suggested URL available')).toBeInTheDocument();
    expect(screen.getByText('No management interface could be inferred.')).toBeInTheDocument();
    expect(screen.getByLabelText(getDiscoveryProvenanceTitle())).toBeInTheDocument();
  });

  it('does not show missing suggested URL diagnostics when a custom URL already exists', async () => {
    render(() => (
      <WebInterfaceUrlField
        metadataKind="guest"
        metadataId="guest-1"
        customUrl="https://198.51.100.100:8080"
        suggestedUrlDiagnostic="No management interface could be inferred."
      />
    ));

    expect(await screen.findByDisplayValue('https://198.51.100.100:8080')).toBeInTheDocument();
    expect(screen.queryByText('No suggested URL available')).toBeNull();
  });

  it('offers discovered URL copy, open, and adopt actions without saving automatically', async () => {
    render(() => (
      <WebInterfaceUrlField
        metadataKind="guest"
        metadataId="guest-1"
        customUrl=""
        suggestedUrl="http://192.0.2.10:8123"
        suggestedUrlReasonText="Detected web port"
      />
    ));

    expect(await screen.findByText('Suggested URL')).toBeInTheDocument();
    expect(screen.getByLabelText(getDiscoveryProvenanceTitle())).toBeInTheDocument();
    expect(screen.getByText('http://192.0.2.10:8123')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open suggested URL' })).toHaveAttribute(
      'href',
      'http://192.0.2.10:8123',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Copy suggested URL' }));
    await waitFor(() => {
      expect(copyToClipboard).toHaveBeenCalledWith('http://192.0.2.10:8123');
    });

    fireEvent.click(screen.getByRole('button', { name: 'Use this' }));
    expect(AgentMetadataAPI.updateMetadata).not.toHaveBeenCalled();
    expect(screen.getByPlaceholderText('https://198.51.100.100:8080')).toHaveValue(
      'http://192.0.2.10:8123',
    );
  });
});
