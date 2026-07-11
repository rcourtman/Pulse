import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getResourceStorageActionSummary,
  getResourceStorageImpactSummary,
  getResourceStorageIssueLabel,
  getResourceStorageProtectionLabel,
  getResourceStorageTopologyLabel,
  isUnraidStorageResource,
} from '@/features/storageBackups/resourceStoragePresentation';

// Mirrors the sibling test factories. Pure-function module, so no Solid root
// is required. Module-private target functions
// (unraidShortSyncLabel / getUnraidShortProtectionLabel /
//  getUnraidShortIssueLabel / isAttentionStatus) are exercised transitively
// through their exported callers.
const makeResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'storage-1',
    type: 'storage',
    name: 'tank',
    platformType: 'truenas',
    sourceType: 'api',
    status: 'online',
    storage: {},
    ...overrides,
  }) as Resource;

// ---------------------------------------------------------------------------
// isUnraidStorageResource (exported)
// ---------------------------------------------------------------------------

describe('isUnraidStorageResource', () => {
  it('returns false when storage metadata is absent', () => {
    const resource = { ...makeResource(), storage: undefined } as unknown as Resource;
    expect(isUnraidStorageResource(resource)).toBe(false);
  });

  it('returns false for empty storage metadata with no array, sync, or risk reasons', () => {
    expect(isUnraidStorageResource(makeResource({ storage: {} }))).toBe(false);
  });

  it('returns true when arrayState is present', () => {
    expect(isUnraidStorageResource(makeResource({ storage: { arrayState: 'STARTED' } }))).toBe(
      true,
    );
  });

  it('returns true when only syncAction is present (no arrayState)', () => {
    expect(isUnraidStorageResource(makeResource({ storage: { syncAction: 'check' } }))).toBe(true);
  });

  it('returns true when a risk reason code starts with unraid_', () => {
    const resource = makeResource({
      storage: {
        risk: {
          level: 'warning',
          reasons: [
            { code: 'zfs_state', severity: 'warning', summary: 'irrelevant' },
            { code: 'unraid_sync_active', severity: 'warning', summary: 'syncing' },
          ],
        },
      },
    });
    expect(isUnraidStorageResource(resource)).toBe(true);
  });

  it('returns false when risk reasons exist but none start with unraid_', () => {
    const resource = makeResource({
      storage: {
        risk: {
          level: 'warning',
          reasons: [{ code: 'zfs_state', severity: 'warning', summary: 'degraded' }],
        },
      },
    });
    expect(isUnraidStorageResource(resource)).toBe(false);
  });

  it('returns false when the risk reasons array is empty', () => {
    const resource = makeResource({
      storage: { risk: { level: 'warning', reasons: [] } },
    });
    expect(isUnraidStorageResource(resource)).toBe(false);
  });

  it('does not throw when a reason code is missing', () => {
    const resource = {
      ...makeResource(),
      storage: {
        risk: {
          level: 'warning',
          reasons: [{ summary: 'no code' }] as unknown as Resource['storage'] extends {
            risk?: { reasons?: infer R };
          }
            ? R
            : never,
        },
      },
    } as unknown as Resource;
    expect(isUnraidStorageResource(resource)).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// unraidShortSyncLabel (private) — exercised via getResourceStorageProtectionLabel
// on Unraid resources with rebuildInProgress === true.
// ---------------------------------------------------------------------------

describe('unraidShortSyncLabel (via getResourceStorageProtectionLabel)', () => {
  const rebuildResource = (syncAction: string | undefined, syncProgress?: number): Resource =>
    makeResource({
      storage: {
        arrayState: 'STARTED',
        rebuildInProgress: true,
        syncAction,
        syncProgress,
      },
    });

  it('labels a parity rebuild for recon-p and rebuild sync actions', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('recon-p'))).toBe('Parity rebuild');
    expect(getResourceStorageProtectionLabel(rebuildResource('rebuild'))).toBe('Parity rebuild');
  });

  it('labels a parity sync action', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('sync'))).toBe('Parity sync');
  });

  it('labels a clear action', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('clear'))).toBe('Clearing');
  });

  it('falls back to Rebuilding for an empty sync action', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource(''))).toBe('Rebuilding');
  });

  it('falls back to Rebuilding for a whitespace-only sync action', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('   '))).toBe('Rebuilding');
  });

  it('titleizes an unknown sync action in the default branch', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('foo'))).toBe('Foo');
  });

  it('titleizes a multi-word unknown sync action', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('parity-sync'))).toBe('Parity Sync');
  });

  it('titleizes an unknown sync action case-insensitively', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('CHECK'))).toBe('Parity check');
  });

  it('omits the progress suffix when syncProgress is zero', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('check', 0))).toBe('Parity check');
  });

  it('omits the progress suffix when syncProgress is NaN (not finite)', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('check', NaN))).toBe('Parity check');
  });

  it('omits the progress suffix when syncProgress is Infinity', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('check', Infinity))).toBe(
      'Parity check',
    );
  });

  it('omits the progress suffix when syncProgress is negative', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('check', -5))).toBe('Parity check');
  });

  it('omits the progress suffix when syncProgress is not a number type', () => {
    const resource = {
      ...makeResource(),
      storage: {
        arrayState: 'STARTED',
        rebuildInProgress: true,
        syncAction: 'check',
        syncProgress: '50' as unknown as number,
      },
    } as unknown as Resource;
    expect(getResourceStorageProtectionLabel(resource)).toBe('Parity check');
  });

  it('rounds and appends positive syncProgress', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('check', 47.6))).toBe(
      'Parity check (48%)',
    );
  });

  it('omits the progress suffix when syncProgress is undefined', () => {
    expect(getResourceStorageProtectionLabel(rebuildResource('recon'))).toBe('Parity rebuild');
  });
});

