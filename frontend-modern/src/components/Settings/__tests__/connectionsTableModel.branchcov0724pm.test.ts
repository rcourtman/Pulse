import { describe, expect, it } from 'vitest';
import type {
  Connection,
  ConnectionAgentModuleStatus,
  ConnectionFleetGovernance,
} from '@/api/connections';
import {
  fleetGovernanceSignalsForConnection,
  surfaceLabel,
  visibleFleetGovernanceSignals,
  type FleetGovernanceSignal,
  type FleetGovernanceSignalKey,
} from '../connectionsTableModel';

// ---- Fixtures ---------------------------------------------------------------
// Mirrors the sibling coverage tests so the private signal builders below are
// driven through the single exported orchestrator (fleetGovernanceSignalsForConnection).

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

const fleetFixture = (
  overrides: Partial<ConnectionFleetGovernance> = {},
): ConnectionFleetGovernance => ({
  enrollmentState: 'enrolled',
  livenessState: 'active',
  versionDrift: 'current',
  adapterHealth: 'healthy',
  configRollout: 'reported',
  credentialStatus: 'verified',
  updateStatus: 'current',
  remoteControl: 'disabled',
  ...overrides,
});

const signalByKey = (
  connection: Connection,
  key: FleetGovernanceSignalKey,
): FleetGovernanceSignal | undefined =>
  fleetGovernanceSignalsForConnection(connection).find((signal) => signal.key === key);

const agentModule = (
  overrides: Partial<ConnectionAgentModuleStatus> = {},
): ConnectionAgentModuleStatus => ({
  name: 'host',
  enabled: true,
  state: 'retrying',
  updatedAt: '2026-07-09T12:00:00Z',
  ...overrides,
});

// ---- surfaceLabel (private fallback arm) ------------------------------------
//
// surfaceLabel returns SURFACE_LABELS[key] ?? key. The sibling suites exercise
// it only through the orchestrator paths with known keys; the cold arm is the
// `?? key` fallback for an unregistered surface.

describe('surfaceLabel', () => {
  it('falls back to the raw key when the surface is unregistered (?? key arm)', () => {
    expect(surfaceLabel('totallyUnknown')).toBe('totallyUnknown');
  });
});

// ---- commandPolicyFromFleet (private) — remoteControl "disconnected" --------
//
// commandPolicyFromFleet switches on remoteControl when no explicit commandPolicy
// is present. The 'disconnected' arm was the only un-exercised case: it derives a
// blocked policy that commandPolicySignal then renders as 'Remote control blocked'.

describe('commandPolicyFromFleet (via fleetGovernanceSignalsForConnection)', () => {
  it('derives a blocked command policy from remoteControl disconnected', () => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ remoteControl: 'disconnected' }) }),
      'command-policy',
    );
    expect(signal).toMatchObject({
      key: 'command-policy',
      label: 'Remote control blocked',
      detail: 'The agent reports command execution enabled, but no command channel is connected.',
      tone: 'critical',
    });
  });
});

// ---- commandPolicySignal (private) — explicit status "unknown" --------------
//
// commandPolicySignal first branches on enforcement (drifted/pending), then
// switches on status. The derived path can never produce status 'unknown' with a
// non-pending enforcement, so the switch's 'unknown' arm is only reachable
// through an explicit fleet.commandPolicy override whose enforcement is neither
// drifted nor pending.

describe('commandPolicySignal (via fleetGovernanceSignalsForConnection)', () => {
  it('surfaces an explicit unknown command-policy status as a warning when enforcement is neutral', () => {
    // enforcement is omitted (undefined) so neither the drifted nor pending
    // short-circuit fires; the status switch reaches its 'unknown' arm.
    const signal = signalByKey(
      connectionFixture({
        fleet: fleetFixture({
          remoteControl: 'enabled',
          commandPolicy: { status: 'unknown' },
        }),
      }),
      'command-policy',
    );
    expect(signal).toMatchObject({
      key: 'command-policy',
      label: 'Remote control unknown',
      detail: 'Pulse has not confirmed command-policy state yet.',
      tone: 'warning',
    });
  });
});

// ---- updateSignal (private) — failed fallback detail ------------------------
//
// updateSignal's 'failed' arm uses `update?.lastError || <fallback>`. The
// sibling suite always supplies a populated agentUpdate.lastError for the failed
// case, so the `|| fallback` arm is cold. It fires when the agent has no update
// object at all, or when lastError is blank.

