import { describe, expect, it } from 'vitest';
import {
  buildVMwareDetailSections,
  buildVMwareDetailsSummary,
} from '@/components/Infrastructure/resourceDetailDrawerVmwareModel';
import type { DetailRow } from '@/components/shared/detailSectionModel';
import type {
  ResourceVMwareHardware,
  ResourceVMwareMeta,
  ResourceVMwareNetworkAdapter,
  ResourceVMwareSnapshot,
  ResourceVMwareTools,
  ResourceVMwareVirtualDisk,
} from '@/types/resource';

// Every target helper (formatVirtualDiskAddress, adapterNetworkName, etc.) is
// module-private, so each case drives it through the two public entry points
// (buildVMwareDetailsSummary / buildVMwareDetailSections) and asserts on the
// rendered summary string or section/row shape. Fixtures are built minimal so
// each branch is exercised in isolation.

type Sections = ReturnType<typeof buildVMwareDetailSections>;

const findSection = (sections: Sections, label: string) =>
  sections.find((section) => section.label === label);

const findRow = (
  sections: Sections,
  sectionLabel: string,
  rowLabel: string,
): DetailRow | undefined =>
  findSection(sections, sectionLabel)?.rows.find((row) => row.label === rowLabel);

const vmSections = (vmware?: Partial<ResourceVMwareMeta>): Sections =>
  buildVMwareDetailSections('vm', (vmware ?? {}) as ResourceVMwareMeta);

const networkRow = (adapter: Partial<ResourceVMwareNetworkAdapter>): DetailRow | undefined =>
  findSection(vmSections({ networkAdapters: [adapter as ResourceVMwareNetworkAdapter] }), 'Network')
    ?.rows[0];

const diskRow = (disk: Partial<ResourceVMwareVirtualDisk>): DetailRow | undefined =>
  findSection(vmSections({ virtualDisks: [disk as ResourceVMwareVirtualDisk] }), 'Virtual disks')
    ?.rows[0];

const snapshotRows = (snapshots: ResourceVMwareSnapshot[]): DetailRow[] =>
  findSection(vmSections({ snapshotTree: snapshots }), 'Snapshot tree')?.rows ?? [];

const hardwareRow = (vmware: Partial<ResourceVMwareMeta>, label: string): DetailRow | undefined =>
  findRow(vmSections(vmware), 'Virtual hardware', label);

const toolsRow = (vmware: Partial<ResourceVMwareMeta>, label: string): DetailRow | undefined =>
  findRow(vmSections(vmware), 'VMware Tools', label);

const summary = (vmware?: Partial<ResourceVMwareMeta>): string | null =>
  buildVMwareDetailsSummary('vm', (vmware ?? {}) as ResourceVMwareMeta);

const summaryWithTools = (tools: Partial<ResourceVMwareTools>): string | null => summary({ tools });

const summaryWithHardware = (hardware: Partial<ResourceVMwareHardware>): string | null =>
  summary({ hardware });

describe('formatVirtualDiskAddress (via Virtual disks section)', () => {
  it('formats a SATA bus:unit address', () => {
    expect(
      diskRow({ type: 'SATA', sataBus: 1, sataUnit: 2, capacityBytes: 10_737_418_240 })?.value,
    ).toBe('SATA 1:2 · 10 GB');
  });

  it('formats an NVMe bus:unit address', () => {
    expect(
      diskRow({ type: 'NVME', nvmeBus: 0, nvmeUnit: 3, capacityBytes: 10_737_418_240 })?.value,
    ).toBe('NVMe 0:3 · 10 GB');
  });

  it('formats an IDE primary master address', () => {
    expect(
      diskRow({ type: 'IDE', idePrimary: true, ideMaster: true, capacityBytes: 10_737_418_240 })
        ?.value,
    ).toBe('IDE primary master · 10 GB');
  });

  it('formats an IDE secondary slave address', () => {
    expect(
      diskRow({ type: 'IDE', idePrimary: false, ideMaster: false, capacityBytes: 10_737_418_240 })
        ?.value,
    ).toBe('IDE secondary slave · 10 GB');
  });

  it('falls back to the bare type when SCSI bus fields are missing', () => {
    expect(diskRow({ type: 'SCSI', capacityBytes: 10_737_418_240 })?.value).toBe('SCSI · 10 GB');
  });

  it('falls back to the uppercased type for an unrecognized controller', () => {
    expect(diskRow({ type: 'custom', capacityBytes: 10_737_418_240 })?.value).toBe(
      'CUSTOM · 10 GB',
    );
  });
});

