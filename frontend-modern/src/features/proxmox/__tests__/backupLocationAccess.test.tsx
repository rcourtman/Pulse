import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';

import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
  identitySnapshot,
} from '../__fixtures__/backupIdentity';
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
  getRecoverableArtifactLocation,
  recoverableArtifactMatchesSearch,
} from '../proxmoxBackupRecoveryModel';
import { ProxmoxCoverageTable } from '../ProxmoxCoverageTable';
import { resolveBackupLocationSelection } from '../proxmoxBackupLocation';
import { ProxmoxRecoverableTable } from '../ProxmoxRecoverableTable';

const namespace = 'pve-1/department-with-a-long-name/production';
const locations = [
  `pbs-east / main / ${namespace}`,
  `pbs-west / main / ${namespace}`,
  'pbs-east / main / (root)',
  'east / local / archive-node-b',
  'east / pve-1',
];
const buildModel = () =>
  buildProxmoxBackupRecoveryModel({
    nowMs: BACKUP_IDENTITY_NOW,
    workloads: [identityGuest('east-100', 'east', 'pve-1')],
    pbsBackups: [
      identityPBS({ id: 'east-nested', instance: 'pbs-east', namespace }),
      identityPBS({ id: 'west-nested', instance: 'pbs-west', namespace }),
      identityPBS({ id: 'east-root', instance: 'pbs-east', namespace: '' }),
    ],
    archives: [identityArchive({ storageNode: 'archive-node-b' })],
    snapshots: [identitySnapshot()],
    tasks: [],
  });

