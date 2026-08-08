import { describe, expect, it } from 'vitest';
import type { RuntimeInventorySource } from '@/api/runtimeInventorySources';
import { buildWorkloadInventorySourceIssues } from '../workloadInventorySourceIssues';

const source = (overrides: Partial<RuntimeInventorySource> = {}): RuntimeInventorySource => ({
  type: 'pve',
  name: 'delly',
  state: 'stale',
  surfaces: ['vms', 'containers'],
  ...overrides,
});

describe('buildWorkloadInventorySourceIssues', () => {
  it('presents the viewer-safe credential issue without diagnostic detail', () => {
    const issues = buildWorkloadInventorySourceIssues([source({ state: 'unauthorized' })]);

    expect(issues).toEqual([
      {
        name: 'delly',
        type: 'pve',
        typeLabel: 'Proxmox VE',
        state: 'unauthorized',
        stateLabel: 'Credentials invalid',
        coverageLabel: 'VMs and containers',
        description:
          'Pulse has VMs and containers enabled for delly, but its Proxmox VE API credentials are invalid.',
      },
    ]);
    expect(issues[0]).not.toHaveProperty('detail');
    expect(issues[0]).not.toHaveProperty('id');
  });

  it('defensively ignores active, non-workload, and coverage-free rows', () => {
    const issues = buildWorkloadInventorySourceIssues([
      source({ name: 'healthy', state: 'active' as never }),
      source({ name: 'unsupported', type: 'pbs' as never }),
      source({ name: 'empty', surfaces: [] }),
    ]);

    expect(issues).toEqual([]);
  });
});
