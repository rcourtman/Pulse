import { describe, expect, it } from 'vitest';
import { getAPITokenExpiryPresentation } from '../apiTokenPresentation';

const now = Date.parse('2026-11-08T21:30:00Z');

describe('saved API token expiry presentation', () => {
  it.each([undefined, null])('distinguishes no saved expiry (%s) from validity', (value) => {
    expect(getAPITokenExpiryPresentation(value, now)).toEqual({
      state: 'none',
      label: 'No expiry',
    });
  });

  it.each([
    '',
    'not-a-timestamp',
    '2026-11-08',
    '2026-11-08T21:30:00',
    '2026-02-30T21:30:00Z',
    '1900-02-29T21:30:00Z',
    '2026-13-01T21:30:00Z',
    '2026-00-01T21:30:00Z',
    '2026-11-00T21:30:00Z',
    '2026-11-08T24:00:00Z',
    '2026-11-08T21:60:00Z',
    '2026-11-08T21:30:60Z',
    '2026-11-08T21:30:00+24:00',
    '2026-11-08T21:30:00+01:60',
    '2026-11-08T21:30:00.1234567890Z',
    '2026-11-08T21:30:00Z trailing',
  ])('does not invent a lifetime from %s', (value) => {
    expect(getAPITokenExpiryPresentation(value, now)).toEqual({
      state: 'unknown',
      label: 'Unavailable',
    });
  });

  it.each([
    ['2026-11-08T23:31:00+02:00', '2026-11-08T21:31:00.000Z', '2026-11-08 21:31:00 UTC'],
    ['2026-11-08T14:31:00-07:00', '2026-11-08T21:31:00.000Z', '2026-11-08 21:31:00 UTC'],
    ['2026-11-08T21:31:00.123456789Z', '2026-11-08T21:31:00.123Z', '2026-11-08 21:31:00.123 UTC'],
    ['2028-02-29T00:00:00Z', '2028-02-29T00:00:00.000Z', '2028-02-29 00:00:00 UTC'],
  ])('keeps the saved zoned instant explicit: %s', (value, dateTime, timestamp) => {
    expect(getAPITokenExpiryPresentation(value, now)).toEqual({
      state: 'scheduled',
      label: 'Expires',
      dateTime,
      timestamp,
    });
  });

  it('follows the server strict-after boundary without declaring broader access valid', () => {
    const value = '2026-11-08T21:30:00Z';
    expect(getAPITokenExpiryPresentation(value, now - 1).state).toBe('scheduled');
    expect(getAPITokenExpiryPresentation(value, now).state).toBe('scheduled');
    expect(getAPITokenExpiryPresentation(value, now + 1)).toMatchObject({
      state: 'expired',
      label: 'Expired',
      dateTime: '2026-11-08T21:30:00.000Z',
    });
    expect(getAPITokenExpiryPresentation(value, NaN)).toEqual({
      state: 'unknown',
      label: 'Unavailable',
    });
  });
});
