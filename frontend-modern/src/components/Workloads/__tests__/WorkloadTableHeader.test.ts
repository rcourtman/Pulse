import { describe, expect, it } from 'vitest';

import { getWorkloadColumnHeaderAlign, getWorkloadColumnHeaderLabel } from '../WorkloadTableHeader';

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
    expect(getWorkloadColumnHeaderLabel('memory', 'Mem', 'host', true)).toBe('Mem · Host');
    expect(getWorkloadColumnHeaderLabel('info', 'Info', 'guest', true)).toBe('ID');
    expect(getWorkloadColumnHeaderLabel('uptime', 'Uptime', 'guest', true)).toBe('Age');
  });
});

describe('getWorkloadColumnHeaderAlign', () => {
  it('centers paired I/O headers over their two directional values', () => {
    expect(getWorkloadColumnHeaderAlign('netIo', 'numeric-value', false)).toBe('center');
    expect(getWorkloadColumnHeaderAlign('diskIo', 'numeric-value', false)).toBe('center');
  });

  it('centers identifiers beside the workload type column', () => {
    expect(getWorkloadColumnHeaderAlign('info', 'numeric-value', false)).toBe('center');
    expect(getWorkloadColumnHeaderAlign('vmid', 'numeric-value', false)).toBe('center');
  });

  it('keeps scalar quantities right aligned', () => {
    expect(getWorkloadColumnHeaderAlign('uptime', 'numeric-value', false)).toBe('right');
  });

  it('keeps the primary identity header left aligned', () => {
    expect(getWorkloadColumnHeaderAlign('name', 'numeric-value', true)).toBe('left');
  });
});
