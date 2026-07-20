import { describe, expect, it } from 'vitest';
import type { Resource, ResourceStorageRiskReason } from '@/types/resource';
import {
  getCanonicalStoragePlatformKey,
  getResourceStorageIssueLabel,
  getResourceStorageIssueSummary,
  getResourceStorageTopologyLabel,
  hasUnraidStorageAttentionIssue,
} from '@/features/storageBackups/resourceStoragePresentation';

// Mirrors the sibling test file's factory. Pure-function module, so no Solid
// root is required.
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

// Module-private functions exercised here only through their public entry
// points (no export hacking):
//   - getResourceStorageRiskIssue -> getResourceStorageIssueSummary / Label
//   - collectRiskReasons          -> getUnraidShortIssueLabel (via Label) /
//                                    findRiskReason (via hasUnraidStorageAttentionIssue)
//   - findRiskReason              -> hasUnraidStorageAttentionIssue
//   - getCompositePostureIssue    -> getResourceStorageIssueSummary / Label

describe('getResourceStorageRiskIssue (via getResourceStorageIssueSummary)', () => {
  it('falls back to the first non-empty storage risk-reason summary when riskSummary is absent', () => {
    const resource = makeResource({
      storage: {
        risk: {
          level: 'warning',
          reasons: [
            { code: 'zfs_state', severity: 'warning', summary: '' },
            { code: 'zfs_state', severity: 'warning', summary: '   ' },
            { code: 'zfs_state', severity: 'warning', summary: '  ZFS pool degraded  ' },
          ],
        },
      },
    });

    // Also proves firstRiskReasonSummary skips blank/whitespace-only summaries.
    expect(getResourceStorageIssueSummary(resource)).toBe('ZFS pool degraded');
  });

  it('falls through to PBS storage-risk reasons when storage risk has no usable summary', () => {
    const resource = makeResource({
      storage: {
        risk: {
          level: 'warning',
          reasons: [{ code: 'zfs_state', severity: 'warning', summary: '   ' }],
        },
      },
      pbs: {
        storageRisk: {
          level: 'warning',
          reasons: [
            { code: 'pbs_datastore_state', severity: 'warning', summary: 'PBS datastore degraded' },
          ],
        },
      },
    });

    expect(getResourceStorageIssueSummary(resource)).toBe('PBS datastore degraded');
  });

  it('returns an empty summary when every risk source is absent', () => {
    expect(getResourceStorageIssueSummary(makeResource())).toBe('');
  });
});

describe('collectRiskReasons (via getResourceStorageIssueLabel on Unraid resources)', () => {
  it('orders storage reasons ahead of PBS reasons so the storage copy wins', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            { code: 'unraid_disabled_disks', severity: 'warning', summary: 'storage-disk' },
          ],
        },
      },
      pbs: {
        storageRisk: {
          level: 'warning',
          reasons: [{ code: 'unraid_disabled_disks', severity: 'warning', summary: 'pbs-disk' }],
        },
      },
    });

    expect(getResourceStorageIssueLabel(resource)).toBe('storage-disk');
  });

  it('skips falsy entries inside the reasons arrays', () => {
    const resource = {
      ...makeResource(),
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            null,
            { code: 'unraid_missing_disks', severity: 'warning', summary: 'missing' },
          ] as unknown as ResourceStorageRiskReason[],
        },
      },
    } as unknown as Resource;

    // A null entry that was not skipped would throw on `reason.code` access.
    expect(getResourceStorageIssueLabel(resource)).toBe('missing');
  });
});

describe('hasUnraidStorageAttentionIssue', () => {
  it('returns false for resources that are not Unraid storage resources', () => {
    expect(hasUnraidStorageAttentionIssue(makeResource())).toBe(false);
  });

  it('returns true when an attention reason code is present', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [{ code: 'unraid_disabled_disks', severity: 'warning', summary: 'disabled' }],
        },
      },
    });

    expect(hasUnraidStorageAttentionIssue(resource)).toBe(true);
  });

  it('returns false when the Unraid resource only carries non-attention reasons', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [{ code: 'unraid_sync_active', severity: 'warning', summary: 'syncing' }],
        },
      },
    });

    expect(hasUnraidStorageAttentionIssue(resource)).toBe(false);
  });
});

