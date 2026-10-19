import { describe, expect, it } from 'vitest';

import { BACKUP_IDENTITY_NOW, identityArchive, identityPBS } from '../__fixtures__/backupIdentity';
import { buildProxmoxBackupRecoveryModel } from '../proxmoxBackupRecoveryModel';

const artifacts = (pbsBackups = [identityPBS()], archives = [identityArchive()]) =>
  buildProxmoxBackupRecoveryModel({
    workloads: [],
    pbsBackups,
    archives,
    snapshots: [],
    tasks: [],
    nowMs: BACKUP_IDENTITY_NOW,
  }).recoverableArtifacts;

// Location identity must be at least as strict as the source inventory. Labels,
// case folding and delimiter framing are not repository identity.
describe('backup-location source identity', () => {
  it('keeps case-sensitive PBS datastores distinct', () => {
    const result = artifacts(
      [
        identityPBS({ id: 'lower', datastore: 'main' }),
        identityPBS({ id: 'upper', datastore: 'Main' }),
      ],
      [],
    );
    expect(new Set(result.map((item) => item.locationKey)).size).toBe(2);
    expect(result.map((item) => item.locationLabel)).toEqual([
      'pbs-server / main',
      'pbs-server / Main',
    ]);
  });

  it('keeps source connection casing and delimiter boundaries', () => {
    const result = artifacts(
      [
        identityPBS({ id: 'one', instance: 'PBS', datastore: 'main' }),
        identityPBS({ id: 'two', instance: 'pbs', datastore: 'main' }),
        identityPBS({ id: 'three', instance: 'a:b', datastore: 'c' }),
        identityPBS({ id: 'four', instance: 'a', datastore: 'b:c' }),
      ],
      [],
    );
    expect(new Set(result.map((item) => item.locationKey)).size).toBe(4);
  });

  it('does not drop a repeated or missing source field from identity', () => {
    const result = artifacts(
      [
        identityPBS({ id: 'both', instance: 'main', datastore: 'main' }),
        identityPBS({ id: 'only-store', instance: '', datastore: 'main' }),
        identityPBS({ id: 'only-source', instance: 'main', datastore: '' }),
      ],
      [],
    );
    expect(new Set(result.map((item) => item.locationKey)).size).toBe(3);
    expect(new Set(result.map((item) => item.locationLabel)).size).toBe(3);
  });

  it('keeps case-sensitive storage nodes and store-wide aggregates distinct', () => {
    const result = artifacts(
      [],
      [
        identityArchive({ id: 'node-lower', storageNode: 'node' }),
        identityArchive({ id: 'node-upper', storageNode: 'Node' }),
        identityArchive({ id: 'store-upper', storage: 'Local', storageNode: 'node' }),
        identityArchive({ id: 'shared' }),
      ],
    );
    expect(new Set(result.map((item) => item.locationKey)).size).toBe(4);
    expect(result[0].locationGroupKey).toBe(result[3].locationKey);
    expect(result[1].locationGroupKey).toBe(result[3].locationKey);
    expect(result[2].locationGroupKey).not.toBe(result[3].locationKey);
  });

  it('keeps duplicate observations and response order from changing a source key', () => {
    const first = identityPBS({ id: 'a', datastore: 'Main' });
    const second = identityPBS({ id: 'b', datastore: 'main' });
    const ordered = artifacts([first, second, { ...first, id: 'copy' }], []);
    const reversed = artifacts([second, first], []);
    expect(ordered[0].locationKey).toBe(ordered[2].locationKey);
    expect(ordered[0].locationKey).toBe(reversed[1].locationKey);
    expect(ordered[1].locationKey).toBe(reversed[0].locationKey);
  });

  it('keeps source kinds distinct and entirely unnamed locations unselectable', () => {
    const result = artifacts([identityPBS({ instance: 'east', datastore: 'local' })]);
    expect(result[0].locationKey).not.toBe(result[1].locationKey);
    expect(
      artifacts([identityPBS({ instance: '', datastore: '' })], [])[0].locationKey,
    ).toBeUndefined();
  });
});
