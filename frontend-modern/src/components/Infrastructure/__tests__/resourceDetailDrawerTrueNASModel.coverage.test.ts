import { describe, expect, it } from 'vitest';
import {
  buildTrueNASDetailSections,
  type ResourceDetailDrawerTrueNASRow,
  type ResourceDetailDrawerTrueNASRowTone,
} from '@/components/Infrastructure/resourceDetailDrawerTrueNASModel';
import type {
  Resource,
  ResourcePhysicalDiskMeta,
  ResourceStorageMeta,
  ResourceTrueNASServiceMeta,
} from '@/types/resource';

// All target helpers are module-private, so every case drives them through the
// public `buildTrueNASDetailSections` entry point and asserts on the rendered
// sections/rows. `baseResource` mirrors the sibling test's factory.

const baseResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: 'truenas-resource',
    type: 'vm',
    name: 'truenas-resource',
    displayName: 'TrueNAS resource',
    platformId: 'truenas-main',
    platformType: 'truenas',
    sourceType: 'api',
    status: 'online',
    ...overrides,
  }) as Resource;

const sectionLabels = (resource: Resource): string[] =>
  buildTrueNASDetailSections(resource).map((section) => section.label);

const allRows = (resource: Resource): ResourceDetailDrawerTrueNASRow[] =>
  buildTrueNASDetailSections(resource).flatMap((section) => section.rows);

const findRow = (resource: Resource, label: string): ResourceDetailDrawerTrueNASRow | undefined =>
  allRows(resource).find((row) => row.label === label);

const systemWithService = (service: ResourceTrueNASServiceMeta): Resource =>
  baseResource({ type: 'agent', truenas: { services: [service] } });

const diskResource = (disk: ResourcePhysicalDiskMeta): Resource =>
  baseResource({ physicalDisk: disk });

const storageResource = (storage: ResourceStorageMeta): Resource => baseResource({ storage });

describe('truenasServiceStatus / serviceStatusLabel / serviceStatusTone', () => {
  // A single service must resolve to exactly one status row; its label and tone
  // come straight from serviceStatusLabel / serviceStatusTone, and its presence
  // is driven by truenasServiceStatus.
  const STATUS_LABELS = ['Running', 'Attention', 'Stopped', 'Disabled'] as const;

  type Case = {
    name: string;
    state?: string;
    enabled?: boolean;
    status: (typeof STATUS_LABELS)[number];
    tone: ResourceDetailDrawerTrueNASRowTone;
  };

  const cases: Case[] = [
    {
      name: 'running token wins over enabled:false (precedence)',
      state: 'running',
      enabled: false,
      status: 'Running',
      tone: 'success',
    },
    { name: 'started -> running', state: 'started', status: 'Running', tone: 'success' },
    { name: 'active -> running', state: 'active', status: 'Running', tone: 'success' },
    { name: 'failed -> attention', state: 'failed', status: 'Attention', tone: 'warning' },
    { name: 'error -> attention', state: 'error', status: 'Attention', tone: 'warning' },
    { name: 'crashed -> attention', state: 'crashed', status: 'Attention', tone: 'warning' },
    { name: 'degraded -> attention', state: 'degraded', status: 'Attention', tone: 'warning' },
    { name: 'unknown -> attention', state: 'unknown', status: 'Attention', tone: 'warning' },
    {
      name: 'stop + enabled -> stopped',
      state: 'stop',
      enabled: true,
      status: 'Stopped',
      tone: 'warning',
    },
    {
      name: 'inactive + enabled -> stopped',
      state: 'inactive',
      enabled: true,
      status: 'Stopped',
      tone: 'warning',
    },
    {
      name: 'stop + disabled -> disabled',
      state: 'stop',
      enabled: false,
      status: 'Disabled',
      tone: 'default',
    },
    {
      name: 'inactive + disabled -> disabled',
      state: 'inactive',
      enabled: false,
      status: 'Disabled',
      tone: 'default',
    },
    {
      name: 'unknown state + enabled -> attention (fallback)',
      state: 'restarting',
      enabled: true,
      status: 'Attention',
      tone: 'warning',
    },
    {
      name: 'unknown state + enabled undefined -> attention (fallback)',
      state: 'restarting',
      status: 'Attention',
      tone: 'warning',
    },
    {
      name: 'unknown state + disabled -> disabled (fallback)',
      state: 'restarting',
      enabled: false,
      status: 'Disabled',
      tone: 'default',
    },
    {
      name: 'empty state + disabled -> disabled (fallback)',
      state: '',
      enabled: false,
      status: 'Disabled',
      tone: 'default',
    },
    {
      name: 'whitespace-only state + disabled -> disabled (fallback)',
      state: '   ',
      enabled: false,
      status: 'Disabled',
      tone: 'default',
    },
    {
      name: 'missing state + enabled -> attention (fallback)',
      enabled: true,
      status: 'Attention',
      tone: 'warning',
    },
  ];

  for (const testCase of cases) {
    it(testCase.name, () => {
      const resource = systemWithService({
        id: '1',
        service: 'svc',
        enabled: testCase.enabled,
        state: testCase.state,
      });

      const presentStatusRows = STATUS_LABELS.filter((label) => Boolean(findRow(resource, label)));
      expect(presentStatusRows).toEqual([testCase.status]);

      const row = findRow(resource, testCase.status);
      expect(row?.value).toBe('1');
      expect(row?.tone).toBe(testCase.tone);
    });
  }
});

