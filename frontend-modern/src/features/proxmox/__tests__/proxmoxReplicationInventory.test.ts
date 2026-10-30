import { describe, expect, it } from 'vitest';
import {
  ReplicationInventoryFormatError,
  parseReplicationInventory,
} from '../proxmoxReplicationInventory';

const job = () => ({
  id: 'site-a-100-0',
  instance: 'site-a',
  jobId: '100-0',
  enabled: true,
});
const payload = (data: unknown[]) => ({ data, meta: { total: data.length } });

describe('replication inventory wire boundary', () => {
  it('accepts only an explicit complete empty observation as absence', () => {
    expect(parseReplicationInventory(payload([]))).toEqual([]);
  });

  it('keeps current Go wire fields, optional omissions and future fields without mutation', () => {
    const current = Object.freeze({
      ...job(),
      guestId: 100,
      guestName: 'database',
      guestType: 'qemu',
      guestNode: 'pve-a',
      sourceNode: 'pve-a',
      targetNode: 'pve-b',
      lastSyncStatus: 'ok',
      lastSyncTime: '2026-10-30T01:00:00Z',
      lastSyncUnix: 1793322000,
      nextSyncTime: '2026-10-30T01:15:00Z',
      nextSyncUnix: 1793322900,
      lastSyncDurationSeconds: 0,
      failCount: 0,
      rateLimitMbps: 0,
      lastPolled: '2026-10-30T01:01:00Z',
      futureField: { present: true },
    });
    const data = [current, { ...job(), id: 'site-b-100-0', instance: 'site-b', enabled: false }];
    const response = payload(data);
    expect(parseReplicationInventory(response)).toBe(data);
    expect(data[0]).toBe(current);
  });

  it('does not invent a health classification or chronology for unfamiliar strings', () => {
    const data = [{ ...job(), lastSyncStatus: 'new-provider-state', lastSyncTime: '' }];
    expect(parseReplicationInventory(payload(data))).toEqual(data);
  });

  it.each([
    null,
    [],
    '',
    {},
    { meta: { total: 0 } },
    { data: null, meta: { total: 0 } },
    { data: {}, meta: { total: 0 } },
    { data: [] },
    { data: [], meta: null },
    { data: [], meta: [] },
    { data: [], meta: {} },
    { data: [], meta: { total: '0' } },
    { data: [], meta: { total: -1 } },
    { data: [], meta: { total: 0.5 } },
    { data: [], meta: { total: 1 } },
    { data: [job()], meta: { total: 0 } },
  ])('rejects an unread or inconsistent envelope %# instead of returning no jobs', (response) => {
    expect(() => parseReplicationInventory(response)).toThrow(ReplicationInventoryFormatError);
  });

  it.each([null, [], true, 'job', {}, { ...job(), enabled: 'false' }])(
    'rejects a malformed row %# without dropping just that job',
    (row) => {
      expect(() => parseReplicationInventory(payload([job(), row]))).toThrow(
        ReplicationInventoryFormatError,
      );
    },
  );

  it.each(['id', 'instance', 'jobId', 'enabled'])('requires %s from the producer', (field) => {
    const row: Record<string, unknown> = job();
    delete row[field];
    expect(() => parseReplicationInventory(payload([row]))).toThrow(
      ReplicationInventoryFormatError,
    );
  });

  it.each([
    ['id', ' '],
    ['instance', ''],
    ['jobId', null],
    ['enabled', 1],
    ['guestId', '100'],
    ['guestId', -1],
    ['guestId', 1.5],
    ['guestName', {}],
    ['guestType', true],
    ['guest', []],
    ['guestNode', null],
    ['sourceNode', []],
    ['targetNode', {}],
    ['sourceStorage', 1],
    ['targetStorage', null],
    ['schedule', []],
    ['lastSyncStatus', {}],
    ['lastSyncTime', {}],
    ['nextSyncTime', true],
    ['lastSyncUnix', '1793322000'],
    ['nextSyncUnix', Infinity],
    ['lastSyncDurationSeconds', -1],
    ['lastSyncDurationHuman', []],
    ['failCount', '0'],
    ['failCount', 0.5],
    ['failCount', NaN],
    ['failCount', -1],
    ['rateLimitMbps', -1],
    ['error', {}],
    ['comment', false],
  ])('rejects wrongly typed %s (%#) before classification, search or links', (field, value) => {
    expect(() =>
      parseReplicationInventory(payload([{ ...job(), [field as string]: value }])),
    ).toThrow(ReplicationInventoryFormatError);
  });

  it('rejects identical and conflicting duplicate disclosure IDs', () => {
    for (const duplicate of [job(), { ...job(), instance: 'site-b', enabled: false }]) {
      expect(() => parseReplicationInventory(payload([job(), duplicate]))).toThrow(
        ReplicationInventoryFormatError,
      );
    }
  });

  it('rejects duplicate native jobs even when the projected IDs differ', () => {
    expect(() => parseReplicationInventory(payload([job(), { ...job(), id: 'other' }]))).toThrow(
      ReplicationInventoryFormatError,
    );
  });

  it('never includes response data in the error', () => {
    const marker = 'private-provider-response';
    let error: unknown;
    try {
      parseReplicationInventory({ error: marker });
    } catch (caught) {
      error = caught;
    }
    expect(error).toBeInstanceOf(ReplicationInventoryFormatError);
    expect((error as Error).message).toContain('Missing jobs do not mean none are configured.');
    expect(String(error)).not.toContain(marker);
  });
});
