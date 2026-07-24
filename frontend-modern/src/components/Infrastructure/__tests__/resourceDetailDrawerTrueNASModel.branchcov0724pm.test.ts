import { describe, expect, it } from 'vitest';
import {
  buildTrueNASDetailSections,
  buildTrueNASDetailsSummary,
  hasTrueNASDetailSections,
  type ResourceDetailDrawerTrueNASRow,
} from '@/components/Infrastructure/resourceDetailDrawerTrueNASModel';
import type {
  Resource,
  ResourcePhysicalDiskMeta,
  ResourceStorageMeta,
  ResourceTrueNASAppMeta,
} from '@/types/resource';

// Every helper under test is module-private, so each case drives it through the
// three public entry points (buildTrueNASDetailSections /
// buildTrueNASDetailsSummary / hasTrueNASDetailSections) and asserts on the
// rendered sections, rows, summaries or boolean verdict.  Fixture builders and
// accessors mirror the sibling `coverage2` spec's conventions.  This file
// targets ONLY the arms the existing specs leave uncovered.

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

const allRows = (resource: Resource): ResourceDetailDrawerTrueNASRow[] =>
  buildTrueNASDetailSections(resource).flatMap((section) => section.rows);

const findRow = (resource: Resource, label: string): ResourceDetailDrawerTrueNASRow | undefined =>
  allRows(resource).find((row) => row.label === label);

const storageRes = (
  // `unknown` so deeply-nested deliberately-partial fixtures (zfsPool devices,
  // scanDetails, poolHealth) bypass strict-null checks at the call site; the
  // cast here is the sanctioned boundary for malformed inputs.
  storage: unknown,
  overrides: Partial<Resource> = {},
): Resource => baseResource({ storage: storage as ResourceStorageMeta, ...overrides });

const diskRes = (disk: Partial<ResourcePhysicalDiskMeta>): Resource =>
  baseResource({ physicalDisk: disk });

const appRes = (app: Partial<ResourceTrueNASAppMeta>): Resource =>
  baseResource({ type: 'app-container', truenas: { app } });

// =========================================================================
// hasTrueNASDetailSections (previously zero hits)
// =========================================================================

describe('hasTrueNASDetailSections', () => {
  it('returns true when a section builder produces at least one section', () => {
    const resource = storageRes({ type: 'zfs-pool' });
    expect(hasTrueNASDetailSections(resource)).toBe(true);
    expect(buildTrueNASDetailSections(resource).map((s) => s.label)).toContain('Storage');
  });

  it('returns false for a resource that is not TrueNAS-scoped (falls through to [])', () => {
    const resource = baseResource({ platformType: 'docker' });
    expect(hasTrueNASDetailSections(resource)).toBe(false);
    expect(buildTrueNASDetailSections(resource)).toEqual([]);
  });

  it('returns false when a builder yields only empty section arrays (bare app)', () => {
    // An app with no fields routes to buildTrueNASAppSections, whose every
    // section is compacted away because every row is null -> sections == [].
    const resource = baseResource({ truenas: { app: {} } });
    expect(hasTrueNASDetailSections(resource)).toBe(false);
    expect(buildTrueNASDetailSections(resource)).toEqual([]);
  });
});

// =========================================================================
// isTrueNASScopedResource alternative scope arms (platformType short-circuits
// the other four operands in every existing fixture)
// =========================================================================

describe('isTrueNASScopedResource alternative scoping arms', () => {
  // Each resource is platformType 'docker' (so the first operand is false) and
  // carries storage so it routes to buildTrueNASStorageSections; producing a
  // 'Storage' section proves isTrueNASScopedResource returned true via the
  // targeted alternative operand.

  it('scopes via platformScopes containing "truenas"', () => {
    const resource = baseResource({
      platformType: 'docker',
      platformScopes: ['truenas'],
      storage: { type: 'zfs-pool' },
    });
    // platformType is 'docker' (first operand false), so isTrueNASScopedResource
    // resolves true via platformScopes -> the Storage section is built at all.
    expect(buildTrueNASDetailSections(resource).map((s) => s.label)).toContain('Storage');
  });

  it('scopes via sources containing "truenas"', () => {
    const resource = baseResource({
      platformType: 'docker',
      sources: ['truenas'],
      storage: { type: 'zfs-pool' },
    });
    expect(buildTrueNASDetailSections(resource).map((s) => s.label)).toContain('Storage');
  });

  it('scopes via storage.platform === "truenas"', () => {
    const resource = baseResource({
      platformType: 'docker',
      storage: { platform: 'truenas', type: 'zfs-pool' },
    });
    expect(buildTrueNASDetailSections(resource).map((s) => s.label)).toContain('Storage');
  });

  it('scopes via tags containing "truenas"', () => {
    const resource = baseResource({
      platformType: 'docker',
      tags: ['truenas'],
      storage: { type: 'zfs-pool' },
    });
    expect(buildTrueNASDetailSections(resource).map((s) => s.label)).toContain('Storage');
  });
});

// =========================================================================
// zfsScanLabel fallback arms
// =========================================================================

describe('zfsScanLabel fallback arms (via Health "Scan / resilver" row)', () => {
  it('returns the raw scan string when scanDetails is absent (early return)', () => {
    // The early-return arm hands the scan straight to asString() without
    // normalizeDelimitedLabel, so the original casing is preserved.
    const resource = storageRes({ zfsPool: { scan: 'RESILVER' } });
    expect(findRow(resource, 'Scan / resilver')?.value).toBe('RESILVER');
  });

  it('defaults the operation to "Scan" and drops progress when function/percentage are absent', () => {
    const resource = storageRes({ zfsPool: { scanDetails: { state: 'SCANNING' } } });
    const row = findRow(resource, 'Scan / resilver');
    expect(row?.value).toBe('Scan Scanning');
    expect(row?.tone).toBe('default');
  });

  it('formats error count and drops the state segment when state is absent', () => {
    const resource = storageRes({ zfsPool: { scanDetails: { function: 'SCRUB', errors: 2 } } });
    const row = findRow(resource, 'Scan / resilver');
    expect(row?.value).toBe('Scrub · 2 errors');
    expect(row?.tone).toBe('warning');
  });
});

