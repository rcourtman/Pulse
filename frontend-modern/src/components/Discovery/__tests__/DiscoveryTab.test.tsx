import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/api/discovery', () => {
  const never = new Promise<null>(() => undefined);
  return {
    getDiscovery: vi.fn(() => never),
    getDiscoveryInfo: vi.fn(async () => null),
    triggerDiscovery: vi.fn(),
    updateDiscoveryNotes: vi.fn(),
    formatDiscoveryAge: vi.fn(() => 'just now'),
    getCategoryDisplayName: vi.fn((category: string) => category),
    getConfidenceLevel: vi.fn(() => ({ label: 'Low confidence', color: 'text-gray-500' })),
    getConnectedAgents: vi.fn(async () => ({ count: 0, agents: [] })),
  };
});

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

import { DiscoveryTab } from '@/components/Discovery/DiscoveryTab';

describe('DiscoveryTab', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps run action visible while discovery lookup is still loading', async () => {
    render(() => (
      <DiscoveryTab
        resourceType="host"
        hostId="host-1"
        resourceId="host-1"
        hostname="pve1"
        urlMetadataKind="host"
        urlMetadataId="host-1"
      />
    ));

    expect(await screen.findByRole('button', { name: 'Run Discovery Now' })).toBeInTheDocument();
  });
});