// ---------------------------------------------------------------------------
// getUnraidShortProtectionLabel (private) — via getResourceStorageProtectionLabel
// on Unraid resources without an active rebuild.
// ---------------------------------------------------------------------------

describe('getUnraidShortProtectionLabel (via getResourceStorageProtectionLabel)', () => {
  it('returns No parity from the unraid_no_parity risk reason when protection is not "none"', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        protection: 'single',
        protectionReduced: true,
        risk: {
          level: 'warning',
          reasons: [
            { code: 'unraid_no_parity', severity: 'warning', summary: 'no parity running' },
          ],
        },
      },
    });
    expect(getResourceStorageProtectionLabel(resource)).toBe('No parity');
  });

  it('returns Parity unavailable when the unraid_parity_unavailable reason is present', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        protection: 'single',
        protectionReduced: true,
        risk: {
          level: 'warning',
          reasons: [
            {
              code: 'unraid_parity_unavailable',
              severity: 'warning',
              summary: 'parity disk unavailable',
            },
          ],
        },
      },
    });
    expect(getResourceStorageProtectionLabel(resource)).toBe('Parity unavailable');
  });

  it('returns Protection reduced when protection is reduced but no parity reason is present', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        protection: 'single',
        protectionReduced: true,
      },
    });
    // getUnraidShortProtectionLabel returns 'Protection reduced'; the caller
    // then does NOT fall through because the label is non-empty.
    expect(getResourceStorageProtectionLabel(resource)).toBe('Protection reduced');
  });

  it('falls through to the generic protection path when the Unraid array is healthy', () => {
    // rebuildInProgress false + protectionReduced false => getUnraidShortProtectionLabel
    // returns '' so getResourceStorageProtectionLabel continues.
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        protection: 'dual',
      },
    });
    expect(getResourceStorageProtectionLabel(resource)).toBe('Dual');
  });
});

// ---------------------------------------------------------------------------
// getUnraidShortIssueLabel (private) — via getResourceStorageIssueLabel
// on Unraid resources.
// ---------------------------------------------------------------------------

describe('getUnraidShortIssueLabel (via getResourceStorageIssueLabel)', () => {
  it('returns Parity unavailable for the unraid_parity_unavailable reason code', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            {
              code: 'unraid_parity_unavailable',
              severity: 'warning',
              summary: 'parity disk is down',
            },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('Parity unavailable');
  });

  it('returns the summary for the unraid_invalid_disks reason code', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            { code: 'unraid_invalid_disks', severity: 'warning', summary: '2 invalid disks' },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('2 invalid disks');
  });

  it('strips a leading "Unraid array reports " prefix from the summary', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            {
              code: 'unraid_disabled_disks',
              severity: 'warning',
              summary: 'Unraid array reports 1 disabled disk',
            },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('1 disabled disk');
  });

  it('leaves a bare "Unraid array reports" prefix intact because trim removes the trailing whitespace the regex needs', () => {
    // trimSummary trims trailing whitespace BEFORE the prefix regex runs, so
    // the regex's required `\s+` can only match internal whitespace — a bare
    // prefix string passes through unchanged.
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            {
              code: 'unraid_missing_disks',
              severity: 'warning',
              summary: 'Unraid array reports   ',
            },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('Unraid array reports');
  });

  it('falls back to Disk issue when the summary is whitespace only', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [{ code: 'unraid_invalid_disks', severity: 'warning', summary: '   ' }],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('Disk issue');
  });

  it('returns Healthy when the Unraid resource carries only non-attention reason codes', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [{ code: 'unraid_sync_active', severity: 'warning', summary: 'syncing' }],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('Healthy');
  });

  it('returns Healthy when the Unraid resource has no risk reasons at all', () => {
    const resource = makeResource({ storage: { arrayState: 'STARTED' } });
    expect(getResourceStorageIssueLabel(resource)).toBe('Healthy');
  });

  it('returns the first matching attention issue, not a later reason', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            { code: 'unraid_sync_active', severity: 'warning', summary: 'syncing' },
            { code: 'unraid_missing_disks', severity: 'warning', summary: '3 missing' },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('3 missing');
  });
});

