import { describe, expect, it } from 'vitest';
import type { RuntimeInventorySource } from '@/api/runtimeInventorySources';
import { buildWorkloadInventorySourceIssues } from '../workloadInventorySourceIssues';

const source = (overrides: Partial<RuntimeInventorySource>): RuntimeInventorySource => ({
  id: 'pve:delly',
  type: 'pve',
  name: 'delly',
  state: 'active',
  surfaces: ['vms', 'containers', 'storage', 'backups'],
  credentialsInvalid: false,
  ...overrides,
});

describe('buildWorkloadInventorySourceIssues', () => {
  it('reports workload-capable sources with invalid credentials', () => {
    const issues = buildWorkloadInventorySourceIssues([
      source({ state: 'unauthorized', credentialsInvalid: true }),
    ]);

    expect(issues).toEqual([
      expect.objectContaining({
        id: 'pve:delly',
        name: 'delly',
        stateLabel: 'Credentials invalid',
        coverageLabel: 'VMs and containers',
        description:
          'Pulse has VMs and containers enabled for delly, but its Proxmox VE API credentials are invalid.',
      }),
    ]);
  });

  it('flags credential failure from the projection flag alone', () => {
    const issues = buildWorkloadInventorySourceIssues([
      source({ state: 'stale', credentialsInvalid: true }),
    ]);

    expect(issues[0]?.stateLabel).toBe('Credentials invalid');
  });

  it('ignores active and non-workload sources', () => {
    const issues = buildWorkloadInventorySourceIssues([
      source({ id: 'pve:pi', name: 'pi', state: 'active' }),
      source({
        id: 'pbs:tower',
        type: 'pbs',
        name: 'pbs-docker',
        state: 'unreachable',
        surfaces: ['backups'],
      }),
    ]);

    expect(issues).toEqual([]);
  });

  // The monitoring:read projection carries no raw error text, so the banner
  // must not reintroduce a detail line. Full diagnostics live on
  // Settings > Infrastructure for the admin who can act on them.
  it('never emits a raw error detail line', () => {
    const issues = buildWorkloadInventorySourceIssues([
      source({ state: 'unreachable', credentialsInvalid: false }),
    ]);

    expect(issues).toHaveLength(1);
    expect(issues[0]).not.toHaveProperty('detail');
  });
});
