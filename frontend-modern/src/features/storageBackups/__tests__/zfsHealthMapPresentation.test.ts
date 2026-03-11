import { describe, expect, it } from 'vitest';
import {
  getZfsHealthMapDeviceClass,
  getZfsHealthMapErrorSummaryClass,
  getZfsHealthMapMessageClass,
  getZfsHealthMapTooltipStyle,
  getZfsHealthMapDevices,
  getZfsHealthMapTooltipPresentation,
  ZFS_HEALTH_MAP_ROOT_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_CARD_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_NAME_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_PORTAL_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_STATE_ROW_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_STATE_TEXT_CLASS,
  ZFS_HEALTH_MAP_TOOLTIP_TRANSFORM,
  ZFS_HEALTH_MAP_TOOLTIP_TYPE_CLASS,
  isZfsHealthMapDeviceResilvering,
} from '@/features/storageBackups/zfsHealthMapPresentation';
import type { ZFSPool } from '@/types/api';

const pool = {
  scan: 'resilver in progress',
  devices: [
    {
      name: 'sda',
      type: 'disk',
      state: 'ONLINE',
      message: '',
      readErrors: 1,
      writeErrors: 2,
      checksumErrors: 3,
    },
  ],
} as unknown as ZFSPool;

describe('zfsHealthMapPresentation', () => {
  it('builds canonical zfs device and tooltip state', () => {
    expect(ZFS_HEALTH_MAP_ROOT_CLASS).toBe('flex items-center gap-0.5');
    expect(ZFS_HEALTH_MAP_TOOLTIP_PORTAL_CLASS).toContain('z-[9999]');
    expect(ZFS_HEALTH_MAP_TOOLTIP_CARD_CLASS).toContain('border-border');
    expect(ZFS_HEALTH_MAP_TOOLTIP_NAME_CLASS).toContain('font-medium');
    expect(ZFS_HEALTH_MAP_TOOLTIP_TYPE_CLASS).toContain('text-muted');
    expect(ZFS_HEALTH_MAP_TOOLTIP_STATE_ROW_CLASS).toContain('border-t');
    expect(ZFS_HEALTH_MAP_TOOLTIP_STATE_TEXT_CLASS).toBe('font-semibold');
    expect(ZFS_HEALTH_MAP_TOOLTIP_TRANSFORM).toBe('translate(-50%, -100%)');

    const [device] = getZfsHealthMapDevices(pool);
    expect(device.name).toBe('sda');
    expect(isZfsHealthMapDeviceResilvering(pool, device)).toBe(true);
    expect(getZfsHealthMapTooltipPresentation(device)).toEqual({
      name: 'sda',
      type: 'disk',
      state: 'ONLINE',
      message: '',
      errorSummary: 'E: 1/2/3',
      hasErrors: true,
    });
    expect(getZfsHealthMapDeviceClass('bg-green-500', true)).toBe(
      'w-2.5 h-3 rounded-sm transition-colors duration-200 bg-green-500 animate-pulse',
    );
    expect(getZfsHealthMapTooltipStyle(10, 20)).toEqual({
      left: '10px',
      top: '12px',
      transform: 'translate(-50%, -100%)',
    });
    expect(getZfsHealthMapErrorSummaryClass()).toBe('text-red-400');
    expect(getZfsHealthMapMessageClass()).toBe('text-muted mt-1 italic max-w-[200px] break-words');
  });
});