// =========================================================================
// zfsDeviceEvidenceLabels: filter fall-through, name resolution, formatting
// =========================================================================

describe('zfsDeviceEvidenceLabels filter and formatting (via "Affected vdevs")', () => {
  it('admits a device via the non-online state group and resolves name to Vdev with no errors', () => {
    // missing is unset -> the `||` falls through to the state group; a
    // non-empty state outside the ONLINE/AVAIL/INUSE allowlist passes the
    // filter. With no disk/path/name the name resolves to the 'Vdev' fallback.
    const resource = storageRes({
      zfsPool: { devices: [{ state: 'DEGRADED', role: 'log', type: 'disk' }] },
    });
    expect(findRow(resource, 'Affected vdevs')?.value).toBe('Vdev (Log / Disk): DEGRADED');
  });

  it('admits a device via readErrors and formats the R/W/C evidence with no context', () => {
    const resource = storageRes({
      zfsPool: {
        devices: [
          { state: 'ONLINE', disk: 'sdb', readErrors: 1, writeErrors: 0, checksumErrors: 0 },
        ],
      },
    });
    expect(findRow(resource, 'Affected vdevs')?.value).toBe('sdb: ONLINE · R 1 W 0 C 0');
  });

  it('admits a device via writeErrors and resolves name via path', () => {
    const resource = storageRes({
      zfsPool: {
        devices: [
          { state: 'ONLINE', path: '/dev/sdc', readErrors: 0, writeErrors: 1, checksumErrors: 0 },
        ],
      },
    });
    expect(findRow(resource, 'Affected vdevs')?.value).toBe('/dev/sdc: ONLINE · R 0 W 1 C 0');
  });

  it('admits a device via checksumErrors and resolves name via the name field', () => {
    const resource = storageRes({
      zfsPool: {
        devices: [
          { state: 'ONLINE', name: 'sdd', readErrors: 0, writeErrors: 0, checksumErrors: 1 },
        ],
      },
    });
    expect(findRow(resource, 'Affected vdevs')?.value).toBe('sdd: ONLINE · R 0 W 0 C 1');
  });

  it('excludes a healthy online device with no errors from the evidence list', () => {
    const resource = storageRes({
      zfsPool: { devices: [{ state: 'ONLINE', role: 'data' }] },
    });
    expect(findRow(resource, 'Affected vdevs')).toBeUndefined();
  });

  it('falls back to the "Unknown" state label for an admitted device with no state string', () => {
    // A device with no missing flag and no state still passes the state group
    // (undefined !== '' is true), so its display state resolves through the
    // asString(device.state) ?? 'Unknown' fallback.
    const resource = storageRes({
      zfsPool: { devices: [{ role: 'log' }] },
    });
    expect(findRow(resource, 'Affected vdevs')?.value).toBe('Vdev (Log): Unknown');
  });
});

// =========================================================================
// Canonical state tone (nested ternary in Health "Canonical state")
// =========================================================================

describe('poolHealth canonical state tone', () => {
  it('tones success for canonicalState "ONLINE"', () => {
    const resource = storageRes({ poolHealth: { canonicalState: 'ONLINE' } });
    const row = findRow(resource, 'Canonical state');
    expect(row?.value).toBe('ONLINE');
    expect(row?.tone).toBe('success');
  });

  it('tones default for a canonicalState outside the known sets', () => {
    const resource = storageRes({ poolHealth: { canonicalState: 'SUSPENDED' } });
    const row = findRow(resource, 'Canonical state');
    expect(row?.value).toBe('SUSPENDED');
    expect(row?.tone).toBe('default');
  });
});

// =========================================================================
// SMART warning tones for offline/CRC/media error rows
// =========================================================================

describe('disk SMART warning tones', () => {
  it('tones warning when offline uncorrectable, CRC and media errors are present', () => {
    const resource = diskRes({
      smart: { offlineUncorrectable: 1, udmaCrcErrors: 2, mediaErrors: 3 },
    });
    expect(findRow(resource, 'Offline uncorrectable')).toEqual({
      label: 'Offline uncorrectable',
      value: '1',
      tone: 'warning',
    });
    expect(findRow(resource, 'CRC errors')).toEqual({
      label: 'CRC errors',
      value: '2',
      tone: 'warning',
    });
    expect(findRow(resource, 'Media errors')).toEqual({
      label: 'Media errors',
      value: '3',
      tone: 'warning',
    });
  });
});

// =========================================================================
// buildTrueNASDetailsSummary edge arms
// =========================================================================

describe('buildTrueNASDetailsSummary edge arms', () => {
  it('reads containers array length in the app summary when containerCount is absent', () => {
    // Exercising app.containers?.length with a defined containers array so the
    // optional-chain .length access is actually performed.
    const resource = appRes({ containers: [{ id: 'c1' }] });
    expect(buildTrueNASDetailsSummary(resource)).toBe('1 container, 0 ports');
  });

  it('returns null for a scoped storage resource whose summary is entirely empty', () => {
    // No kind, no pool/array state, no usage, no risk summary, and a blank
    // resource.status so storageStateLabel also resolves to null -> every
    // summary element is nullish -> [] -> null.
    const resource = baseResource({ status: undefined, storage: {} });
    expect(buildTrueNASDetailsSummary(resource)).toBeNull();
  });
});