describe('backup location access without hover', () => {
  afterEach(cleanup);

  it('retains the observed source connection without changing location/filter identity', () => {
    const model = buildModel();
    expect(
      Object.fromEntries(
        model.recoverableArtifacts.map((artifact) => [artifact.nativeId, artifact.connection]),
      ),
    ).toEqual({
      'east-nested': 'pbs-east',
      'west-nested': 'pbs-west',
      'east-root': 'pbs-east',
      'file-100': 'east',
      'snapshot-100': 'east',
    });
    const pbs = model.recoverableArtifacts.filter((artifact) => artifact.sourceKind === 'pbs');
    expect(pbs.map((artifact) => artifact.location)).toEqual([
      `main / ${namespace}`,
      `main / ${namespace}`,
      'main / (root)',
    ]);
    expect(pbs.map((artifact) => artifact.locationKey)).toEqual([
      'backup-location-v2:["pbs","pbs-east","main"]',
      'backup-location-v2:["pbs","pbs-west","main"]',
      'backup-location-v2:["pbs","pbs-east","main"]',
    ]);
    expect(
      model.recoverableArtifacts.find((artifact) => artifact.nativeId === 'file-100')
        ?.locationGroupKey,
    ).toBe('backup-location-v2:["archive","east","local"]');
    // Old saved filters are aliases, not the source identity. Each must still
    // resolve to its own exact tuple without pooling east and west evidence.
    for (const connection of ['pbs-east', 'pbs-west']) {
      expect(
        resolveBackupLocationSelection(model.recoverableArtifacts, `pbs:${connection}:main`),
      ).toEqual({
        state: 'matched',
        key: `backup-location-v2:["pbs","${connection}","main"]`,
        label: `${connection} / main`,
      });
    }
    expect(
      resolveBackupLocationSelection(model.recoverableArtifacts, 'archive:east:local'),
    ).toEqual({
      state: 'matched',
      key: 'backup-location-v2:["archive","east","local"]',
      label: 'east / local',
    });
    expect(model.recoverableArtifacts.map(getRecoverableArtifactLocation).sort()).toEqual(
      [...locations].sort(),
    );
  });

  it('does not infer a missing repository connection from the matched guest', () => {
    const model = buildProxmoxBackupRecoveryModel({
      nowMs: BACKUP_IDENTITY_NOW,
      workloads: [identityGuest('east-100', 'east', 'pve-1')],
      pbsBackups: [identityPBS({ instance: '   ' })],
      archives: [],
      snapshots: [],
      tasks: [],
    });
    expect(model.recoverableArtifacts[0].connection).toBeUndefined();
    expect(model.recoverableArtifacts[0].workload.instance).toBe('east');
  });

  it('searches the connection now shown in both views without mixing repository identities', () => {
    const model = buildModel();
    expect(
      model.recoverableArtifacts
        .filter((artifact) => recoverableArtifactMatchesSearch(artifact, ' PBS-WEST '))
        .map((artifact) => artifact.nativeId),
    ).toEqual(['west-nested']);
    expect(coverageRowMatchesSearch(model.coverageRows[0], 'pbs-west')).toBe(true);
    expect(coverageRowMatchesSearch(model.coverageRows[0], 'pbs-missing')).toBe(false);
  });

  it.each([320, 640, 1365])('keeps complete By date provenance at %s px', (width) => {
    const { container } = render(() => (
      <ProxmoxRecoverableTable
        artifacts={buildModel().recoverableArtifacts}
        hasAnyArtifacts
        emptyIcon={<span />}
        emptyTitle="No backups"
        emptyDescription="No backups"
        sortKey={() => 'created'}
        sortDirection={() => 'desc'}
        onSort={() => undefined}
        sizeMaxBytes={2048}
        layoutWidth={() => width}
      />
    ));
    for (const location of locations) {
      const label = screen.getByText(location);
      expect(label).toHaveClass('whitespace-normal', 'wrap-anywhere');
      expect(label).not.toHaveClass('truncate');
      expect(label.closest('[hidden]')).toBeNull();
      if (width === 320) expect(label.closest('td')).toHaveAttribute('colspan', '4');
    }
    expect(container.querySelectorAll('[data-backup-location]')).toHaveLength(5);
    expect(container.querySelectorAll('[data-proxmox-backup-row="recoverable"]')).toHaveLength(5);
  });

  it.each([320, 640, 1365])('keeps complete expanded Coverage provenance at %s px', (width) => {
    const model = buildModel();
    const { container } = render(() => (
      <ProxmoxCoverageTable
        rows={model.coverageRows}
        hasAnyRows
        emptyIcon={<span />}
        emptyTitle="No guests"
        emptyDescription="No guests"
        sortKey={() => 'workload'}
        sortDirection={() => 'asc'}
        onSort={() => undefined}
        expandedKeys={new Set(model.coverageRows.map((row) => row.key))}
        onToggleExpand={() => undefined}
        showTaskColumn={false}
        layoutWidth={() => width}
      />
    ));
    for (const location of locations) {
      const label = screen.getByText(location);
      expect(label).toHaveClass('whitespace-normal', 'wrap-anywhere');
      expect(label).not.toHaveClass('truncate');
      if (width === 320) expect(label.closest('td')).toHaveAttribute('colspan', '3');
    }
    expect(container.querySelectorAll('[data-backup-location]')).toHaveLength(5);
  });

  it('moves provenance when the existing table resizes without hiding or duplicating it', () => {
    const [width, setWidth] = createSignal(1365);
    const { container } = render(() => (
      <ProxmoxRecoverableTable
        artifacts={buildModel().recoverableArtifacts}
        hasAnyArtifacts
        emptyIcon={<span />}
        emptyTitle="No backups"
        emptyDescription="No backups"
        sortKey={() => 'created'}
        sortDirection={() => 'desc'}
        onSort={() => undefined}
        sizeMaxBytes={2048}
        layoutWidth={width}
      />
    ));
    for (const next of [320, 640, 1365]) {
      setWidth(next);
      for (const location of locations) expect(screen.getByText(location)).toBeInTheDocument();
      expect(container.querySelectorAll('[data-backup-location]')).toHaveLength(5);
    }
  });
});
