import { describe, expect, it } from 'vitest';

import { makeDetailTimestampRow } from '../platformDetailTimestampRow';

describe('makeDetailTimestampRow', () => {
  // A second past the hour, so the fractional stamp below is a full hour old.
  const now = Date.parse('2026-10-02T22:13:01Z');

  it('shows the absolute time and its age, wrapping, with the raw stamp as its title', () => {
    const row = makeDetailTimestampRow('Created', '2026-10-02T21:13:00.123456789Z', now);
    expect(row?.label).toBe('Created');
    expect(row?.value).toMatch(/2026/);
    expect(row?.value).toMatch(/\(1h ago\)$/);
    expect(row?.wrap).toBe(true);
    expect(row?.title).toBe('2026-10-02T21:13:00.123456789Z');
  });

  it('omits empty stamps', () => {
    expect(makeDetailTimestampRow('Created', '  ', now)).toBeNull();
    expect(makeDetailTimestampRow('Created', undefined, now)).toBeNull();
  });

  it('reads an unparseable stamp as reported unless a sentinel year is set', () => {
    expect(makeDetailTimestampRow('Seen', 'yesterday-ish', now)?.value).toBe('yesterday-ish');
    expect(
      makeDetailTimestampRow('Started', '0001-01-01T00:00:00Z', now, { minYear: 2000 }),
    ).toBeNull();
  });
});
