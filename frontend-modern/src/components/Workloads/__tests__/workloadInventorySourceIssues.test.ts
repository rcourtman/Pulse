import { describe, expect, it } from 'vitest';
import type { Connection } from '@/api/connections';
import { buildWorkloadInventorySourceIssues } from '../workloadInventorySourceIssues';

const connection = (overrides: Partial<Connection>): Connection =>
  ({
    id: 'pve:delly',
    type: 'pve',
    name: 'delly',
    address: 'https://delly:8006',
    state: 'active',
    enabled: true,
    surfaces: ['vms', 'containers', 'storage', 'backups'],
    scope: { vms: true, containers: true, storage: true, backups: true },
    lastSeen: null,
    lastError: null,
    source: 'agent',
    fleet: {
      enrollmentState: 'configured',
      livenessState: 'active',
      versionDrift: 'not-applicable',
      adapterHealth: 'healthy',
      configRollout: 'configured',
      credentialStatus: 'verified',
      updateStatus: 'not-applicable',
      remoteControl: 'not-applicable',
    },
    capabilities: {
      supportsPause: true,
      supportsScope: true,
      supportsTest: true,
    },
    ...overrides,
  }) as Connection;

describe('buildWorkloadInventorySourceIssues', () => {
  it('reports enabled workload-capable sources with invalid credentials', () => {
    const issues = buildWorkloadInventorySourceIssues([
      connection({
        state: 'unauthorized',
        fleet: {
          enrollmentState: 'configured',
          livenessState: 'unauthorized',
          versionDrift: 'not-applicable',
          adapterHealth: 'blocked',
          configRollout: 'configured',
          credentialStatus: 'invalid',
          updateStatus: 'not-applicable',
          remoteControl: 'not-applicable',
        },
        lastError: {
          at: '2026-05-13T23:58:54Z',
          message: 'Authentication failed - check API token or credentials',
        },
      }),
    ]);

    expect(issues).toEqual([
      expect.objectContaining({
        id: 'pve:delly',
        name: 'delly',
        stateLabel: 'Credentials invalid',
        coverageLabel: 'VMs and containers',
        description:
          'Pulse has VMs and containers enabled for delly, but its Proxmox VE API credentials are invalid.',
        detail: 'Authentication failed. Re-check the API token or username/password.',
      }),
    ]);
  });

  it('ignores active, disabled, and non-workload sources', () => {
    const issues = buildWorkloadInventorySourceIssues([
      connection({ id: 'pve:pi', name: 'pi', state: 'active' }),
      connection({ id: 'pve:paused', enabled: false, state: 'unauthorized' }),
      connection({
        id: 'pbs:tower',
        type: 'pbs',
        name: 'pbs-docker',
        state: 'unreachable',
        surfaces: ['backups'],
        scope: { backups: true },
      }),
    ]);

    expect(issues).toEqual([]);
  });
});
