import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getCanonicalStoragePlatformKey,
  getResourceStorageActionSummary,
  getResourceStorageImpactSummary,
  getResourceStorageIssueLabel,
  getResourceStorageIssueSummary,
  getResourceStoragePlatformLabel,
  getResourceStorageProtectionLabel,
  getResourceStorageProtectionSummary,
  getResourceStorageTopologyLabel,
} from '@/features/storageBackups/resourceStoragePresentation';

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

describe('resourceStoragePresentation', () => {
  it('normalizes canonical storage platform keys and labels', () => {
    const resource = makeResource({ platformType: 'proxmox-pbs' });

    expect(getCanonicalStoragePlatformKey(resource)).toBe('proxmox-pbs');
    expect(getResourceStoragePlatformLabel('proxmox-pbs')).toBe('PBS');
  });

  it('derives canonical topology labels for storage resources', () => {
    expect(
      getResourceStorageTopologyLabel(
        makeResource({
          type: 'datastore',
          platformType: 'proxmox-pbs',
          storage: {
            platform: 'proxmox-pbs',
            type: 'pbs',
          },
        }),
        'pbs',
      ),
    ).toBe('Backup Target');
    expect(
      getResourceStorageTopologyLabel(
        makeResource({
          platformType: 'vmware-vsphere',
          storage: {
            platform: 'vmware-vsphere',
            type: 'vmfs',
          },
          vmware: {
            entityType: 'datastore',
          },
        }),
        'vmfs',
      ),
    ).toBe('Datastore');
    expect(getResourceStorageTopologyLabel(makeResource(), 'rbd')).toBe('Cluster Storage');
    expect(getResourceStorageTopologyLabel(makeResource(), 'ignored', 'rebuild target')).toBe(
      'Rebuild Target',
    );
  });

  it('derives canonical issue, impact, action, and protection summaries', () => {
    const resource = makeResource({
      incidentCategory: 'recoverability',
      incidentLabel: 'Backup Coverage At Risk',
      incidentSummary: 'No recent successful backups',
      incidentImpactSummary: 'Puts backups for 2 protected workloads at risk',
      incidentAction: 'Restore backup target health immediately',
      storage: {
        protectionReduced: true,
        protectionSummary: 'Protection Reduced',
      },
      pbs: {
        postureSummary: 'Backup posture degraded',
      },
    });

    expect(getResourceStorageIssueLabel(resource)).toBe('Backup Coverage At Risk');
    expect(getResourceStorageIssueSummary(resource)).toBe('No recent successful backups');
    expect(getResourceStorageImpactSummary(resource)).toBe(
      'Puts backups for 2 protected workloads at risk',
    );
    expect(getResourceStorageActionSummary(resource)).toBe(
      'Restore backup target health immediately',
    );
    expect(getResourceStorageProtectionLabel(resource)).toBe('Protection Reduced');
  });

  it('falls back to healthy and monitor defaults when posture is absent', () => {
    const resource = makeResource({
      type: 'pbs',
      storage: {
        platform: 'proxmox-pbs',
        protection: 'mirrored cache',
      },
    });

    expect(getResourceStorageIssueLabel(resource)).toBe('Healthy');
    expect(getResourceStorageIssueSummary(resource)).toBe('');
    expect(getResourceStorageImpactSummary(resource)).toBe('No dependent resources');
    expect(getResourceStorageActionSummary(resource)).toBe('Monitor');
    expect(getResourceStorageProtectionLabel(resource)).toBe('Mirrored Cache');
  });

  it('keeps dependent impact out of primary issue copy for healthy storage', () => {
    const impact = 'Affects 2 dependent resources: pulse, tailscale-pve3';
    const resource = makeResource({
      status: 'online',
      storage: {
        consumerCount: 2,
        consumerImpactSummary: impact,
        postureSummary: impact,
      },
    });

    expect(getResourceStorageIssueLabel(resource)).toBe('Healthy');
    expect(getResourceStorageIssueSummary(resource)).toBe('');
    expect(getResourceStorageImpactSummary(resource)).toBe(impact);
  });

  it('uses canonical risk copy as the primary issue when posture also carries impact', () => {
    const resource = makeResource({
      status: 'degraded',
      storage: {
        riskSummary: 'ZFS pool tank is DEGRADED',
        consumerImpactSummary: 'Affects 2 dependent resources: app01, media01',
        postureSummary: 'ZFS pool tank is DEGRADED. Affects 2 dependent resources: app01, media01',
      },
    });

    expect(getResourceStorageIssueLabel(resource)).toBe('ZFS pool tank is DEGRADED');
    expect(getResourceStorageIssueSummary(resource)).toBe('ZFS pool tank is DEGRADED');
    expect(getResourceStorageImpactSummary(resource)).toBe(
      'Affects 2 dependent resources: app01, media01',
    );
  });

  it('falls back to PBS storage-risk reasons without treating backup impact as an issue', () => {
    const resource = makeResource({
      type: 'pbs',
      status: 'degraded',
      pbs: {
        postureSummary: 'Puts backups for 3 protected workloads at risk: app01, db01, media01',
        protectedWorkloadSummary:
          'Puts backups for 3 protected workloads at risk: app01, db01, media01',
        storageRisk: {
          level: 'warning',
          reasons: [
            {
              code: 'pbs_datastore_state',
              severity: 'warning',
              summary: 'Backup datastore archive is degraded',
            },
          ],
        },
      },
    });

    expect(getResourceStorageIssueLabel(resource)).toBe('Backup datastore archive is degraded');
    expect(getResourceStorageIssueSummary(resource)).toBe('Backup datastore archive is degraded');
    expect(getResourceStorageImpactSummary(resource)).toBe(
      'Puts backups for 3 protected workloads at risk: app01, db01, media01',
    );
  });

  it('produces short Unraid protection labels and matching summaries when a parity check is running', () => {
    const resource = makeResource({
      type: 'storage',
      name: 'Tower Array',
      status: 'warning',
      storage: {
        protection: 'none',
        protectionReduced: true,
        protectionSummary: 'Unraid array is running without parity protection',
        rebuildInProgress: true,
        rebuildSummary: 'Unraid array is running check',
        riskSummary: 'Unraid array is running without parity protection',
        arrayState: 'STARTED',
        syncAction: 'check',
        syncProgress: 0,
        risk: {
          level: 'warning',
          reasons: [
            {
              code: 'unraid_no_parity',
              severity: 'warning',
              summary: 'Unraid array is running without parity protection',
            },
            {
              code: 'unraid_sync_active',
              severity: 'warning',
              summary: 'Unraid array is running check',
            },
          ],
        },
      },
    });

    expect(getResourceStorageProtectionLabel(resource)).toBe('Parity check');
    expect(getResourceStorageProtectionSummary(resource)).toBe('Unraid array is running check');
    expect(getResourceStorageIssueLabel(resource)).toBe('No parity protection');
    expect(getResourceStorageIssueSummary(resource)).toBe(
      'Unraid array is running without parity protection',
    );
  });

  it('marks an Unraid array without parity as Unprotected when no rebuild is running', () => {
    const resource = makeResource({
      type: 'storage',
      name: 'Tower Array',
      status: 'warning',
      storage: {
        protection: 'none',
        protectionReduced: true,
        protectionSummary: 'Unraid array is running without parity protection',
        rebuildInProgress: false,
        riskSummary: 'Unraid array is running without parity protection',
        arrayState: 'STARTED',
        risk: {
          level: 'warning',
          reasons: [
            {
              code: 'unraid_no_parity',
              severity: 'warning',
              summary: 'Unraid array is running without parity protection',
            },
          ],
        },
      },
    });

    expect(getResourceStorageProtectionLabel(resource)).toBe('Unprotected');
    expect(getResourceStorageIssueLabel(resource)).toBe('No parity protection');
  });

  it('appends sync progress to Unraid parity-rebuild labels when known', () => {
    const resource = makeResource({
      type: 'storage',
      name: 'Tower Array',
      status: 'warning',
      storage: {
        protectionReduced: false,
        rebuildInProgress: true,
        rebuildSummary: 'Unraid array is running recon (47%)',
        arrayState: 'STARTED',
        syncAction: 'recon',
        syncProgress: 47,
        risk: {
          level: 'warning',
          reasons: [
            {
              code: 'unraid_sync_active',
              severity: 'warning',
              summary: 'Unraid array is running recon (47%)',
            },
          ],
        },
      },
    });

    expect(getResourceStorageProtectionLabel(resource)).toBe('Parity rebuild (47%)');
  });

  it('keeps VMware datastore protection copy neutral on the shared storage path', () => {
    const resource = makeResource({
      platformType: 'vmware-vsphere',
      storage: {
        platform: 'vmware-vsphere',
        type: 'vmfs',
        topology: 'datastore',
      },
      vmware: {
        entityType: 'datastore',
      },
    });

    expect(getResourceStorageProtectionLabel(resource)).toBe('Healthy');
  });
});
