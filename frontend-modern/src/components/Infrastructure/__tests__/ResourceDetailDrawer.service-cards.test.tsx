import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, within } from '@solidjs/testing-library';

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

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getResourceIntelligence: vi.fn().mockResolvedValue({
      resource_id: 'resource-1',
      health: {
        score: 92,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Stable',
      },
      recent_changes: [],
      dependencies: [],
      dependents: [],
      correlations: [],
      note_count: 0,
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

    const { getByText, getByRole, getByTestId, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Service details')).toBeInTheDocument();
    expect(getByText('2 datastores · 3 jobs')).toBeInTheDocument();
    expect(queryByText('PBS Service')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show details' }));
    fireEvent.click(getByRole('button', { name: 'Show service details' }));
    const serviceDetails = within(getByTestId('resource-service-details-section'));
    expect(serviceDetails.getByText('PBS')).toBeInTheDocument();
    expect(serviceDetails.queryByText('Connection')).toBeNull();
    expect(serviceDetails.getAllByText('State').length).toBeGreaterThan(0);
    expect(serviceDetails.getAllByText('pbs-main.local').length).toBeGreaterThan(0);
    expect(queryByText('Backup summary')).toBeNull();
    expect(queryByText('Job breakdown')).toBeNull();
    expect(queryByText('Types')).toBeNull();
    expect(queryByText('Show job detail')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show jobs' }));
    expect(getByText('Datastores')).toBeInTheDocument();
    expect(getByText('Jobs')).toBeInTheDocument();
    expect(getByText('Types')).toBeInTheDocument();
    expect(getByRole('link', { name: /open pbs backups/i })).toHaveAttribute(
      'href',
      '/recovery?provider=proxmox-pbs&mode=remote',
    );
  });

  it('renders PMG card with compact summary and queue/mail breakdown sections', () => {
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
          lastUpdated: '2026-03-19T23:00:00Z',
          queueTotal: 519,
          queueDeferred: 12,
          queueHold: 4,
          mailCountTotal: 1200,
          spamIn: 32,
          virusIn: 2,
        },
      },
    });

    const { getByText, getByRole, getByTestId, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Service details')).toBeInTheDocument();
    expect(getByText('519 queue total · 16 backlog')).toBeInTheDocument();
    expect(queryByText('Mail Gateway')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show service details' }));
    const serviceDetails = within(getByTestId('resource-service-details-section'));
    expect(serviceDetails.getByText('PMG')).toBeInTheDocument();
    expect(serviceDetails.queryByText('Connection')).toBeNull();
    expect(serviceDetails.getAllByText('State').length).toBeGreaterThan(0);
    expect(serviceDetails.getAllByText('pmg-main.local').length).toBeGreaterThan(0);
    fireEvent.click(getByRole('button', { name: 'Show details' }));
    expect(queryByText('Mail flow summary')).toBeNull();
    expect(queryByText('Queue breakdown')).toBeNull();
    expect(queryByText('Mail processing')).toBeNull();
    expect(queryByText('Queue detail')).toBeNull();
    expect(queryByText('Mail detail')).toBeNull();
    expect(queryByText('Show mail flow detail')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show mail flow' }));
    expect(getByText('Queue')).toBeInTheDocument();
    expect(getByText('Backlog')).toBeInTheDocument();
    const pmgSupportContext = within(getByTestId('pmg-support-context'));
    expect(pmgSupportContext.getByText('Nodes')).toBeInTheDocument();
    expect(pmgSupportContext.getByText('Updated')).toBeInTheDocument();
    expect(getByText('Queue detail').closest('summary')?.textContent).toBe('Queue detail');
    expect(getByText('Mail detail').closest('summary')?.textContent).toBe('Mail detail');
    expect(getByRole('link', { name: /open pmg thresholds/i })).toHaveAttribute(
      'href',
      '/alerts/thresholds/mail-gateway',
    );
  });

  it('keeps PMG freshness in support context even without a node count', () => {
    const resource = baseResource({
      id: 'pmg-2',
      type: 'pmg',
      name: 'pmg-edge',
      displayName: 'PMG Edge',
      platformId: 'pmg-edge',
      platformType: 'proxmox-pmg',
      platformData: {
        sources: ['pmg'],
        pmg: {
          hostname: 'pmg-edge.local',
          connectionHealth: 'online',
          lastUpdated: '2026-03-19T23:00:00Z',
          queueTotal: 12,
          mailCountTotal: 320,
        },
      },
    });

    const { getByRole, getByTestId } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    fireEvent.click(getByRole('button', { name: 'Show service details' }));
    fireEvent.click(getByRole('button', { name: 'Show mail flow' }));
    const pmgSupportContext = within(getByTestId('pmg-support-context'));
    expect(pmgSupportContext.queryByText('Nodes')).toBeNull();
    expect(pmgSupportContext.getByText('Updated')).toBeInTheDocument();
  });

  it('keeps docker update controls behind a secondary reveal', () => {
    const resource = baseResource({
      id: 'docker-host-1',
      type: 'docker-host',
      name: 'docker-main',
      displayName: 'Docker Main',
      platformId: 'docker-main',
      platformType: 'docker',
      sourceType: 'agent',
      platformData: {
        sources: ['docker', 'agent'],
        docker: {
          hostSourceId: 'docker-host-1',
          hostname: 'docker-main.local',
          runtime: 'Docker Engine 28.0',
          containerCount: 18,
          updatesAvailableCount: 4,
        },
      },
    });

    const { getByText, getByRole, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Service details')).toBeInTheDocument();
    expect(getByText('18 containers · 4 updates')).toBeInTheDocument();
    fireEvent.click(getByRole('button', { name: 'Show service details' }));
    expect(getByText('Docker runtime')).toBeInTheDocument();
    expect(queryByText('Container Updates')).toBeNull();
    expect(queryByText('Check now')).toBeNull();
    expect(queryByText('Show update controls')).toBeNull();
    fireEvent.click(getByRole('button', { name: 'Show actions' }));
    expect(getByText('Check now')).toBeInTheDocument();
    expect(getByText('Update all (4)')).toBeInTheDocument();
    expect(queryByText('Updates Available')).toBeNull();
    expect(queryByText('Last Check')).toBeNull();
  });
});
