import { render, screen, within } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { RecoveryPointDetails } from './RecoveryPointDetails';

const wsState = vi.hoisted(() => ({ resources: [] as Resource[] }));

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    state: wsState,
  }),
}));

describe('RecoveryPointDetails', () => {
  beforeEach(() => {
    wsState.resources = [];
  });

  it('renders platform-neutral details framing while preserving PBS-specific metadata', () => {
    wsState.resources = [
      {
        id: 'pbs-resource-1',
        type: 'pbs',
        name: 'pbs-main',
        displayName: 'pbs-main',
        platformId: 'pbs-main',
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        status: 'online',
        lastSeen: Date.parse('2026-03-10T10:00:00Z'),
        platformData: {
          pbs: {
            instanceId: 'pbs-main',
            datastores: [
              {
                name: 'fast-store',
                used: 500,
                total: 1000,
                usage: 50,
                status: 'ok',
                deduplicationFactor: 2.25,
              },
            ],
          },
        },
      } as Resource,
    ];

    render(() => (
      <RecoveryPointDetails
        point={{
          id: 'point-1',
          platform: 'proxmox-pbs',
          kind: 'backup',
          mode: 'remote',
          outcome: 'success',
          startedAt: '2026-03-10T09:58:00Z',
          completedAt: '2026-03-10T10:00:00Z',
          verified: true,
          immutable: true,
          display: {
            clusterLabel: 'Lab Cluster',
            nodeHostLabel: 'pve-01',
            namespaceLabel: 'Finance',
          },
          itemRef: {
            type: 'proxmox-vm',
            name: '100',
          },
          repositoryRef: {
            type: 'pbs-datastore',
            namespace: 'pbs-main',
            name: 'fast-store',
          },
          details: {
            comment: 'Nightly retention protected copy',
            owner: 'root@pam',
            files: ['vm/100/2026-03-10T10:00:00Z'],
            verificationState: 'ok',
          },
        }}
      />
    ));

    expect(screen.getByText('Platform Details')).toBeInTheDocument();
    expect(screen.queryByText('PBS Details')).not.toBeInTheDocument();
    expect(screen.getByText('Platform-specific recovery metadata, verification state, and target health.')).toBeInTheDocument();
    expect(screen.getByText('Target Health')).toBeInTheDocument();
    expect(screen.getByText('Verification')).toBeInTheDocument();
    expect(screen.getByText('Item Type')).toBeInTheDocument();
    expect(screen.getByText('Cluster / Site')).toBeInTheDocument();
    expect(screen.getByText('Host / Agent')).toBeInTheDocument();
    expect(screen.getByText('Namespace / Group')).toBeInTheDocument();
    expect(screen.getByText('Point Type')).toBeInTheDocument();
    expect(screen.getByText('Method')).toBeInTheDocument();
    expect(screen.getByText('Outcome')).toBeInTheDocument();
    expect(screen.getByText('VM')).toBeInTheDocument();
    expect(screen.getByText('Lab Cluster')).toBeInTheDocument();
    expect(screen.getByText('pve-01')).toBeInTheDocument();
    expect(screen.getByText('Finance')).toBeInTheDocument();
    expect(screen.getByText('Backup')).toBeInTheDocument();
    expect(screen.getByText('Remote Copy')).toBeInTheDocument();
    expect(screen.getAllByText('Success').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Verified').length).toBeGreaterThan(0);
    expect(screen.getByText('Target Ref')).toBeInTheDocument();

    const platformCard = screen.getByText('Platform').parentElement?.parentElement;
    expect(platformCard).not.toBeNull();
    expect(within(platformCard as HTMLDivElement).getByText('PBS')).toBeInTheDocument();
  });

  it('uses canonical platform labels without forcing provider detail panels for other platforms', () => {
    render(() => (
      <RecoveryPointDetails
        point={{
          id: 'point-2',
          platform: 'truenas',
          display: {
            nodeHostLabel: 'tn-scale-01',
          },
          itemRef: {
            type: 'truenas-dataset',
            name: 'tank/apps',
          },
          kind: 'snapshot',
          mode: 'snapshot',
          outcome: 'failed',
          completedAt: '2026-03-10T10:00:00Z',
        }}
      />
    ));

    expect(screen.queryByText('Platform Details')).not.toBeInTheDocument();
    expect(screen.queryByText('PBS Details')).not.toBeInTheDocument();
    expect(screen.getByText('Item Type')).toBeInTheDocument();
    expect(screen.getByText('Host / Agent')).toBeInTheDocument();
    expect(screen.getByText('Dataset')).toBeInTheDocument();
    expect(screen.getByText('tn-scale-01')).toBeInTheDocument();
    expect(screen.getAllByText('Snapshot').length).toBeGreaterThan(0);
    expect(screen.queryByText('Repository Ref')).not.toBeInTheDocument();

    const platformCard = screen.getByText('Platform').parentElement?.parentElement;
    expect(platformCard).not.toBeNull();
    expect(within(platformCard as HTMLDivElement).getByText('TrueNAS')).toBeInTheDocument();
  });
});
