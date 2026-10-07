import { describe, expect, it } from 'vitest';
import {
  RAID_LAST_KNOWN_DEVICE_BADGE_CLASS,
  getRaidDeviceBadgeClass,
  getRaidDeviceStateVariant,
  getRaidLastKnownDeviceLabel,
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

  it('reads the mdadm detail state of a healthy member as healthy', () => {
    // The agent reports mdadm --detail member states verbatim.
    for (const state of ['active sync', 'Active Sync', 'active sync writemostly']) {
      expect(getRaidDeviceStateVariant({ device: 'sda', state, slot: 0 })).toBe('success');
      expect(getRaidDeviceBadgeClass({ device: 'sda', state, slot: 0 })).toContain('emerald');
    }
    expect(getRaidDeviceStateVariant({ device: 'sdc', state: 'spare rebuilding', slot: 2 })).toBe(
      'warning',
    );
  });

  it('maps failed RAID devices to red badges', () => {
    expect(getRaidDeviceBadgeClass({ device: 'sdb', state: 'faulty', slot: 1 })).toContain('red');
  });

  it('names the state of a retained member that was not healthy', () => {
    expect(getRaidLastKnownDeviceLabel({ device: 'sda', state: 'active sync', slot: 0 })).toBe(
      'sda',
    );
    expect(getRaidLastKnownDeviceLabel({ device: 'sdb', state: 'faulty', slot: 1 })).toBe(
      'sdb · faulty',
    );
    expect(getRaidLastKnownDeviceLabel({ device: 'sdc', state: ' spare ', slot: 2 })).toBe(
      'sdc · spare',
    );
    expect(getRaidLastKnownDeviceLabel({ device: 'sdd', state: '', slot: 3 })).toBe('sdd');
    expect(RAID_LAST_KNOWN_DEVICE_BADGE_CLASS).not.toMatch(/emerald|amber|red/);
  });
});