describe('storageKindLabel', () => {
  it('prefers topology over type', () => {
    expect(findRow(storageResource({ topology: 'dataset', type: 'zfs_pool' }), 'Kind')?.value).toBe(
      'Dataset',
    );
  });

  it('falls back to type when topology is absent', () => {
    expect(findRow(storageResource({ type: 'zfs_pool' }), 'Kind')?.value).toBe('Zfs Pool');
  });

  it('returns null when both topology and type are absent', () => {
    expect(findRow(storageResource({}), 'Kind')).toBeUndefined();
  });
});

describe('diskStateTone', () => {
  it('maps passed/healthy/ok to success', () => {
    expect(findRow(diskResource({ health: 'PASSED' }), 'Health')?.tone).toBe('success');
    expect(findRow(diskResource({ health: 'OK' }), 'Health')?.tone).toBe('success');
  });

  it('maps failed to warning', () => {
    expect(findRow(diskResource({ health: 'FAILED' }), 'Health')?.tone).toBe('warning');
  });

  it('maps unrecognized health to default', () => {
    expect(findRow(diskResource({ health: 'SCRUBBED' }), 'Health')?.tone).toBe('default');
  });

  it('omits the Health row when health is missing (default tone still computed)', () => {
    expect(findRow(diskResource({}), 'Health')).toBeUndefined();
  });
});

describe('formatDiskHours', () => {
  it('formats positive hours with thousands separators and the h suffix', () => {
    expect(findRow(diskResource({ smart: { powerOnHours: 1_234_567 } }), 'Power on')?.value).toBe(
      '1,234,567h',
    );
  });

  it('formats a single-digit hour value', () => {
    expect(findRow(diskResource({ smart: { powerOnHours: 1 } }), 'Power on')?.value).toBe('1h');
  });

  const nullCases: Array<[string, number | undefined]> = [
    ['undefined', undefined],
    ['zero', 0],
    ['negative', -5],
    ['NaN', Number.NaN],
  ];
  for (const [label, powerOnHours] of nullCases) {
    it(`omits the Power on row for a non-positive hour value (${label})`, () => {
      expect(findRow(diskResource({ smart: { powerOnHours } }), 'Power on')).toBeUndefined();
    });
  }

  it('omits the Power on row for a non-number value', () => {
    const resource = diskResource({
      smart: { powerOnHours: 'lots' as unknown as number },
    });
    expect(findRow(resource, 'Power on')).toBeUndefined();
  });
});

