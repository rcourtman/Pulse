import { describe, expect, it } from 'vitest';
import { formatBackupCompletionTime } from '../format';

describe('backup completion timestamp formatting', () => {
  it.each([
    Date.parse('2026-10-12T00:30:00.123Z'),
    new Date('2026-10-12T00:30:00.123Z'),
    '2026-10-11T17:30:00.123-07:00',
    '2026-10-12T09:30:00.123+09:00',
  ])('normalises equivalent instant %s without a local calendar or timezone', (value) => {
    expect(formatBackupCompletionTime(value)).toEqual({
      completedAt: '2026-10-12T00:30:00.123Z',
      dateLabel: '2026-10-12 00:30:00 UTC',
    });
  });

  it.each(['invalid', '', NaN, Infinity, -1, 0, 8.64e15 + 1, new Date(NaN)])(
    'does not throw or manufacture an instant from unusable input %s',
    (value) => {
      expect(formatBackupCompletionTime(value)).toBeNull();
    },
  );
});
