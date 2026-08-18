import { describe, expect, it } from 'vitest';

import {
  getProxmoxHostColumnWidthStyle,
  getProxmoxHostTableMinWidthClass,
  getProxmoxHostTableLayoutModeForContainer,
  getProxmoxHostVisibleColumnsForLayout,
} from '../proxmoxHostTableModel';

describe('proxmoxHostTableModel', () => {
  it('shows the full operational metric track at phone width', () => {
    const columns = getProxmoxHostVisibleColumnsForLayout('phone');
    const ids = columns.map((column) => column.id);

    expect(ids).toEqual(['node', 'cpu', 'memory', 'disk', 'temp', 'uptime']);
    expect(getProxmoxHostTableMinWidthClass('phone')).toBe('min-w-[0px]');
    expect(getProxmoxHostColumnWidthStyle('node', 'phone', ids)).toEqual({ width: '30%' });
    expect(getProxmoxHostColumnWidthStyle('disk', 'phone', ids)).toEqual({ width: '13.3824%' });
    expect(getProxmoxHostColumnWidthStyle('temp', 'phone', ids)).toEqual({ width: '15.4412%' });
    expect(getProxmoxHostColumnWidthStyle('uptime', 'phone', ids)).toEqual({ width: '14.4118%' });
  });

  it('prioritizes live utilization columns in the mobile host table', () => {
    const columns = getProxmoxHostVisibleColumnsForLayout('mobile');
    const ids = columns.map((column) => column.id);

    expect(ids).toEqual(['node', 'cpu', 'memory', 'disk', 'temp', 'uptime']);
    expect(getProxmoxHostTableMinWidthClass('mobile')).toBe('min-w-[0px]');
    expect(getProxmoxHostColumnWidthStyle('node', 'mobile', ids)).toEqual({ width: '30%' });
    expect(getProxmoxHostColumnWidthStyle('cpu', 'mobile', ids)).toEqual({ width: '13.3824%' });
    expect(getProxmoxHostColumnWidthStyle('memory', 'mobile', ids)).toEqual({
      width: '13.3824%',
    });
    expect(getProxmoxHostColumnWidthStyle('disk', 'mobile', ids)).toEqual({ width: '13.3824%' });
    expect(getProxmoxHostColumnWidthStyle('temp', 'mobile', ids)).toEqual({ width: '15.4412%' });
    expect(getProxmoxHostColumnWidthStyle('uptime', 'mobile', ids)).toEqual({
      width: '14.4118%',
    });
  });

  it('adds operational context before inventory metadata on tablet', () => {
    expect(getProxmoxHostVisibleColumnsForLayout('tablet').map((column) => column.id)).toEqual([
      'node',
      'cpu',
      'memory',
      'disk',
      'temp',
      'uptime',
      'cluster',
    ]);
    expect(getProxmoxHostTableMinWidthClass('tablet')).toBe('min-w-[50rem]');
  });

  it('adds guest counts on compact and reserves version for wide layouts', () => {
    const compactIds = getProxmoxHostVisibleColumnsForLayout('compact').map((column) => column.id);

    expect(compactIds).toEqual([
      'node',
      'cpu',
      'memory',
      'disk',
      'temp',
      'uptime',
      'vms',
      'cts',
      'cluster',
    ]);
    expect(getProxmoxHostColumnWidthStyle('cluster', 'compact', compactIds)).toEqual({
      width: '10%',
    });
    expect(getProxmoxHostVisibleColumnsForLayout('wide').map((column) => column.id)).toEqual([
      'node',
      'version',
      'cpu',
      'memory',
      'disk',
      'temp',
      'uptime',
      'vms',
      'cts',
      'cluster',
    ]);
  });

  it('chooses host columns from available container width', () => {
    expect(getProxmoxHostTableLayoutModeForContainer(439)).toBe('phone');
    expect(getProxmoxHostTableLayoutModeForContainer(440)).toBe('mobile');
    expect(getProxmoxHostTableLayoutModeForContainer(799)).toBe('mobile');
    expect(getProxmoxHostTableLayoutModeForContainer(800)).toBe('tablet');
    expect(getProxmoxHostTableLayoutModeForContainer(1039)).toBe('tablet');
    expect(getProxmoxHostTableLayoutModeForContainer(1040)).toBe('compact');
    expect(getProxmoxHostTableLayoutModeForContainer(1319)).toBe('compact');
    expect(getProxmoxHostTableLayoutModeForContainer(1320)).toBe('wide');
  });

  it('fits the container on compact and reserves the fixed floor only on wide', () => {
    // The compact band covers most laptops. Forcing a 1240px floor
    // there pushed the rightmost column behind a horizontal scroll, so compact
    // now fits its container; only wide keeps the fixed-width floor.
    expect(getProxmoxHostTableMinWidthClass('compact')).toBe('min-w-[64rem]');
    expect(getProxmoxHostTableMinWidthClass('wide')).toBe('min-w-[1240px]');
  });
});
