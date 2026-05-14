import { describe, expect, it } from 'vitest';
import type { Connection } from '@/api/connections';
import {
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
    expect(visibleFleetGovernanceSignals(rawSignals).map((signal) => signal.label)).toEqual([
      'Remote control disabled',
    ]);
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
    expect(visibleFleetGovernanceSignals(rawSignals).map((signal) => signal.label)).toEqual([
      'Remote control disabled',
    ]);
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
      'Remote control disabled',
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

  it('falls back to Fleet OK only when every warning was a passive handshake', () => {
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

    expect(visibleFleetGovernanceSignals(signals)).toMatchObject([
      {
        key: 'liveness',
        label: 'Fleet OK',
        tone: 'ok',
      },
    ]);
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

    expect(visibleFleetGovernanceSignals(rawSignals).map((signal) => signal.label)).toEqual([
      'Remote control disabled',
    ]);
  });
});
