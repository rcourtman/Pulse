import { describe, expect, it } from 'vitest';
import type { RuntimeInventorySource } from '@/api/runtimeInventorySources';
import { buildWorkloadInventorySourceIssues } from '../workloadInventorySourceIssues';

const source = (overrides: Partial<RuntimeInventorySource>): RuntimeInventorySource => ({
  id: 'pve:node',
  type: 'pve',
  name: 'node',
  state: 'active',
  surfaces: ['vms'],
  credentialsInvalid: false,
  ...overrides,
});

describe('workloadInventorySourceIssues (branch coverage)', () => {
  describe('credentialInvalid', () => {
    it('is true when state is unauthorized (first OR arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'pve:unauth', name: 'unauth', state: 'unauthorized' }),
      ]);

      expect(issues).toHaveLength(1);
      expect(issues[0]?.stateLabel).toBe('Credentials invalid');
    });

    it('is true when the projection flags credentialsInvalid (second OR arm, active state)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({
          id: 'pve:badstatus',
          name: 'badstatus',
          state: 'active',
          credentialsInvalid: true,
        }),
      ]);

      expect(issues).toHaveLength(1);
      expect(issues[0]?.stateLabel).toBe('Credentials invalid');
    });

    it('is false when credentialsInvalid is absent from the payload', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({
          id: 'pve:noflag',
          name: 'noflag',
          state: 'paused',
          credentialsInvalid: undefined as unknown as boolean,
        }),
      ]);

      expect(issues).toHaveLength(1);
      expect(issues[0]?.stateLabel).toBe('Collection paused');
    });
  });

  describe('stateLabelFor switch arms (credentialInvalid false)', () => {
    it('maps pending to "Collection pending"', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'pve:pend', name: 'pend', state: 'pending' }),
      ]);

      expect(issues[0]?.stateLabel).toBe('Collection pending');
    });

    it('maps stale to "Collection stale"', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'pve:stale', name: 'stale', state: 'stale' }),
      ]);

      expect(issues[0]?.stateLabel).toBe('Collection stale');
    });

    it('maps unreachable to "Source unreachable"', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'vmware:down', type: 'vmware', name: 'down', state: 'unreachable' }),
      ]);

      expect(issues[0]?.stateLabel).toBe('Source unreachable');
    });
  });

  describe('descriptionFor branches', () => {
    it('credentialInvalid arm names the type-label API credentials', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({
          id: 'docker:bad',
          type: 'docker',
          name: 'dockerhost',
          state: 'active',
          surfaces: ['containers'],
          credentialsInvalid: true,
        }),
      ]);

      expect(issues[0]?.description).toBe(
        'Pulse has containers enabled for dockerhost, but its Docker API credentials are invalid.',
      );
    });

    it('paused arm says collection is paused', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'pve:paused', name: 'paused', state: 'paused' }),
      ]);

      expect(issues[0]?.description).toBe(
        'Pulse has VMs enabled for paused, but collection is paused.',
      );
    });

    it('pending arm says collection has not completed yet', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'pve:pend', name: 'pend', state: 'pending' }),
      ]);

      expect(issues[0]?.description).toBe(
        'Pulse has VMs enabled for pend, but collection has not completed yet.',
      );
    });

    it('stale arm says inventory data is stale', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'pve:stale', name: 'stale', state: 'stale' }),
      ]);

      expect(issues[0]?.description).toBe(
        'Pulse has VMs enabled for stale, but the last inventory data is stale.',
      );
    });

    it('unreachable arm interpolates the type label before "API is unreachable"', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'vmware:down', type: 'vmware', name: 'vc1', state: 'unreachable' }),
      ]);

      expect(issues[0]?.description).toBe(
        'Pulse has VMs enabled for vc1, but the VMware vCenter API is unreachable.',
      );
    });
  });

  describe('formatCoverage', () => {
    it('returns the single label unchanged for one surface (length === 1 arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'pve:one', name: 'one', state: 'paused' }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs');
    });

    it('joins two labels with "and" (length === 2 arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({
          id: 'pve:two',
          name: 'two',
          state: 'paused',
          surfaces: ['vms', 'containers'],
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs and containers');
    });

    it('joins three-plus labels with Oxford comma (length >= 3 arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({
          id: 'pve:three',
          type: 'kubernetes',
          name: 'three',
          state: 'paused',
          surfaces: ['vms', 'containers', 'kubernetes'],
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs, containers, and Kubernetes workloads');
    });
  });

  // Scope-over-surfaces resolution now happens server-side in
  // runtimeInventorySourceSurfaces; these cover what the client still decides.
  describe('activeWorkloadSurfaces', () => {
    it('drops surfaces with no label', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({
          id: 'pve:scoped',
          name: 'scoped',
          state: 'paused',
          surfaces: ['vms', 'storage'],
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs');
    });

    it('tolerates a missing surfaces array (?? [] arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({
          id: 'pve:nosurfaces',
          name: 'nosurfaces',
          state: 'paused',
          surfaces: undefined as unknown as string[],
        }),
      ]);

      expect(issues).toEqual([]);
    });

    it('deduplicates surfaces that map to the same label (seen.has arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({
          id: 'docker:dup',
          type: 'docker',
          name: 'dup',
          state: 'paused',
          surfaces: ['containers', 'docker'],
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('containers');
    });

    it('sorts an unknown surface via the -1 rank normalization branch', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'pve:unknown', name: 'unknown', state: 'paused', surfaces: ['zzz', 'vms'] }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs');
    });
  });

  describe('buildWorkloadInventorySourceIssues pipeline', () => {
    it('excludes non-workload-type and active-valid sources', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({
          id: 'pbs:tower',
          type: 'pbs',
          name: 'tower',
          state: 'unreachable',
          surfaces: ['backups'],
        }),
        source({ id: 'pve:healthy', name: 'healthy', state: 'active' }),
        source({ id: 'pve:blocked', name: 'blocked', state: 'stale' }),
      ]);

      expect(issues).toHaveLength(1);
      expect(issues[0]?.id).toBe('pve:blocked');
    });

    it('orders by descending STATE_RANK when states differ', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'pve:aaa', name: 'aaa', state: 'paused' }),
        source({ id: 'pve:zzz', name: 'zzz', state: 'unreachable' }),
      ]);

      expect(issues.map((issue) => issue.state)).toEqual(['unreachable', 'paused']);
      expect(issues.map((issue) => issue.name)).toEqual(['zzz', 'aaa']);
    });

    it('breaks state-rank ties with name localeCompare', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({ id: 'pve:zeta', name: 'zeta', state: 'paused' }),
        source({ id: 'pve:alpha', name: 'alpha', state: 'paused' }),
      ]);

      expect(issues.map((issue) => issue.name)).toEqual(['alpha', 'zeta']);
    });

    it('emits a fully-shaped issue for a kubernetes source, with no detail field', () => {
      const issues = buildWorkloadInventorySourceIssues([
        source({
          id: 'kubernetes:k1',
          type: 'kubernetes',
          name: 'k1',
          state: 'pending',
          surfaces: ['kubernetes'],
        }),
      ]);

      expect(issues).toStrictEqual([
        {
          id: 'kubernetes:k1',
          name: 'k1',
          type: 'kubernetes',
          typeLabel: 'Kubernetes',
          state: 'pending',
          stateLabel: 'Collection pending',
          coverageLabel: 'Kubernetes workloads',
          description:
            'Pulse has Kubernetes workloads enabled for k1, but collection has not completed yet.',
        },
      ]);
    });
  });
});