describe('buildVMwareDetailsSummary', () => {
  it('returns null when vmware is undefined', () => {
    expect(buildVMwareDetailsSummary('vm', undefined)).toBeNull();
  });

  it('falls back to vcenterHost when connectionName is absent', () => {
    expect(buildVMwareDetailsSummary('vm', { vcenterHost: 'vc-01.lab' })).toBe(
      'vc-01.lab · Read-only vCenter context',
    );
  });

  it('omits the connection prefix when both connectionName and vcenterHost are blank', () => {
    expect(buildVMwareDetailsSummary('vm', {})).toBe('Read-only vCenter context');
  });

  it('uses the explicit snapshotCount number in preference to the tree', () => {
    expect(
      buildVMwareDetailsSummary('vm', {
        snapshotCount: 3,
        snapshotTree: [{ snapshot: 's-1' }],
      }),
    ).toBe('Read-only vCenter context · 3 snapshots');
  });

  it('counts the snapshot tree when snapshotCount is not a number', () => {
    expect(
      buildVMwareDetailsSummary('vm', {
        snapshotTree: [{ snapshot: 's-1', children: [{ snapshot: 's-2' }, { snapshot: 's-3' }] }],
      }),
    ).toBe('Read-only vCenter context · 3 snapshots');
  });

  it('clamps a negative snapshotCount to zero and omits the snapshot part', () => {
    expect(buildVMwareDetailsSummary('vm', { snapshotCount: -1 })).toBe(
      'Read-only vCenter context',
    );
  });

  it('includes vNIC and disk counts for a vm', () => {
    expect(
      buildVMwareDetailsSummary('vm', {
        networkAdapters: [{ nic: '1' }],
        virtualDisks: [{ disk: '1', type: 'SCSI' }],
      }),
    ).toBe('Read-only vCenter context · 1 vNIC · 1 disk');
  });

  it('includes network host/vm counts via names', () => {
    expect(
      buildVMwareDetailsSummary('network', {
        networkHostNames: ['h1', 'h2'],
        networkVmNames: ['vm1'],
      }),
    ).toBe('Read-only vCenter context · 2 hosts · 1 VM');
  });

  it('falls back to network host/vm ids when names are absent', () => {
    expect(
      buildVMwareDetailsSummary('network', {
        networkHostIds: ['h-1', 'h-2', 'h-3'],
        networkVmIds: ['vm-1', 'vm-2'],
      }),
    ).toBe('Read-only vCenter context · 3 hosts · 2 VMs');
  });

  it('includes alarm and task counts', () => {
    expect(buildVMwareDetailsSummary('vm', { activeAlarmCount: 2, recentTaskCount: 5 })).toBe(
      'Read-only vCenter context · 2 alarms · 5 tasks',
    );
  });

  it('omits hardware and tools parts for a non-vm resource type', () => {
    expect(buildVMwareDetailsSummary('datastore', { hardware: { version: 'VMX_19' } })).toBe(
      'Read-only vCenter context',
    );
  });
});

describe('hardwareSummary (via buildVMwareDetailsSummary)', () => {
  it('surfaces a non-default upgrade status as "Hardware <status>"', () => {
    expect(summaryWithHardware({ upgradeStatus: 'FAILED' })).toBe(
      'Read-only vCenter context · Hardware failed',
    );
  });

  it('falls back to the hardware version when upgrade status is NONE', () => {
    expect(summaryWithHardware({ version: 'VMX_17', upgradeStatus: 'NONE' })).toBe(
      'Read-only vCenter context · VMX 17',
    );
  });

  it('contributes no hardware part when hardware is absent', () => {
    expect(summary()).toBe('Read-only vCenter context');
  });
});

