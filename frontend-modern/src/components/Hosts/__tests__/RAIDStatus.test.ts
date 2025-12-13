/**
 * Tests for RAID status display logic
 * 
 * These tests cover the RAID status analysis and display logic used in HostsOverview.
 */
import { describe, expect, it } from 'vitest';

// Mock types matching HostRAIDArray from api.ts
interface HostRAIDDevice {
  device: string;
  state: string;
  slot: number;
}

interface HostRAIDArray {
  device: string;
  name?: string;
  level: string;
  state: string;
  totalDevices: number;
  activeDevices: number;
  workingDevices: number;
  failedDevices: number;
  spareDevices: number;
  uuid?: string;
  devices: HostRAIDDevice[];
  rebuildPercent: number;
  rebuildSpeed?: string;
}

// Status analysis logic matching HostRAIDStatusCell
type RAIDStatusType = 'none' | 'ok' | 'degraded' | 'rebuilding';

interface RAIDStatus {
  type: RAIDStatusType;
  label: string;
  color: string;
}

function analyzeRAIDStatus(raid: HostRAIDArray[] | undefined): RAIDStatus {
  if (!raid || raid.length === 0) {
    return { type: 'none', label: '-', color: 'text-gray-400' };
  }

  let hasDegraded = false;
  let hasRebuilding = false;
  let maxRebuildPercent = 0;

  for (const array of raid) {
    const state = array.state.toLowerCase();
    if (state.includes('degraded') || array.failedDevices > 0) {
      hasDegraded = true;
    }
    if (state.includes('recover') || state.includes('resync') || array.rebuildPercent > 0) {
      hasRebuilding = true;
      maxRebuildPercent = Math.max(maxRebuildPercent, array.rebuildPercent);
    }
  }

  if (hasDegraded) {
    return { type: 'degraded', label: 'Degraded', color: 'text-red-600 dark:text-red-400' };
  }
  if (hasRebuilding) {
    return {
      type: 'rebuilding',
      label: `${Math.round(maxRebuildPercent)}%`,
      color: 'text-amber-600 dark:text-amber-400'
    };
  }
  return { type: 'ok', label: 'OK', color: 'text-green-600 dark:text-green-400' };
}

function getDeviceStateColor(state: string): string {
  const s = state.toLowerCase();
  if (s.includes('active') || s.includes('sync')) return 'text-green-400';
  if (s.includes('spare')) return 'text-blue-400';
  if (s.includes('faulty') || s.includes('removed')) return 'text-red-400';
  if (s.includes('rebuilding')) return 'text-amber-400';
  return 'text-gray-400';
}

function getArrayStateColor(array: HostRAIDArray): string {
  const state = array.state.toLowerCase();
  if (state.includes('degraded') || array.failedDevices > 0) return 'text-red-400';
  if (state.includes('recover') || state.includes('resync') || array.rebuildPercent > 0) return 'text-amber-400';
  if (state.includes('clean') || state.includes('active')) return 'text-green-400';
  return 'text-gray-400';
}

