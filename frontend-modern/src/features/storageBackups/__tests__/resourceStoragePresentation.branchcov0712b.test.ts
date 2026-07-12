import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getResourceStorageIssueLabel,
  getResourceStorageProtectionLabel,
  getResourceStorageProtectionSummary,
  getResourceStorageTopologyLabel,
} from '@/features/storageBackups/resourceStoragePresentation';

// Mirrors the sibling test factories (see resourceStoragePresentation.test.ts
// and resourceStoragePresentation.branchcov.test.ts). Pure-function module, so
// no Solid root is required. The two private target functions
// (getUnraidShortProtectionLabel / getUnraidShortIssueLabel) are exercised
// transitively through their exported callers
// (getResourceStorageProtectionLabel / getResourceStorageIssueLabel) — no
// export hacking. Scope is limited to the four named functions.
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
// getUnraidShortProtectionLabel (private) — exercised via
// getResourceStorageProtectionLabel on Unraid resources (arrayState set so
// isUnraidStorageResource returns true). Targets the protectionReduced
// sub-branches the sibling suites do not reach: case/whitespace-insensitive
// "none" matching, the protection-undefined `|| ""` arm, and the
// no-rebuild/no-reduced fall-through.
// ---------------------------------------------------------------------------

describe('getUnraidShortProtectionLabel (via getResourceStorageProtectionLabel)', () => {
  it('treats an uppercase "NONE" protection as No parity via the toLowerCase arm', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        protection: 'NONE',
        protectionReduced: true,
      },
    });
    expect(getResourceStorageProtectionLabel(resource)).toBe('No parity');
  });

  it('treats a whitespace-padded mixed-case protection as No parity via the trim arm', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        protection: '  NoNe  ',
        protectionReduced: true,
      },
    });
    expect(getResourceStorageProtectionLabel(resource)).toBe('No parity');
  });

  it('hits the protection-undefined `|| ""` falsy arm then resolves No parity from the unraid_no_parity reason', () => {
    // protectionReduced true, protection absent => `(storage.protection || "")`
    // takes the falsy arm; the `=== "none"` check is false, so the
    // unraid_no_parity risk reason decides the label.
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        protectionReduced: true,
        risk: {
          level: 'warning',
          reasons: [{ code: 'unraid_no_parity', severity: 'warning', summary: 'no parity' }],
        },
      },
    });
    expect(getResourceStorageProtectionLabel(resource)).toBe('No parity');
  });

  it('returns Protection reduced when protection is absent and neither parity risk reason is present', () => {
    // Same `|| ""` falsy arm, but both findRiskReason calls return undefined,
    // landing on the terminal `return "Protection reduced"`.
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        protectionReduced: true,
      },
    });
    expect(getResourceStorageProtectionLabel(resource)).toBe('Protection reduced');
  });

  it('returns an empty Unraid label when rebuild and protectionReduced are both false, letting the caller titleize the raw protection', () => {
    // Exercises getUnraidShortProtectionLabel's final `return ""` so the
    // exported caller falls through to its generic protection path.
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        protection: 'dual-parity',
      },
    });
    expect(getResourceStorageProtectionLabel(resource)).toBe('Dual Parity');
  });
});

// ---------------------------------------------------------------------------
// getUnraidShortIssueLabel (private) — exercised via
// getResourceStorageIssueLabel on Unraid resources. Targets the empty-code
// `|| ""` arm inside the loop, the loop fall-through to Healthy, the
// case-insensitive prefix regex, and PBS-storageRisk-only reason collection.
// ---------------------------------------------------------------------------

describe('getUnraidShortIssueLabel (via getResourceStorageIssueLabel)', () => {
  it('continues past a reason with an empty code via the `(reason.code || "")` falsy arm', () => {
    // collectRiskReasons keeps the truthy reason object; inside the loop the
    // empty code becomes "" and the switch matches no case, so iteration
    // proceeds to the next reason.
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            { code: '', severity: 'warning', summary: 'ignored empty code' },
            { code: 'unraid_missing_disks', severity: 'warning', summary: '2 missing' },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('2 missing');
  });

  it('returns Healthy when every collected reason carries an empty or non-attention code', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            { code: '', severity: 'warning', summary: 'empty code reason' },
            { code: 'unraid_sync_active', severity: 'warning', summary: 'syncing' },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('Healthy');
  });

  it('strips an uppercase "UNRAID ARRAY REPORTS " prefix case-insensitively via the /i regex', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            {
              code: 'unraid_invalid_disks',
              severity: 'warning',
              summary: 'unRAID aRRay reports 4 invalid disks',
            },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('4 invalid disks');
  });

  it('returns the unraid_disabled_disks summary verbatim when it carries no prefix', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            { code: 'unraid_disabled_disks', severity: 'warning', summary: 'disk3 disabled' },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('disk3 disabled');
  });

  it('surfaces a reason collected only from pbs.storageRisk for an Unraid resource', () => {
    // collectRiskReasons walks storage.risk.reasons then pbs.storageRisk.reasons;
    // with no storage risk, the PBS reason is still collected and matched.
    const resource = makeResource({
      storage: { arrayState: 'STARTED' },
      pbs: {
        storageRisk: {
          level: 'warning',
          reasons: [
            { code: 'unraid_parity_unavailable', severity: 'warning', summary: 'parity down' },
          ],
        },
      },
    });
    expect(getResourceStorageIssueLabel(resource)).toBe('Parity unavailable');
  });
});

