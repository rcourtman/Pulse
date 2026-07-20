import { describe, expect, it } from 'vitest';
import type { ZFSPool } from '@/types/api';
import type { NormalizedHealth, StorageRecord } from '@/features/storageBackups/models';
import {
  getStorageRecordActionSummary,
  getStorageRecordContent,
  getStorageRecordHostLabel,
  getStorageRecordImpactSummary,
  getStorageRecordIssueLabel,
  getStorageRecordIssueSummary,
  getStorageRecordNodeHints,
  getStorageRecordNodeLabel,
  getStorageRecordPlatformLabel,
  getStorageRecordProtectionLabel,
  getStorageRecordShared,
  getStorageRecordStats,
  getStorageRecordStatus,
  getStorageRecordTopologyLabel,
  getStorageRecordType,
  getStorageRecordUsagePercent,
  getStorageRecordZfsPool,
} from '@/features/storageBackups/recordPresentation';

// ---------------------------------------------------------------------------
// Fixture builder — mirrors recordPresentation.test.ts verbatim so casts,
// import paths and field defaults match the sibling suite. Each case below
// overrides only what its target branch needs.
// ---------------------------------------------------------------------------

const makeRecord = (overrides: Partial<StorageRecord> = {}): StorageRecord => ({
  id: 'storage-1',
  name: 'tank',
  category: 'pool',
  health: 'healthy',
  location: { label: 'truenas01/pool/tank', scope: 'host' },
  capacity: { totalBytes: 1_000, usedBytes: 400, freeBytes: 600, usagePercent: 40 },
  capabilities: ['capacity', 'health'],
  source: {
    platform: 'truenas',
    family: 'onprem',
    origin: 'resource',
    adapterId: 'resource-storage',
  },
  observedAt: Date.now(),
  ...overrides,
});

// ===========================================================================
// getStorageRecordNodeHints — defensive-coercion and filter branches
// ===========================================================================

describe('getStorageRecordNodeHints branch coverage', () => {
  it('coerces non-string detail.node / detail.parentId / detail.parentName to "" via the typeof guards', () => {
    // All three typeof guards take their false arm simultaneously; the only
    // surviving hint is the location.label root + full label.
    const record = makeRecord({
      details: {
        node: 42,
        parentId: ['x'],
        parentName: { deep: true },
      },
    });
    expect(getStorageRecordNodeHints(record)).toEqual(['truenas01', 'truenas01/pool/tank']);
  });

  it('returns an empty array when every candidate hint trims to empty', () => {
    // detail.* missing, location.label is whitespace, refs undefined.
    // Every entry falls into the `.map((v) => v ? ... : '')` false arm and is
    // then dropped by the `.filter((v) => v.length > 0)` predicate.
    const record = makeRecord({
      location: { label: '   ', scope: 'host' },
      refs: undefined,
      details: {},
    });
    expect(getStorageRecordNodeHints(record)).toEqual([]);
  });

  it('exercises the `(record.details || {})` falsy arm and the location.label no-slash arm', () => {
    // details is undefined → `(record.details || {})` returns `{}`. location
    // label has no '/', so `label.split('/')[0]` is the whole label; both the
    // `locationRoot` and `record.location.label` candidates therefore resolve
    // to the same string and the function emits it twice (deduplication is
    // NOT performed — only filtering of empty strings).
    const record = makeRecord({
      details: undefined,
      location: { label: 'solo-host', scope: 'host' },
      refs: undefined,
    });
    expect(getStorageRecordNodeHints(record)).toEqual(['solo-host', 'solo-host']);
  });

  it('drops whitespace-only nodeHint entries and trims surrounding whitespace from valid ones', () => {
    // getRecordStringArrayDetail keeps only non-blank strings after trim.
    // The `value` argument also exercises the `Array.isArray` true arm with a
    // mixed-content array (some entries filtered, some kept). The
    // location.label has no '/', so its root and full-label candidates both
    // resolve to 'host-only' and appear twice in the output.
    const record = makeRecord({
      details: {
        nodeHints: ['  kept-hint  ', '   ', 'second-hint', 7] as unknown as string[],
      },
      refs: undefined,
      location: { label: 'host-only', scope: 'host' },
    });
    expect(getStorageRecordNodeHints(record)).toEqual([
      'kept-hint',
      'second-hint',
      'host-only',
      'host-only',
    ]);
  });

  it('returns [] from getRecordStringArrayDetail when the nodeHints detail is not an array', () => {
    // Array.isArray false arm: the `nodeHints` value is an object, so the
    // helper short-circuits to []. Only the location-derived candidates
    // remain — and because the label has no slash, both resolve to the same
    // value, so 'host-only' appears twice.
    const record = makeRecord({
      details: { nodeHints: { not: 'array' } },
      refs: undefined,
      location: { label: 'host-only', scope: 'host' },
    });
    expect(getStorageRecordNodeHints(record)).toEqual(['host-only', 'host-only']);
  });
});