describe('toolsSummary (via buildVMwareDetailsSummary)', () => {
  it('surfaces a non-current version status', () => {
    expect(summaryWithTools({ versionStatus: 'OUT_OF_DATE' })).toBe(
      'Read-only vCenter context · Tools out of date',
    );
  });

  it('surfaces the run state when the version status is current', () => {
    expect(summaryWithTools({ versionStatus: 'CURRENT', runState: 'NOT_RUNNING' })).toBe(
      'Read-only vCenter context · Tools not running',
    );
  });

  it('contributes no tools part when there is no actionable state', () => {
    expect(summaryWithTools({ versionStatus: 'OK' })).toBe('Read-only vCenter context');
  });
});

describe('adapterNetworkName (via Network adapter row value)', () => {
  it('prefers networkName', () => {
    expect(networkRow({ networkName: 'VM Network', networkId: 'network-9' })?.value).toBe(
      'VM Network',
    );
  });

  it('falls back to networkId', () => {
    expect(networkRow({ networkId: 'network-101' })?.value).toBe('network-101');
  });

  it('falls back to opaqueNetworkId', () => {
    expect(networkRow({ opaqueNetworkId: 'opaque-5' })?.value).toBe('opaque-5');
  });

  it('falls back to hostDevice', () => {
    expect(networkRow({ hostDevice: 'vmnic0' })?.value).toBe('vmnic0');
  });

  it('falls back to backingType', () => {
    expect(networkRow({ backingType: 'DVS' })?.value).toBe('DVS');
  });

  it('omits the network part when no network field is set', () => {
    expect(networkRow({ type: 'VMXNET3' })?.value).toBe('VMXNET3');
  });
});

describe('adapterConnectionLabel (via Network adapter row value)', () => {
  it('emits "starts connected" / "does not start connected"', () => {
    expect(networkRow({ startConnected: true })?.value).toBe('starts connected');
    expect(networkRow({ startConnected: false })?.value).toBe('does not start connected');
  });

  it('emits "guest control" / "no guest control"', () => {
    expect(networkRow({ allowGuestControl: true })?.value).toBe('guest control');
    expect(networkRow({ allowGuestControl: false })?.value).toBe('no guest control');
  });

  it('emits only the state when the toggles are undefined', () => {
    expect(networkRow({ state: 'CONNECTED' })?.value).toBe('CONNECTED');
  });

  it('joins all three connection parts with a separator', () => {
    expect(
      networkRow({ state: 'DISCONNECTED', startConnected: false, allowGuestControl: false })?.value,
    ).toBe('DISCONNECTED · does not start connected · no guest control');
  });
});

describe('adapterDisplayName (via Network adapter row label)', () => {
  // A type is carried so the row value is non-empty and makeDetailRow keeps
  // the row, isolating the display-name fallback logic to the label field.
  it('prefers label', () => {
    expect(networkRow({ label: 'eth0', nic: '4000', type: 'VMXNET3' })?.label).toBe('eth0');
  });

  it('falls back to nic', () => {
    expect(networkRow({ nic: '4000', type: 'VMXNET3' })?.label).toBe('4000');
  });

  it('falls back to macAddress', () => {
    expect(networkRow({ macAddress: 'AA:BB:CC' })?.label).toBe('AA:BB:CC');
  });

  it('defaults to "Network adapter" when nothing identifies the adapter', () => {
    expect(networkRow({ type: 'VMXNET3' })?.label).toBe('Network adapter');
  });
});

