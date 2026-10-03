import { describe, expect, it } from 'vitest';
import {
  BackupInventoryFormatError,
  parsePBSBackupInventory,
  parsePVEBackupInventory,
  readBackupInventoryJSON,
} from '../proxmoxBackupInventory';

const task = { id: 'task-112', vmid: 112, type: 'ct', status: 'OK', startTime: '' };
const archive = { id: 'archive-112', vmid: 112, type: 'ct', time: '' };
const snapshot = { id: 'snapshot-112', vmid: 112, type: 'ct', time: '' };
const pbs = { id: 'pbs-112', vmid: '112', backupType: 'ct', backupTime: '', files: [] };
const pveData = { backupTasks: [task], storageBackups: [archive], guestSnapshots: [snapshot] };
const pbsData = { backups: [pbs] };

describe('backup inventory response boundary', () => {
  it('keeps observed rows, missing optional facts and future fields without coercion', () => {
    const data = {
      ...pveData,
      storageBackups: [{ ...archive, inProgress: true, futureField: { enabled: true } }],
    };
    const result = parsePVEBackupInventory({ data });
    expect(result).toEqual(data);
    expect(result.storageBackups).toBe(data.storageBackups);
    expect(result.storageBackups[0].size).toBeUndefined();
    expect(parsePBSBackupInventory({ data: pbsData })).toEqual(pbsData);
  });

  it('accepts empty arrays and explicitly nil PVE collections, not missing collections', () => {
    const empty = { backupTasks: [], storageBackups: [], guestSnapshots: [] };
    expect(parsePVEBackupInventory({ data: empty })).toEqual(empty);
    expect(
      parsePVEBackupInventory({
        data: { backupTasks: null, storageBackups: null, guestSnapshots: null },
      }),
    ).toEqual(empty);
    expect(parsePBSBackupInventory({ data: { backups: [] } })).toEqual({ backups: [] });
    expect(() => parsePBSBackupInventory({ data: { backups: null } })).toThrow(
      BackupInventoryFormatError,
    );
  });

  it.each([undefined, null, false, 'response', [], {}, { data: null }, { data: [] }, { data: {} }])(
    'rejects a missing or invalid envelope: %j',
    (payload) => {
      expect(() => parsePVEBackupInventory(payload)).toThrow(BackupInventoryFormatError);
      expect(() => parsePBSBackupInventory(payload)).toThrow(BackupInventoryFormatError);
    },
  );

  it.each(['backupTasks', 'storageBackups', 'guestSnapshots'] as const)(
    'rejects missing, scalar, object and invalid %s rows without silently dropping them',
    (key) => {
      for (const value of [undefined, false, '', {}, [null], ['row'], [pveData[key][0], {}]]) {
        expect(() => parsePVEBackupInventory({ data: { ...pveData, [key]: value } })).toThrow(
          BackupInventoryFormatError,
        );
      }
    },
  );

  it.each([undefined, false, '', {}, [null], ['row'], [pbs, {}]])(
    'rejects a malformed PBS collection or row: %j',
    (backups) => {
      expect(() => parsePBSBackupInventory({ data: { backups } })).toThrow(
        BackupInventoryFormatError,
      );
    },
  );

  it.each([
    ['id', ''],
    ['vmid', 112],
    ['backupType', {}],
    ['backupTime', null],
    ['files', undefined],
    ['files', null],
    ['files', {}],
    ['files', [42]],
    ['instance', {}],
    ['datastore', []],
    ['namespace', 1],
    ['size', '1000'],
    ['size', -1],
    ['size', Number.NaN],
    ['protected', 'false'],
    ['verified', 1],
    ['inProgress', 'true'],
    ['writeActivityObserved', {}],
    ['writeActive', null],
  ])(
    'rejects mistyped PBS %s without inventing an identity or completion state',
    (field, value) => {
      expect(() =>
        parsePBSBackupInventory({ data: { backups: [{ ...pbs, [field as string]: value }] } }),
      ).toThrow(BackupInventoryFormatError);
    },
  );

  it.each([
    ['backupTasks', task, 'status', {}],
    ['backupTasks', task, 'startTime', null],
    ['backupTasks', task, 'endTime', 1],
    ['backupTasks', task, 'error', []],
    ['storageBackups', archive, 'id', ' '],
    ['storageBackups', archive, 'vmid', '112'],
    ['storageBackups', archive, 'vmid', 1.5],
    ['storageBackups', archive, 'vmid', -1],
    ['storageBackups', archive, 'time', {}],
    ['storageBackups', archive, 'type', null],
    ['storageBackups', archive, 'node', {}],
    ['storageBackups', archive, 'instance', 1],
    ['storageBackups', archive, 'storage', []],
    ['storageBackups', archive, 'format', {}],
    ['storageBackups', archive, 'volid', []],
    ['storageBackups', archive, 'size', Number.POSITIVE_INFINITY],
    ['storageBackups', archive, 'protected', 'false'],
    ['storageBackups', archive, 'verified', 1],
    ['storageBackups', archive, 'isPBS', 'false'],
    ['storageBackups', archive, 'inProgress', 'true'],
    ['guestSnapshots', snapshot, 'name', {}],
    ['guestSnapshots', snapshot, 'description', []],
    ['guestSnapshots', snapshot, 'sizeBytes', -1],
  ] as const)('rejects mistyped PVE %s.%s', (key, row, field, value) => {
    expect(() =>
      parsePVEBackupInventory({ data: { ...pveData, [key]: [{ ...row, [field]: value }] } }),
    ).toThrow(BackupInventoryFormatError);
  });

  it('preserves valid zero values, host identifiers and boolean completion facts', () => {
    const row = {
      ...pbs,
      vmid: '',
      backupType: 'host',
      namespace: '',
      size: 0,
      protected: false,
      verified: false,
      inProgress: true,
      writeActivityObserved: true,
      writeActive: false,
    };
    expect(parsePBSBackupInventory({ data: { backups: [row] } }).backups[0]).toEqual(row);
  });

  it('replaces JSON decoding errors with fixed copy that contains no response content', async () => {
    const response = new Response('SYNTHETIC_PRIVATE_BODY_DO_NOT_DISPLAY');
    await expect(readBackupInventoryJSON(response)).rejects.toEqual(
      new BackupInventoryFormatError(),
    );
    await expect(readBackupInventoryJSON(new Response('null'))).resolves.toBeNull();
  });
});