// ===========================================================================
// getStorageRecordType — three-way fallback chain
// ===========================================================================

describe('getStorageRecordType branch coverage', () => {
  it('returns detail.type verbatim when it is a non-empty string', () => {
    const record = makeRecord({ details: { type: 'cephfs' } });
    expect(getStorageRecordType(record)).toBe('cephfs');
  });

  it('falls back to record.category when detail.type is absent', () => {
    // getRecordStringDetail returns '' (typeof undefined !== 'string') →
    // `'' || record.category` truthy arm returns category.
    const record = makeRecord({ details: {}, category: 'datastore' });
    expect(getStorageRecordType(record)).toBe('datastore');
  });

  it('returns "other" when detail.type is absent and category coerces to empty', () => {
    // Both `||` operands falsy → final 'other' fallback. category is typed as
    // a non-empty StorageCategory so the empty string requires a cast.
    const record = makeRecord({
      details: {},
      category: '' as StorageRecord['category'],
    });
    expect(getStorageRecordType(record)).toBe('other');
  });

  it('treats a non-string detail.type as absent and falls through to category', () => {
    // typeof value !== 'string' → '' → category wins.
    const record = makeRecord({
      details: { type: 99 },
      category: 'volume',
    });
    expect(getStorageRecordType(record)).toBe('volume');
  });
});

// ===========================================================================
// getStorageRecordContent — typeof guard both arms
// ===========================================================================

describe('getStorageRecordContent branch coverage', () => {
  it('returns the detail.content string when present', () => {
    const record = makeRecord({ details: { content: 'root dataset' } });
    expect(getStorageRecordContent(record)).toBe('root dataset');
  });

  it('returns "" when detail.content is absent', () => {
    const record = makeRecord({ details: {} });
    expect(getStorageRecordContent(record)).toBe('');
  });

  it('returns "" when detail.content is a non-string value', () => {
    // typeof false arm of the inner guard.
    const record = makeRecord({
      details: { content: { nested: 'object' } },
    });
    expect(getStorageRecordContent(record)).toBe('');
  });
});

// ===========================================================================
// getStorageRecordStatus — every health enum arm + detail.status override
// ===========================================================================

describe('getStorageRecordStatus branch coverage', () => {
  it('prefers detail.status over health-derived status', () => {
    const record = makeRecord({
      health: 'healthy',
      details: { status: 'scrubbing' },
    });
    expect(getStorageRecordStatus(record)).toBe('scrubbing');
  });

  it('maps health "warning" to "degraded"', () => {
    const record = makeRecord({ health: 'warning', details: {} });
    expect(getStorageRecordStatus(record)).toBe('degraded');
  });

  it('maps health "offline" to "offline"', () => {
    const record = makeRecord({ health: 'offline', details: {} });
    expect(getStorageRecordStatus(record)).toBe('offline');
  });

  it('maps health "critical" to "critical"', () => {
    const record = makeRecord({ health: 'critical', details: {} });
    expect(getStorageRecordStatus(record)).toBe('critical');
  });

  it('maps health "unknown" to "unknown" (final fallback arm)', () => {
    const record = makeRecord({ health: 'unknown', details: {} });
    expect(getStorageRecordStatus(record)).toBe('unknown');
  });
});

// ===========================================================================
// getStorageRecordPlatformLabel — platformLabel override both arms
// ===========================================================================