describe('adapterTone (via Network adapter row tone)', () => {
  it('is warning when state is not_connected', () => {
    expect(networkRow({ state: 'not_connected', type: 'VMXNET3' })?.tone).toBe('warning');
  });

  it('is default for a connected adapter', () => {
    expect(networkRow({ state: 'CONNECTED', type: 'VMXNET3' })?.tone).toBe('default');
  });

  it('is default when state is absent', () => {
    expect(networkRow({ type: 'VMXNET3' })?.tone).toBe('default');
  });
});

describe('hardwareRows (via Virtual hardware section)', () => {
  it('drops the Virtual hardware section when hardware is absent', () => {
    expect(findSection(vmSections({ cpuCount: 4 }), 'Virtual hardware')).toBeUndefined();
  });

  it('surfaces instant clone frozen as a warning row', () => {
    expect(hardwareRow({ hardware: { instantCloneFrozen: true } }, 'Instant clone frozen')).toEqual(
      {
        label: 'Instant clone frozen',
        value: 'Yes',
        tone: 'warning',
      },
    );
  });

  it('surfaces enter setup mode as a warning row', () => {
    expect(hardwareRow({ hardware: { enterSetupMode: true } }, 'Enter setup mode')).toEqual({
      label: 'Enter setup mode',
      value: 'Yes',
      tone: 'warning',
    });
  });

  it('surfaces an upgrade error message as a warning row', () => {
    expect(
      hardwareRow({ hardware: { upgradeErrorMessage: 'timed out' } }, 'Upgrade error'),
    ).toEqual({
      label: 'Upgrade error',
      value: 'timed out',
      tone: 'warning',
    });
  });

  it('omits the upgrade status row when the status is NONE/OK', () => {
    const vmware = { hardware: { version: 'VMX_19', upgradeStatus: 'NONE' } };
    expect(hardwareRow(vmware, 'Upgrade status')).toBeUndefined();
    expect(hardwareRow(vmware, 'Hardware version')?.value).toBe('VMX 19');
  });

  it('omits the instant clone / setup mode rows when the flags are false', () => {
    const vmware = { hardware: { instantCloneFrozen: false, enterSetupMode: false } };
    expect(hardwareRow(vmware, 'Instant clone frozen')).toBeUndefined();
    expect(hardwareRow(vmware, 'Enter setup mode')).toBeUndefined();
  });
});

describe('formatMiB (via Memory size row)', () => {
  const memoryRow = (memorySizeMib?: number): DetailRow | undefined =>
    hardwareRow({ hardware: { version: 'VMX_10' }, memorySizeMib }, 'Memory size');

  it('formats 1024 MiB as 1 GB', () => {
    expect(memoryRow(1024)?.value).toBe('1 GB');
  });

  it('formats zero as "0 B"', () => {
    expect(memoryRow(0)?.value).toBe('0 B');
  });

  it('omits the row when memorySizeMib is undefined', () => {
    expect(memoryRow(undefined)).toBeUndefined();
  });

  it('omits the row for a negative value', () => {
    expect(memoryRow(-512)).toBeUndefined();
  });

  it('omits the row for NaN', () => {
    expect(memoryRow(Number.NaN)).toBeUndefined();
  });
});

describe('toolsRows (via VMware Tools section)', () => {
  it('drops the VMware Tools section when tools is absent', () => {
    expect(findSection(vmSections({}), 'VMware Tools')).toBeUndefined();
  });

  it('flags a non-running run state with a warning tone', () => {
    expect(toolsRow({ tools: { runState: 'NOT_RUNNING' } }, 'Run state')).toEqual({
      label: 'Run state',
      value: 'Not Running',
      tone: 'warning',
    });
  });

  it('keeps the default tone for STARTED', () => {
    expect(toolsRow({ tools: { runState: 'STARTED' } }, 'Run state')).toEqual({
      label: 'Run state',
      value: 'Started',
      tone: 'default',
    });
  });

  it('surfaces a non-current version status with a warning tone', () => {
    expect(toolsRow({ tools: { versionStatus: 'OUT_OF_DATE' } }, 'Version status')).toEqual({
      label: 'Version status',
      value: 'Out Of Date',
      tone: 'warning',
    });
  });

  it('surfaces the last install error as a warning row', () => {
    expect(toolsRow({ tools: { errorMessage: 'boom' } }, 'Last install error')).toEqual({
      label: 'Last install error',
      value: 'boom',
      tone: 'warning',
    });
  });
});

