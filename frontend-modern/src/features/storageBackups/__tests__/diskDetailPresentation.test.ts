import { describe, expect, it } from 'vitest';
import {
  getDiskDetailAttributeCards,
  getLinkedDiskHealthDotVariant,
  getLinkedDiskTemperatureTextClass,
} from '@/features/storageBackups/diskDetailPresentation';

describe('diskDetailPresentation', () => {
  it('labels a retained temperature as last known without a health tone', () => {
    const disk = {
      node: 'tower',
      instance: 'cluster-main',
      devPath: '/dev/sda',
      model: 'Archive HDD',
      serial: 'SERIAL-1',
      wwn: '',
      size: 1,
      health: 'PASSED',
      wearout: -1,
      type: 'sata',
      temperature: 72,
      rpm: 7200,
      used: '',
      riskReasons: [] as string[],
    };
    const live = getDiskDetailAttributeCards(
      { ...disk, collection: { temperature: { state: 'available', source: 'host_agent' } } },
      { warning: 50, critical: 55 },
    );
    expect(live).toEqual([{ label: 'Temperature', value: '72°C', ok: false }]);

    const retained = getDiskDetailAttributeCards(
      {
        ...disk,
        collection: {
          temperature: {
            state: 'unavailable',
            source: 'smartctl',
            reason: 'disk is in standby',
          },
        },
      },
      { warning: 50, critical: 55 },
    );
    expect(retained).toEqual([
      { label: 'Last known temperature', value: '72°C', ok: true, lastKnown: true },
    ]);
  });

  it('returns canonical linked-disk state presentation', () => {
    expect(getLinkedDiskHealthDotVariant(true)).toBe('warning');
    expect(getLinkedDiskHealthDotVariant(false)).toBe('success');
    // Default agent thresholds (warning 50, critical 55).
    expect(getLinkedDiskTemperatureTextClass(65)).toBe('text-red-500');
    expect(getLinkedDiskTemperatureTextClass(52)).toBe('text-yellow-500');
    expect(getLinkedDiskTemperatureTextClass(45)).toBe('text-muted');
    // Disk temperature alerting off for the disk: no reading is warm or hot.
    expect(getLinkedDiskTemperatureTextClass(80, null)).toBe('text-muted');
    // Explicit thresholds (e.g. resolved for an NVMe disk) shift the colors.
    expect(getLinkedDiskTemperatureTextClass(55, { warning: 65, critical: 70 })).toBe('text-muted');
    expect(getLinkedDiskTemperatureTextClass(67, { warning: 65, critical: 70 })).toBe(
      'text-yellow-500',
    );
    expect(getLinkedDiskTemperatureTextClass(71, { warning: 65, critical: 70 })).toBe(
      'text-red-500',
    );
  });

  it('builds canonical SATA and NVMe attribute cards', () => {
    expect(
      getDiskDetailAttributeCards({
        node: 'tower',
        instance: 'cluster-main',
        devPath: '/dev/sda',
        model: 'Archive HDD',
        serial: 'SERIAL-1',
        wwn: '',
        size: 1,
        health: 'PASSED',
        wearout: -1,
        type: 'hdd',
        temperature: 42,
        rpm: 7200,
        used: '',
        riskReasons: [],
        smartAttributes: {
          powerOnHours: 100,
          powerCycles: 12,
          reallocatedSectors: 0,
          pendingSectors: 0,
        },
      }),
    ).toEqual(
      expect.arrayContaining([
        { label: 'Power-On Time', value: '4 days', ok: true },
        { label: 'Temperature', value: '42°C', ok: true },
        { label: 'Power Cycles', value: '12', ok: true },
        { label: 'Reallocated Sectors', value: '0', ok: true },
        { label: 'Pending Sectors', value: '0', ok: true },
      ]),
    );

    expect(
      getDiskDetailAttributeCards(
        {
          node: 'tower',
          instance: 'cluster-main',
          devPath: '/dev/nvme0n1',
          model: 'NVMe',
          serial: 'SERIAL-2',
          wwn: '',
          size: 1,
          health: 'PASSED',
          wearout: -1,
          type: 'nvme',
          temperature: 55,
          rpm: 0,
          used: '',
          riskReasons: [],
          smartAttributes: {
            percentageUsed: 12,
            availableSpare: 97,
            mediaErrors: 0,
            unsafeShutdowns: 4,
          },
        },
        { warning: 65, critical: 70 },
      ),
    ).toEqual(
      expect.arrayContaining([
        { label: 'Temperature', value: '55°C', ok: true },
        { label: 'Life Used', value: '12%', ok: true },
        { label: 'Available Spare', value: '97%', ok: true },
        { label: 'Media Errors', value: '0', ok: true },
        { label: 'Unsafe Shutdowns', value: '4', ok: true },
      ]),
    );
  });
});
