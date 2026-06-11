import { describe, expect, it } from 'vitest';

import {
  getProxmoxHostColumnWidthStyle,
  getProxmoxHostTableMinWidthClass,
  getProxmoxHostVisibleColumnsForLayout,
} from '../proxmoxHostTableModel';

describe('proxmoxHostTableModel', () => {
  it('prioritizes live utilization columns in the mobile host table', () => {
    const columns = getProxmoxHostVisibleColumnsForLayout('mobile');
    const ids = columns.map((column) => column.id);

    expect(ids).toEqual(['node', 'cpu', 'memory', 'disk']);
    expect(getProxmoxHostTableMinWidthClass('mobile')).toBe('min-w-full');
    expect(getProxmoxHostColumnWidthStyle('node', 'mobile', ids)).toEqual({ width: '40%' });
    expect(getProxmoxHostColumnWidthStyle('cpu', 'mobile', ids)).toEqual({ width: '20%' });
    expect(getProxmoxHostColumnWidthStyle('memory', 'mobile', ids)).toEqual({ width: '20%' });
    expect(getProxmoxHostColumnWidthStyle('disk', 'mobile', ids)).toEqual({ width: '20%' });
  });

  it('adds temperature and guest counts before slower-changing metadata on tablet', () => {
    expect(getProxmoxHostVisibleColumnsForLayout('tablet').map((column) => column.id)).toEqual([
      'node',
      'cpu',
      'memory',
      'disk',
      'temp',
      'vms',
      'cts',
    ]);
    expect(getProxmoxHostTableMinWidthClass('tablet')).toBe('min-w-full');
  });

  it('keeps the full host inventory table on compact and wide layouts', () => {
    const compactIds = getProxmoxHostVisibleColumnsForLayout('compact').map((column) => column.id);

    expect(compactIds).toEqual([
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
      'web',
    ]);
    expect(getProxmoxHostColumnWidthStyle('cluster', 'compact', compactIds)).toEqual({
      width: '10.3093%',
    });
  });

  it('fits the container on compact and reserves the fixed floor only on wide', () => {
    // The compact band (900-1440px) covers most laptops. Forcing a 1240px floor
    // there pushed the rightmost column behind a horizontal scroll, so compact
    // now fits its container; only wide keeps the fixed-width floor.
    expect(getProxmoxHostTableMinWidthClass('compact')).toBe('min-w-full');
    expect(getProxmoxHostTableMinWidthClass('wide')).toBe('min-w-[1240px]');
  });
});
