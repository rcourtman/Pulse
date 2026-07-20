import { describe, expect, it } from 'vitest';
import {
  buildVMwareDetailSections,
  buildVMwareDetailsSummary,
} from '@/components/Infrastructure/resourceDetailDrawerVmwareModel';
import type { DetailRow } from '@/components/shared/detailSectionModel';
import type { ResourceVMwareMeta, ResourceVMwareSnapshot } from '@/types/resource';

// Branch-coverage complement to resourceDetailDrawerVmwareModel.coverage.test.ts.
// Scope is strictly: buildVMwareDetailsSummary, formatMiB, flattenSnapshotRows.
// formatMiB and flattenSnapshotRows are module-private, so they are driven
// through the public entry points exactly as the sibling coverage test does:
//   - formatMiB         -> "Memory size" row of the Virtual hardware section
//   - flattenSnapshotRows -> "Snapshot tree" section rows
// buildVMwareDetailsSummary is exported and is asserted on directly.

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

const hardwareRow = (vmware: Partial<ResourceVMwareMeta>, label: string): DetailRow | undefined =>
  findRow(vmSections(vmware), 'Virtual hardware', label);

const snapshotSectionRows = (snapshots: ResourceVMwareSnapshot[]): DetailRow[] =>
  findSection(vmSections({ snapshotTree: snapshots }), 'Snapshot tree')?.rows ?? [];

describe('buildVMwareDetailsSummary (additional branches)', () => {
  it('falls back to vcenterHost when connectionName is only whitespace', () => {
    expect(
      buildVMwareDetailsSummary('vm', { connectionName: '   ', vcenterHost: 'vc-02.lab' }),
    ).toBe('vc-02.lab · Read-only vCenter context');
  });

  it('omits the snapshot part for a non-vm type even when snapshotCount is positive', () => {
    expect(buildVMwareDetailsSummary('datastore', { snapshotCount: 5 })).toBe(
      'Read-only vCenter context',
    );
  });

  it('omits vNIC and disk counts for a non-vm type even when adapters/disks are present', () => {
    expect(
      buildVMwareDetailsSummary('datastore', {
        networkAdapters: [{ nic: '1' }],
        virtualDisks: [{ disk: '1', type: 'SCSI' }],
      }),
    ).toBe('Read-only vCenter context');
  });

  it('omits both host and vm counts for a network resource with no network membership', () => {
    expect(buildVMwareDetailsSummary('network', {})).toBe('Read-only vCenter context');
  });

  it('counts only hosts when the network has hosts but no vms', () => {
    expect(buildVMwareDetailsSummary('network', { networkHostNames: ['esxi-01'] })).toBe(
      'Read-only vCenter context · 1 host',
    );
  });

  it('counts only vms when the network has vms but no hosts', () => {
    expect(buildVMwareDetailsSummary('network', { networkVmNames: ['vm-01', 'vm-02'] })).toBe(
      'Read-only vCenter context · 2 VMs',
    );
  });

  it('clamps an explicit snapshotCount of 0 to no snapshot part', () => {
    expect(buildVMwareDetailsSummary('vm', { snapshotCount: 0 })).toBe('Read-only vCenter context');
  });

  it('omits alarm and task parts when both counts are explicitly zero', () => {
    expect(buildVMwareDetailsSummary('vm', { activeAlarmCount: 0, recentTaskCount: 0 })).toBe(
      'Read-only vCenter context',
    );
  });

  // SUSPECT SOURCE BEHAVIOR (reported, not fixed): the host/vm count chains use
  // `names?.length ?? ids?.length ?? 0`. Because an empty names array has
  // length 0 (not nullish), the `??` ids fallback is skipped, so present ids are
  // silently ignored whenever the names array exists but is empty.
  it('ignores networkHostIds when networkHostNames is an empty array (?? short-circuit)', () => {
    expect(
      buildVMwareDetailsSummary('network', {
        networkHostNames: [],
        networkHostIds: ['h-1', 'h-2'],
      }),
    ).toBe('Read-only vCenter context');
  });
});

describe('formatMiB (via Memory size row)', () => {
  const memoryRow = (memorySizeMib?: number): DetailRow | undefined =>
    hardwareRow({ hardware: { version: 'VMX_10' }, memorySizeMib }, 'Memory size');

  it('formats a fractional gibibyte value with compact precision', () => {
    expect(memoryRow(1536)?.value).toBe('1.5 GB');
  });

  it('scales up to the terabyte unit for a large memory size', () => {
    expect(memoryRow(1_048_576)?.value).toBe('1 TB');
  });

  it('keeps a sub-gigibyte value in megabytes', () => {
    expect(memoryRow(768)?.value).toBe('768 MB');
  });

  it('returns empty for +Infinity (the !Number.isFinite guard arm)', () => {
    // memorySizeMib is typed number | undefined; Infinity is a valid number and
    // is rejected by formatMiB's Number.isFinite guard.
    expect(memoryRow(Number.POSITIVE_INFINITY)).toBeUndefined();
  });

  it('returns empty when MiB overflow makes bytes Infinity (the ?? "" fallback arm)', () => {
    // 2e302 is itself finite and non-negative, so it passes formatMiB's guard,
    // but 2e302 * 1024 * 1024 overflows to Infinity and formatDetailBytesValue
    // returns null, exercising formatMiB's `?? ""` fallback.
    expect(memoryRow(2e302)).toBeUndefined();
  });
});

describe('flattenSnapshotRows (via Snapshot tree section)', () => {
  it('falls back to the snapshot ref when snapshotValue resolves to empty', () => {
    // No current/state/createdAt/quiesced/description => snapshotValue() is '',
    // so the row value comes from `|| asTrimmedString(snapshot.snapshot)`.
    expect(snapshotSectionRows([{ snapshot: 'snap-9' }])[0]).toEqual({
      label: 'snap-9',
      value: 'snap-9',
      tone: 'default',
    });
  });

  it('drops a snapshot whose row makeDetailRow returns null, keeping valid siblings', () => {
    // A bare {} snapshot has no value (snapshotValue '' and no snapshot ref) so
    // makeDetailRow returns null and the row is skipped; the sibling survives.
    expect(
      snapshotSectionRows([{}, { snapshot: 's-1', state: 'poweredOn' }]).map((row) => row.label),
    ).toEqual(['s-1']);
  });

  it('indents grandchildren two levels deep via "-".repeat(depth)', () => {
    expect(
      snapshotSectionRows([
        {
          name: 'root',
          state: 'poweredOn',
          children: [
            {
              name: 'child',
              state: 'poweredOn',
              children: [{ name: 'grandchild', state: 'poweredOn' }],
            },
          ],
        },
      ]).map((row) => row.label),
    ).toEqual(['root', '- child', '-- grandchild']);
  });

  it('marks a non-current nested snapshot with the default tone', () => {
    const rows = snapshotSectionRows([
      {
        name: 'root',
        state: 'poweredOn',
        current: true,
        children: [{ name: 'child', state: 'poweredOn' }],
      },
    ]);
    expect(rows).toEqual([
      { label: 'root', value: 'current · poweredOn', tone: 'accent' },
      { label: '- child', value: 'poweredOn', tone: 'default' },
    ]);
  });
});