describe('virtualDiskDisplayName (via Virtual disks row label)', () => {
  // A SCSI address is carried so the row value is non-empty and makeDetailRow
  // keeps the row, isolating the display-name fallback to the label field.
  it('prefers label', () => {
    expect(
      diskRow({ label: 'Hard disk 1', disk: '2000', type: 'SCSI', scsiBus: 0, scsiUnit: 0 })?.label,
    ).toBe('Hard disk 1');
  });

  it('falls back to the disk id', () => {
    expect(diskRow({ disk: '2000', type: 'SCSI', scsiBus: 0, scsiUnit: 0 })?.label).toBe('2000');
  });

  it('defaults to "Virtual disk" when neither is set', () => {
    expect(diskRow({ type: 'SCSI', scsiBus: 0, scsiUnit: 0 })?.label).toBe('Virtual disk');
  });
});

describe('formatBoolLabel (via State section)', () => {
  const sharedRow = (vmware: Partial<ResourceVMwareMeta>): DetailRow | undefined =>
    findRow(vmSections(vmware), 'State', 'Shared access');

  const accessibleRow = (vmware: Partial<ResourceVMwareMeta>): DetailRow | undefined =>
    findRow(vmSections(vmware), 'State', 'Accessible');

  it('renders "Yes" for true (Shared access)', () => {
    expect(sharedRow({ multipleHostAccess: true })?.value).toBe('Yes');
  });

  it('renders "No" for false (Shared access)', () => {
    expect(sharedRow({ multipleHostAccess: false })?.value).toBe('No');
  });

  it('drops the row for undefined (Shared access)', () => {
    expect(sharedRow({})).toBeUndefined();
  });

  it('flags an inaccessible datastore with a warning tone', () => {
    expect(accessibleRow({ datastoreAccessible: false })).toEqual({
      label: 'Accessible',
      value: 'No',
      tone: 'warning',
    });
  });

  it('renders an accessible datastore as "Yes" with the default tone', () => {
    expect(accessibleRow({ datastoreAccessible: true })).toEqual({
      label: 'Accessible',
      value: 'Yes',
      tone: 'default',
    });
  });
});

describe('getStatusTone (via Overall status row)', () => {
  // null is deliberately fed to exercise getStatusTone's null branch; the field
  // itself is typed string | undefined, so the value is cast to the field type.
  const statusRow = (overallStatus?: string | null): DetailRow | undefined =>
    findRow(
      vmSections({ overallStatus: overallStatus as ResourceVMwareMeta['overallStatus'] }),
      'State',
      'Overall status',
    );

  it('maps red to warning', () => {
    expect(statusRow('red')).toEqual({ label: 'Overall status', value: 'red', tone: 'warning' });
  });

  it('maps any other non-empty status to accent', () => {
    expect(statusRow('yellow')).toEqual({
      label: 'Overall status',
      value: 'yellow',
      tone: 'accent',
    });
  });

  it('drops the row for an empty status (default tone branch)', () => {
    expect(statusRow('')).toBeUndefined();
  });

  it('drops the row for a null status (default tone branch)', () => {
    expect(statusRow(null)).toBeUndefined();
  });
});