describe('getStorageRecordPlatformLabel branch coverage', () => {
  it('returns record.platformLabel verbatim when it trims to non-empty', () => {
    const record = makeRecord({
      platformLabel: 'Custom NAS Appliance',
      source: { platform: 'truenas', family: 'onprem', origin: 'resource', adapterId: 'a' },
    });
    expect(getStorageRecordPlatformLabel(record)).toBe('Custom NAS Appliance');
  });

  it('falls back to getSourcePlatformLabel when platformLabel is whitespace-only', () => {
    // '   '.trim() === '' → falsy → fallback arm fires.
    const record = makeRecord({
      platformLabel: '   ',
      source: { platform: 'truenas', family: 'onprem', origin: 'resource', adapterId: 'a' },
    });
    expect(getStorageRecordPlatformLabel(record)).toBe('TrueNAS');
  });

  it('returns a title-cased label for an unrecognized platform via the getSourcePlatformLabel fallback', () => {
    // Unknown platform key → SOURCE_PLATFORM_PRESENTATION has no entry →
    // titleCaseDelimitedLabel('acme-store') → 'Acme Store'.
    const record = makeRecord({
      platformLabel: undefined,
      source: {
        platform: 'acme-store',
        family: 'generic',
        origin: 'resource',
        adapterId: 'a',
      },
    });
    expect(getStorageRecordPlatformLabel(record)).toBe('Acme Store');
  });
});

// ===========================================================================
// getStorageRecordNodeLabel — parentName → node → location.label → 'unassigned'
// ===========================================================================

describe('getStorageRecordNodeLabel branch coverage', () => {
  it('falls back to detail.node when detail.parentName is absent', () => {
    const record = makeRecord({ details: { node: 'node-7' } });
    expect(getStorageRecordNodeLabel(record)).toBe('node-7');
  });

  it('falls back to location.label when both parentName and node are absent', () => {
    const record = makeRecord({
      details: {},
      location: { label: 'bare-host', scope: 'host' },
    });
    expect(getStorageRecordNodeLabel(record)).toBe('bare-host');
  });

  it('returns "unassigned" when parentName, node, and location.label are all empty', () => {
    const record = makeRecord({
      details: {},
      location: { label: '', scope: 'host' },
    });
    expect(getStorageRecordNodeLabel(record)).toBe('unassigned');
  });

  it('skips a whitespace-only detail.parentName and falls through to detail.node', () => {
    // parentName.trim() === '' → falsy → next arm fires.
    const record = makeRecord({ details: { parentName: '   ', node: 'node-9' } });
    expect(getStorageRecordNodeLabel(record)).toBe('node-9');
  });
});

// ===========================================================================
// getStorageRecordHostLabel — hostLabel truthy arm
// ===========================================================================

describe('getStorageRecordHostLabel branch coverage', () => {
  it('returns record.hostLabel verbatim when it trims to non-empty', () => {
    const record = makeRecord({
      hostLabel: 'primary-storage-host',
      details: {},
    });
    expect(getStorageRecordHostLabel(record)).toBe('primary-storage-host');
  });

  it('falls back to getStorageRecordNodeLabel when hostLabel is whitespace-only', () => {
    // hostLabel '   '.trim() === '' → falsy → node label derived from location.
    const record = makeRecord({
      hostLabel: '   ',
      details: {},
      location: { label: 'lonely-host', scope: 'host' },
    });
    expect(getStorageRecordHostLabel(record)).toBe('lonely-host');
  });
});

// ===========================================================================
// getStorageRecordTopologyLabel — topologyLabel override both arms
// ===========================================================================

describe('getStorageRecordTopologyLabel branch coverage', () => {
  it('returns record.topologyLabel verbatim when it trims to non-empty', () => {
    const record = makeRecord({
      topologyLabel: 'Stretched Cluster',
      details: {},
    });
    expect(getStorageRecordTopologyLabel(record)).toBe('Stretched Cluster');
  });

  it('falls back to getStorageRecordType when topologyLabel is absent', () => {
    // topologyLabel undefined → falsy → getStorageRecordType returns
    // detail.type ('rbd') via the truthy arm of its first `||`.
    const record = makeRecord({
      topologyLabel: undefined,
      details: { type: 'rbd' },
    });
    expect(getStorageRecordTopologyLabel(record)).toBe('rbd');
  });
});

// ===========================================================================
// getStorageRecordProtectionLabel — both arms
// ===========================================================================

describe('getStorageRecordProtectionLabel branch coverage', () => {
  it('returns record.protectionLabel verbatim when it trims to non-empty', () => {
    const record = makeRecord({ protectionLabel: 'Replicated · Snapshots' });
    expect(getStorageRecordProtectionLabel(record)).toBe('Replicated · Snapshots');
  });

  it('returns "Healthy" when protectionLabel is absent', () => {
    const record = makeRecord({ protectionLabel: undefined });
    expect(getStorageRecordProtectionLabel(record)).toBe('Healthy');
  });

  it('returns "Healthy" when protectionLabel is whitespace-only', () => {
    const record = makeRecord({ protectionLabel: '   ' });
    expect(getStorageRecordProtectionLabel(record)).toBe('Healthy');
  });
});

