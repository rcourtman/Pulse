import { describe, expect, it } from 'vitest';
import type { NodeConfigWithStatus } from '@/types/nodes';
import type { Resource } from '@/types/resource';
import type { TrueNASConnection } from '@/api/truenas';
import type { VMwareConnection } from '@/api/vmware';
import {
  agentConnectionRow,
  buildConnectionRows,
  pbsConnectionRow,
  pmgConnectionRow,
  pveConnectionRow,
  truenasConnectionRow,
  vmwareConnectionRow,
} from '../connectionsTableModel';

const pveNode = (overrides: Partial<NodeConfigWithStatus> = {}): NodeConfigWithStatus =>
  ({
    id: 'pve-1',
    name: 'pve-1',
    host: '10.0.0.1',
    user: 'root',
    type: 'pve',
    verifySSL: true,
    monitorVMs: true,
    monitorContainers: true,
    monitorStorage: true,
    monitorBackups: true,
    monitorPhysicalDisks: false,
    status: 'connected',
    ...overrides,
  }) as NodeConfigWithStatus;

const truenas = (overrides: Partial<TrueNASConnection> = {}): TrueNASConnection =>
  ({
    id: 'tn-1',
    name: 'Tower NAS',
    host: '10.0.0.20',
    useHttps: true,
    insecureSkipVerify: false,
    enabled: true,
    ...overrides,
  }) as TrueNASConnection;

const vmware = (overrides: Partial<VMwareConnection> = {}): VMwareConnection =>
  ({
    id: 'vm-1',
    name: 'vCenter',
    host: '10.0.0.30',
    insecureSkipVerify: false,
    enabled: true,
    ...overrides,
  }) as VMwareConnection;

const agentResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'agent-1',
    type: 'agent',
    name: 'tower',
    displayName: 'tower',
    platformId: 'agent-tower',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1700000000000,
    ...overrides,
  }) as Resource;

describe('connectionsTableModel', () => {
  it('maps PVE / PBS / PMG node status into unified reporting states', () => {
    expect(pveConnectionRow(pveNode()).status).toBe('reporting');
    expect(pveConnectionRow(pveNode({ status: 'pending' })).status).toBe('pending');
    expect(pveConnectionRow(pveNode({ status: 'disconnected' })).status).toBe('offline');
    expect(pveConnectionRow(pveNode({ status: 'error' })).status).toBe('error');
    expect(pbsConnectionRow(pveNode({ type: 'pbs', status: 'offline' })).kind).toBe('pbs');
    expect(pmgConnectionRow(pveNode({ type: 'pmg', status: 'connected' })).kind).toBe('pmg');
  });

  it('prefers the display name over the internal name', () => {
    const row = pveConnectionRow(pveNode({ displayName: 'Production cluster', name: 'pve-1' }));
    expect(row.name).toBe('Production cluster');
  });

  it('treats a disabled TrueNAS or VMware connection as offline regardless of poll state', () => {
    const tnRow = truenasConnectionRow(truenas({ enabled: false, poll: { lastSuccessAt: '2026-01-01T00:00:00Z' } }));
    const vmRow = vmwareConnectionRow(vmware({ enabled: false, poll: { lastSuccessAt: '2026-01-01T00:00:00Z' } }));
    expect(tnRow.status).toBe('offline');
    expect(vmRow.status).toBe('offline');
  });

  it('reports pending when a connection has no poll success and no failures', () => {
    expect(truenasConnectionRow(truenas()).status).toBe('pending');
    expect(vmwareConnectionRow(vmware()).status).toBe('pending');
  });

  it('reports reporting with a lastReportedMs once a poll has succeeded', () => {
    const row = truenasConnectionRow(
      truenas({ poll: { lastSuccessAt: '2026-01-01T00:00:00Z', consecutiveFailures: 0 } }),
    );
    expect(row.status).toBe('reporting');
    expect(row.lastReportedMs).toBe(Date.parse('2026-01-01T00:00:00Z'));
  });

  it('reports error when consecutive failures exceed the tolerance', () => {
    expect(
      truenasConnectionRow(
        truenas({ poll: { lastSuccessAt: '2026-01-01T00:00:00Z', consecutiveFailures: 5 } }),
      ).status,
    ).toBe('error');
    expect(
      vmwareConnectionRow(vmware({ poll: { consecutiveFailures: 1 } })).status,
    ).toBe('error');
  });

  it('maps agent resource status into unified reporting states and carries lastSeen', () => {
    expect(agentConnectionRow(agentResource()).status).toBe('reporting');
    expect(agentConnectionRow(agentResource()).lastReportedMs).toBe(1700000000000);
    expect(agentConnectionRow(agentResource({ status: 'offline' })).status).toBe('offline');
    expect(agentConnectionRow(agentResource({ status: 'degraded' })).status).toBe('error');
    expect(agentConnectionRow(agentResource({ status: 'unknown' })).status).toBe('unknown');
  });

  it('merges every source into a single alpha-sorted row set with stable composite ids', () => {
    const rows = buildConnectionRows({
      pveNodes: [pveNode({ id: 'a', name: 'zeus' })],
      pbsNodes: [pveNode({ id: 'b', type: 'pbs', name: 'backup' })],
      pmgNodes: [],
      truenasConnections: [truenas({ id: 'c', name: 'mira' })],
      vmwareConnections: [vmware({ id: 'd', name: 'apex' })],
      agentResources: [agentResource({ id: 'e', name: 'tower', displayName: 'tower' })],
    });

    expect(rows.map((r) => r.name)).toEqual(['apex', 'backup', 'mira', 'tower', 'zeus']);
    expect(new Set(rows.map((r) => r.id)).size).toBe(rows.length);
    expect(rows.find((r) => r.name === 'tower')?.method).toBe('agent');
    expect(rows.find((r) => r.name === 'apex')?.method).toBe('api');
  });
});
