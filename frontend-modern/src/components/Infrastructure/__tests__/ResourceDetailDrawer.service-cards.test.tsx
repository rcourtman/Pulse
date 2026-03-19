import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

const wsState = vi.hoisted(() => ({ pmg: [] as any[] }));
const reconnectSpy = vi.hoisted(() => vi.fn());

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    state: wsState,
    connected: () => true,
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: reconnectSpy,
  }),
  useDarkMode: () => () => false,
}));

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab" />,
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getFacetBundle: vi.fn().mockResolvedValue({
      capabilities: [],
      relationships: [],
      recentChanges: [],
    }),
  },
}));

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'host-1',
  displayName: 'host-1',
  platformId: 'host-1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['proxmox'] },
  ...overrides,
});

describe('ResourceDetailDrawer service cards', () => {
  it('renders PBS card with compact summary and job breakdown section', () => {
    const resource = baseResource({
      id: 'pbs-1',
      type: 'pbs',
      name: 'pbs-main',
      displayName: 'PBS Main',
      platformId: 'pbs-main',
      platformType: 'proxmox-pbs',
      platformData: {
        sources: ['pbs'],
        pbs: {
          hostname: 'pbs-main.local',
          connectionHealth: 'online',
          datastoreCount: 2,
          backupJobCount: 3,
        },
      },
    });

    const { getByText, getAllByText, getByRole, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Service details')).toBeInTheDocument();
    expect(getByText('2 datastores · 3 jobs')).toBeInTheDocument();
    expect(queryByText('PBS Service')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show service details' }));
    expect(getByText('PBS Service')).toBeInTheDocument();
    expect(getAllByText('pbs-main.local').length).toBeGreaterThan(0);
    expect(getByText('Datastores')).toBeInTheDocument();
    expect(getByText('Total Jobs')).toBeInTheDocument();
    expect(getByText('Job breakdown')).toBeInTheDocument();
    expect(getByRole('link', { name: /open pbs backups/i })).toHaveAttribute(
      'href',
      '/recovery?provider=proxmox-pbs&mode=remote',
    );
  });

  it('renders PMG card with compact summary and queue/mail breakdown sections', async () => {
    const resource = baseResource({
      id: 'pmg-1',
      type: 'pmg',
      name: 'pmg-main',
      displayName: 'PMG Main',
      platformId: 'pmg-main',
      platformType: 'proxmox-pmg',
      platformData: {
        sources: ['pmg'],
        pmg: {
          hostname: 'pmg-main.local',
          connectionHealth: 'online',
          nodeCount: 1,
          queueTotal: 519,
          queueDeferred: 12,
          queueHold: 4,
          mailCountTotal: 1200,
          spamIn: 32,
          virusIn: 2,
        },
      },
    });

    const { getByText, getAllByText, getByRole, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Service details')).toBeInTheDocument();
    expect(getByText('519 queue total · 16 backlog')).toBeInTheDocument();
    expect(queryByText('Mail Gateway')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show service details' }));
    await waitFor(() => {
      expect(getByText('Mail Gateway')).toBeInTheDocument();
    });
    expect(getAllByText('pmg-main.local').length).toBeGreaterThan(0);
    expect(getByText('Queue Total')).toBeInTheDocument();
    expect(getByText('Backlog')).toBeInTheDocument();
    expect(getByText('Queue breakdown')).toBeInTheDocument();
    expect(getByText('Mail processing')).toBeInTheDocument();
    expect(getByRole('link', { name: /open pmg thresholds/i })).toHaveAttribute(
      'href',
      '/alerts/thresholds/mail-gateway',
    );
  });
});