// ---------------------------------------------------------------------------
// isAttentionStatus (private) — exercised via getResourceStorageIssueLabel,
// which surfaces the composite posture summary only for attention statuses.
// ---------------------------------------------------------------------------

describe('isAttentionStatus (via getResourceStorageIssueLabel)', () => {
  const postureResource = (status: string | undefined): Resource =>
    makeResource({
      status: status as Resource['status'],
      storage: { postureSummary: 'At risk' },
    });

  it.each([
    ['warn'],
    ['critical'],
    ['faulted'],
    ['failed'],
    ['error'],
    ['unhealthy'],
    ['down'],
    ['unavailable'],
  ])('treats status %s as an attention status', (status) => {
    expect(getResourceStorageIssueLabel(postureResource(status))).toBe('At risk');
  });

  it('treats uppercase attention statuses case-insensitively', () => {
    expect(getResourceStorageIssueLabel(postureResource('CRITICAL'))).toBe('At risk');
  });

  it.each([['unknown'], ['running'], ['idle'], ['online']])(
    'does not treat status %s as an attention status',
    (status) => {
      expect(getResourceStorageIssueLabel(postureResource(status))).toBe('Healthy');
    },
  );

  it('returns false for an empty-string status', () => {
    expect(getResourceStorageIssueLabel(postureResource(''))).toBe('Healthy');
  });

  it('returns false for a whitespace-only status', () => {
    expect(getResourceStorageIssueLabel(postureResource('   '))).toBe('Healthy');
  });
});

// ---------------------------------------------------------------------------
// getResourceStorageImpactSummary (exported)
// ---------------------------------------------------------------------------

describe('getResourceStorageImpactSummary', () => {
  it('prefers incidentImpactSummary over every storage/PBS impact summary', () => {
    const resource = makeResource({
      incidentImpactSummary: 'Incident-level impact',
      storage: { consumerImpactSummary: 'consumer impact' },
      pbs: {
        protectedWorkloadSummary: 'protected workload impact',
        affectedDatastoreSummary: 'affected datastore impact',
      },
    });
    expect(getResourceStorageImpactSummary(resource)).toBe('Incident-level impact');
  });

  it('returns the PBS affected datastore summary when no other impact is present', () => {
    const resource = makeResource({
      pbs: { affectedDatastoreSummary: 'Datastore ds-2 unavailable' },
    });
    expect(getResourceStorageImpactSummary(resource)).toBe('Datastore ds-2 unavailable');
  });

  it('prefers the storage consumer impact over PBS summaries', () => {
    const resource = makeResource({
      storage: { consumerImpactSummary: 'Affects 1 dependent resource' },
      pbs: { affectedDatastoreSummary: 'should not win' },
    });
    expect(getResourceStorageImpactSummary(resource)).toBe('Affects 1 dependent resource');
  });

  it('prefers the PBS protected workload summary over the affected datastore summary', () => {
    const resource = makeResource({
      pbs: {
        protectedWorkloadSummary: 'workload impact',
        affectedDatastoreSummary: 'datastore impact',
      },
    });
    expect(getResourceStorageImpactSummary(resource)).toBe('workload impact');
  });

  it('returns the default dependent-resources copy when every summary is blank', () => {
    expect(getResourceStorageImpactSummary(makeResource())).toBe('No dependent resources');
  });

  it('returns the default when summaries are whitespace only', () => {
    const resource = makeResource({
      incidentImpactSummary: '   ',
      storage: { consumerImpactSummary: ' ' },
    });
    expect(getResourceStorageImpactSummary(resource)).toBe('No dependent resources');
  });
});

// ---------------------------------------------------------------------------
// getResourceStorageActionSummary (exported)
// ---------------------------------------------------------------------------

