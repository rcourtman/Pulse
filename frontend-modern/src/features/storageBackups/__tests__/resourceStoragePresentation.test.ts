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
    expect(getResourceStorageTopologyLabel(makeResource({ type: 'datastore' }), 'pbs')).toBe(
      'Backup Target',
    );
    expect(getResourceStorageTopologyLabel(makeResource(), 'rbd')).toBe('Cluster Storage');
    expect(
      getResourceStorageTopologyLabel(makeResource(), 'ignored', 'rebuild target'),
    ).toBe('Rebuild Target');
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
        protection: 'mirrored cache',
      },
    });

    expect(getResourceStorageIssueLabel(resource)).toBe('Healthy');
    expect(getResourceStorageIssueSummary(resource)).toBe('');
    expect(getResourceStorageImpactSummary(resource)).toBe('No dependent resources');
    expect(getResourceStorageActionSummary(resource)).toBe('Monitor');
    expect(getResourceStorageProtectionLabel(resource)).toBe('Mirrored Cache');
  });
});
