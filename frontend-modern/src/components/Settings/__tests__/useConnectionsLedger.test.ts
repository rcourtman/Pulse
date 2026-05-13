import { renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ConnectionsAPI, type Connection, type ConnectionSystem } from '@/api/connections';
import { useConnectionsLedger } from '../useConnectionsLedger';

describe('useConnectionsLedger', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders standalone agent rows with compact host identity and endpoint context', async () => {
    const connections: Connection[] = [
      {
        id: 'agent:tower',
        type: 'agent',
        name: 'Tower',
        address: 'Tower',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['host'],
        scope: { host: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        fleet: {
          enrollmentState: 'enrolled',
          livenessState: 'active',
          versionDrift: 'behind',
          adapterHealth: 'healthy',
          configRollout: 'reported',
          credentialStatus: 'verified',
          updateStatus: 'update-available',
          remoteControl: 'enabled',
        },
        agentIdentity: {
          hostname: 'tower',
          platform: 'linux',
          hostProfile: 'unraid',
          osName: 'Unraid',
          osVersion: '7.1.0',
          kernelVersion: '6.12.0',
          architecture: 'x86_64',
          reportIp: '192.168.0.10',
          commandsEnabled: true,
        },
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
    ];
    vi.spyOn(ConnectionsAPI, 'list').mockResolvedValue({ connections, systems: [] });

    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0]).toMatchObject({
      id: 'agent:tower',
      ownerType: 'agent',
      name: 'Tower',
      subtitle: 'via Pulse Agent',
      identitySubtitle: 'Unraid 7.1.0',
      source: 'agent',
      host: '192.168.0.10',
      isAgent: true,
      isCluster: false,
      coverageLabels: ['Host telemetry'],
    });
    expect(result.rows()[0].fleetHighlights.map((signal) => signal.label)).toEqual([
      'Version behind',
      'Update available',
      'Remote control enabled',
    ]);
  });

  it('prioritizes explicit rollout drift, credential, and command-policy posture', async () => {
    const connections: Connection[] = [
      {
        id: 'agent:drifted',
        type: 'agent',
        name: 'drifted',
        address: 'drifted',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['host'],
        scope: { host: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        fleet: {
          enrollmentState: 'enrolled',
          livenessState: 'active',
          versionDrift: 'behind',
          adapterHealth: 'healthy',
          configRollout: 'reported',
          credentialStatus: 'verified',
          updateStatus: 'update-available',
          remoteControl: 'disabled',
          configDrift: {
            status: 'drifted',
            desired: { version: 'host-agent-config/v1', hash: 'sha256:desired' },
            applied: { version: 'host-agent-config/v1', hash: 'sha256:applied' },
          },
          rollout: { status: 'pending', stage: 'canary' },
          credentialHealth: { status: 'verified', kind: 'agent-token' },
          commandPolicy: {
            status: 'disabled',
            desired: 'disabled',
            applied: 'disabled',
            enforcement: 'in-sync',
          },
        },
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
      {
        id: 'pve:invalid',
        type: 'pve',
        name: 'invalid',
        address: 'https://invalid:8006',
        state: 'unauthorized',
        stateReason: '403 forbidden',
        enabled: true,
        surfaces: ['vms'],
        scope: { vms: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: { message: '403 forbidden', at: '2026-04-23T12:00:00Z' },
        source: 'manual',
        fleet: {
          enrollmentState: 'configured',
          livenessState: 'unauthorized',
          versionDrift: 'not-applicable',
          adapterHealth: 'blocked',
          configRollout: 'configured',
          credentialStatus: 'invalid',
          updateStatus: 'not-applicable',
          remoteControl: 'not-applicable',
          configDrift: { status: 'current' },
          rollout: { status: 'blocked' },
          credentialHealth: { status: 'invalid', kind: 'token' },
          commandPolicy: { status: 'not-applicable' },
        },
        capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
      },
      {
        id: 'pve:paused',
        type: 'pve',
        name: 'paused',
        address: 'https://paused:8006',
        state: 'paused',
        stateReason: 'paused by user',
        enabled: false,
        surfaces: ['vms'],
        scope: { vms: true },
        lastSeen: null,
        lastError: null,
        source: 'manual',
        fleet: {
          enrollmentState: 'paused',
          livenessState: 'paused',
          versionDrift: 'not-applicable',
          adapterHealth: 'paused',
          configRollout: 'paused',
          credentialStatus: 'paused',
          updateStatus: 'not-applicable',
          remoteControl: 'not-applicable',
          configDrift: { status: 'paused' },
          rollout: { status: 'paused' },
          credentialHealth: { status: 'paused' },
          commandPolicy: { status: 'not-applicable' },
        },
        capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
      },
      {
        id: 'agent:remote-disabled',
        type: 'agent',
        name: 'remote-disabled',
        address: 'remote-disabled',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['host'],
        scope: { host: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        fleet: {
          enrollmentState: 'enrolled',
          livenessState: 'active',
          versionDrift: 'current',
          adapterHealth: 'healthy',
          configRollout: 'reported',
          credentialStatus: 'verified',
          updateStatus: 'current',
          remoteControl: 'disabled',
          configDrift: { status: 'current' },
          rollout: { status: 'current' },
          credentialHealth: { status: 'verified', kind: 'agent-token' },
          commandPolicy: {
            status: 'disabled',
            desired: 'disabled',
            applied: 'disabled',
            enforcement: 'in-sync',
          },
        },
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
    ];
    vi.spyOn(ConnectionsAPI, 'list').mockResolvedValue({ connections, systems: [] });

    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(result.rows()).toHaveLength(4));
    const byID = new Map(result.rows().map((row) => [row.id, row]));

    expect(byID.get('agent:drifted')?.fleetHighlights.map((signal) => signal.label)).toEqual([
      'Config drift',
      'Rollout pending',
      'Version behind',
    ]);
    expect(byID.get('pve:invalid')?.fleetHighlights.map((signal) => signal.label)).toEqual([
      'Unauthorized',
      'Credentials invalid',
      'Rollout blocked',
    ]);
    expect(byID.get('pve:paused')?.fleetHighlights.map((signal) => signal.label)).toEqual([
      'Rollout paused',
    ]);
    expect(
      byID.get('agent:remote-disabled')?.fleetHighlights.map((signal) => signal.label),
    ).toEqual(['Remote control disabled']);
  });

  it('renders a Proxmox cluster row from the canonical system metadata', async () => {
    const connections: Connection[] = [
      {
        id: 'pve:delly',
        type: 'pve',
        name: 'delly',
        address: 'https://delly:8006',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['vms', 'containers', 'storage', 'backups'],
        scope: { vms: true, containers: true, storage: true, backups: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
      },
      {
        id: 'agent:agent-delly',
        type: 'agent',
        name: 'delly',
        address: 'delly',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['host'],
        scope: { host: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
      {
        id: 'agent:agent-minipc',
        type: 'agent',
        name: 'minipc',
        address: 'minipc',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['host'],
        scope: { host: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
    ];
    const systems: ConnectionSystem[] = [
      {
        id: 'pve:delly',
        type: 'pve',
        clusterName: 'homelab',
        components: [
          { connectionId: 'pve:delly', type: 'pve', role: 'primary' },
          { connectionId: 'agent:agent-delly', type: 'agent', role: 'attachment' },
          { connectionId: 'agent:agent-minipc', type: 'agent', role: 'attachment' },
        ],
        members: [
          {
            id: 'node-delly',
            name: 'delly',
            endpoint: 'https://delly:8006',
            hostAliases: ['delly', '192.168.0.10'],
            state: 'active',
            lastSeen: '2026-04-23T12:00:00Z',
            primary: true,
            agentConnectionId: 'agent:agent-delly',
          },
          {
            id: 'node-minipc',
            name: 'minipc',
            endpoint: 'https://minipc:8006',
            hostAliases: ['minipc', '192.168.0.11'],
            state: 'active',
            lastSeen: '2026-04-23T12:00:00Z',
            agentConnectionId: 'agent:agent-minipc',
          },
        ],
      },
    ];
    vi.spyOn(ConnectionsAPI, 'list').mockResolvedValue({ connections, systems });

    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0]).toMatchObject({
      id: 'pve:delly',
      ownerType: 'pve',
      name: 'homelab',
      subtitle: 'Cluster · 2 nodes',
      source: 'both',
      host: undefined,
      isCluster: true,
      coverageLabels: ['VMs', 'Containers', 'Storage', 'Backups'],
    });
    expect(result.rows()[0].attachedConnections.map((connection) => connection.id)).toEqual([
      'agent:agent-delly',
      'agent:agent-minipc',
    ]);
    expect(result.rows()[0].members).toMatchObject([
      {
        id: 'node-delly',
        name: 'delly',
        subtitle: 'Primary node',
        source: 'agent',
        host: 'https://delly:8006',
        hostAliases: ['delly', '192.168.0.10'],
        coverageLabels: ['Host telemetry'],
        statusLabel: 'Active',
        primary: true,
      },
      {
        id: 'node-minipc',
        name: 'minipc',
        subtitle: 'Cluster member',
        source: 'agent',
        host: 'https://minipc:8006',
        hostAliases: ['minipc', '192.168.0.11'],
        coverageLabels: ['Host telemetry'],
        statusLabel: 'Active',
        primary: false,
      },
    ]);
  });

  it('rolls Proxmox cluster status down when a member is unreachable', async () => {
    const connections: Connection[] = [
      {
        id: 'pve:delly',
        type: 'pve',
        name: 'delly',
        address: 'https://delly:8006',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['vms', 'containers', 'storage', 'backups'],
        scope: { vms: true, containers: true, storage: true, backups: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
      },
      {
        id: 'agent:agent-minipc',
        type: 'agent',
        name: 'minipc',
        address: 'minipc',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['host'],
        scope: { host: true },
        lastSeen: '2026-04-23T12:00:00Z',
        lastError: null,
        source: 'agent',
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
    ];
    const systems: ConnectionSystem[] = [
      {
        id: 'pve:delly',
        type: 'pve',
        clusterName: 'homelab',
        components: [
          { connectionId: 'pve:delly', type: 'pve', role: 'primary' },
          { connectionId: 'agent:agent-minipc', type: 'agent', role: 'attachment' },
        ],
        members: [
          {
            id: 'node-delly',
            name: 'delly',
            endpoint: 'https://delly:8006',
            state: 'active',
            lastSeen: '2026-04-23T12:00:00Z',
            primary: true,
          },
          {
            id: 'node-minipc',
            name: 'minipc',
            endpoint: 'https://minipc:8006',
            state: 'unreachable',
            lastSeen: '2026-04-23T11:55:00Z',
            agentConnectionId: 'agent:agent-minipc',
          },
        ],
      },
    ];
    vi.spyOn(ConnectionsAPI, 'list').mockResolvedValue({ connections, systems });

    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0].statusLabel).toBe('Unreachable');
    expect(result.rows()[0].members.find((member) => member.id === 'node-minipc')).toMatchObject({
      statusLabel: 'Unreachable',
    });
  });
});