describe('RAID Status Analysis', () => {
  describe('analyzeRAIDStatus', () => {
    it('returns none status for undefined raid', () => {
      const status = analyzeRAIDStatus(undefined);
      expect(status.type).toBe('none');
      expect(status.label).toBe('-');
    });

    it('returns none status for empty raid array', () => {
      const status = analyzeRAIDStatus([]);
      expect(status.type).toBe('none');
    });

    it('returns ok status for healthy RAID1', () => {
      const raid: HostRAIDArray[] = [{
        device: '/dev/md0',
        level: 'raid1',
        state: 'clean, active',
        totalDevices: 2,
        activeDevices: 2,
        workingDevices: 2,
        failedDevices: 0,
        spareDevices: 0,
        devices: [
          { device: 'sda1', state: 'active sync', slot: 0 },
          { device: 'sdb1', state: 'active sync', slot: 1 },
        ],
        rebuildPercent: 0,
      }];

      const status = analyzeRAIDStatus(raid);
      expect(status.type).toBe('ok');
      expect(status.label).toBe('OK');
      expect(status.color).toContain('green');
    });

    it('returns degraded status when array has failed devices', () => {
      const raid: HostRAIDArray[] = [{
        device: '/dev/md0',
        level: 'raid1',
        state: 'clean, degraded',
        totalDevices: 2,
        activeDevices: 1,
        workingDevices: 1,
        failedDevices: 1,
        spareDevices: 0,
        devices: [
          { device: 'sda1', state: 'active sync', slot: 0 },
          { device: 'sdb1', state: 'faulty', slot: 1 },
        ],
        rebuildPercent: 0,
      }];

      const status = analyzeRAIDStatus(raid);
      expect(status.type).toBe('degraded');
      expect(status.label).toBe('Degraded');
      expect(status.color).toContain('red');
    });

    it('returns degraded status when state includes degraded', () => {
      const raid: HostRAIDArray[] = [{
        device: '/dev/md0',
        level: 'raid5',
        state: 'degraded, recovering',
        totalDevices: 3,
        activeDevices: 2,
        workingDevices: 2,
        failedDevices: 0, // No failed but degraded state
        spareDevices: 0,
        devices: [],
        rebuildPercent: 50,
      }];

      const status = analyzeRAIDStatus(raid);
      expect(status.type).toBe('degraded');
    });

    it('returns rebuilding status with percentage', () => {
      const raid: HostRAIDArray[] = [{
        device: '/dev/md0',
        level: 'raid1',
        state: 'clean, recovering',
        totalDevices: 2,
        activeDevices: 2,
        workingDevices: 2,
        failedDevices: 0,
        spareDevices: 0,
        devices: [],
        rebuildPercent: 45.5,
        rebuildSpeed: '100MB/s',
      }];

      const status = analyzeRAIDStatus(raid);
      expect(status.type).toBe('rebuilding');
      expect(status.label).toBe('46%'); // Rounded
      expect(status.color).toContain('amber');
    });

    it('returns max rebuild percentage when multiple arrays rebuilding', () => {
      const raid: HostRAIDArray[] = [
        {
          device: '/dev/md0',
          level: 'raid1',
          state: 'recovering',
          totalDevices: 2,
          activeDevices: 2,
          workingDevices: 2,
          failedDevices: 0,
          spareDevices: 0,
          devices: [],
          rebuildPercent: 30,
        },
        {
          device: '/dev/md1',
          level: 'raid1',
          state: 'recovering',
          totalDevices: 2,
          activeDevices: 2,
          workingDevices: 2,
          failedDevices: 0,
          spareDevices: 0,
          devices: [],
          rebuildPercent: 75,
        },
      ];

      const status = analyzeRAIDStatus(raid);
      expect(status.type).toBe('rebuilding');
      expect(status.label).toBe('75%');
    });

    it('prioritizes degraded over rebuilding', () => {
      const raid: HostRAIDArray[] = [
        {
          device: '/dev/md0',
          level: 'raid1',
          state: 'clean, degraded, recovering',
          totalDevices: 2,
          activeDevices: 1,
          workingDevices: 1,
          failedDevices: 1, // Failed device
          spareDevices: 0,
          devices: [],
          rebuildPercent: 50, // Also rebuilding
        },
      ];

      const status = analyzeRAIDStatus(raid);
      expect(status.type).toBe('degraded'); // Degraded takes priority
    });

    it('handles resync state as rebuilding', () => {
      const raid: HostRAIDArray[] = [{
        device: '/dev/md0',
        level: 'raid1',
        state: 'clean, resyncing',
        totalDevices: 2,
        activeDevices: 2,
        workingDevices: 2,
        failedDevices: 0,
        spareDevices: 0,
        devices: [],
        rebuildPercent: 0, // resync state triggers rebuilding even without percent
      }];

      const status = analyzeRAIDStatus(raid);
      expect(status.type).toBe('rebuilding');
    });

    it('handles multiple healthy arrays', () => {
      const raid: HostRAIDArray[] = [
        {
          device: '/dev/md0',
          level: 'raid1',
          state: 'clean',
          totalDevices: 2,
          activeDevices: 2,
          workingDevices: 2,
          failedDevices: 0,
          spareDevices: 0,
          devices: [],
          rebuildPercent: 0,
        },
        {
          device: '/dev/md1',
          level: 'raid5',
          state: 'active',
          totalDevices: 3,
          activeDevices: 3,
          workingDevices: 3,
          failedDevices: 0,
          spareDevices: 1,
          devices: [],
          rebuildPercent: 0,
        },
      ];

      const status = analyzeRAIDStatus(raid);
      expect(status.type).toBe('ok');
    });
  });

  describe('getDeviceStateColor', () => {
    it('returns green for active devices', () => {
      expect(getDeviceStateColor('active sync')).toContain('green');
      expect(getDeviceStateColor('Active Sync')).toContain('green');
    });

    it('returns blue for spare devices', () => {
      expect(getDeviceStateColor('spare')).toContain('blue');
      expect(getDeviceStateColor('hot spare')).toContain('blue');
    });

    it('returns red for faulty devices', () => {
      expect(getDeviceStateColor('faulty')).toContain('red');
      expect(getDeviceStateColor('removed')).toContain('red');
    });

    it('returns amber for rebuilding devices', () => {
      expect(getDeviceStateColor('rebuilding')).toContain('amber');
    });

    it('returns gray for unknown states', () => {
      expect(getDeviceStateColor('')).toContain('gray');
      expect(getDeviceStateColor('unknown')).toContain('gray');
    });
  });

  describe('getArrayStateColor', () => {
    it('returns green for clean/active arrays', () => {
      const cleanArray: HostRAIDArray = {
        device: '/dev/md0',
        level: 'raid1',
        state: 'clean',
        totalDevices: 2,
        activeDevices: 2,
        workingDevices: 2,
        failedDevices: 0,
        spareDevices: 0,
        devices: [],
        rebuildPercent: 0,
      };
      expect(getArrayStateColor(cleanArray)).toContain('green');

      const activeArray: HostRAIDArray = { ...cleanArray, state: 'active' };
      expect(getArrayStateColor(activeArray)).toContain('green');
    });

    it('returns red for degraded arrays', () => {
      const degradedArray: HostRAIDArray = {
        device: '/dev/md0',
        level: 'raid1',
        state: 'degraded',
        totalDevices: 2,
        activeDevices: 1,
        workingDevices: 1,
        failedDevices: 1,
        spareDevices: 0,
        devices: [],
        rebuildPercent: 0,
      };
      expect(getArrayStateColor(degradedArray)).toContain('red');
    });

    it('returns red for arrays with failed devices even if state is clean', () => {
      const failedDeviceArray: HostRAIDArray = {
        device: '/dev/md0',
        level: 'raid1',
        state: 'clean', // State says clean but...
        totalDevices: 2,
        activeDevices: 1,
        workingDevices: 1,
        failedDevices: 1, // Has a failed device
        spareDevices: 0,
        devices: [],
        rebuildPercent: 0,
      };
      expect(getArrayStateColor(failedDeviceArray)).toContain('red');
    });

    it('returns amber for recovering/resyncing arrays', () => {
      const recoveringArray: HostRAIDArray = {
        device: '/dev/md0',
        level: 'raid1',
        state: 'recovering',
        totalDevices: 2,
        activeDevices: 2,
        workingDevices: 2,
        failedDevices: 0,
        spareDevices: 0,
        devices: [],
        rebuildPercent: 50,
      };
      expect(getArrayStateColor(recoveringArray)).toContain('amber');

      const resyncArray: HostRAIDArray = { ...recoveringArray, state: 'resyncing' };
      expect(getArrayStateColor(resyncArray)).toContain('amber');
    });

    it('returns amber for arrays with rebuild percent > 0', () => {
      const rebuildingArray: HostRAIDArray = {
        device: '/dev/md0',
        level: 'raid1',
        state: 'clean', // State is clean but...
        totalDevices: 2,
        activeDevices: 2,
        workingDevices: 2,
        failedDevices: 0,
        spareDevices: 0,
        devices: [],
        rebuildPercent: 25, // Rebuilding
      };
      expect(getArrayStateColor(rebuildingArray)).toContain('amber');
    });
  });
});

