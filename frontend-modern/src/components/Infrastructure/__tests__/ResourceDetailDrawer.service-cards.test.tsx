import { describe, expect, it, vi } from 'vitest';
import { render } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab" />,
}));

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'host',
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

    const { getByText, getAllByText, getByRole } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getByText('PBS Service')).toBeInTheDocument();
    expect(getAllByText('pbs-main.local').length).toBeGreaterThan(0);
    expect(getByText('Datastores')).toBeInTheDocument();
    expect(getByText('Total Jobs')).toBeInTheDocument();
    expect(getByText('Job breakdown')).toBeInTheDocument();
    expect(getByRole('link', { name: /open pbs backups/i })).toHaveAttribute(
      'href',
      '/backups?source=pbs&backupType=remote',
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
          queueTotal: 519,
          queueDeferred: 12,
          queueHold: 4,
          mailCountTotal: 1200,
          spamIn: 32,
          virusIn: 2,
        },
      },
    });

    const { getByText, getAllByText, getByRole } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getByText('Mail Gateway')).toBeInTheDocument();
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
