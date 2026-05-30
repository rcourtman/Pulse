import { describe, expect, it } from 'vitest';
import type { Connection, ConnectionAgentIdentity } from '@/api/connections';
import {
  connectionAgentIdentitySummary,
  fleetGovernanceSignalsForConnection,
  visibleFleetGovernanceSignals,
  type FleetGovernanceSignal,
} from '../connectionsTableModel';

const connectionFixture = (overrides: Partial<Connection> = {}): Connection => ({
  id: 'agent:host-1',
  type: 'agent',
  name: 'host-1',
  address: 'host-1',
  state: 'active',
  stateReason: '',
  enabled: true,
  surfaces: ['host'],
  scope: { host: true },
  lastSeen: '2026-04-23T12:00:00Z',
  lastError: null,
  source: 'agent',
  capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
  ...overrides,
});

describe('visibleFleetGovernanceSignals', () => {
  it('hides passive agent config fingerprint handshakes while preserving raw fleet facts', () => {
    const rawSignals = fleetGovernanceSignalsForConnection(
      connectionFixture({
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
      }),
    );

    expect(rawSignals.map((signal) => signal.label)).toEqual(
      expect.arrayContaining(['Config pending', 'Rollout pending', 'Remote control disabled']),
    );
    expect(visibleFleetGovernanceSignals(rawSignals).map((signal) => signal.label)).toEqual([]);
  });

  it('does not turn default unmanaged agent config into setup attention', () => {
    const rawSignals = fleetGovernanceSignalsForConnection(
      connectionFixture({
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
            status: 'not-applicable',
            reason: 'no managed agent configuration override is assigned',
          },
          rollout: {
            status: 'current',
            stage: 'applied',
            reason: 'no managed agent configuration rollout is assigned',
          },
          credentialHealth: { status: 'verified', kind: 'agent-token' },
          commandPolicy: {
            status: 'disabled',
            desired: 'unknown',
            applied: 'disabled',
            enforcement: 'not-applicable',
          },
        },
      }),
    );

    expect(rawSignals.map((signal) => signal.label)).toEqual(
      expect.arrayContaining(['No config drift', 'Rollout current', 'Remote control disabled']),
    );
    expect(visibleFleetGovernanceSignals(rawSignals).map((signal) => signal.label)).toEqual([]);
  });

  it('keeps actionable config and rollout warnings visible', () => {
    const rawSignals = fleetGovernanceSignalsForConnection(
      connectionFixture({
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
            reason: 'Desired config v2 has been assigned and the agent has not applied it yet.',
          },
          rollout: {
            status: 'pending',
            reason: 'The staged rollout is waiting for the next canary batch.',
          },
          credentialHealth: { status: 'verified', kind: 'agent-token' },
          commandPolicy: {
            status: 'disabled',
            desired: 'disabled',
            applied: 'disabled',
            enforcement: 'in-sync',
          },
        },
      }),
    );

    expect(visibleFleetGovernanceSignals(rawSignals).map((signal) => signal.label)).toEqual([
      'Config pending',
      'Rollout pending',
    ]);
  });

  it('does not hide unrelated warning signals just because their detail mentions fingerprints', () => {
    const signals: FleetGovernanceSignal[] = [
      {
        key: 'credentials',
        label: 'Credentials expiring',
        detail: 'Token rotation must refresh the credential fingerprint before expiry.',
        tone: 'warning',
      },
      {
        key: 'config-drift',
        label: 'Config pending',
        detail: 'Pulse has not received a comparable applied agent configuration fingerprint yet.',
        tone: 'warning',
      },
    ];

    expect(visibleFleetGovernanceSignals(signals).map((signal) => signal.label)).toEqual([
      'Credentials expiring',
    ]);
  });

  it('returns no chips when every warning was a passive handshake', () => {
    const signals: FleetGovernanceSignal[] = [
      {
        key: 'config-drift',
        label: 'Config pending',
        detail: 'Pulse has not received a comparable applied agent config fingerprint yet.',
        tone: 'warning',
      },
      {
        key: 'rollout',
        label: 'Rollout pending',
        detail: 'Waiting for the agent to report an applied configuration fingerprint.',
        tone: 'warning',
      },
    ];

    expect(visibleFleetGovernanceSignals(signals)).toEqual([]);
  });

  it('hides rollout fallback copy when a passive config confirmation marks the handshake', () => {
    const rawSignals = fleetGovernanceSignalsForConnection(
      connectionFixture({
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
          },
          credentialHealth: { status: 'verified', kind: 'agent-token' },
          commandPolicy: {
            status: 'disabled',
            desired: 'unknown',
            applied: 'disabled',
            enforcement: 'not-applicable',
          },
        },
      }),
    );

    expect(visibleFleetGovernanceSignals(rawSignals).map((signal) => signal.label)).toEqual([]);
  });
});

