import { describe, expect, it } from 'vitest';
import {
  getRaidDeviceBadgeClass,
  getRaidStateTextClass,
  getRaidStateVariant,
} from '@/utils/raidPresentation';

describe('raidPresentation', () => {
  it('maps healthy array states to success', () => {
    expect(getRaidStateVariant('active')).toBe('success');
    expect(getRaidStateTextClass('clean')).toContain('text-emerald-600');
  });

  it('maps failed array states to danger', () => {
    expect(getRaidStateVariant('offline')).toBe('danger');
    expect(getRaidStateTextClass('failed')).toContain('text-red-600');
  });

  it('maps mixed array states to warning', () => {
    expect(getRaidStateVariant('recovering')).toBe('warning');
    expect(getRaidStateTextClass('recovering')).toContain('text-amber-600');
  });

  it('maps healthy RAID devices to green badges', () => {
    expect(getRaidDeviceBadgeClass({ device: 'sda', state: 'in_sync', slot: 0 })).toContain(
      'emerald',
    );
  });

  it('maps failed RAID devices to red badges', () => {
    expect(getRaidDeviceBadgeClass({ device: 'sdb', state: 'faulty', slot: 1 })).toContain('red');
  });
});
