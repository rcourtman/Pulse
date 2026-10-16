import { cleanup, render } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import {
  identityArchive,
  identityGuest,
  identityPBS,
  BACKUP_IDENTITY_NOW,
} from '../__fixtures__/backupIdentity';
import {
  buildProxmoxBackupRecoveryModel,
  type RecoverableArtifact,
} from '../proxmoxBackupRecoveryModel';
import { ProxmoxRecoverableTable } from '../ProxmoxRecoverableTable';

const buildArtifacts = () => {
  const guests = [
    identityGuest('east-100', 'east', 'pve-east.example.test', { nodeDisplayName: 'Display node' }),
    identityGuest('east-101', 'east', 'pve-east.example.test', {
      vmid: 101,
      nodeDisplayName: 'Display node',
    }),
    identityGuest('west-100', 'west', 'pve-west.example.test', { nodeDisplayName: 'Display node' }),
  ].map((guest) => ({ ...guest, name: 'database' }));
  return buildProxmoxBackupRecoveryModel({
    nowMs: BACKUP_IDENTITY_NOW,
    workloads: guests,
    pbsBackups: [
      identityPBS({
        id: 'pbs-east',
        instance: 'central-backups',
        namespace: 'pve-east.example.test',
      }),
    ],
    archives: [
      identityArchive({ id: 'east-101', vmid: 101, node: 'pve-east.example.test' }),
      identityArchive({ id: 'west-100', instance: 'west', node: 'pve-west.example.test' }),
    ],
    snapshots: [],
    tasks: [],
  }).recoverableArtifacts;
};

function Table(props: { artifacts: RecoverableArtifact[]; width: number }) {
  return (
    <ProxmoxRecoverableTable
      artifacts={props.artifacts}
      hasAnyArtifacts
      emptyIcon={<span />}
      emptyTitle="No points"
      emptyDescription="No points"
      sortKey={() => 'created'}
      sortDirection={() => 'desc'}
      onSort={() => undefined}
      sizeMaxBytes={2048}
      layoutWidth={() => props.width}
    />
  );
}

describe('By date workload identity without hover', () => {
  afterEach(cleanup);

  it.each([320, 600, 800, 1000, 1365])(
    'keeps repeated names and IDs distinct at %s px',
    (width) => {
      const { container } = render(() => <Table artifacts={buildArtifacts()} width={width} />);
      const rows = [...container.querySelectorAll('[data-proxmox-backup-row="recoverable"]')];
      expect(rows).toHaveLength(3);
      expect(new Set(rows.map((row) => row.querySelector('td')!.textContent)).size).toBe(
        width >= 880 ? 2 : 3,
      );
      for (const row of rows) {
        const key = row.getAttribute('data-proxmox-backup-key');
        const west = key === 'artifact:archive:west-100';
        const targetID = key === 'artifact:archive:east-101' ? '101' : '100';
        const identity = row.querySelector('td')!;
        const connection = identity.querySelector('[data-backup-point-workload-connection]');
        expect(connection).toHaveTextContent(west ? 'Connection: west' : 'Connection: east');
        const node = identity.querySelector('[data-backup-point-workload-node]');
        expect(node).toHaveTextContent(
          west ? 'Node: pve-west.example.test' : 'Node: pve-east.example.test',
        );
        expect(node).not.toHaveTextContent('Display node');
        expect(connection).toHaveClass('whitespace-normal', 'wrap-anywhere');
        expect(node).toHaveClass('whitespace-normal', 'wrap-anywhere');
        const target = identity.querySelector('[data-backup-point-workload-target]');
        if (width < 880) expect(target).toHaveTextContent(`VM ${targetID}`);
        else expect(target).toBeNull();
      }
      // The PBS repository connection is not substituted for the matched workload.
      expect(
        container.querySelector(
          '[data-proxmox-backup-key="artifact:pbs:pbs-east"] [data-backup-point-workload-connection]',
        ),
      ).not.toHaveTextContent('central-backups');
      expect(
        [...container.querySelectorAll('[data-backup-location]')].some((location) =>
          location.textContent?.includes('central-backups'),
        ),
      ).toBe(true);
    },
  );

  it('does not invent workload connection or node identity from an unmatched PBS namespace', () => {
    const artifacts = buildProxmoxBackupRecoveryModel({
      nowMs: BACKUP_IDENTITY_NOW,
      workloads: [],
      archives: [],
      snapshots: [],
      tasks: [],
      pbsBackups: [identityPBS({ namespace: 'not-an-observed-node', instance: 'repository-only' })],
    }).recoverableArtifacts;
    const { container } = render(() => <Table artifacts={artifacts} width={320} />);
    expect(container.querySelector('[data-backup-point-workload-connection]')).toHaveTextContent(
      'Connection: Unknown',
    );
    expect(container.querySelector('[data-backup-point-workload-node]')).toBeNull();
    expect(container.querySelector('[data-backup-location]')).toHaveTextContent(
      'repository-only / main / not-an-observed-node',
    );
  });

  it('updates identity and folded columns on a mounted resize and inventory refresh', () => {
    const [width, setWidth] = createSignal(1365);
    const [artifacts, setArtifacts] = createSignal(buildArtifacts());
    const { container } = render(() => <Table artifacts={artifacts()} width={width()} />);
    for (const next of [800, 320, 600, 1000, 1365]) {
      setWidth(next);
      expect(container.querySelectorAll('[data-backup-point-workload-connection]')).toHaveLength(3);
      expect(container.querySelectorAll('[data-backup-point-workload-target]')).toHaveLength(
        next < 880 ? 3 : 0,
      );
    }
    setArtifacts(
      artifacts().map((artifact, index) =>
        index === 1
          ? {
              ...artifact,
              workload: {
                ...artifact.workload,
                instance: 'east-new',
                vmid: '102',
                nativeNode: 'pve-new.example.test',
              },
            }
          : artifact,
      ),
    );
    setWidth(320);
    const row = container.querySelectorAll('[data-proxmox-backup-row="recoverable"]')[1];
    expect(row.querySelector('[data-backup-point-workload-target]')).toHaveTextContent('VM 102');
    expect(row.querySelector('[data-backup-point-workload-connection]')).toHaveTextContent(
      'Connection: east-new',
    );
    expect(row.querySelector('[data-backup-point-workload-node]')).toHaveTextContent(
      'Node: pve-new.example.test',
    );
  });
});
