import { renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ConnectionsAPI, type Connection, type ConnectionSystem } from '@/api/connections';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { useConnectionsLedger } from '../useConnectionsLedger';
import useConnectionsLedgerSource from '../useConnectionsLedger.ts?raw';

describe('useConnectionsLedger', () => {
  beforeEach(() => {
    resetCreateNonSuspendingQueryCacheForTest();
  });

  afterEach(() => {
    resetCreateNonSuspendingQueryCacheForTest();
    vi.restoreAllMocks();
  });

  it('keeps connection-ledger refreshes out of app-level Suspense', () => {
    expect(useConnectionsLedgerSource).toContain('createNonSuspendingQuery');
    expect(useConnectionsLedgerSource).toContain('pollMs: POLL_INTERVAL_MS');
    expect(useConnectionsLedgerSource).not.toContain('createResource');
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
    ]);
  });

  it('presents successful TrueNAS runtime state as verified activity', async () => {
    const lastSeen = '2026-05-28T12:00:00Z';
    const connections: Connection[] = [
      {
        id: 'truenas:tn1',
        type: 'truenas',
        name: 'tower',
        address: 'https://truenas.local:443',
        state: 'active',
        stateReason: '',
        enabled: true,
        surfaces: ['datasets', 'pools', 'replication'],
        scope: { datasets: true, pools: true, replication: true },
        lastSeen,
        lastError: null,
        source: 'manual',
        fleet: {
          enrollmentState: 'configured',
          livenessState: 'active',
          versionDrift: 'not-applicable',
          adapterHealth: 'healthy',
          configRollout: 'configured',
          credentialStatus: 'verified',
          updateStatus: 'not-applicable',
          remoteControl: 'not-applicable',
          configDrift: { status: 'current' },
          rollout: { status: 'current' },
          credentialHealth: { status: 'verified', kind: 'api-key', lastVerifiedAt: lastSeen },
          commandPolicy: { status: 'not-applicable' },
        },
        capabilities: { supportsPause: true, supportsScope: true, supportsTest: true },
      },
    ];
    const systems: ConnectionSystem[] = [
      {
        id: 'truenas:tn1',
        type: 'truenas',
        components: [{ connectionId: 'truenas:tn1', type: 'truenas', role: 'primary' }],
      },
    ];
    vi.spyOn(ConnectionsAPI, 'list').mockResolvedValue({ connections, systems });

    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    expect(result.rows()[0]).toMatchObject({
      id: 'truenas:tn1',
      ownerType: 'truenas',
      name: 'tower',
      source: 'api',
      statusLabel: 'Active',
      coverageLabels: ['Datasets', 'Pools', 'Replication'],
    });
    expect(result.rows()[0].lastActivityText).not.toBe('No activity yet');
    expect(result.rows()[0].lastActivityText).not.toBe('Unknown');
    expect(result.rows()[0].problem).toBeUndefined();
    expect(result.rows()[0].fleetHighlights.map((signal) => signal.label)).not.toContain(
      'Credentials unknown',
    );
  });

  it('reuses large stable row models across equivalent ledger refreshes', async () => {
    const connections: Connection[] = Array.from({ length: 120 }, (_, index) => ({
      id: `agent:stable-${index}`,
      type: 'agent',
      name: `stable-${index}`,
      address: `stable-${index}`,
      state: 'active',
      stateReason: '',
      enabled: true,
      surfaces: ['host'],
      scope: { host: true },
      lastSeen: null,
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
        commandPolicy: { status: 'disabled' },
      },
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    }));
    const listSpy = vi.spyOn(ConnectionsAPI, 'list').mockImplementation(async () => ({
      connections: connections.map((connection) => ({
        ...connection,
        scope: { ...(connection.scope ?? {}) },
        fleet: connection.fleet ? { ...connection.fleet } : undefined,
        capabilities: { ...connection.capabilities },
      })),
      systems: [],
    }));

    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(result.rows()).toHaveLength(120));
    const firstRows = result.rows();

    result.reload();

    await waitFor(() => expect(listSpy).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.loading()).toBe(false));
    expect(result.rows()).toHaveLength(120);
    expect(result.rows()[0]).toBe(firstRows[0]);
    expect(result.rows()[75]).toBe(firstRows[75]);
    expect(result.rows()[119]).toBe(firstRows[119]);
  });

  it('retains the fulfilled ledger while a reload is in flight', async () => {
    const firstConnection: Connection = {
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
      capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
    };
    const nextConnection: Connection = {
      ...firstConnection,
      id: 'agent:pi',
      name: 'pi',
      address: 'pi',
    };
    let resolveReload:
      ((value: { connections: Connection[]; systems: ConnectionSystem[] }) => void) | undefined;
    vi.spyOn(ConnectionsAPI, 'list')
      .mockResolvedValueOnce({ connections: [firstConnection], systems: [] })
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveReload = resolve;
          }),
      );

    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(result.rows()).toHaveLength(1));
    const firstRows = result.rows();
    expect(firstRows[0]?.name).toBe('Tower');

    result.reload();

    await waitFor(() => expect(result.loading()).toBe(true));
    expect(result.rows()).toBe(firstRows);
    expect(result.rows()[0]?.name).toBe('Tower');

    resolveReload?.({ connections: [nextConnection], systems: [] });

    await waitFor(() => expect(result.loading()).toBe(false));
    await waitFor(() => expect(result.rows()[0]?.name).toBe('pi'));
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
      {
        id: 'agent:command-mismatch',
        type: 'agent',
        name: 'command-mismatch',
        address: 'command-mismatch',
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
          remoteControl: 'enabled',
          configDrift: { status: 'current' },
          rollout: { status: 'current' },
          credentialHealth: { status: 'verified', kind: 'agent-token' },
          commandPolicy: {
            status: 'enabled',
            desired: 'disabled',
            applied: 'enabled',
            enforcement: 'drifted',
            reason:
              'agent still reports command execution enabled while desired policy disables it',
          },
        },
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
      {
        id: 'agent:config-pending',
        type: 'agent',
        name: 'config-pending',
        address: 'config-pending',
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
          remoteControl: 'enabled',
          configDrift: { status: 'pending' },
          rollout: { status: 'current' },
          credentialHealth: { status: 'verified', kind: 'agent-token' },
          commandPolicy: { status: 'enabled' },
        },
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
      {
        id: 'agent:config-unknown',
        type: 'agent',
        name: 'config-unknown',
        address: 'config-unknown',
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
          remoteControl: 'enabled',
          configDrift: { status: 'unknown' },
          rollout: { status: 'current' },
          credentialHealth: { status: 'verified', kind: 'agent-token' },
          commandPolicy: { status: 'enabled' },
        },
        capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
      },
    ];
    vi.spyOn(ConnectionsAPI, 'list').mockResolvedValue({ connections, systems: [] });

    const { result } = renderHook(() => useConnectionsLedger());

    await waitFor(() => expect(result.rows()).toHaveLength(7));
    const byID = new Map(result.rows().map((row) => [row.id, row]));

    expect(byID.get('agent:drifted')?.fleetHighlights.map((signal) => signal.label)).toEqual([
      'Config drift',
      'Rollout pending',
      'Version behind',
    ]);
    // Pull-based API sources (PVE/PBS/etc.) have no Pulse Agent, so agent-fleet
    // governance (rollout/config/version/command-policy) must not surface on
    // them. Only source-agnostic posture like credential health applies; an
    // invalid PVE token still reads "Credentials invalid", and a paused PVE
    // connection surfaces no fleet highlight at all.
    expect(byID.get('pve:invalid')?.fleetHighlights.map((signal) => signal.label)).toEqual([
      'Credentials invalid',
    ]);
    expect(byID.get('pve:paused')?.fleetHighlights.map((signal) => signal.label)).toEqual([]);
    expect(
      byID.get('agent:remote-disabled')?.fleetHighlights.map((signal) => signal.label),
    ).toEqual([]);
    expect(
      byID.get('agent:command-mismatch')?.fleetHighlights.map((signal) => signal.label),
    ).toEqual(['Command policy mismatch']);
    expect(byID.get('agent:command-mismatch')?.fleetHighlights[0]?.tone).toBe('critical');
    expect(byID.get('agent:config-pending')?.fleetHighlights.map((signal) => signal.label)).toEqual(
      ['Config pending'],
    );
    expect(byID.get('agent:config-unknown')?.fleetHighlights.map((signal) => signal.label)).toEqual(
      ['Config unknown'],
    );
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
        fleet: {
          enrollmentState: 'configured',
          livenessState: 'active',
          versionDrift: 'not-applicable',
          adapterHealth: 'healthy',
          configRollout: 'configured',
          credentialStatus: 'verified',
          updateStatus: 'not-applicable',
          remoteControl: 'not-applicable',
          configDrift: { status: 'current' },
          rollout: { status: 'current' },
          credentialHealth: { status: 'verified', kind: 'token' },
          commandPolicy: { status: 'not-applicable' },
        },
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
        fleet: {
          enrollmentState: 'enrolled',
          livenessState: 'active',
          versionDrift: 'current',
          adapterHealth: 'healthy',
          configRollout: 'reported',
          credentialStatus: 'verified',
          updateStatus: 'current',
          remoteControl: 'disabled',
          configDrift: {
            status: 'pending',
            reason:
              'Pulse has not received a comparable applied agent configuration fingerprint yet',
          },
          rollout: {
            status: 'pending',
            reason: 'waiting for the agent to report an applied configuration fingerprint',
          },
          credentialHealth: { status: 'verified', kind: 'agent-token' },
          commandPolicy: {
            status: 'disabled',
            desired: 'unknown',
            applied: 'disabled',
            enforcement: 'not-applicable',
          },
        },
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
        fleet: {
          enrollmentState: 'enrolled',
          livenessState: 'active',
          versionDrift: 'current',
          adapterHealth: 'healthy',
          configRollout: 'reported',
          credentialStatus: 'verified',
          updateStatus: 'current',
          remoteControl: 'disabled',
          configDrift: {
            status: 'pending',
            reason:
              'Pulse has not received a comparable applied agent configuration fingerprint yet',
          },
          rollout: {
            status: 'pending',
            reason: 'waiting for the agent to report an applied configuration fingerprint',
          },
          credentialHealth: { status: 'verified', kind: 'agent-token' },
          commandPolicy: {
            status: 'disabled',
            desired: 'unknown',
            applied: 'disabled',
            enforcement: 'not-applicable',
          },
        },
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
    expect(result.rows()[0].fleetHighlights).toEqual([]);
    expect(result.rows()[0].members).toMatchObject([
      {
        id: 'node-delly',
        name: 'delly',
        subtitle: 'Primary node',
        source: 'both',
        host: 'https://delly:8006',
        hostAliases: ['delly', '192.168.0.10'],
        coverageLabels: ['Host telemetry'],
        statusLabel: 'Active',
        fleetHighlights: [],
        primary: true,
      },
      {
        id: 'node-minipc',
        name: 'minipc',
        subtitle: 'Cluster member',
        source: 'both',
        host: 'https://minipc:8006',
        hostAliases: ['minipc', '192.168.0.11'],
        coverageLabels: ['Host telemetry'],
        statusLabel: 'Active',
        fleetHighlights: [],
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
