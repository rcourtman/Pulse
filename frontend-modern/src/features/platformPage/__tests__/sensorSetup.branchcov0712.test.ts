import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { collectOutdatedSensorSetupNodes } from '../sensorSetup';

// Branch-coverage tests for the two UNEXPORTED helpers inside sensorSetup.ts:
//   - getNodeName                         (3-way `||` fallback chain + `.trim()`)
//   - isSMARTOnlyDiskWithoutTemperature   (no-meta guard, diskType normalize,
//                                          SMART-set membership, `temperature ?? 0`)
// Both helpers are private, so every case drives them through the public
// `collectOutdatedSensorSetupNodes` and asserts the concrete observable shape.
//
// Conventions mirror the sibling `sensorSetup.test.ts` fixtures. Disks are
// linked to nodes either by `platformData.proxmox.nodeName` (the name-matching
// path, used to prove `getNodeName` resolution) or by `parentId` (which makes
// `matchesPhysicalDiskNode` short-circuit true, used to isolate the SMART-only
// predicate from name resolution).

type LegacyNodeOver = {
  id?: string;
  name?: string;
  proxmox?: Partial<Resource['proxmox']>;
};

const legacyNode = (over: LegacyNodeOver = {}): Resource =>
  ({
    id: over.id ?? 'node-1',
    name: over.name ?? '',
    displayName: over.name ?? '',
    type: 'node',
    proxmox: {
      temperatureDetails: { available: true, legacySensorsFormat: true },
      ...(over.proxmox ?? {}),
    },
  }) as unknown as Resource;

type DiskOver = {
  id?: string;
  parentId?: string;
  nodeName?: string;
  diskType?: string;
  temperature?: number;
  noMeta?: boolean;
};

const disk = (over: DiskOver = {}): Resource =>
  ({
    id: over.id ?? 'disk-1',
    name: 'disk',
    displayName: 'disk',
    type: 'physical_disk',
    parentId: over.parentId,
    physicalDisk: over.noMeta
      ? undefined
      : {
          diskType: over.diskType,
          temperature: over.temperature,
        },
    platformData: over.nodeName ? { proxmox: { nodeName: over.nodeName } } : {},
  }) as unknown as Resource;

describe('getNodeName (via collectOutdatedSensorSetupNodes)', () => {
  it('falls back to proxmox.nodeName when proxmox.node is absent', () => {
    const node = legacyNode({ id: 'n-nodeName', proxmox: { nodeName: 'bravo' } });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      // Disk resolves to the node by the nodeName-derived identity, proving
      // getNodeName returned 'bravo' (not '' and not node.name).
      [disk({ id: 'd1', nodeName: 'bravo', diskType: 'sata', temperature: 0 })],
    );
    expect(result).toStrictEqual([{ id: 'n-nodeName', name: 'bravo' }]);
  });

  it('falls back to the resource name when both proxmox.node and nodeName are absent', () => {
    const node = legacyNode({ id: 'n-name', name: 'charlie' });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      [disk({ id: 'd2', nodeName: 'charlie', diskType: 'sata', temperature: 0 })],
    );
    expect(result).toStrictEqual([{ id: 'n-name', name: 'charlie' }]);
  });

  it('trims surrounding whitespace from proxmox.node before using it as the node identity', () => {
    const node = legacyNode({ id: 'n-trim', proxmox: { node: '  delta  ' } });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      // matchesPhysicalDiskNode lowercases/trims both sides, so 'delta' must be
      // the trimmed getNodeName output for the disk to match.
      [disk({ id: 'd3', nodeName: 'delta', diskType: 'sata', temperature: 0 })],
    );
    expect(result).toStrictEqual([{ id: 'n-trim', name: 'delta' }]);
  });

  it('returns the empty string when every name source is blank, observable via a parentId-linked disk', () => {
    // No proxmox.node, no proxmox.nodeName, name is whitespace -> getNodeName
    // hits the final `|| ''` arm and trims to ''. The disk is matched by
    // parentId so matchesPhysicalDiskNode short-circuits true regardless of the
    // (empty) name, letting the flagged node surface with name: ''.
    const node = legacyNode({ id: 'n-empty', name: '   ' });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      [disk({ id: 'd4', parentId: 'n-empty', diskType: 'sata', temperature: 0 })],
    );
    expect(result).toStrictEqual([{ id: 'n-empty', name: '' }]);
  });
});

