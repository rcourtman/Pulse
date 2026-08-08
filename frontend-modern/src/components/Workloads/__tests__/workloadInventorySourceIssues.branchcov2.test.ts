import { describe, expect, it } from 'vitest';
import type { RuntimeInventorySource } from '@/api/runtimeInventorySources';
import { buildWorkloadInventorySourceIssues } from '../workloadInventorySourceIssues';

const source = (overrides: Partial<RuntimeInventorySource> = {}): RuntimeInventorySource => ({
  type: 'pve',
  name: 'source',
  state: 'paused',
  surfaces: ['vms'],
  ...overrides,
});

describe('workloadInventorySourceIssues branch contract', () => {
  it.each([
    ['paused', 'Collection paused', 'collection is paused.'],
    ['pending', 'Collection pending', 'collection has not completed yet.'],
    ['stale', 'Collection stale', 'the last inventory data is stale.'],
    ['unauthorized', 'Credentials invalid', 'API credentials are invalid.'],
    ['unreachable', 'Source unreachable', 'API is unreachable.'],
  ] as const)('presents %s source state', (state, stateLabel, descriptionFragment) => {
    const [issue] = buildWorkloadInventorySourceIssues([source({ state })]);

    expect(issue?.stateLabel).toBe(stateLabel);
    expect(issue?.description).toContain(descriptionFragment);
  });

  it.each([
    ['pve', 'Proxmox VE'],
    ['vmware', 'VMware vCenter'],
    ['docker', 'Docker'],
    ['kubernetes', 'Kubernetes'],
  ] as const)('uses the canonical %s label', (type, typeLabel) => {
    const [issue] = buildWorkloadInventorySourceIssues([source({ type })]);

    expect(issue?.typeLabel).toBe(typeLabel);
  });

  it('formats, orders, and deduplicates workload coverage', () => {
    const [issue] = buildWorkloadInventorySourceIssues([
      source({
        type: 'kubernetes',
        surfaces: ['pods', 'unknown', 'docker', 'containers', 'vms', 'kubernetes'],
      }),
    ]);

    expect(issue?.coverageLabel).toBe('VMs, containers, pods, and Kubernetes workloads');
  });

  it('uses the one- and two-surface grammar', () => {
    const issues = buildWorkloadInventorySourceIssues([
      source({ name: 'one', surfaces: ['vms'] }),
      source({ name: 'two', surfaces: ['vms', 'containers'] }),
    ]);

    expect(issues.find((issue) => issue.name === 'one')?.coverageLabel).toBe('VMs');
    expect(issues.find((issue) => issue.name === 'two')?.coverageLabel).toBe('VMs and containers');
  });

  it('sorts the most severe state first and names as the tie-breaker', () => {
    const issues = buildWorkloadInventorySourceIssues([
      source({ name: 'Zulu', state: 'paused' }),
      source({ name: 'Alpha', state: 'unreachable' }),
      source({ name: 'Beta', state: 'unreachable' }),
      source({ name: 'Middle', state: 'stale' }),
    ]);

    expect(issues.map((issue) => issue.name)).toEqual(['Alpha', 'Beta', 'Middle', 'Zulu']);
  });

  it('tolerates a missing surfaces array without creating a generic issue', () => {
    const issues = buildWorkloadInventorySourceIssues([source({ surfaces: undefined as never })]);

    expect(issues).toEqual([]);
  });
});
