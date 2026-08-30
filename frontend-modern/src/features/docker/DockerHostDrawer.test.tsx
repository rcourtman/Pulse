import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { resetAIRuntimeState, syncAIRuntimeSettings } from '@/stores/aiRuntimeState';
import type { Resource } from '@/types/resource';
import { DockerHostDrawer } from './DockerHostDrawer';

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="docker-host-discovery" />,
}));

vi.mock('@/components/Workloads/GuestDrawerHistory', () => ({
  GuestDrawerHistory: () => <div data-testid="docker-host-history" />,
  GuestDrawerHistoryRangeSelect: () => <select aria-label="History range" />,
}));

const host = (): Resource =>
  ({
    id: 'agent:docker-1',
    name: 'docker-1',
    displayName: 'Docker 1',
    type: 'agent',
    platformId: 'docker-1',
    platformType: 'docker',
    sourceType: 'agent',
    status: 'online',
    lastSeen: Date.now(),
    discoveryTarget: {
      resourceType: 'agent',
      agentId: 'agent:docker-1',
      resourceId: 'agent:docker-1',
      hostname: 'docker-1',
    },
    docker: {
      hostSourceId: 'docker-source-1',
    },
  }) as Resource;

beforeEach(() => {
  resetAIRuntimeState();
});

afterEach(() => {
  cleanup();
  resetAIRuntimeState();
  vi.clearAllMocks();
});

describe('DockerHostDrawer Discovery availability', () => {
  it('collapses from the full shared drawer header surface', async () => {
    const onClose = vi.fn();
    render(() => <DockerHostDrawer host={host()} onClose={onClose} />);

    await fireEvent.click(screen.getByRole('button', { name: 'Collapse docker-1 details' }));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('does not expose Discovery when the feature is disabled', () => {
    syncAIRuntimeSettings({ discovery_enabled: false } as Parameters<
      typeof syncAIRuntimeSettings
    >[0]);

    render(() => <DockerHostDrawer host={host()} />);

    expect(screen.queryByRole('tab', { name: 'Discovery' })).toBeNull();
    expect(screen.queryByTestId('docker-host-discovery')).toBeNull();
  });

  it('exposes Discovery when both the feature and target are available', () => {
    syncAIRuntimeSettings({ discovery_enabled: true } as Parameters<
      typeof syncAIRuntimeSettings
    >[0]);

    render(() => <DockerHostDrawer host={host()} />);

    expect(screen.getByRole('tab', { name: 'Discovery' })).toBeInTheDocument();
    expect(screen.getByTestId('docker-host-discovery')).toBeInTheDocument();
  });
});

describe('DockerHostDrawer typed-helper summary mode', () => {
  it('warns about reduced coverage and hides container update controls', async () => {
    const summaryHost = host();
    if (summaryHost.docker) {
      summaryHost.docker.collectionMode = 'typed-helper-summary';
    }

    render(() => <DockerHostDrawer host={summaryHost} />);

    expect(screen.getByText('Reduced container coverage')).toBeInTheDocument();
    expect(screen.getByText(/typed helper reports container summaries only/i)).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('tab', { name: 'Manage' }));
    expect(screen.queryByTestId('docker-host-management-actions')).toBeNull();
  });
});