describe('fleetGovernanceSignalsForConnection connection-type gating', () => {
  it('omits agent-fleet governance signals for pull-based API sources', () => {
    const signals = fleetGovernanceSignalsForConnection(
      connectionFixture({
        id: 'pbs:pbs-docker',
        type: 'pbs',
        name: 'pbs-docker',
        state: 'unreachable',
        source: 'manual',
        fleet: {
          enrollmentState: 'configured',
          livenessState: 'unreachable',
          versionDrift: 'unknown',
          adapterHealth: 'blocked',
          configRollout: 'unknown',
          credentialStatus: 'unknown',
          updateStatus: 'unknown',
          remoteControl: 'not-applicable',
          rollout: { status: 'blocked', reason: 'blocked by the current connection state' },
        },
      }),
    );
    const keys = signals.map((signal) => signal.key);
    // The backend echoed a blocked rollout, but an API source has no agent, so
    // none of the agent-binary / managed-config governance signals must surface
    // (this is what produced the misleading "Rollout blocked" problem line).
    expect(signals.map((signal) => signal.label)).not.toContain('Rollout blocked');
    expect(keys).not.toContain('rollout');
    expect(keys).not.toContain('config-drift');
    expect(keys).not.toContain('version');
    expect(keys).not.toContain('updates');
    expect(keys).not.toContain('command-policy');
    // Source-agnostic signals still apply to API sources.
    expect(keys).toContain('enrollment');
    expect(keys).toContain('liveness');
    expect(keys).toContain('credential-health');
  });

  it('keeps agent-fleet governance signals for agent connections', () => {
    const keys = fleetGovernanceSignalsForConnection(
      connectionFixture({
        fleet: {
          enrollmentState: 'enrolled',
          livenessState: 'active',
          versionDrift: 'behind',
          adapterHealth: 'healthy',
          configRollout: 'reported',
          credentialStatus: 'verified',
          updateStatus: 'update-available',
          remoteControl: 'disabled',
          rollout: { status: 'blocked', reason: 'staged rollout halted' },
        },
      }),
    ).map((signal) => signal.key);
    expect(keys).toContain('rollout');
    expect(keys).toContain('version');
    expect(keys).toContain('command-policy');
  });
});

describe('connectionAgentIdentitySummary', () => {
  const withAgentIdentity = (
    agentIdentity: Partial<ConnectionAgentIdentity> | undefined,
  ): Connection =>
    connectionFixture({ agentIdentity: agentIdentity as ConnectionAgentIdentity | undefined });

  it('uses osName platform identity over the broader agent platform family', () => {
    const summary = connectionAgentIdentitySummary(
      withAgentIdentity({
        platform: 'debian',
        osName: 'Proxmox VE',
        osVersion: '9.1.9',
      }),
    );
    expect(summary).toBe('Proxmox VE 9.1.9');
  });

  it('uses Unraid osName over Linux platform family', () => {
    const summary = connectionAgentIdentitySummary(
      withAgentIdentity({
        platform: 'linux',
        osName: 'Unraid',
        osVersion: '7.2.2',
      }),
    );
    expect(summary).toBe('Unraid 7.2.2');
  });

  it('falls back to prettified platform when osName is missing', () => {
    const summary = connectionAgentIdentitySummary(
      withAgentIdentity({
        platform: 'linux',
        osVersion: '6.1.0',
      }),
    );
    expect(summary).toBe('Linux 6.1.0');
  });

  it('returns null when no agent identity is present', () => {
    const summary = connectionAgentIdentitySummary(withAgentIdentity(undefined));
    expect(summary).toBeNull();
  });
});
