import { describe, expect, it } from 'vitest';

import {
  buildWorkloadColumnLayoutEntries,
  parseWorkloadColumnLayoutParam,
  resolveWorkloadColumnLayoutEntries,
  serializeWorkloadColumnLayout,
  toggleWorkloadColumnLayoutEntry,
  workloadColumnLayoutIds,
  workloadColumnLayoutWidths,
  WORKLOAD_COLUMNS_URL_PARAM,
} from '../workloadColumnLayoutUrl';
import { WORKLOAD_COLUMN_MAX_WIDTH, WORKLOAD_COLUMN_MIN_WIDTH } from '../workloadColumnWidths';

describe('workloadColumnLayoutUrl', () => {
  it('uses a single compact query parameter', () => {
    expect(WORKLOAD_COLUMNS_URL_PARAM).toBe('cols');
  });

  describe('parseWorkloadColumnLayoutParam', () => {
    it('reads ordered id:width pairs', () => {
      expect(parseWorkloadColumnLayoutParam('name:172,cpu:151,netIo:170')).toEqual([
        { id: 'name', width: 172 },
        { id: 'cpu', width: 151 },
        { id: 'netIo', width: 170 },
      ]);
    });

    it('treats an absent or empty parameter as no layout', () => {
      expect(parseWorkloadColumnLayoutParam(null)).toEqual([]);
      expect(parseWorkloadColumnLayoutParam(undefined)).toEqual([]);
      expect(parseWorkloadColumnLayoutParam('')).toEqual([]);
    });

    it('skips malformed pairs rather than rejecting the whole link', () => {
      expect(
        parseWorkloadColumnLayoutParam('name:172,broken,cpu:,:80,memory:abc,disk:163'),
      ).toEqual([
        { id: 'name', width: 172 },
        { id: 'disk', width: 163 },
      ]);
    });

    it('rejects ids that are not plain column identifiers', () => {
      expect(parseWorkloadColumnLayoutParam('<script>:100,na me:100,name:172')).toEqual([
        { id: 'name', width: 172 },
      ]);
    });

    it('keeps the first entry when a link repeats a column', () => {
      expect(parseWorkloadColumnLayoutParam('name:172,name:400')).toEqual([
        { id: 'name', width: 172 },
      ]);
    });

    it('clamps widths from a hand-edited link', () => {
      expect(parseWorkloadColumnLayoutParam('name:1,cpu:99999')).toEqual([
        { id: 'name', width: WORKLOAD_COLUMN_MIN_WIDTH },
        { id: 'cpu', width: WORKLOAD_COLUMN_MAX_WIDTH },
      ]);
    });

    it('caps how many columns a link can request', () => {
      const raw = Array.from({ length: 60 }, (_, index) => `c${index}:100`).join(',');
      expect(parseWorkloadColumnLayoutParam(raw)).toHaveLength(32);
    });
  });

  describe('serializeWorkloadColumnLayout', () => {
    it('round-trips through the parser', () => {
      const entries = [
        { id: 'name', width: 172 },
        { id: 'netIo', width: 170 },
      ];
      expect(serializeWorkloadColumnLayout(entries)).toBe('name:172,netIo:170');
      expect(parseWorkloadColumnLayoutParam(serializeWorkloadColumnLayout(entries))).toEqual(
        entries,
      );
    });
  });

  describe('resolveWorkloadColumnLayoutEntries', () => {
    it('drops columns this view cannot render but keeps the link order', () => {
      const entries = [
        { id: 'netIo', width: 170 },
        { id: 'vsphereOnly', width: 120 },
        { id: 'name', width: 172 },
      ];
      expect(
        resolveWorkloadColumnLayoutEntries(entries, new Set(['name', 'netIo', 'cpu'])),
      ).toEqual([
        { id: 'netIo', width: 170 },
        { id: 'name', width: 172 },
      ]);
    });
  });

  describe('buildWorkloadColumnLayoutEntries', () => {
    it('emits the rendered order and skips columns with no pinned width', () => {
      expect(
        buildWorkloadColumnLayoutEntries(['name', 'cpu', 'memory'], { name: 172, memory: 163 }),
      ).toEqual([
        { id: 'name', width: 172 },
        { id: 'memory', width: 163 },
      ]);
    });
  });

  describe('workloadColumnLayoutWidths / Ids', () => {
    it('projects entries for the render path', () => {
      const entries = [
        { id: 'name', width: 172 },
        { id: 'cpu', width: 151 },
      ];
      expect(workloadColumnLayoutWidths(entries)).toEqual({ name: 172, cpu: 151 });
      expect(workloadColumnLayoutIds(entries)).toEqual(['name', 'cpu']);
    });
  });

  describe('toggleWorkloadColumnLayoutEntry', () => {
    it('appends a new column at its design width', () => {
      expect(toggleWorkloadColumnLayoutEntry([{ id: 'name', width: 172 }], 'netIo', 170)).toEqual([
        { id: 'name', width: 172 },
        { id: 'netIo', width: 170 },
      ]);
    });

    it('removes a column that is already in the link', () => {
      expect(
        toggleWorkloadColumnLayoutEntry(
          [
            { id: 'name', width: 172 },
            { id: 'netIo', width: 170 },
          ],
          'netIo',
          170,
        ),
      ).toEqual([{ id: 'name', width: 172 }]);
    });

    it('refuses to empty the table', () => {
      expect(toggleWorkloadColumnLayoutEntry([{ id: 'name', width: 172 }], 'name', 200)).toEqual([
        { id: 'name', width: 172 },
      ]);
    });
  });
});