// ===========================================================================
// getStorageRecordIssueLabel — truthy arm
// ===========================================================================

describe('getStorageRecordIssueLabel branch coverage', () => {
  it('returns record.issueLabel verbatim when it trims to non-empty', () => {
    const record = makeRecord({ issueLabel: 'Scrub errors detected' });
    expect(getStorageRecordIssueLabel(record)).toBe('Scrub errors detected');
  });
});

// ===========================================================================
// getStorageRecordIssueSummary — three-way fallback
// ===========================================================================

describe('getStorageRecordIssueSummary branch coverage', () => {
  it('returns record.issueSummary verbatim when it trims to non-empty', () => {
    const record = makeRecord({
      issueSummary: '3 scrub errors on disk da1',
      issueLabel: 'placeholder',
    });
    expect(getStorageRecordIssueSummary(record)).toBe('3 scrub errors on disk da1');
  });

  it('falls back to record.issueLabel when issueSummary is absent', () => {
    // issueSummary undefined → `record.issueSummary?.trim()` is undefined →
    // the `||` chain advances to issueLabel.
    const record = makeRecord({
      issueSummary: undefined,
      issueLabel: 'Degraded mirror',
    });
    expect(getStorageRecordIssueSummary(record)).toBe('Degraded mirror');
  });

  it('returns "" when both issueSummary and issueLabel are absent', () => {
    const record = makeRecord({ issueSummary: undefined, issueLabel: undefined });
    expect(getStorageRecordIssueSummary(record)).toBe('');
  });

  it('returns "" when issueSummary and issueLabel are both whitespace-only', () => {
    // Both `.trim()` calls yield '' → final '' fallback.
    const record = makeRecord({ issueSummary: '   ', issueLabel: '\t' });
    expect(getStorageRecordIssueSummary(record)).toBe('');
  });
});

// ===========================================================================
// getStorageRecordImpactSummary — both arms
// ===========================================================================

describe('getStorageRecordImpactSummary branch coverage', () => {
  it('returns record.impactSummary verbatim when it trims to non-empty', () => {
    const record = makeRecord({ impactSummary: '12 VMs affected' });
    expect(getStorageRecordImpactSummary(record)).toBe('12 VMs affected');
  });

  it('returns "No dependent resources" when impactSummary is absent', () => {
    const record = makeRecord({ impactSummary: undefined });
    expect(getStorageRecordImpactSummary(record)).toBe('No dependent resources');
  });
});

// ===========================================================================
// getStorageRecordActionSummary — truthy arm
// ===========================================================================

describe('getStorageRecordActionSummary branch coverage', () => {
  it('returns record.actionSummary verbatim when it trims to non-empty', () => {
    const record = makeRecord({ actionSummary: 'Replace disk da2 within 24h' });
    expect(getStorageRecordActionSummary(record)).toBe('Replace disk da2 within 24h');
  });
});

// ===========================================================================
// getStorageRecordShared — boolean true/false/non-boolean arms
// ===========================================================================

describe('getStorageRecordShared branch coverage', () => {
  it('returns false when detail.shared is explicitly false', () => {
    const record = makeRecord({ details: { shared: false } });
    expect(getStorageRecordShared(record)).toBe(false);
  });

  it('returns null when detail.shared is a non-boolean primitive', () => {
    // typeof shared !== 'boolean' → null. Use a string to defeat TS.
    const record = makeRecord({
      details: { shared: 'true' } as Record<string, unknown>,
    });
    expect(getStorageRecordShared(record)).toBe(null);
  });

  it('returns null when detail.shared is missing entirely', () => {
    const record = makeRecord({ details: {} });
    expect(getStorageRecordShared(record)).toBe(null);
  });

  it('returns null when details itself is undefined (defensive `(record.details || {})` arm)', () => {
    const record = makeRecord({ details: undefined });
    expect(getStorageRecordShared(record)).toBe(null);
  });
});

// ===========================================================================
// getStorageRecordUsagePercent — fallback / NaN / division-guard branches
// ===========================================================================