describe('updateSignal (via fleetGovernanceSignalsForConnection)', () => {
  it('uses the generic update-failed detail when no agentUpdate object is present', () => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ updateStatus: 'failed' }) }),
      'updates',
    );
    expect(signal).toMatchObject({
      key: 'updates',
      label: 'Agent update failed',
      detail: 'The agent could not complete its latest update check or update attempt.',
      tone: 'critical',
    });
  });

  it('falls back to the generic detail when agentUpdate.lastError is an empty string', () => {
    const signal = signalByKey(
      connectionFixture({
        fleet: fleetFixture({ updateStatus: 'failed' }),
        agentUpdate: { state: 'error', autoUpdate: false, lastError: '' },
      }),
      'updates',
    );
    expect(signal?.detail).toBe(
      'The agent could not complete its latest update check or update attempt.',
    );
  });
});

// ---- moduleHealthSignal (private) — displayName ternary arms -----------------
//
// moduleHealthSignal maps module.name to a display name via a nested ternary
// (docker/kubernetes/host/<raw>). The sibling suites cover the docker and
// kubernetes names; the remaining arms (host + the raw fallback) are cold.

describe('moduleHealthSignal (via fleetGovernanceSignalsForConnection)', () => {
  it('prettifies a failing host module to "Host" and uses the generic detail when lastError is absent', () => {
    const signal = signalByKey(
      connectionFixture({
        agentModules: [agentModule({ name: 'host', state: 'retrying' })],
        fleet: fleetFixture(),
      }),
      'module-health',
    );
    expect(signal).toMatchObject({
      key: 'module-health',
      label: 'Host module retrying',
      tone: 'warning',
    });
    expect(signal?.detail).toBe('Host monitoring is enabled but the module is not running yet.');
  });

  it('uses the raw module name for a non-canonical module', () => {
    const signal = signalByKey(
      connectionFixture({
        agentModules: [agentModule({ name: 'backup-collector', state: 'starting' })],
        fleet: fleetFixture(),
      }),
      'module-health',
    );
    expect(signal?.label).toBe('backup-collector module starting');
    expect(signal?.detail).toBe(
      'backup-collector monitoring is enabled but the module is not running yet.',
    );
  });
});

// ---- isPassiveAgentRolloutConfirmationFallbackSignal (private) --------------
//
// This guard returns true for a rollout warning whose detail contains EITHER of
// two confirmation phrases, but only when a passive config-confirmation signal
// is also present. The sibling suites cover the first phrase
// ("staged rollout is waiting for confirmation"); the second phrase
// ("rollout state cannot be confirmed without comparable desired and applied
// agent config fingerprints") is cold. Driving it through
// visibleFleetGovernanceSignals asserts the observable effect: such a rollout
// signal is filtered out as a passive handshake.

describe('isPassiveAgentRolloutConfirmationFallbackSignal (via visibleFleetGovernanceSignals)', () => {
  it('hides a rollout signal whose detail carries the second confirmation phrase', () => {
    const signals: FleetGovernanceSignal[] = [
      {
        key: 'config-drift',
        label: 'Config pending',
        detail: 'Pulse has not received a comparable applied agent configuration fingerprint yet.',
        tone: 'warning',
      },
      {
        key: 'rollout',
        label: 'Rollout pending',
        detail:
          'rollout state cannot be confirmed without comparable desired and applied agent config fingerprints',
        tone: 'warning',
      },
    ];
    // Both signals are passive handshakes, so neither surfaces as a chip.
    expect(visibleFleetGovernanceSignals(signals)).toEqual([]);
  });

  it('keeps a rollout warning visible when its detail matches neither confirmation phrase', () => {
    const signals: FleetGovernanceSignal[] = [
      {
        key: 'config-drift',
        label: 'Config pending',
        detail: 'Pulse has not received a comparable applied agent configuration fingerprint yet.',
        tone: 'warning',
      },
      {
        key: 'rollout',
        label: 'Rollout pending',
        detail: 'The canary batch for this rollout is stalled.',
        tone: 'warning',
      },
    ];
    // The config-drift handshake is hidden, but the rollout carries an
    // actionable, non-handshake reason and must remain visible.
    expect(visibleFleetGovernanceSignals(signals).map((signal) => signal.label)).toEqual([
      'Rollout pending',
    ]);
  });
});
