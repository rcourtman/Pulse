import { describe, expect, it } from 'vitest';

import {
  getRecoveryLocationFacetAllLabel,
  getRecoveryLocationFacetLabel,
  getRecoveryPointLocationEntries,
} from '@/utils/recoveryLocationPresentation';

describe('recoveryLocationPresentation', () => {
  it('returns platform-neutral placement labels for recovery facets', () => {
    expect(getRecoveryLocationFacetLabel('cluster')).toBe('Cluster / Site');
    expect(getRecoveryLocationFacetLabel('node')).toBe('Host / Agent');
    expect(getRecoveryLocationFacetLabel('namespace')).toBe('Namespace / Group');
    expect(getRecoveryLocationFacetAllLabel('cluster')).toBe('Any cluster or site');
    expect(getRecoveryLocationFacetAllLabel('node')).toBe('Any host or agent');
    expect(getRecoveryLocationFacetAllLabel('namespace')).toBe('Any namespace or group');
  });

  it('builds recovery point placement entries from the canonical display contract', () => {
    expect(
      getRecoveryPointLocationEntries({
        id: 'p1',
        provider: 'proxmox-pve',
        kind: 'backup',
        mode: 'local',
        outcome: 'success',
        cluster: 'cluster-a',
        node: 'node-a',
        namespace: 'tenant-a',
        display: {
          clusterLabel: 'Lab Cluster',
          nodeHostLabel: 'pve-01',
          namespaceLabel: 'Tenant A',
        },
      }),
    ).toEqual([
      { key: 'cluster', label: 'Cluster / Site', value: 'Lab Cluster' },
      { key: 'node', label: 'Host / Agent', value: 'pve-01' },
      { key: 'namespace', label: 'Namespace / Group', value: 'Tenant A' },
    ]);
  });
});
