import { describe, expect, it } from 'vitest';
import {
  BackupInventoryFormatError,
  parsePBSBackupInventory,
  parsePVEBackupInventory,
} from '../proxmoxBackupInventory';

const task = { id: 'shared-id', vmid: 112, type: 'ct', status: 'OK', startTime: '' };
const archive = { id: 'shared-id', vmid: 112, type: 'ct', time: '' };
const snapshot = { id: 'shared-id', vmid: 112, type: 'ct', time: '' };
const pbs = { id: 'shared-id', vmid: '112', backupType: 'ct', backupTime: '', files: [] };
const emptyPVE = { backupTasks: [], storageBackups: [], guestSnapshots: [] };

describe('backup artifact identity admission', () => {
  for (const conflict of [false, true]) {
    it.each(['storageBackups', 'guestSnapshots'] as const)(
      `rejects duplicate %s IDs, conflicting fields: ${conflict}`,
      (key) => {
        const first = key === 'storageBackups' ? archive : snapshot;
        const second = { ...first, ...(conflict ? { vmid: 999, instance: 'other-source' } : {}) };
        for (const rows of [
          [first, second],
          [second, first],
        ]) {
          expect(() => parsePVEBackupInventory({ data: { ...emptyPVE, [key]: rows } })).toThrow(
            BackupInventoryFormatError,
          );
        }
      },
    );

    it(`rejects duplicate PBS IDs, conflicting fields: ${conflict}`, () => {
      const second = { ...pbs, ...(conflict ? { verified: true, instance: 'other-pbs' } : {}) };
      for (const rows of [
        [pbs, second],
        [second, pbs],
      ]) {
        expect(() => parsePBSBackupInventory({ data: { backups: rows } })).toThrow(
          BackupInventoryFormatError,
        );
      }
    });
  }

  it('does not conflate separately prefixed artifact collections or task observations', () => {
    const data = {
      backupTasks: [task, task],
      storageBackups: [archive],
      guestSnapshots: [snapshot],
    };
    expect(parsePVEBackupInventory({ data })).toEqual(data);
    expect(parsePBSBackupInventory({ data: { backups: [pbs] } })).toEqual({ backups: [pbs] });
  });

  it('keeps IDs opaque and admits distinct points of the same workload', () => {
    const backups = ['point', 'Point', ' point'].map((id) => ({ ...pbs, id }));
    expect(parsePBSBackupInventory({ data: { backups } })).toEqual({ backups });
    const storageBackups = backups.map(({ id }) => ({ ...archive, id }));
    expect(
      parsePVEBackupInventory({ data: { ...emptyPVE, storageBackups } }).storageBackups,
    ).toEqual(storageBackups);
  });
});