// ---------------------------------------------------------------------------
// getResourceStorageTopologyLabel (exported) — edge branches complementing
// the existing coverage: topology-arg normalization, datastore precedence over
// a pool-like storageType, and whitespace-only storageType fall-through.
// ---------------------------------------------------------------------------

describe('getResourceStorageTopologyLabel (branch coverage)', () => {
  it('normalizes a whitespace-padded mixed-case topology arg before titleizing', () => {
    // `(topology || "").trim().toLowerCase()` => 'backup-target' => titleize.
    expect(getResourceStorageTopologyLabel(makeResource(), 'pool', '  BackUP-Target  ')).toBe(
      'Backup Target',
    );
  });

  it('classifies a datastore entityType ahead of a pool-like storageType', () => {
    // resource.type !== 'datastore' but vmware.entityType === 'datastore' wins
    // over storageType 'zfspool', proving the datastore check precedes the
    // switch and that the `||` short-circuits to the entityType arm.
    const resource = makeResource({
      type: 'storage',
      vmware: { entityType: 'datastore' },
    });
    expect(getResourceStorageTopologyLabel(resource, 'zfspool')).toBe('Datastore');
  });

  it('falls through a whitespace-only storageType to the titleized resource type', () => {
    // The switch input `(storageType || "").trim().toLowerCase()` becomes "" =>
    // default branch => titleize("   ") => "" => `||` titleize(resource.type).
    const resource = { ...makeResource(), type: 'nvme-tank' } as unknown as Resource;
    expect(getResourceStorageTopologyLabel(resource, '   ')).toBe('Nvme Tank');
  });

  it('returns Storage when both storageType and resource type are whitespace-only', () => {
    // titleize("") || titleize("") || "Storage" => "Storage".
    const resource = { ...makeResource(), type: '   ' } as unknown as Resource;
    expect(getResourceStorageTopologyLabel(resource, '   ')).toBe('Storage');
  });
});

// ---------------------------------------------------------------------------
// getResourceStorageProtectionSummary (exported) — drives every guard arm and
// the trimSummary fallbacks for rebuild/protection/incident states, including
// the optional-chain falsy arms when storage metadata is absent.
// ---------------------------------------------------------------------------

describe('getResourceStorageProtectionSummary (branch coverage)', () => {
  it('returns an empty string when a rebuild is in progress but rebuildSummary is absent', () => {
    // trimSummary(undefined) => "".
    const resource = makeResource({ storage: { rebuildInProgress: true } });
    expect(getResourceStorageProtectionSummary(resource)).toBe('');
  });

  it('trims a whitespace-padded rebuildSummary during an active rebuild', () => {
    const resource = makeResource({
      storage: { rebuildInProgress: true, rebuildSummary: '  Resilvering in progress  ' },
    });
    expect(getResourceStorageProtectionSummary(resource)).toBe('Resilvering in progress');
  });

  it('returns an empty string when a rebuild is active with a whitespace-only rebuildSummary', () => {
    const resource = makeResource({
      storage: { rebuildInProgress: true, rebuildSummary: '   ' },
    });
    expect(getResourceStorageProtectionSummary(resource)).toBe('');
  });

  it('returns the trimmed protectionSummary when protection is reduced and no rebuild is active', () => {
    const resource = makeResource({
      storage: { protectionReduced: true, protectionSummary: '  Redundancy degraded  ' },
    });
    expect(getResourceStorageProtectionSummary(resource)).toBe('Redundancy degraded');
  });

  it('returns an empty string when protection is reduced but protectionSummary is absent', () => {
    const resource = makeResource({ storage: { protectionReduced: true } });
    expect(getResourceStorageProtectionSummary(resource)).toBe('');
  });

  it('returns the trimmed incidentSummary for a recoverability incident', () => {
    const resource = makeResource({
      incidentCategory: 'recoverability',
      incidentSummary: '  No recent backups  ',
    });
    expect(getResourceStorageProtectionSummary(resource)).toBe('No recent backups');
  });

  it('returns an empty string for a recoverability incident with no incidentSummary', () => {
    const resource = makeResource({ incidentCategory: 'recoverability' });
    expect(getResourceStorageProtectionSummary(resource)).toBe('');
  });

  it('returns an empty string when storage metadata is entirely absent (optional-chain falsy arms)', () => {
    // resource.storage?.rebuildInProgress and ?.protectionReduced both evaluate
    // to undefined and skip their guards.
    const resource = { ...makeResource(), storage: undefined } as unknown as Resource;
    expect(getResourceStorageProtectionSummary(resource)).toBe('');
  });

  it('returns an empty string for a healthy resource with no protection state', () => {
    expect(getResourceStorageProtectionSummary(makeResource())).toBe('');
  });
});
