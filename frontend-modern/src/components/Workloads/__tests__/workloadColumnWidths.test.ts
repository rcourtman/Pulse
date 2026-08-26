import { describe, expect, it } from 'vitest';

import {
  clampWorkloadColumnWidth,
  shouldEngageColumnResize,
  hasWorkloadColumnWidths,
  isWorkloadManualSizingSupported,
  normalizeWorkloadColumnWidths,
  pruneWorkloadColumnWidths,
  snapshotWorkloadColumnWidths,
  withWorkloadColumnWidth,
  withoutWorkloadColumnWidth,
  seedWorkloadColumnWidthsFromDefaults,
  sumWorkloadColumnWidths,
  workloadColumnWidthsStorageKey,
  WORKLOAD_COLUMN_FALLBACK_WIDTH,
  WORKLOAD_COLUMN_MAX_WIDTH,
  WORKLOAD_COLUMN_MIN_WIDTH,
} from '../workloadColumnWidths';

describe('workloadColumnWidths', () => {
  describe('clampWorkloadColumnWidth', () => {
    it('keeps a column wide enough to stay clickable and narrow enough to stay usable', () => {
      expect(clampWorkloadColumnWidth(1)).toBe(WORKLOAD_COLUMN_MIN_WIDTH);
      expect(clampWorkloadColumnWidth(10_000)).toBe(WORKLOAD_COLUMN_MAX_WIDTH);
      expect(clampWorkloadColumnWidth(180.4)).toBe(180);
    });
  });

  describe('shouldEngageColumnResize', () => {
    it('treats a press with no travel as a click, not a resize', () => {
      // Committing a zero-delta drag would pin every width and switch the
      // table into manual sizing without the operator asking for it.
      expect(shouldEngageColumnResize(0, false)).toBe(false);
      expect(shouldEngageColumnResize(2, false)).toBe(false);
      expect(shouldEngageColumnResize(-2, false)).toBe(false);
    });

    it('engages once the pointer has actually moved, in either direction', () => {
      expect(shouldEngageColumnResize(3, false)).toBe(true);
      expect(shouldEngageColumnResize(-40, false)).toBe(true);
    });

    it('stays engaged for the rest of the drag even when the pointer returns', () => {
      expect(shouldEngageColumnResize(0, true)).toBe(true);
    });

    it('ignores a non-finite delta', () => {
      expect(shouldEngageColumnResize(Number.NaN, false)).toBe(false);
    });
  });

  describe('isWorkloadManualSizingSupported', () => {
    it('is offered on pointer-sized layouts only', () => {
      expect(isWorkloadManualSizingSupported('wide')).toBe(true);
      expect(isWorkloadManualSizingSupported('compact')).toBe(true);
      expect(isWorkloadManualSizingSupported('tablet')).toBe(true);
      expect(isWorkloadManualSizingSupported('mobile')).toBe(false);
      expect(isWorkloadManualSizingSupported('phone')).toBe(false);
      expect(isWorkloadManualSizingSupported('narrow')).toBe(false);
    });
  });

  describe('normalizeWorkloadColumnWidths', () => {
    it('returns an empty map for anything that is not a width record', () => {
      expect(normalizeWorkloadColumnWidths(null)).toEqual({});
      expect(normalizeWorkloadColumnWidths(undefined)).toEqual({});
      expect(normalizeWorkloadColumnWidths('name')).toEqual({});
      expect(normalizeWorkloadColumnWidths(['name'])).toEqual({});
    });

    it('drops unusable entries instead of throwing so one bad key cannot wedge the table', () => {
      expect(
        normalizeWorkloadColumnWidths({
          name: 240,
          cpu: 'not-a-number',
          memory: Number.NaN,
          disk: 0,
          netIo: -20,
          '': 100,
          diskIo: '180',
        }),
      ).toEqual({ name: 240, diskIo: 180 });
    });

    it('clamps persisted widths that fall outside the supported range', () => {
      expect(normalizeWorkloadColumnWidths({ name: 5, cpu: 5_000 })).toEqual({
        name: WORKLOAD_COLUMN_MIN_WIDTH,
        cpu: WORKLOAD_COLUMN_MAX_WIDTH,
      });
    });
  });

  describe('hasWorkloadColumnWidths', () => {
    it('treats an empty or missing map as untouched', () => {
      expect(hasWorkloadColumnWidths(undefined)).toBe(false);
      expect(hasWorkloadColumnWidths({})).toBe(false);
      expect(hasWorkloadColumnWidths({ name: 200 })).toBe(true);
    });
  });

  describe('withWorkloadColumnWidth', () => {
    it('pins a clamped width without mutating the previous map', () => {
      const current = { name: 200 };
      const next = withWorkloadColumnWidth(current, 'cpu', 12);
      expect(next).toEqual({ name: 200, cpu: WORKLOAD_COLUMN_MIN_WIDTH });
      expect(current).toEqual({ name: 200 });
    });

    it('ignores an unusable column id or width', () => {
      const current = { name: 200 };
      expect(withWorkloadColumnWidth(current, '  ', 120)).toBe(current);
      expect(withWorkloadColumnWidth(current, 'cpu', Number.NaN)).toBe(current);
    });
  });

  describe('withoutWorkloadColumnWidth', () => {
    it('clears one pin and leaves the map identical when there was nothing to clear', () => {
      const current = { name: 200, cpu: 140 };
      expect(withoutWorkloadColumnWidth(current, 'cpu')).toEqual({ name: 200 });
      expect(withoutWorkloadColumnWidth(current, 'memory')).toBe(current);
    });
  });

  describe('pruneWorkloadColumnWidths', () => {
    it('drops pins for columns the operator has since hidden', () => {
      expect(pruneWorkloadColumnWidths({ name: 200, cpu: 140 }, ['name'])).toEqual({ name: 200 });
    });

    it('returns the same reference when every pin is still rendered', () => {
      const current = { name: 200, cpu: 140 };
      expect(pruneWorkloadColumnWidths(current, ['name', 'cpu', 'memory'])).toBe(current);
    });
  });

  describe('snapshotWorkloadColumnWidths', () => {
    it('freezes the widths the operator can currently see', () => {
      expect(snapshotWorkloadColumnWidths({}, { name: 302.4, cpu: 151.2 })).toEqual({
        name: 302,
        cpu: 151,
      });
    });

    it('never overwrites a width the operator already pinned', () => {
      expect(snapshotWorkloadColumnWidths({ name: 240 }, { name: 302, cpu: 151 })).toEqual({
        name: 240,
        cpu: 151,
      });
    });

    it('skips columns the browser reported no usable width for', () => {
      expect(snapshotWorkloadColumnWidths({}, { name: 0, cpu: Number.NaN, disk: 140 })).toEqual({
        disk: 140,
      });
    });
  });

  describe('seedWorkloadColumnWidthsFromDefaults', () => {
    it('gives columns that were never on screen their design width', () => {
      expect(
        seedWorkloadColumnWidthsFromDefaults({ name: 172 }, [
          { id: 'name', width: '200px' },
          { id: 'netIo', width: '170px', minWidth: '170px' },
          { id: 'tags', width: '60px' },
        ]),
      ).toEqual({ name: 172, netIo: 170, tags: 60 });
    });

    it('falls back to minWidth, then to a usable default, for columns with no design width', () => {
      expect(
        seedWorkloadColumnWidthsFromDefaults({}, [
          { id: 'a', minWidth: '96px' },
          { id: 'b' },
          { id: 'c', width: '50%' },
        ]),
      ).toEqual({
        a: 96,
        b: WORKLOAD_COLUMN_FALLBACK_WIDTH,
        c: WORKLOAD_COLUMN_FALLBACK_WIDTH,
      });
    });
  });

  describe('sumWorkloadColumnWidths', () => {
    it('publishes the exact total so table-layout: fixed keeps applying', () => {
      expect(sumWorkloadColumnWidths({ name: 172, cpu: 151, memory: 163 }, ['name', 'cpu'])).toBe(
        323,
      );
    });

    it('returns null while nothing is pinned so the percentage layout is untouched', () => {
      expect(sumWorkloadColumnWidths({}, ['name', 'cpu'])).toBeNull();
      expect(sumWorkloadColumnWidths({ name: 172 }, [])).toBeNull();
    });

    it('returns null when any rendered column is still unpinned', () => {
      // A partial total would under-size the table and let the browser
      // redistribute, which is exactly what the pins are meant to prevent.
      expect(sumWorkloadColumnWidths({ name: 172 }, ['name', 'cpu'])).toBeNull();
    });
  });

  describe('workloadColumnWidthsStorageKey', () => {
    it('scopes the key per surface so Proxmox and vSphere do not share widths', () => {
      expect(workloadColumnWidthsStorageKey('workloadsColumnWidths')).toBe('workloadsColumnWidths');
      expect(workloadColumnWidthsStorageKey('workloadsColumnWidths', ' proxmox ')).toBe(
        'workloadsColumnWidths:proxmox',
      );
      expect(workloadColumnWidthsStorageKey('workloadsColumnWidths', '   ')).toBe(
        'workloadsColumnWidths',
      );
    });
  });
});