describe('getStorageRecordUsagePercent branch coverage', () => {
  it('falls back to the total/used computation when usagePercent is null', () => {
    // usagePercent null → typeof check false → total 1000 / used 250 → 25.
    const record = makeRecord({
      capacity: { totalBytes: 1_000, usedBytes: 250, freeBytes: 750, usagePercent: null },
    });
    expect(getStorageRecordUsagePercent(record)).toBe(25);
  });

  it('treats NaN usagePercent as missing (Number.isFinite false arm)', () => {
    const record = makeRecord({
      capacity: { totalBytes: 1_000, usedBytes: 500, freeBytes: 500, usagePercent: NaN },
    });
    expect(getStorageRecordUsagePercent(record)).toBe(50);
  });

  it('falls back when usagePercent is a non-number value (typeof guard false arm)', () => {
    // Cast required: CapacitySnapshot.usagePercent is `number | null`.
    const record = makeRecord({
      capacity: {
        totalBytes: 200,
        usedBytes: 50,
        freeBytes: 150,
        usagePercent: '50' as unknown as number,
      },
    });
    expect(getStorageRecordUsagePercent(record)).toBe(25);
  });

  it('returns 0 when total <= 0 (the `total <= 0` true arm of the guard)', () => {
    const record = makeRecord({
      capacity: { totalBytes: 0, usedBytes: 500, freeBytes: 0, usagePercent: null },
    });
    expect(getStorageRecordUsagePercent(record)).toBe(0);
  });

  it('returns 0 when total is negative (boundary below zero)', () => {
    const record = makeRecord({
      capacity: { totalBytes: -100, usedBytes: 50, freeBytes: 0, usagePercent: null },
    });
    expect(getStorageRecordUsagePercent(record)).toBe(0);
  });

  it('coerces null total/used bytes to 0 via the `|| 0` arms before the guard', () => {
    // total null → 0 → `0 <= 0` true → 0. Exercises both `|| 0` falsy arms.
    const record = makeRecord({
      capacity: { totalBytes: null, usedBytes: null, freeBytes: null, usagePercent: null },
    });
    expect(getStorageRecordUsagePercent(record)).toBe(0);
  });
});

// ===========================================================================
// getStorageRecordZfsPool — toZfsPool null/invalid arms reached via the
// public surface (the module-private toZfsPool is only exercised here).
// ===========================================================================

describe('getStorageRecordZfsPool branch coverage', () => {
  it('returns null when details.zfsPool is missing', () => {
    const record = makeRecord({ details: {} });
    expect(getStorageRecordZfsPool(record)).toBeNull();
  });

  it('returns null when details.zfsPool is null (`!value` truthy arm)', () => {
    const record = makeRecord({
      details: { zfsPool: null } as Record<string, unknown>,
    });
    expect(getStorageRecordZfsPool(record)).toBeNull();
  });

  it('returns null when details.zfsPool is a primitive (`typeof !== "object"` arm)', () => {
    const record = makeRecord({
      details: { zfsPool: 'DEGRADED' } as Record<string, unknown>,
    });
    expect(getStorageRecordZfsPool(record)).toBeNull();
  });

  it('returns null when zfsPool.state is a non-string', () => {
    // typeof candidate.state !== 'string' false arm.
    const record = makeRecord({
      details: {
        zfsPool: { state: 7, devices: [] },
      } as Record<string, unknown>,
    });
    expect(getStorageRecordZfsPool(record)).toBeNull();
  });

  it('returns null when zfsPool.devices is not an array', () => {
    // state is a valid string but devices is an object → Array.isArray false.
    const record = makeRecord({
      details: {
        zfsPool: { state: 'ONLINE', devices: { count: 0 } },
      } as Record<string, unknown>,
    });
    expect(getStorageRecordZfsPool(record)).toBeNull();
  });

  it('returns the validated zfsPool payload (with extra fields) when both guards pass', () => {
    const pool = {
      state: 'DEGRADED',
      devices: [{ name: 'da0', type: 'disk', state: 'ONLINE' }],
      name: 'tank',
      status: 'Degraded',
      scan: 'none',
      readErrors: 0,
      writeErrors: 0,
      checksumErrors: 0,
    } as ZFSPool;
    const record = makeRecord({ details: { zfsPool: pool } });
    expect(getStorageRecordZfsPool(record)).toEqual(pool);
  });
});

// ===========================================================================
// getStorageRecordStats — empty input, every health arm, shared=false,
// total=0, shared dedup with differing platforms.
// ===========================================================================