describe('findRiskReason (exercised via hasUnraidStorageAttentionIssue)', () => {
  it('matches reason codes case-insensitively', () => {
    const resource = makeResource({
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [{ code: 'UNRAID_PARITY_UNAVAILABLE', severity: 'warning', summary: 'parity' }],
        },
      },
    });

    expect(hasUnraidStorageAttentionIssue(resource)).toBe(true);
  });

  it('does not match reasons with empty or missing codes', () => {
    const resource = {
      ...makeResource(),
      storage: {
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            { code: '', summary: 'x' },
            { summary: 'y' },
          ] as unknown as ResourceStorageRiskReason[],
        },
      },
    } as unknown as Resource;

    expect(hasUnraidStorageAttentionIssue(resource)).toBe(false);
  });

  it('finds a matching reason collected from the PBS storage risk', () => {
    const resource = makeResource({
      storage: { arrayState: 'STARTED' },
      pbs: {
        storageRisk: {
          level: 'warning',
          reasons: [{ code: 'unraid_missing_disks', severity: 'warning', summary: 'missing' }],
        },
      },
    });

    expect(hasUnraidStorageAttentionIssue(resource)).toBe(true);
  });
});

describe('getCompositePostureIssue (via getResourceStorageIssueSummary)', () => {
  it('surfaces the storage posture summary when status is an attention state', () => {
    const resource = makeResource({
      status: 'degraded',
      storage: { postureSummary: 'Redundancy degraded' },
    });

    expect(getResourceStorageIssueSummary(resource)).toBe('Redundancy degraded');
  });

  it('keeps a distinct posture even when non-matching impact summaries are present', () => {
    const resource = makeResource({
      status: 'degraded',
      storage: {
        postureSummary: 'Redundancy degraded',
        consumerImpactSummary: 'Affects 3 dependent resources',
      },
    });

    expect(getResourceStorageIssueSummary(resource)).toBe('Redundancy degraded');
  });

  it('prefers the PBS posture summary when storage posture is absent', () => {
    const resource = makeResource({
      status: 'warning',
      pbs: { postureSummary: 'Backup posture at risk' },
    });

    expect(getResourceStorageIssueSummary(resource)).toBe('Backup posture at risk');
  });

  it('suppresses posture that merely echoes the storage consumer impact summary', () => {
    const resource = makeResource({
      status: 'degraded',
      storage: {
        postureSummary: 'Affects 2 vms',
        consumerImpactSummary: 'Affects 2 vms',
      },
    });

    expect(getResourceStorageIssueSummary(resource)).toBe('');
  });

  it('suppresses posture that echoes the PBS protected workload summary', () => {
    const resource = makeResource({
      status: 'offline',
      pbs: {
        postureSummary: 'Puts 4 workloads at risk',
        protectedWorkloadSummary: 'Puts 4 workloads at risk',
      },
    });

    expect(getResourceStorageIssueSummary(resource)).toBe('');
  });

  it('suppresses posture that echoes the PBS affected datastore summary', () => {
    const resource = makeResource({
      status: 'degraded',
      pbs: {
        postureSummary: 'Datastore ds-1 unavailable',
        affectedDatastoreSummary: 'Datastore ds-1 unavailable',
      },
    });

    expect(getResourceStorageIssueSummary(resource)).toBe('');
  });

  it('returns no posture issue when the status is not an attention state', () => {
    const resource = makeResource({
      status: 'running',
      storage: { postureSummary: 'Something happened' },
    });

    expect(getResourceStorageIssueSummary(resource)).toBe('');
  });

  it('returns no posture issue when no posture summary is present', () => {
    const resource = makeResource({ status: 'degraded' });

    expect(getResourceStorageIssueSummary(resource)).toBe('');
  });
});

describe('getCanonicalStoragePlatformKey', () => {
  it('prefers a known storagePlatform argument over the resource platformType', () => {
    const resource = makeResource({ platformType: 'truenas' });

    expect(getCanonicalStoragePlatformKey(resource, 'vmware-vsphere')).toBe('vmware-vsphere');
  });

  it('normalizes aliased storagePlatform values', () => {
    const resource = makeResource({ platformType: 'generic' });

    expect(getCanonicalStoragePlatformKey(resource, 'pbs')).toBe('proxmox-pbs');
  });

  it('normalizes the storagePlatform case-insensitively', () => {
    const resource = makeResource({ platformType: 'generic' });

    expect(getCanonicalStoragePlatformKey(resource, 'TRUENAS')).toBe('truenas');
  });

  it('falls back to a known resource platformType when storagePlatform is unknown', () => {
    const resource = makeResource({ platformType: 'proxmox-pve' });

    expect(getCanonicalStoragePlatformKey(resource, 'mystery-box')).toBe('proxmox-pve');
  });

  it('returns the raw resource platformType when neither value normalizes', () => {
    const resource = { ...makeResource(), platformType: 'custom-platform' } as unknown as Resource;

    expect(getCanonicalStoragePlatformKey(resource)).toBe('custom-platform');
  });

  it('lowercases an unknown storagePlatform when platformType is empty', () => {
    const resource = { ...makeResource(), platformType: '' } as unknown as Resource;

    expect(getCanonicalStoragePlatformKey(resource, '  Custom Box ')).toBe('custom box');
  });

  it('falls back to generic when both platformType and storagePlatform are empty', () => {
    const resource = { ...makeResource(), platformType: '' } as unknown as Resource;

    expect(getCanonicalStoragePlatformKey(resource)).toBe('generic');
  });
});