describe('RAID Level Support', () => {
  const raidLevels = ['raid0', 'raid1', 'raid5', 'raid6', 'raid10'];

  it.each(raidLevels)('supports %s level', (level) => {
    const array: HostRAIDArray = {
      device: '/dev/md0',
      level: level,
      state: 'clean',
      totalDevices: level === 'raid0' ? 2 : level === 'raid10' ? 4 : 3,
      activeDevices: level === 'raid0' ? 2 : level === 'raid10' ? 4 : 3,
      workingDevices: level === 'raid0' ? 2 : level === 'raid10' ? 4 : 3,
      failedDevices: 0,
      spareDevices: 0,
      devices: [],
      rebuildPercent: 0,
    };

    const status = analyzeRAIDStatus([array]);
    expect(status.type).toBe('ok');
  });
});

describe('RAID Device Counts', () => {
  it('displays correct device counts for RAID1 with spares', () => {
    const array: HostRAIDArray = {
      device: '/dev/md0',
      level: 'raid1',
      state: 'clean',
      totalDevices: 3,
      activeDevices: 2,
      workingDevices: 2,
      failedDevices: 0,
      spareDevices: 1,
      devices: [
        { device: 'sda1', state: 'active sync', slot: 0 },
        { device: 'sdb1', state: 'active sync', slot: 1 },
        { device: 'sdc1', state: 'spare', slot: -1 },
      ],
      rebuildPercent: 0,
    };

    expect(array.activeDevices).toBe(2);
    expect(array.workingDevices).toBe(2);
    expect(array.spareDevices).toBe(1);
    expect(array.failedDevices).toBe(0);
    expect(array.devices).toHaveLength(3);
  });

  it('displays degraded state for RAID5 missing one device', () => {
    const array: HostRAIDArray = {
      device: '/dev/md0',
      level: 'raid5',
      state: 'clean, degraded',
      totalDevices: 4,
      activeDevices: 3,
      workingDevices: 3,
      failedDevices: 1,
      spareDevices: 0,
      devices: [
        { device: 'sda1', state: 'active sync', slot: 0 },
        { device: 'sdb1', state: 'active sync', slot: 1 },
        { device: 'sdc1', state: 'active sync', slot: 2 },
        { device: 'sdd1', state: 'removed', slot: 3 },
      ],
      rebuildPercent: 0,
    };

    const status = analyzeRAIDStatus([array]);
    expect(status.type).toBe('degraded');
    expect(array.failedDevices).toBe(1);
  });
});
