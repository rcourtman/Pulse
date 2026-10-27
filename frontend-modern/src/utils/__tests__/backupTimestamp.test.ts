import { describe, expect, it } from 'vitest';
import { parseBackupTimestampMs } from '../backupTimestamp';
import { formatBackupCompletionTime, getBackupInfo } from '../format';

const now = Date.parse('2026-10-04T12:00:00Z');

describe('shared backup timestamp evidence', () => {
  it.each([
    '2026-09-31T11:00:00Z',
    '2026-02-29T11:00:00Z',
    '2026-04-31T11:00:00Z',
    '1900-02-29T11:00:00Z',
    '2026-10-04T11:00:00',
    '2026-10-04',
    '10/04/2026',
    '2026-10-04T24:00:00Z',
    '2026-10-04T11:60:00Z',
    '2026-10-04T11:00:60Z',
    '2026-10-04T11:00:00+24:00',
    '2026-10-04T11:00:00+01:60',
    '2026-10-04T11:00:00.1234567890Z',
    '2026-10-04T11:00:00Z trailing',
    'invalid',
  ])('[regression] never manufactures backup age or completion from %s', (value) => {
    expect(parseBackupTimestampMs(value)).toBeUndefined();
    expect(getBackupInfo(value, undefined, now)).toEqual({
      status: 'unknown',
      ageMs: null,
      ageFormatted: 'Backup time unavailable: invalid timestamp.',
    });
    expect(formatBackupCompletionTime(value)).toBeNull();
  });

  it.each([
    ['2026-10-04T11:00:00Z', '2026-10-04T11:00:00.000Z'],
    ['2026-10-04T13:00:00+02:00', '2026-10-04T11:00:00.000Z'],
    ['2026-10-04T04:00:00-07:00', '2026-10-04T11:00:00.000Z'],
    ['2026-10-04T11:00:00.123456789Z', '2026-10-04T11:00:00.123Z'],
    ['2024-02-29T11:00:00Z', '2024-02-29T11:00:00.000Z'],
    ['2000-02-29T11:00:00Z', '2000-02-29T11:00:00.000Z'],
    ['2026-10-04T00:30:00+14:00', '2026-10-03T10:30:00.000Z'],
  ])('[control] preserves a real zoned instant %s', (value, iso) => {
    const ms = Date.parse(iso);
    expect(parseBackupTimestampMs(value)).toBe(ms);
    expect(getBackupInfo(value, undefined, now)).toEqual(getBackupInfo(ms, undefined, now));
    expect(formatBackupCompletionTime(value)).toEqual(formatBackupCompletionTime(ms));
    expect(formatBackupCompletionTime(value)?.completedAt).toBe(iso);
  });

  it.each([null, undefined, '', 0])('[control] keeps absence sentinel %s distinct', (value) => {
    expect(getBackupInfo(value, undefined, now).status).toBe('never');
  });

  it('[control] keeps numeric/Date completions, future uncertainty and policy limits', () => {
    const ms = now - 2 * 3600_000;
    expect(formatBackupCompletionTime(new Date(ms))).toEqual(formatBackupCompletionTime(ms));
    expect(getBackupInfo(ms, { freshHours: 1 }, now).status).toBe('stale');
    expect(getBackupInfo(ms, { freshHours: 3 }, now).status).toBe('fresh');
    expect(getBackupInfo('2026-10-04T12:00:01Z', undefined, now).status).toBe('unknown');
  });
});
