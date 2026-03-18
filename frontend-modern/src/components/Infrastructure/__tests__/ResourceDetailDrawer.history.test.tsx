import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

const facetBundleMock = vi.hoisted(() => ({
  getFacetBundle: vi.fn(),
}));

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    state: { pmg: [] as any[] },
    connected: () => true,
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: vi.fn(),
  }),
  useDarkMode: () => () => false,
}));

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab" />,
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getFacetBundle: facetBundleMock.getFacetBundle,
  },
}));

class ResizeObserverMock {
  constructor(_callback: ResizeObserverCallback) {}
  observe() {}
  unobserve() {}
  disconnect() {}
}

if (typeof globalThis.ResizeObserver === 'undefined') {
  vi.stubGlobal('ResizeObserver', ResizeObserverMock);
}

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'host-1',
  displayName: 'host-1',
  platformId: 'host-1',
  platformType: 'agent',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['agent'] },
  ...overrides,
});

describe('ResourceDetailDrawer history tab', () => {
  it('surfaces compact facet summary chips in the overview runtime card', async () => {
    facetBundleMock.getFacetBundle.mockResolvedValueOnce({
      capabilities: [
        {
          name: 'restart',
          type: 'common',
          description: 'Restart the resource safely.',
          minimumApprovalLevel: 'admin',
        },
      ],
      relationships: [
        {
          sourceId: 'node:pve-1',
          targetId: 'vm:42',
          type: 'runs_on',
          confidence: 1,
          active: true,
          discoverer: 'proxmox_adapter',
          observedAt: '2026-03-18T12:00:00Z',
          lastSeenAt: '2026-03-18T12:05:00Z',
          metadata: {
            cluster: 'pve-prod',
            hypervisor: 'pve-1',
          },
        },
      ],
      recentChanges: [
        {
          id: 'change-1',
          observedAt: '2026-03-18T12:06:00Z',
          occurredAt: '2026-03-18T12:04:00Z',
          resourceId: 'vm:42',
          kind: 'restart',
          from: 'running',
          to: 'restarting',
          sourceType: 'platform_event',
          sourceAdapter: 'proxmox_adapter',
          confidence: 'high',
          actor: 'agent:oncall-helper',
          relatedResources: ['node:pve-1'],
          reason: 'Routine restart requested',
          metadata: {
            ticket: 'INC-1234',
            policy: 'routine-maintenance',
          },
        },
      ],
      counts: {
        capabilities: 1,
        relationships: 1,
        recentChanges: 3,
      },
    });

    const resource = baseResource({
      id: 'vm:42',
      type: 'vm',
      name: 'vm-42',
      displayName: 'VM 42',
      platformId: 'vm-42',
      platformType: 'proxmox-pve',
      platformData: { sources: ['proxmox'] },
    });

    render(() => <ResourceDetailDrawer resource={resource} />);

    expect(await screen.findByText('Capabilities 1')).toBeInTheDocument();
    expect(screen.getByText('Relationships 1')).toBeInTheDocument();
    expect(screen.getByText('Timeline 3')).toBeInTheDocument();
  });

  it('renders resource capability, relationship, and timeline facets', async () => {
    facetBundleMock.getFacetBundle.mockResolvedValueOnce({
      capabilities: [
        {
          name: 'restart',
          type: 'common',
          description: 'Restart the resource safely.',
          minimumApprovalLevel: 'admin',
          platform: 'proxmox',
          params: [
            {
              name: 'force',
              type: 'boolean',
              required: false,
              isSensitive: false,
              description: 'Force the restart when needed.',
            },
          ],
        },
      ],
      relationships: [
        {
          sourceId: 'node:pve-1',
          targetId: 'vm:42',
          type: 'runs_on',
          confidence: 1,
          active: true,
          discoverer: 'proxmox_adapter',
          observedAt: '2026-03-18T12:00:00Z',
          lastSeenAt: '2026-03-18T12:05:00Z',
          metadata: {
            cluster: 'pve-prod',
            hypervisor: 'pve-1',
          },
        },
      ],
      recentChanges: [
        {
          id: 'change-1',
          observedAt: '2026-03-18T12:06:00Z',
          occurredAt: '2026-03-18T12:04:00Z',
          resourceId: 'vm:42',
          kind: 'restart',
          from: 'running',
          to: 'restarting',
          sourceType: 'platform_event',
          sourceAdapter: 'proxmox_adapter',
          confidence: 'high',
          actor: 'agent:oncall-helper',
          relatedResources: ['node:pve-1'],
          reason: 'Routine restart requested',
          metadata: {
            ticket: 'INC-1234',
            policy: 'routine-maintenance',
          },
        },
      ],
      counts: {
        capabilities: 1,
        relationships: 1,
        recentChanges: 3,
      },
    });

    const resource = baseResource({
      id: 'vm:42',
      type: 'vm',
      name: 'vm-42',
      displayName: 'VM 42',
      platformId: 'vm-42',
      platformType: 'proxmox-pve',
      platformData: { sources: ['proxmox'] },
    });

    render(() => <ResourceDetailDrawer resource={resource} />);

    fireEvent.click(screen.getByRole('button', { name: 'History' }));

    await screen.findByText('Resource History');
    const historyPanel = screen.getByTestId('resource-history-tab');
    const panel = within(historyPanel);
    expect(await panel.findByText('restart')).toBeInTheDocument();
    expect(panel.getAllByText('Capabilities')).toHaveLength(2);
    expect(panel.getAllByText('Relationships')).toHaveLength(2);
    expect(panel.getByText('Timeline')).toBeInTheDocument();
    expect(screen.getByText('Timeline 3')).toBeInTheDocument();
    expect(
      panel.getByRole('link', { name: 'Open source resource node:pve-1 in Infrastructure' }),
    ).toHaveAttribute('href', '/infrastructure?resource=node%3Apve-1');
    expect(
      panel.getByRole('link', { name: 'Open target resource vm:42 in Infrastructure' }),
    ).toHaveAttribute('href', '/infrastructure?resource=vm%3A42');
    expect(
      panel.getByRole('link', { name: 'Open related resource node:pve-1 in Infrastructure' }),
    ).toHaveAttribute('href', '/infrastructure?resource=node%3Apve-1');
    expect(panel.getByText('Routine restart requested')).toBeInTheDocument();
    expect(panel.getByText('Last Seen')).toBeInTheDocument();
    expect(panel.getAllByText('Metadata')).toHaveLength(2);
    expect(panel.getByText(/"hypervisor": "pve-1"/)).toBeInTheDocument();
    expect(panel.getByText(/"ticket": "INC-1234"/)).toBeInTheDocument();
  });
});
