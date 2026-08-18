import { describe, expect, it } from 'vitest';

import { getWorkloadColumnHeaderLabel } from '../WorkloadTableHeader';

describe('getWorkloadColumnHeaderLabel', () => {
  it('keeps the default memory label for guest-relative values', () => {
    expect(getWorkloadColumnHeaderLabel('memory', 'Mem', 'guest')).toBe('Mem');
  });

  it('keeps host-relative memory visible after the preference moves into View', () => {
    expect(getWorkloadColumnHeaderLabel('memory', 'Mem', 'host')).toBe('Mem · Host');
  });

  it('does not relabel unrelated columns', () => {
    expect(getWorkloadColumnHeaderLabel('cpu', 'CPU', 'host')).toBe('CPU');
  });

  it('uses compact labels for narrow operational columns', () => {
    expect(getWorkloadColumnHeaderLabel('availability', 'Avail', 'guest', true)).toBe('Up');
    expect(getWorkloadColumnHeaderLabel('memory', 'Mem', 'host', true)).toBe('Mem');
    expect(getWorkloadColumnHeaderLabel('info', 'Info', 'guest', true)).toBe('ID');
    expect(getWorkloadColumnHeaderLabel('uptime', 'Uptime', 'guest', true)).toBe('Age');
  });
});
