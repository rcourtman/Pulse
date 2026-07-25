import { describe, expect, it } from 'vitest';
import type { PhysicalDiskPresentationData } from '@/features/storageBackups/diskPresentation';
import {
  getDiskDetailAttributeCards,
  getDiskDetailHistoryCharts,
  getLinkedDiskTemperatureTextClass,
} from '@/features/storageBackups/diskDetailPresentation';

function makeDiskData(
  overrides: Partial<PhysicalDiskPresentationData> = {},
): PhysicalDiskPresentationData {
  return {
    node: '',
    instance: '',
    devPath: '',
    model: '',
    serial: '',
    wwn: '',
    size: 0,
    health: 'UNKNOWN',
    riskReasons: [],
    wearout: -1,
    type: '',
    temperature: 0,
    rpm: 0,
    used: '',
    ...overrides,
  };
}

describe('diskDetailPresentation.branchcov0724pm', () => {
  describe('getLinkedDiskTemperatureTextClass non-positive / non-finite guard (L20)', () => {
    it('returns text-muted for NaN temperature', () => {
      expect(getLinkedDiskTemperatureTextClass(NaN)).toBe('text-muted');
    });

    it('returns text-muted for zero temperature', () => {
      expect(getLinkedDiskTemperatureTextClass(0)).toBe('text-muted');
    });

    it('short-circuits before consulting thresholds when the guard trips', () => {
      // 0 would be flagged critical by these inverted thresholds, but the
      // guard returns text-muted without ever calling getMetricSeverity.
      expect(getLinkedDiskTemperatureTextClass(0, { warning: -10, critical: -5 })).toBe(
        'text-muted',
      );
    });
  });

  describe('getDiskDetailAttributeCards empty-collection guard (L86)', () => {
    it('returns an empty array when smartAttributes is absent', () => {
      expect(getDiskDetailAttributeCards(makeDiskData({ type: 'hdd' }))).toEqual([]);
    });
  });

  describe('getDiskDetailAttributeCards SATA offlineUncorrectable arm (L129)', () => {
    it('emits an Offline Uncorrectable card marked ok when the count is zero', () => {
      const cards = getDiskDetailAttributeCards(
        makeDiskData({ type: 'hdd', smartAttributes: { offlineUncorrectable: 0 } }),
      );
      expect(cards).toContainEqual({
        label: 'Offline Uncorrectable',
        value: '0',
        ok: true,
      });
    });

    it('emits an Offline Uncorrectable card marked not-ok when the count is non-zero', () => {
      const cards = getDiskDetailAttributeCards(
        makeDiskData({ type: 'hdd', smartAttributes: { offlineUncorrectable: 7 } }),
      );
      expect(cards).toContainEqual({
        label: 'Offline Uncorrectable',
        value: '7',
        ok: false,
      });
    });

    it('omits the Offline Uncorrectable card for an NVMe disk even when the field is set', () => {
      const cards = getDiskDetailAttributeCards(
        makeDiskData({ type: 'nvme', smartAttributes: { offlineUncorrectable: 7 } }),
      );
      expect(cards.find((c) => c.label === 'Offline Uncorrectable')).toBeUndefined();
    });
  });

  describe('getDiskDetailAttributeCards SATA udmaCrcErrors arm (L136)', () => {
    it('emits a CRC Errors card marked ok when the count is zero', () => {
      const cards = getDiskDetailAttributeCards(
        makeDiskData({ type: 'hdd', smartAttributes: { udmaCrcErrors: 0 } }),
      );
      expect(cards).toContainEqual({ label: 'CRC Errors', value: '0', ok: true });
    });

    it('emits a CRC Errors card marked not-ok when the count is non-zero', () => {
      const cards = getDiskDetailAttributeCards(
        makeDiskData({ type: 'hdd', smartAttributes: { udmaCrcErrors: 42 } }),
      );
      expect(cards).toContainEqual({ label: 'CRC Errors', value: '42', ok: false });
    });
  });

  describe('getDiskDetailHistoryCharts SATA reallocated-sectors arm (L192)', () => {
    it('emits only the smart_reallocated_sectors chart for a SATA disk with the field set', () => {
      const charts = getDiskDetailHistoryCharts(
        makeDiskData({
          type: 'hdd',
          temperature: 0,
          smartAttributes: { reallocatedSectors: 3 },
        }),
      );
      expect(charts.map((c) => c.metric)).toEqual(['smart_reallocated_sectors']);
      expect(charts[0]).toMatchObject({
        metric: 'smart_reallocated_sectors',
        label: 'Reallocated Sectors',
        unit: 'sectors',
        color: '#f59e0b',
      });
    });

    it('omits the reallocated-sectors chart for an NVMe disk even when the field is set', () => {
      const charts = getDiskDetailHistoryCharts(
        makeDiskData({ type: 'nvme', smartAttributes: { reallocatedSectors: 3 } }),
      );
      expect(charts.find((c) => c.metric === 'smart_reallocated_sectors')).toBeUndefined();
    });

    it('returns an empty chart list when smartAttributes is absent', () => {
      expect(getDiskDetailHistoryCharts(makeDiskData({ type: 'hdd' }))).toEqual([]);
    });
  });
});