describe('vmwareEntityLabel (via Entity row)', () => {
  const entityRow = (entityType?: string): DetailRow | undefined =>
    findRow(vmSections({ entityType }), 'State', 'Entity');

  it.each<[string, string]>([
    ['host', 'Host'],
    ['hostsystem', 'Host'],
    ['HOSTSYSTEM', 'Host'],
    ['vm', 'VM'],
    ['virtualmachine', 'VM'],
    ['datastore', 'Datastore'],
    ['network', 'Network'],
  ])('maps entity type "%s" to "%s"', (entityType, expected) => {
    expect(entityRow(entityType)?.value).toBe(expected);
  });

  it('passes an unrecognized entity type through verbatim', () => {
    expect(entityRow('resourcePool')?.value).toBe('resourcePool');
  });

  it('drops the Entity row when entityType is blank', () => {
    expect(entityRow('   ')).toBeUndefined();
  });
});

describe('formatSnapshotDate (via Snapshot tree rows)', () => {
  it('omits the date when createdAt is undefined', () => {
    expect(snapshotRows([{ snapshot: 's-1', state: 'poweredOn' }])[0]?.value).toBe('poweredOn');
  });

  it('returns the raw string for an unparseable date', () => {
    expect(snapshotRows([{ snapshot: 's-1', createdAt: 'not-a-date' }])[0]?.value).toBe(
      'not-a-date',
    );
  });

  it('formats a valid ISO timestamp in UTC', () => {
    expect(snapshotRows([{ snapshot: 's-1', createdAt: '2026-01-05T09:30:00Z' }])[0]?.value).toBe(
      '2026-01-05 09:30 UTC',
    );
  });
});

describe('snapshotDisplayName (via Snapshot tree row label)', () => {
  // Each fixture carries a state so the row value is non-empty and the row
  // survives makeDetailRow, isolating the display-name fallback logic.
  const label = (snapshot: ResourceVMwareSnapshot): string | undefined =>
    snapshotRows([{ state: 'poweredOn', ...snapshot }])[0]?.label;

  it('prefers name', () => {
    expect(label({ name: 'pre-upgrade', snapshot: 's-1' })).toBe('pre-upgrade');
  });

  it('falls back to the snapshot ref', () => {
    expect(label({ snapshot: 'snapshot-201' })).toBe('snapshot-201');
  });

  it('falls back to "Snapshot <id>" when only a numeric id is set', () => {
    expect(label({ id: 7 })).toBe('Snapshot 7');
  });

  it('falls back to "Snapshot" when nothing identifies it', () => {
    expect(label({})).toBe('Snapshot');
  });
});

describe('snapshotValue (via Snapshot tree row value)', () => {
  const value = (snapshot: ResourceVMwareSnapshot): string | undefined =>
    snapshotRows([snapshot])[0]?.value;

  it('joins current marker, state, date and quiesced', () => {
    expect(
      value({
        current: true,
        state: 'poweredOn',
        createdAt: '2026-02-03T00:00:00Z',
        quiesced: true,
      }),
    ).toBe('current · poweredOn · 2026-02-03 00:00 UTC · quiesced');
  });

  it('emits "not quiesced" for a false quiesced flag', () => {
    expect(value({ state: 'poweredOff', quiesced: false })).toBe('poweredOff · not quiesced');
  });

  it('omits the quiesced part when it is undefined', () => {
    expect(value({ state: 'poweredOn' })).toBe('poweredOn');
  });

  it('appends the description', () => {
    expect(value({ state: 'poweredOn', description: 'pre-patch' })).toBe('poweredOn · pre-patch');
  });
});

describe('flattenSnapshotRows (via Snapshot tree section)', () => {
  it('drops the section when there are no snapshots', () => {
    expect(findSection(vmSections({ snapshotTree: [] }), 'Snapshot tree')).toBeUndefined();
  });

  it('prefixes nested children by depth', () => {
    expect(
      snapshotRows([
        {
          name: 'root',
          state: 'poweredOn',
          children: [{ name: 'child', state: 'poweredOn' }],
        },
      ]).map((row) => row.label),
    ).toEqual(['root', '- child']);
  });

  it('marks the current snapshot with an accent tone', () => {
    expect(snapshotRows([{ name: 'now', state: 'poweredOn', current: true }])[0]).toEqual({
      label: 'now',
      value: 'current · poweredOn',
      tone: 'accent',
    });
  });
});
