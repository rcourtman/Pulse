import { describe, expect, it } from 'vitest';
import type { Connection, ConnectionFleetGovernance } from '@/api/connections';
import { buildWorkloadInventorySourceIssues } from '../workloadInventorySourceIssues';

const fleet = (overrides: Partial<ConnectionFleetGovernance> = {}): ConnectionFleetGovernance => ({
  enrollmentState: 'configured',
  livenessState: 'active',
  versionDrift: 'not-applicable',
  adapterHealth: 'healthy',
  configRollout: 'configured',
  credentialStatus: 'verified',
  updateStatus: 'not-applicable',
  remoteControl: 'not-applicable',
  ...overrides,
});

const connection = (overrides: Partial<Connection>): Connection =>
  ({
    id: 'pve:node',
    type: 'pve',
    name: 'node',
    address: 'https://node:8006',
    state: 'active',
    stateReason: '',
    enabled: true,
    surfaces: ['vms'],
    scope: { vms: true },
    lastSeen: null,
    lastError: null,
    source: 'agent',
    fleet: fleet(),
    capabilities: {
      supportsPause: true,
      supportsScope: true,
      supportsTest: true,
    },
    ...overrides,
  }) as Connection;

describe('workloadInventorySourceIssues (branch coverage)', () => {
  describe('credentialInvalid', () => {
    it('is true when state is unauthorized (first OR arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:unauth',
          name: 'unauth',
          state: 'unauthorized',
          fleet: fleet({ credentialStatus: 'verified' }),
        }),
      ]);

      expect(issues).toHaveLength(1);
      expect(issues[0]?.stateLabel).toBe('Credentials invalid');
    });

    it('is true when fleet.credentialStatus is invalid (second OR arm, active state)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:badstatus',
          name: 'badstatus',
          state: 'active',
          fleet: fleet({ credentialStatus: 'invalid' }),
        }),
      ]);

      expect(issues).toHaveLength(1);
      expect(issues[0]?.stateLabel).toBe('Credentials invalid');
    });

    it('is true when fleet.credentialHealth.status is invalid (third OR arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:healthbad',
          name: 'healthbad',
          state: 'active',
          fleet: fleet({ credentialHealth: { status: 'invalid' } }),
        }),
      ]);

      expect(issues).toHaveLength(1);
      expect(issues[0]?.stateLabel).toBe('Credentials invalid');
    });

    it('is true when fleet.credentialHealth.status is expired (fourth OR arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:expired',
          name: 'expired',
          state: 'active',
          fleet: fleet({ credentialHealth: { status: 'expired' } }),
        }),
      ]);

      expect(issues).toHaveLength(1);
      expect(issues[0]?.stateLabel).toBe('Credentials invalid');
    });

    it('short-circuits gracefully when fleet is undefined (optional-chain false arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:nofleet',
          name: 'nofleet',
          state: 'paused',
          fleet: undefined as unknown as Connection['fleet'],
        }),
      ]);

      expect(issues).toHaveLength(1);
      expect(issues[0]?.stateLabel).toBe('Collection paused');
    });
  });

  describe('stateLabelFor switch arms (credentialInvalid false)', () => {
    it('maps pending to "Collection pending"', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({ id: 'pve:pend', name: 'pend', state: 'pending' }),
      ]);

      expect(issues[0]?.stateLabel).toBe('Collection pending');
    });

    it('maps stale to "Collection stale"', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({ id: 'pve:stale', name: 'stale', state: 'stale' }),
      ]);

      expect(issues[0]?.stateLabel).toBe('Collection stale');
    });

    it('maps unreachable to "Source unreachable"', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'vmware:down',
          type: 'vmware',
          name: 'down',
          state: 'unreachable',
        }),
      ]);

      expect(issues[0]?.stateLabel).toBe('Source unreachable');
    });
  });

  describe('descriptionFor branches', () => {
    it('credentialInvalid arm names the type-label API credentials', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'docker:bad',
          type: 'docker',
          name: 'dockerhost',
          state: 'active',
          surfaces: ['containers'],
          scope: { containers: true },
          fleet: fleet({ credentialStatus: 'invalid' }),
        }),
      ]);

      expect(issues[0]?.description).toBe(
        'Pulse has containers enabled for dockerhost, but its Docker API credentials are invalid.',
      );
    });

    it('paused arm says collection is paused', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({ id: 'pve:paused', name: 'paused', state: 'paused' }),
      ]);

      expect(issues[0]?.description).toBe(
        'Pulse has VMs enabled for paused, but collection is paused.',
      );
    });

    it('pending arm says collection has not completed yet', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({ id: 'pve:pend', name: 'pend', state: 'pending' }),
      ]);

      expect(issues[0]?.description).toBe(
        'Pulse has VMs enabled for pend, but collection has not completed yet.',
      );
    });

    it('stale arm says inventory data is stale', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({ id: 'pve:stale', name: 'stale', state: 'stale' }),
      ]);

      expect(issues[0]?.description).toBe(
        'Pulse has VMs enabled for stale, but the last inventory data is stale.',
      );
    });

    it('unreachable arm interpolates the type label before "API is unreachable"', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'vmware:down',
          type: 'vmware',
          name: 'vc1',
          state: 'unreachable',
        }),
      ]);

      expect(issues[0]?.description).toBe(
        'Pulse has VMs enabled for vc1, but the VMware vCenter API is unreachable.',
      );
    });
  });

  describe('formatCoverage', () => {
    it('returns the single label unchanged for one surface (length === 1 arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({ id: 'pve:one', name: 'one', state: 'paused' }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs');
    });

    it('joins two labels with "and" (length === 2 arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:two',
          name: 'two',
          state: 'paused',
          surfaces: ['vms', 'containers'],
          scope: { vms: true, containers: true },
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs and containers');
    });

    it('joins three-plus labels with Oxford comma (length >= 3 arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:three',
          type: 'kubernetes',
          name: 'three',
          state: 'paused',
          surfaces: ['vms', 'containers', 'kubernetes'],
          scope: { vms: true, containers: true, kubernetes: true },
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe(
        'VMs, containers, and Kubernetes workloads',
      );
    });
  });

  describe('activeWorkloadSurfaces', () => {
    it('keeps only truthy scope entries and drops surfaces with no label', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:scoped',
          name: 'scoped',
          state: 'paused',
          surfaces: ['vms', 'containers', 'storage'],
          scope: { vms: true, containers: false, storage: true },
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs');
    });

    it('falls back to connection.surfaces when scope is empty', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:fallback',
          name: 'fallback',
          state: 'paused',
          surfaces: ['vms', 'pods'],
          scope: {},
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs and pods');
    });

    it('falls back to surfaces when scope is undefined (?? {} arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:noscope',
          name: 'noscope',
          state: 'paused',
          surfaces: ['vms'],
          scope: undefined as unknown as Connection['scope'],
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs');
    });

    it('deduplicates surfaces that map to the same label (seen.has arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'docker:dup',
          type: 'docker',
          name: 'dup',
          state: 'paused',
          surfaces: ['containers', 'docker'],
          scope: {},
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('containers');
    });

    it('sorts an unknown surface via the -1 rank normalization branch', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:unknown',
          name: 'unknown',
          state: 'paused',
          surfaces: ['zzz', 'vms'],
          scope: {},
        }),
      ]);

      expect(issues[0]?.coverageLabel).toBe('VMs');
    });
  });

  describe('compactDetail', () => {
    it('returns undefined when no error message is available (!formatted arm)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:nomsg',
          name: 'nomsg',
          state: 'paused',
          stateReason: '',
          lastError: null,
        }),
      ]);

      expect(issues[0]?.detail).toBeUndefined();
    });

    it('formats a short lastError.message (left ?? operand non-null)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:shorterr',
          name: 'shorterr',
          state: 'paused',
          stateReason: 'should-not-be-used',
          lastError: {
            at: '2026-07-12T00:00:00Z',
            message: 'no such host',
          },
        }),
      ]);

      expect(issues[0]?.detail).toBe('Host not found. Check the hostname or IP address.');
    });

    it('falls back to stateReason when lastError is null (right ?? operand)', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:reason',
          name: 'reason',
          state: 'paused',
          stateReason: 'connection refused',
          lastError: null,
        }),
      ]);

      expect(issues[0]?.detail).toBe(
        'Connection refused. The host is reachable but rejected the connection on this port. Check the port is correct and the service is running.',
      );
    });

    it('truncates a formatted message longer than 220 characters (>220 arm)', () => {
      const longMessage = 'x'.repeat(300);
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'pve:longerr',
          name: 'longerr',
          state: 'paused',
          lastError: {
            at: '2026-07-12T00:00:00Z',
            message: longMessage,
          },
        }),
      ]);

      const detail = issues[0]?.detail;
      expect(detail).toBeDefined();
      expect(detail?.length).toBe(220);
      expect(detail).toBe(`${'x'.repeat(217)}...`);
      expect(detail?.endsWith('...')).toBe(true);
    });
  });

  describe('buildWorkloadInventorySourceIssues pipeline', () => {
    it('excludes disabled, non-workload-type, and active-valid connections', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({ id: 'pve:disabled', name: 'disabled', enabled: false, state: 'unauthorized' }),
        connection({
          id: 'pbs:tower',
          type: 'pbs',
          name: 'tower',
          state: 'unreachable',
          surfaces: ['backups'],
          scope: { backups: true },
        }),
        connection({ id: 'pve:healthy', name: 'healthy', state: 'active' }),
        connection({ id: 'pve:blocked', name: 'blocked', state: 'stale' }),
      ]);

      expect(issues).toHaveLength(1);
      expect(issues[0]?.id).toBe('pve:blocked');
    });

    it('orders by descending STATE_RANK when states differ', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({ id: 'pve:aaa', name: 'aaa', state: 'paused' }),
        connection({ id: 'pve:zzz', name: 'zzz', state: 'unreachable' }),
      ]);

      expect(issues.map((issue) => issue.state)).toEqual(['unreachable', 'paused']);
      expect(issues.map((issue) => issue.name)).toEqual(['zzz', 'aaa']);
    });

    it('breaks state-rank ties with name localeCompare', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({ id: 'pve:zeta', name: 'zeta', state: 'paused' }),
        connection({ id: 'pve:alpha', name: 'alpha', state: 'paused' }),
      ]);

      expect(issues.map((issue) => issue.name)).toEqual(['alpha', 'zeta']);
    });

    it('emits a fully-shaped issue for a kubernetes source', () => {
      const issues = buildWorkloadInventorySourceIssues([
        connection({
          id: 'kubernetes:k1',
          type: 'kubernetes',
          name: 'k1',
          state: 'pending',
          surfaces: ['kubernetes'],
          scope: { kubernetes: true },
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
          detail: undefined,
        },
      ]);
    });
  });
});
