import { describe, expect, it } from 'vitest';
import {
  getZfsDeviceBlockClass,
  getZfsDeviceStateTextClass,
  getZfsPoolErrorOverlayClass,
  getZfsPoolStateTextClass,
  getZfsTooltipErrorTextClass,
} from '@/features/storageBackups/zfsPresentation';

describe('zfsPresentation', () => {
  it('returns canonical device block classes by state', () => {
    expect(getZfsDeviceBlockClass({ state: 'ONLINE' } as any)).toContain('bg-green-500');
    expect(getZfsDeviceBlockClass({ state: 'DEGRADED' } as any)).toContain('bg-yellow-500');
    expect(getZfsDeviceBlockClass({ state: 'FAULTED' } as any)).toContain('bg-red-500');
  });

  it('returns canonical device state text classes', () => {
    expect(getZfsDeviceStateTextClass({ state: 'ONLINE' } as any)).toBe('text-green-400');
    expect(getZfsDeviceStateTextClass({ state: 'DEGRADED' } as any)).toBe('text-yellow-400');
    expect(getZfsDeviceStateTextClass({ state: 'OFFLINE' } as any)).toBe('text-red-400');
  });

  it('returns canonical pool tooltip classes', () => {
    expect(getZfsTooltipErrorTextClass(true)).toBe('text-red-400');
    expect(getZfsTooltipErrorTextClass(false)).toBe('text-green-400');
    expect(getZfsPoolStateTextClass(true)).toBe('text-red-400 font-bold');
    expect(getZfsPoolStateTextClass(false)).toBe('text-green-400');
    expect(getZfsPoolErrorOverlayClass(true)).toContain('border-red-500');
    expect(getZfsPoolErrorOverlayClass(false)).toBe('');
  });
});
