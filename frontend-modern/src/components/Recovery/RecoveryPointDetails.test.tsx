import { render, screen } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { RecoveryPointDetails } from './RecoveryPointDetails';

const wsState = vi.hoisted(() => ({ resources: [] as Resource[] }));

vi.mock('@/contexts/appRuntime', () => ({
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
            vmid: '100',
            datastore: 'fast-store',
            namespace: 'Finance',
          },
        }}
      />
    ));

    expect(screen.getByText('Target Details')).toBeInTheDocument();
    expect(screen.queryByText('Platform Details')).not.toBeInTheDocument();
    expect(screen.queryByText('PBS Details')).not.toBeInTheDocument();
    expect(
      screen.getByText(
        'Repository owner, target capacity, and file inventory for this recovery point.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Restore action path')).not.toBeInTheDocument();
    expect(screen.getByText('Restore readiness')).toBeInTheDocument();
    expect(screen.getByText('Verified candidate')).toBeInTheDocument();
    expect(screen.getByText('Verification provenance')).toBeInTheDocument();
    expect(screen.getByText('PBS catalog verification')).toBeInTheDocument();
    expect(screen.getAllByText('High confidence').length).toBeGreaterThan(0);
    expect(screen.getByText('Catalog check passed')).toBeInTheDocument();
    expect(screen.queryByText('State: ok')).not.toBeInTheDocument();
    expect(screen.getByText('Target Health')).toBeInTheDocument();
    expect(screen.queryByText('UPID:')).not.toBeInTheDocument();
    expect(screen.getByText('Recovery metadata')).toBeInTheDocument();
    expect(screen.getByText('Item Type')).toBeInTheDocument();
    expect(screen.getByText('Cluster / Site')).toBeInTheDocument();
    expect(screen.getByText('Host / Agent')).toBeInTheDocument();
    expect(screen.getByText('Namespace / Group')).toBeInTheDocument();
    expect(screen.getAllByText('Outcome').length).toBeGreaterThan(0);
    expect(screen.getByText('VM')).toBeInTheDocument();
    expect(screen.getByText('Lab Cluster')).toBeInTheDocument();
    expect(screen.getByText('pve-01')).toBeInTheDocument();
    expect(screen.getByText('Finance')).toBeInTheDocument();
    expect(screen.getByText('Backup / Remote Copy')).toBeInTheDocument();
    expect(screen.getAllByText('Success').length).toBeGreaterThan(0);
    expect(screen.queryByText('Target Ref')).not.toBeInTheDocument();
    expect(screen.queryByText('Item Ref')).not.toBeInTheDocument();
    expect(screen.getByText('VMID')).toBeInTheDocument();
    expect(screen.getByText('Datastore: fast-store')).toBeInTheDocument();
    expect(screen.queryByText('Namespace')).not.toBeInTheDocument();
    expect(screen.queryByText('details.vmid')).not.toBeInTheDocument();

    expect(screen.getAllByText('PBS').length).toBeGreaterThan(0);
  });

  it('surfaces restore readiness when a successful point has not been verified', () => {
    render(() => (
      <RecoveryPointDetails
        point={{
          id: 'point-unverified',
          platform: 'truenas',
          kind: 'snapshot',
          mode: 'snapshot',
          outcome: 'success',
          completedAt: '2026-03-10T10:00:00Z',
          display: {
            itemLabel: 'tank/apps',
            itemType: 'dataset',
            nodeHostLabel: 'tn-scale-01',
          },
        }}
      />
    ));

    expect(screen.queryByText('Restore action path')).not.toBeInTheDocument();
    expect(screen.getByText('Restore readiness')).toBeInTheDocument();
    expect(screen.getByText('Available candidate')).toBeInTheDocument();
    expect(screen.queryByText('Verification provenance')).not.toBeInTheDocument();
    expect(screen.queryByText('Needs verification')).not.toBeInTheDocument();
    expect(screen.queryByText('No verification timestamp recorded')).not.toBeInTheDocument();
  });

  it('keeps PBS container identifiers and empty verification details operator-safe', () => {
    render(() => (
      <RecoveryPointDetails
        point={{
          id: 'point-pbs-container',
          platform: 'proxmox-pbs',
          kind: 'backup',
          mode: 'remote',
          outcome: 'success',
          completedAt: '2026-03-10T10:00:00Z',
          verified: true,
          display: {
            clusterLabel: 'delly',
            nodeHostLabel: 'minipc',
            namespaceLabel: 'minipc',
            itemType: 'lxc',
          },
          repositoryRef: {
            type: 'pbs-datastore',
            namespace: 'minipc',
            name: 'main',
          },
          details: {
            verificationState: 'ok',
            vmid: '113',
            datastore: 'main',
          },
        }}
      />
    ));

    expect(screen.getByText('Verification provenance')).toBeInTheDocument();
    expect(screen.getByText('PBS catalog verification')).toBeInTheDocument();
    expect(screen.getByText('Catalog check passed')).toBeInTheDocument();
    expect(screen.queryByText('State: ok')).not.toBeInTheDocument();
    expect(screen.queryByText('No verification timestamp recorded')).not.toBeInTheDocument();
    expect(screen.getByText('CTID')).toBeInTheDocument();
    expect(screen.queryByText('VMID')).not.toBeInTheDocument();
    expect(screen.queryByText('Namespace / Group')).not.toBeInTheDocument();
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

    expect(screen.queryByText('Target Details')).not.toBeInTheDocument();
    expect(screen.queryByText('Platform Details')).not.toBeInTheDocument();
    expect(screen.queryByText('PBS Details')).not.toBeInTheDocument();
    expect(screen.getByText('Item Type')).toBeInTheDocument();
    expect(screen.getByText('Host / Agent')).toBeInTheDocument();
    expect(screen.getByText('Dataset')).toBeInTheDocument();
    expect(screen.getByText('tn-scale-01')).toBeInTheDocument();
    expect(screen.getAllByText('Snapshot').length).toBeGreaterThan(0);
    expect(screen.queryByText('Repository Ref')).not.toBeInTheDocument();

    expect(screen.getAllByText('TrueNAS').length).toBeGreaterThan(0);
  });

  it('uses platform-aware PVE wording and suppresses meaningless task details', () => {
    render(() => (
      <RecoveryPointDetails
        point={{
          id: 'point-3',
          platform: 'proxmox-pve',
          itemResourceId: 'res:vm-105',
          kind: 'snapshot',
          mode: 'snapshot',
          outcome: 'success',
          completedAt: '2026-03-31T10:00:00Z',
          details: {
            instance: 'delly',
            node: 'delly',
            snapshotName: 'vzdump',
            vmid: '105',
          },
        }}
        relatedPoints={[
          {
            id: 'point-4',
            platform: 'proxmox-pbs',
            itemResourceId: 'res:vm-105',
            kind: 'backup',
            mode: 'remote',
            outcome: 'success',
            verified: true,
            completedAt: '2026-03-31T10:10:00Z',
            repositoryRef: {
              type: 'pbs-datastore',
              namespace: 'pbs-main',
              name: 'fast-store',
            },
          },
        ]}
      />
    ));

    expect(screen.queryByText('Snapshot / Snapshot')).not.toBeInTheDocument();
    expect(screen.getAllByText('Snapshot').length).toBeGreaterThan(0);
    expect(screen.getByText('Local snapshot on delly')).toBeInTheDocument();
    expect(screen.getByText('Protection chain')).toBeInTheDocument();
    expect(screen.getByText('Remote copy')).toBeInTheDocument();
    expect(screen.getByText(/Verified/)).toBeInTheDocument();
  });

  it('turns generic PVE job errors into actionable failure copy and hides VMID zero', () => {
    render(() => (
      <RecoveryPointDetails
        point={{
          id: 'point-5',
          platform: 'proxmox-pve',
          kind: 'backup',
          mode: 'local',
          outcome: 'failed',
          completedAt: '2026-03-31T10:00:00Z',
          details: {
            status: 'job errors',
            taskName: 'vzdump',
            vmid: '0',
          },
        }}
      />
    ));

    expect(screen.getByText('Failure detail')).toBeInTheDocument();
    expect(
      screen.getAllByText(/Inspect the platform task log for the failing step/i).length,
    ).toBeGreaterThan(0);
    expect(screen.getByText('Not restorable')).toBeInTheDocument();
    expect(screen.queryByText('Investigate source task')).not.toBeInTheDocument();
    expect(screen.queryByText('Verification provenance')).not.toBeInTheDocument();
    expect(screen.queryByText('VMID')).not.toBeInTheDocument();
    expect(screen.queryByText('0')).not.toBeInTheDocument();
  });
});