describe('isSMARTOnlyDiskWithoutTemperature (via collectOutdatedSensorSetupNodes)', () => {
  it('returns false for a disk with no physicalDisk metadata, so the node is not flagged', () => {
    // Disk is linked to the node by parentId (match guaranteed true); the only
    // reason the node drops out is the `if (!meta) return false` guard.
    const node = legacyNode({ id: 'n-nometa', proxmox: { node: 'pve' } });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      [disk({ id: 'd-nometa', parentId: 'n-nometa', noMeta: true })],
    );
    expect(result).toStrictEqual([]);
  });

  it('returns false when diskType is missing, so a matched disk does not flag the node', () => {
    // diskType undefined -> `(meta.diskType || '')` -> '' not in the SMART-only
    // set. parentId link isolates the cause to the diskType check.
    const node = legacyNode({ id: 'n-notype', proxmox: { node: 'pve' } });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      [disk({ id: 'd-notype', parentId: 'n-notype', temperature: 0 })],
    );
    expect(result).toStrictEqual([]);
  });

  it('normalizes a whitespace-padded uppercase SAS diskType and flags the node', () => {
    // Exercises `diskType.trim().toLowerCase()` -> 'sas' (in the SMART-only
    // set) and temperature 0 -> `(0 ?? 0) <= 0` true.
    const node = legacyNode({ id: 'n-sas', proxmox: { node: 'pve' } });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      [
        disk({
          id: 'd-sas',
          parentId: 'n-sas',
          diskType: '  SAS  ',
          temperature: 0,
        }),
      ],
    );
    expect(result).toStrictEqual([{ id: 'n-sas', name: 'pve' }]);
  });

  it('uses 0 in place of an undefined temperature and flags the node', () => {
    // temperature is omitted entirely -> `(meta.temperature ?? 0)` takes the
    // nullish right-hand side (0) -> `<= 0` true.
    const node = legacyNode({ id: 'n-undef-temp', proxmox: { node: 'pve' } });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      [disk({ id: 'd-undef-temp', parentId: 'n-undef-temp', diskType: 'sata' })],
    );
    expect(result).toStrictEqual([{ id: 'n-undef-temp', name: 'pve' }]);
  });

  it('uses 0 in place of an explicit null temperature and flags the node', () => {
    // Deliberately-malformed payload: temperature typed as number | undefined
    // but carrying null. The `?? 0` coalesces it to 0 -> `<= 0` true.
    const node = legacyNode({ id: 'n-null-temp', proxmox: { node: 'pve' } });
    const nullTempDisk = {
      id: 'd-null-temp',
      name: 'disk',
      displayName: 'disk',
      type: 'physical_disk',
      parentId: 'n-null-temp',
      physicalDisk: { diskType: 'sata', temperature: null },
    } as unknown as Resource;
    const result = collectOutdatedSensorSetupNodes([node], [nullTempDisk]);
    expect(result).toStrictEqual([{ id: 'n-null-temp', name: 'pve' }]);
  });

  it('flags the node for a SATA disk reporting a negative temperature', () => {
    // Concrete negative value exercises the `<= 0` true arm (not the ?? arm).
    const node = legacyNode({ id: 'n-neg-temp', proxmox: { node: 'pve' } });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      [disk({ id: 'd-neg-temp', parentId: 'n-neg-temp', diskType: 'sata', temperature: -5 })],
    );
    expect(result).toStrictEqual([{ id: 'n-neg-temp', name: 'pve' }]);
  });

  it('does not flag the node when a SMART-only disk reports a positive temperature', () => {
    // SAS at 41 degrees -> `(41 ?? 0) <= 0` is false.
    const node = legacyNode({ id: 'n-warm', proxmox: { node: 'pve' } });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      [disk({ id: 'd-warm', parentId: 'n-warm', diskType: 'sas', temperature: 41 })],
    );
    expect(result).toStrictEqual([]);
  });

  it('evaluates the no-meta guard on the first disk but still flags the node via a second matching disk', () => {
    // Drives the `!meta` branch on d-bad, then `.some()` advances to d-good
    // which is SMART-only without a temperature -> node flagged.
    const node = legacyNode({ id: 'n-mixed', proxmox: { node: 'pve' } });
    const result = collectOutdatedSensorSetupNodes(
      [node],
      [
        disk({ id: 'd-bad', parentId: 'n-mixed', noMeta: true }),
        disk({ id: 'd-good', parentId: 'n-mixed', diskType: 'sata', temperature: 0 }),
      ],
    );
    expect(result).toStrictEqual([{ id: 'n-mixed', name: 'pve' }]);
  });
});
