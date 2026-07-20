import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  alertTypeDisplayLabel,
  getAlertResourceDisplayLabel,
  getLocalTimezone,
  normalizeMetricDelayMap,
  unifiedTypeToAlertDisplayType,
} from '@/features/alerts/helpers';

describe('alerts helpers — branch coverage (batch 2)', () => {
  describe('alertTypeDisplayLabel', () => {
    it('maps the remaining standard metric arms and aliases', () => {
      expect(alertTypeDisplayLabel('disk-usage')).toBe('Disk');
      expect(alertTypeDisplayLabel('usage')).toBe('Usage');
      expect(alertTypeDisplayLabel('network')).toBe('Network');
      expect(alertTypeDisplayLabel('load')).toBe('Load');
    });

    it('maps all three temperature aliases to the same label', () => {
      expect(alertTypeDisplayLabel('temperature')).toBe('Temperature');
      expect(alertTypeDisplayLabel('disk_temperature')).toBe('Temperature');
      expect(alertTypeDisplayLabel('diskTemperature')).toBe('Temperature');
    });

    it('maps the remaining docker-container and docker-host arms', () => {
      expect(alertTypeDisplayLabel('docker-container-cpu')).toBe('Container CPU');
      expect(alertTypeDisplayLabel('docker-container-disk')).toBe('Container Disk');
      expect(alertTypeDisplayLabel('docker-container-update')).toBe('Update Available');
      expect(alertTypeDisplayLabel('docker-host-offline')).toBe('Host Offline');
    });

    it('maps the remaining infrastructure and storage arms', () => {
      expect(alertTypeDisplayLabel('node')).toBe('Node');
      expect(alertTypeDisplayLabel('zfs-device')).toBe('ZFS Device');
      expect(alertTypeDisplayLabel('raid')).toBe('RAID');
      expect(alertTypeDisplayLabel('resource-incident')).toBe('Resource Health');
    });

    it('maps the remaining standalone arms', () => {
      expect(alertTypeDisplayLabel('pbs')).toBe('PBS');
      expect(alertTypeDisplayLabel('message-age')).toBe('Message Age');
    });

    it('title-cases unknown types containing a mix of hyphens and underscores', () => {
      expect(alertTypeDisplayLabel('snapshot_disk-usage')).toBe('Snapshot Disk Usage');
    });

    it('returns the empty string for an empty unknown type', () => {
      expect(alertTypeDisplayLabel('')).toBe('');
    });
  });

  describe('unifiedTypeToAlertDisplayType', () => {
    it('returns the canonical resource-type label for known types', () => {
      expect(unifiedTypeToAlertDisplayType('vm')).toBe('VM');
      expect(unifiedTypeToAlertDisplayType('pbs')).toBe('PBS');
      expect(unifiedTypeToAlertDisplayType('storage')).toBe('Storage');
      expect(unifiedTypeToAlertDisplayType('ceph')).toBe('Ceph');
    });

    it('falls back to the raw type string when no canonical label resolves', () => {
      expect(
        unifiedTypeToAlertDisplayType(
          '' as unknown as Parameters<typeof unifiedTypeToAlertDisplayType>[0],
        ),
      ).toBe('');
    });
  });

  describe('getLocalTimezone', () => {
    afterEach(() => {
      vi.restoreAllMocks();
    });

    it('falls back to UTC when the resolved timezone is empty', () => {
      const spy = vi.spyOn(Intl, 'DateTimeFormat').mockImplementation((() => ({
        resolvedOptions: () => ({ timeZone: '' }),
      })) as unknown as typeof Intl.DateTimeFormat);
      expect(getLocalTimezone()).toBe('UTC');
      spy.mockRestore();
    });

    it('returns the resolved IANA timezone when one is available', () => {
      const spy = vi.spyOn(Intl, 'DateTimeFormat').mockImplementation((() => ({
        resolvedOptions: () => ({ timeZone: 'Australia/Sydney' }),
      })) as unknown as typeof Intl.DateTimeFormat);
      expect(getLocalTimezone()).toBe('Australia/Sydney');
      spy.mockRestore();
    });
  });

  describe('normalizeMetricDelayMap', () => {
    it('returns an empty object for nullish input', () => {
      expect(normalizeMetricDelayMap(undefined)).toEqual({});
      expect(normalizeMetricDelayMap(null)).toEqual({});
    });

    it('returns an empty object for an empty record', () => {
      expect(normalizeMetricDelayMap({})).toEqual({});
    });

    it('trims and lowercases type and metric keys and rounds fractional values', () => {
      const input = { '  VM ': { ' CPU ': 3.6, Memory: 7 } };
      expect(normalizeMetricDelayMap(input)).toEqual({ vm: { cpu: 4, memory: 7 } });
    });

    it('rounds 0.5 up and 0.4 down', () => {
      expect(normalizeMetricDelayMap({ vm: { a: 0.5, b: 0.4 } })).toEqual({ vm: { a: 1, b: 0 } });
    });

    it('skips entries whose metrics value is null', () => {
      expect(
        normalizeMetricDelayMap({ vm: null } as unknown as Parameters<
          typeof normalizeMetricDelayMap
        >[0]),
      ).toEqual({});
    });

    it('skips entries with a whitespace-only type key', () => {
      expect(normalizeMetricDelayMap({ '   ': { cpu: 5 }, vm: { cpu: 1 } })).toEqual({
        vm: { cpu: 1 },
      });
    });

    it('drops non-number, NaN, and negative metric values but keeps zero', () => {
      const input = {
        vm: {
          cpu: 5,
          badString: 'x' as unknown as number,
          nanVal: Number.NaN,
          negative: -1,
          zero: 0,
        },
      };
      expect(normalizeMetricDelayMap(input)).toEqual({ vm: { cpu: 5, zero: 0 } });
    });

    it('skips metric entries with a whitespace-only metric key', () => {
      expect(normalizeMetricDelayMap({ vm: { '  ': 5, cpu: 1 } })).toEqual({ vm: { cpu: 1 } });
    });

    it('omits a type entirely when none of its metrics survive validation', () => {
      expect(
        normalizeMetricDelayMap({
          vm: { onlyBad: -1 },
          host: { cpu: 2 },
        }),
      ).toEqual({ host: { cpu: 2 } });
    });
  });

  describe('getAlertResourceDisplayLabel', () => {
    const makeResource = (overrides: Partial<Resource>): Resource =>
      ({ id: 'r1', name: 'r1', type: 'vm', ...overrides }) as Resource;

    it('returns the preferred display name when it differs from the id', () => {
      const resource = makeResource({ id: 'node-1', displayName: 'Tower' });
      expect(getAlertResourceDisplayLabel(resource)).toBe('Tower');
    });

    it('returns the fallback when the preferred name equals the id', () => {
      const resource = makeResource({ id: 'r1', name: 'r1', displayName: '' });
      expect(getAlertResourceDisplayLabel(resource, 'fallback-label')).toBe('fallback-label');
    });

    it('falls through to the preferred (= id) when no fallback is supplied', () => {
      const resource = makeResource({ id: 'r1', name: 'r1', displayName: '' });
      expect(getAlertResourceDisplayLabel(resource)).toBe('r1');
    });

    it('returns the empty id when nothing resolves and no fallback is given', () => {
      const resource = makeResource({ id: '', name: '', displayName: '' });
      expect(getAlertResourceDisplayLabel(resource)).toBe('');
    });

    it('prefers a non-id display name over a supplied fallback', () => {
      const resource = makeResource({ id: 'r1', displayName: 'My Host' });
      expect(getAlertResourceDisplayLabel(resource, 'ignored')).toBe('My Host');
    });
  });
});