describe('getResourceStorageActionSummary', () => {
  it('short-circuits to Monitor for an Unraid resource without an attention issue', () => {
    const resource = makeResource({
      incidentAction: 'should not win',
      storage: {
        arrayState: 'STARTED',
        syncAction: 'check',
      },
    });
    expect(getResourceStorageActionSummary(resource)).toBe('Monitor');
  });

  it('uses the rebuild summary when a non-Unraid rebuild is in progress', () => {
    const resource = makeResource({
      storage: {
        rebuildInProgress: true,
        rebuildSummary: 'Resilvering in progress',
      },
    });
    expect(getResourceStorageActionSummary(resource)).toBe('Resilvering in progress');
  });

  it('falls back to Monitor rebuild progress when rebuildSummary is absent', () => {
    const resource = makeResource({ storage: { rebuildInProgress: true } });
    expect(getResourceStorageActionSummary(resource)).toBe('Monitor rebuild progress');
  });

  it('uses the protection summary when protection is reduced', () => {
    const resource = makeResource({
      storage: {
        protectionReduced: true,
        protectionSummary: 'Restore redundancy immediately',
      },
    });
    expect(getResourceStorageActionSummary(resource)).toBe('Restore redundancy immediately');
  });

  it('falls back to Restore redundancy when protectionSummary is absent', () => {
    const resource = makeResource({ storage: { protectionReduced: true } });
    expect(getResourceStorageActionSummary(resource)).toBe('Restore redundancy');
  });

  it('prefers an incident action over rebuild/protection states', () => {
    const resource = makeResource({
      incidentAction: 'Investigate now',
      storage: { rebuildInProgress: true, rebuildSummary: 'rebuilding' },
    });
    expect(getResourceStorageActionSummary(resource)).toBe('Investigate now');
  });

  it('returns Monitor for a healthy resource with no actionable state', () => {
    expect(getResourceStorageActionSummary(makeResource())).toBe('Monitor');
  });
});

// ---------------------------------------------------------------------------
// getResourceStorageTopologyLabel (exported) — edge cases complementing the
// already-thorough coverage test.
// ---------------------------------------------------------------------------

describe('getResourceStorageTopologyLabel (edge cases)', () => {
  it('lowercases storageType before switching so uppercase pool types classify', () => {
    expect(getResourceStorageTopologyLabel(makeResource(), 'POOL')).toBe('Pool');
  });

  it('classifies a CEPHFS storage type case-insensitively', () => {
    expect(getResourceStorageTopologyLabel(makeResource(), 'CephFS')).toBe('Cluster Storage');
  });

  it('titleizes an explicit topology arg that uses multiple delimiters', () => {
    expect(getResourceStorageTopologyLabel(makeResource(), 'rbd', 'rebuilt_target')).toBe(
      'Rebuilt Target',
    );
  });

  it('falls back through titleized storageType to titleized resource type for an unknown type', () => {
    const resource = makeResource({ type: 'network' });
    expect(getResourceStorageTopologyLabel(resource, '')).toBe('Network');
  });
});

// ---------------------------------------------------------------------------
// getResourceStorageIssueLabel (exported) — Unraid issue surfacing.
// ---------------------------------------------------------------------------

describe('getResourceStorageIssueLabel (Unraid issue surfacing)', () => {
  it('returns the Unraid attention issue rather than the Healthy default', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            { code: 'unraid_parity_unavailable', severity: 'warning', summary: 'parity down' },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('Parity unavailable');
  });

  it('returns Healthy for an Unraid resource whose issue is not an attention issue', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [{ code: 'unraid_sync_active', severity: 'warning', summary: 'syncing' }],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('Healthy');
  });
});

// ---------------------------------------------------------------------------
// getResourceStorageProtectionLabel (exported) — fallback branches.
// ---------------------------------------------------------------------------

describe('getResourceStorageProtectionLabel (fallback branches)', () => {
  it('uses Rebuild In Progress when a non-Unraid rebuild has no rebuildSummary', () => {
    const resource = makeResource({ storage: { rebuildInProgress: true } });
    expect(getResourceStorageProtectionLabel(resource)).toBe('Rebuild In Progress');
  });

  it('uses Protection Reduced when protection is reduced with no protectionSummary', () => {
    const resource = makeResource({ storage: { protectionReduced: true } });
    expect(getResourceStorageProtectionLabel(resource)).toBe('Protection Reduced');
  });

  it('uses Backup Risk for a recoverability incident with no incident label', () => {
    const resource = makeResource({ incidentCategory: 'recoverability' });
    expect(getResourceStorageProtectionLabel(resource)).toBe('Backup Risk');
  });

  it('returns Protected for a backup-repository resource with no explicit protection', () => {
    const resource = makeResource({
      type: 'pbs',
      platformType: 'proxmox-pbs',
      storage: { platform: 'proxmox-pbs', type: 'pbs' },
    });
    expect(getResourceStorageProtectionLabel(resource)).toBe('Protected');
  });

  it('returns Healthy for a plain non-repository resource with no protection metadata', () => {
    expect(getResourceStorageProtectionLabel(makeResource({ storage: {} }))).toBe('Healthy');
  });
});