describe('getStorageRecordStats branch coverage', () => {
  it('returns all-zero totals for an empty items array', () => {
    expect(getStorageRecordStats([])).toEqual({
      totalBytes: 0,
      usedBytes: 0,
      usagePercent: 0,
      byHealth: { healthy: 0, warning: 0, critical: 0, offline: 0, unknown: 0 },
    });
  });

  it('counts critical / offline / unknown records in byHealth', () => {
    const critical = makeRecord({
      id: 'crit',
      health: 'critical',
      capacity: { totalBytes: 100, usedBytes: 100, freeBytes: 0, usagePercent: 100 },
    });
    const offline = makeRecord({
      id: 'off',
      health: 'offline',
      capacity: { totalBytes: 200, usedBytes: 0, freeBytes: 200, usagePercent: 0 },
    });
    const unknown = makeRecord({
      id: 'unk',
      health: 'unknown',
      capacity: { totalBytes: 300, usedBytes: 150, freeBytes: 150, usagePercent: 50 },
    });
    expect(getStorageRecordStats([critical, offline, unknown])).toEqual({
      totalBytes: 600,
      usedBytes: 250,
      usagePercent: (250 / 600) * 100,
      byHealth: { healthy: 0, warning: 0, critical: 1, offline: 1, unknown: 1 },
    });
  });

  it('counts shared=false records toward totals without registering them in the dedup set', () => {
    // shared=false → `getStorageRecordShared(record) === true` is false → the
    // `if (isShared)` block is skipped entirely; totals still aggregate.
    const a = makeRecord({
      id: 'a',
      name: 'tank',
      capacity: { totalBytes: 500, usedBytes: 100, freeBytes: 400, usagePercent: 20 },
      details: { shared: false },
    });
    const b = makeRecord({
      id: 'b',
      name: 'tank',
      capacity: { totalBytes: 500, usedBytes: 200, freeBytes: 300, usagePercent: 40 },
      details: { shared: false },
    });
    expect(getStorageRecordStats([a, b])).toEqual({
      totalBytes: 1_000,
      usedBytes: 300,
      usagePercent: 30,
      byHealth: { healthy: 2, warning: 0, critical: 0, offline: 0, unknown: 0 },
    });
  });

  it('returns usagePercent 0 via the `totals.total > 0` false arm when totals sum to zero', () => {
    // totalBytes 0 on every record → totals.total = 0 → guard returns 0
    // instead of dividing.
    const dead = makeRecord({
      id: 'dead',
      health: 'offline',
      capacity: { totalBytes: 0, usedBytes: 0, freeBytes: 0, usagePercent: null },
    });
    expect(getStorageRecordStats([dead])).toEqual({
      totalBytes: 0,
      usedBytes: 0,
      usagePercent: 0,
      byHealth: { healthy: 0, warning: 0, critical: 0, offline: 1, unknown: 0 },
    });
  });

  it('deduplicates shared records only within the same platform+name key', () => {
    // Two shared records with the SAME name but DIFFERENT platforms are NOT
    // deduplicated — both contribute to totals. Exercises the
    // `seenShared.has(sharedKey)` false arm for a differing-platform key.
    const pbsShared = makeRecord({
      id: 'pbs',
      name: 'tank',
      source: { platform: 'proxmox-pbs', family: 'onprem', origin: 'resource', adapterId: 'x' },
      capacity: { totalBytes: 1_000, usedBytes: 500, freeBytes: 500, usagePercent: null },
      details: { shared: true },
    });
    const truenasShared = makeRecord({
      id: 'tn',
      name: 'tank',
      source: { platform: 'truenas', family: 'onprem', origin: 'resource', adapterId: 'y' },
      capacity: { totalBytes: 1_000, usedBytes: 200, freeBytes: 800, usagePercent: null },
      details: { shared: true },
    });
    expect(getStorageRecordStats([pbsShared, truenasShared])).toEqual({
      totalBytes: 2_000,
      usedBytes: 700,
      usagePercent: 35,
      byHealth: { healthy: 2, warning: 0, critical: 0, offline: 0, unknown: 0 },
    });
  });

  it('coerces null capacity bytes to 0 in the totals accumulator via the `|| 0` arms', () => {
    // record.capacity.totalBytes null → `|| 0` falsy arm → 0. Same for used.
    const nullish = makeRecord({
      id: 'nul',
      health: 'warning' as NormalizedHealth,
      capacity: { totalBytes: null, usedBytes: null, freeBytes: null, usagePercent: null },
    });
    expect(getStorageRecordStats([nullish])).toEqual({
      totalBytes: 0,
      usedBytes: 0,
      usagePercent: 0,
      byHealth: { healthy: 0, warning: 1, critical: 0, offline: 0, unknown: 0 },
    });
  });
});