describe('getResourceStorageTopologyLabel', () => {
  it('titleizes an explicit topology argument and bypasses storageType classification', () => {
    expect(getResourceStorageTopologyLabel(makeResource(), 'rbd', 'zfs-pool')).toBe('Zfs Pool');
  });

  it('titleizes multi-word delimited topology values', () => {
    expect(getResourceStorageTopologyLabel(makeResource(), 'anything', 'backup-target')).toBe(
      'Backup Target',
    );
  });

  it('classifies a backup-target topology from storage metadata as Backup Target', () => {
    const resource = makeResource({ storage: { topology: 'backup-target' } });

    expect(getResourceStorageTopologyLabel(resource, 'dir')).toBe('Backup Target');
  });

  it('classifies PBS-platform storage as Backup Target from platformData when storage meta is absent', () => {
    const resource = makeResource({ storage: {}, platformData: { platform: 'proxmox-pbs' } });

    expect(getResourceStorageTopologyLabel(resource, 'dir')).toBe('Backup Target');
  });

  it('classifies a backup-target topology from platformData when storage meta is absent', () => {
    const resource = makeResource({ storage: {}, platformData: { topology: 'backup-target' } });

    expect(getResourceStorageTopologyLabel(resource, 'dir')).toBe('Backup Target');
  });

  it('labels a datastore resource type as Datastore', () => {
    const resource = makeResource({ type: 'datastore' });

    expect(getResourceStorageTopologyLabel(resource, 'vmfs')).toBe('Datastore');
  });

  it('labels a datastore VMware entity type as Datastore even when the resource type differs', () => {
    const resource = makeResource({
      type: 'storage',
      vmware: { entityType: 'datastore' },
    });

    expect(getResourceStorageTopologyLabel(resource, 'vmfs')).toBe('Datastore');
  });

  it('labels pool storage types', () => {
    const resource = makeResource();
    expect(getResourceStorageTopologyLabel(resource, 'zfspool')).toBe('Pool');
    expect(getResourceStorageTopologyLabel(resource, 'zfs-pool')).toBe('Pool');
    expect(getResourceStorageTopologyLabel(resource, 'pool')).toBe('Pool');
  });

  it('labels dataset storage types', () => {
    const resource = makeResource();
    expect(getResourceStorageTopologyLabel(resource, 'zfs-dataset')).toBe('Dataset');
    expect(getResourceStorageTopologyLabel(resource, 'dataset')).toBe('Dataset');
  });

  it('labels filesystem storage types', () => {
    const resource = makeResource();
    expect(getResourceStorageTopologyLabel(resource, 'dir')).toBe('Filesystem');
    expect(getResourceStorageTopologyLabel(resource, 'filesystem')).toBe('Filesystem');
  });

  it('labels ceph cluster storage types', () => {
    const resource = makeResource();
    expect(getResourceStorageTopologyLabel(resource, 'rbd')).toBe('Cluster Storage');
    expect(getResourceStorageTopologyLabel(resource, 'cephfs')).toBe('Cluster Storage');
  });

  it('titleizes unknown storage types in the default branch', () => {
    expect(getResourceStorageTopologyLabel(makeResource(), 'nfs-share')).toBe('Nfs Share');
  });

  it('falls back to the titleized resource type when storageType is empty', () => {
    const resource = makeResource({ type: 'network-share' });

    expect(getResourceStorageTopologyLabel(resource, '')).toBe('Network Share');
  });

  it('falls back to Storage when both storageType and resource type are empty', () => {
    const resource = { ...makeResource(), type: '' } as unknown as Resource;

    expect(getResourceStorageTopologyLabel(resource, '')).toBe('Storage');
  });
});
