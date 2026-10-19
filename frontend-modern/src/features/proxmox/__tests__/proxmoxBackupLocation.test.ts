import { describe, expect, it } from 'vitest';
import { BACKUP_IDENTITY_NOW, identityArchive, identityPBS } from '../__fixtures__/backupIdentity';
import { buildProxmoxBackupRecoveryModel } from '../proxmoxBackupRecoveryModel';
import { resolveBackupLocationSelection } from '../proxmoxBackupLocation';

const artifacts = (pbsBackups = [identityPBS()], archives = [identityArchive()]) =>
  buildProxmoxBackupRecoveryModel({
    workloads: [],
    pbsBackups,
    archives,
    snapshots: [],
    tasks: [],
    nowMs: BACKUP_IDENTITY_NOW,
  }).recoverableArtifacts;

const resolve = resolveBackupLocationSelection;
describe('saved backup-location resolution', () => {
  it('retains a unique legacy PBS location with a readable exact label', () => {
    const list = artifacts([identityPBS()], []);
    expect(resolve(list, 'pbs:pbs-server:main')).toEqual({
      state: 'matched',
      key: list[0].locationKey,
      label: 'pbs-server / main',
    });
    expect(resolve(list, list[0].locationKey!)).toEqual(resolve(list, 'pbs:pbs-server:main'));
  });

  it.each([false, true])(
    'refuses ambiguous case-folded aliases in either order (%s)',
    (reversed) => {
      const list = artifacts(
        [
          identityPBS({ id: 'one', datastore: 'main' }),
          identityPBS({ id: 'two', datastore: 'Main' }),
        ],
        [],
      );
      if (reversed) list.reverse();
      expect(resolve(list, 'pbs:pbs-server:main')).toEqual({ state: 'ambiguous' });
      for (const item of list)
        expect(resolve(list, item.locationKey!)).toEqual({
          state: 'matched',
          key: item.locationKey,
          label: item.locationLabel,
        });
    },
  );

  it('refuses ambiguous delimiter and missing-field aliases', () => {
    expect(
      resolve(
        artifacts(
          [
            identityPBS({ id: 'one', instance: 'a:b', datastore: 'c' }),
            identityPBS({ id: 'two', instance: 'a', datastore: 'b:c' }),
          ],
          [],
        ),
        'pbs:a:b:c',
      ),
    ).toEqual({ state: 'ambiguous' });
    expect(
      resolve(
        artifacts(
          [
            identityPBS({ id: 'one', instance: 'main', datastore: 'main' }),
            identityPBS({ id: 'two', instance: '', datastore: 'main' }),
          ],
          [],
        ),
        'pbs:main',
      ),
    ).toEqual({ state: 'ambiguous' });
  });

  it('preserves legacy local-node and store-wide filters without pooling repositories', () => {
    const list = artifacts(
      [],
      [
        identityArchive({ id: 'one', storageNode: 'node-a' }),
        identityArchive({ id: 'two', storageNode: 'node-b' }),
        identityArchive({ id: 'other', storage: 'other' }),
      ],
    );
    expect(resolve(list, '["archive","east","local","node-a"]')).toEqual({
      state: 'matched',
      key: list[0].locationKey,
      label: 'east / local / node-a',
    });
    expect(resolve(list, 'archive:east:local')).toEqual({
      state: 'matched',
      key: list[0].locationGroupKey,
      label: 'east / local',
    });
    expect(resolve(list, list[0].locationGroupKey!)).toEqual(resolve(list, 'archive:east:local'));
    list.push(
      ...artifacts([], [identityArchive({ id: 'case', storage: 'Local', storageNode: 'node-a' })]),
    );
    expect(resolve(list, 'archive:east:local')).toEqual({ state: 'ambiguous' });
    expect(resolve(list, '["archive","east","local","node-a"]')).toEqual({ state: 'ambiguous' });
  });

  it('does not confuse duplicate observations or a shared store with an ambiguous source', () => {
    const list = artifacts(
      [],
      [
        identityArchive({ id: 'one', storageNode: 'node-a' }),
        identityArchive({ id: 'copy', storageNode: 'node-a' }),
        identityArchive({ id: 'shared' }),
      ],
    );
    expect(resolve(list, 'archive:east:local').state).toBe('matched');
    expect(resolve(list, '["archive","east","local","node-a"]').state).toBe('matched');
  });

  it('keeps unknown saved selections missing, never silently All', () => {
    const list = artifacts();
    expect(resolve(list, '')).toEqual({ state: 'all' });
    expect(resolve([], 'pbs:old:main')).toEqual({ state: 'missing' });
    expect(resolve(list, 'backup-location-v2:["pbs","removed","main"]')).toEqual({
      state: 'missing',
    });
    expect(resolve(list, '<img src=x onerror=alert(1)>')).toEqual({ state: 'missing' });
  });

  it('withdraws a unique alias when a collision arrives and restores only unambiguous resolution', () => {
    const list = artifacts([identityPBS()], []);
    const selected = 'pbs:pbs-server:main';
    expect(resolve(list, selected).state).toBe('matched');
    list.push(...artifacts([identityPBS({ id: 'new-store', datastore: 'Main' })], []));
    expect(resolve(list, selected).state).toBe('ambiguous');
    list.pop();
    expect(resolve(list, selected).state).toBe('matched');
  });
});