describe('buildTrueNAS sections empty-input guards', () => {
  it('buildTrueNASSystemSections: minimal truenas yields only the System section', () => {
    const resource = baseResource({ type: 'agent', truenas: {} });

    expect(sectionLabels(resource)).toEqual(['System']);
    // Hostname falls back to resource.name; Status falls back to resource.status.
    expect(findRow(resource, 'Hostname')?.value).toBe('truenas-resource');
    expect(findRow(resource, 'Status')?.value).toBe('Online');
    // Empty services/pids/names guards drop every Services-section row.
    expect(findRow(resource, 'Services')).toBeUndefined();
    expect(findRow(resource, 'Running')).toBeUndefined();
    expect(findRow(resource, 'Attention')).toBeUndefined();
    expect(findRow(resource, 'Stopped')).toBeUndefined();
    expect(findRow(resource, 'Disabled')).toBeUndefined();
    expect(findRow(resource, 'PIDs')).toBeUndefined();
    expect(findRow(resource, 'Names')).toBeUndefined();
  });

  it('buildTrueNASStorageSections: minimal storage yields only the Storage section', () => {
    const resource = baseResource({ storage: {} });

    expect(sectionLabels(resource)).toEqual(['Storage']);
    // State derives from resource.status when no zfs/array state is set.
    expect(findRow(resource, 'State')).toEqual({
      label: 'State',
      value: 'Online',
      tone: 'success',
    });
    expect(findRow(resource, 'Kind')).toBeUndefined();
    expect(findRow(resource, 'Shared')).toBeUndefined();
  });

  it('buildTrueNASDiskSections: minimal disk yields only the Disk section', () => {
    const resource = baseResource({ physicalDisk: {} });

    expect(sectionLabels(resource)).toEqual(['Disk']);
    // Device falls back to resource.name.
    expect(findRow(resource, 'Device')?.value).toBe('truenas-resource');
    expect(findRow(resource, 'Health')).toBeUndefined();
    expect(findRow(resource, 'Power on')).toBeUndefined();
  });

  it('buildTrueNASAppSections: a bare app yields no sections', () => {
    expect(sectionLabels(baseResource({ truenas: { app: {} } }))).toEqual([]);
  });

  it('buildTrueNASVMSections: a bare vm yields no sections', () => {
    expect(sectionLabels(baseResource({ truenas: { vm: {} } }))).toEqual([]);
  });

  it('buildTrueNASShareSections: a bare share yields only the Share section with a default Enabled state', () => {
    const resource = baseResource({ truenas: { share: {} } });

    expect(sectionLabels(resource)).toEqual(['Share']);
    expect(findRow(resource, 'State')).toEqual({
      label: 'State',
      value: 'Enabled',
      tone: 'success',
    });
  });
});

describe('yesNoValue (via the Shared row)', () => {
  it('returns "Yes" for true', () => {
    expect(findRow(storageResource({ shared: true }), 'Shared')?.value).toBe('Yes');
  });

  it('returns "No" for false', () => {
    expect(findRow(storageResource({ shared: false }), 'Shared')?.value).toBe('No');
  });

  it('returns null (row omitted) for undefined', () => {
    expect(findRow(storageResource({}), 'Shared')).toBeUndefined();
  });
});

describe('booleanValue (via the VM Autostart flag)', () => {
  it('returns "Enabled" for true', () => {
    const resource = baseResource({ truenas: { vm: { autostart: true } } });
    expect(sectionLabels(resource)).toEqual(['Flags']);
    expect(findRow(resource, 'Autostart')?.value).toBe('Enabled');
  });

  it('returns "Disabled" for false', () => {
    expect(
      findRow(baseResource({ truenas: { vm: { autostart: false } } }), 'Autostart')?.value,
    ).toBe('Disabled');
  });

  it('returns null (row omitted) for undefined', () => {
    const resource = baseResource({ truenas: { vm: { state: 'running' } } });
    expect(findRow(resource, 'Autostart')).toBeUndefined();
  });
});
