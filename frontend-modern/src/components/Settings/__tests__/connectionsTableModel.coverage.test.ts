import { describe, expect, it } from 'vitest';
import type {
  Connection,
  ConnectionFleetGovernance,
  ConnectionFleetAdapterHealth,
  ConnectionFleetConfigRollout,
  ConnectionFleetCredentialStatus,
  ConnectionFleetEnrollmentState,
  ConnectionFleetLivenessState,
  ConnectionFleetRemoteControl,
  ConnectionFleetUpdateStatus,
  ConnectionFleetVersionDrift,
} from '@/api/connections';
import {
  connectionAgentEndpointDisplay,
  fleetGovernanceSignalsForConnection,
  fleetSignalClassName,
  type FleetGovernanceSignal,
  type FleetGovernanceSignalKey,
  type FleetGovernanceSignalTone,
} from '../connectionsTableModel';

// ---- Fixtures ---------------------------------------------------------------

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

// Defaults to a fully "healthy/current" agent fleet. The optional derived
// fields (configDrift/rollout/credentialHealth/commandPolicy) are left
// undefined so the *FromFleet derivation paths are exercised unless a test
// overrides them explicitly.
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

// ---- isIPv4Literal (private) via connectionAgentEndpointDisplay -------------
//
// isIPv4Literal is module-private and only reachable through
// firstDistinctAgentIPv4Alias -> connectionAgentEndpointDisplay. To exercise
// its branches we drive endpoint display with reportIp unset so the alias
// scan runs, and assert which alias (if any) is returned.

describe('isIPv4Literal (via connectionAgentEndpointDisplay)', () => {
  // A connection whose name/address are distinct from any alias, with no
  // reportIp/hostname, so the alias IPv4 scan is the decisive path.
  const aliasConnection = (hostAliases: string[]): Connection =>
    connectionFixture({
      name: 'host-1',
      address: 'host-1',
      agentIdentity: undefined,
      hostAliases,
    });

  it('recognizes canonical IPv4 literals', () => {
    expect(connectionAgentEndpointDisplay(aliasConnection(['1.2.3.4']))).toBe('1.2.3.4');
    expect(connectionAgentEndpointDisplay(aliasConnection(['12.34.56.78']))).toBe('12.34.56.78');
  });

  it('recognizes single-digit and triple-digit octet boundaries', () => {
    expect(connectionAgentEndpointDisplay(aliasConnection(['1.1.1.1']))).toBe('1.1.1.1');
    expect(connectionAgentEndpointDisplay(aliasConnection(['255.255.255.255']))).toBe(
      '255.255.255.255',
    );
    expect(connectionAgentEndpointDisplay(aliasConnection(['0.0.0.0']))).toBe('0.0.0.0');
  });

  it('treats out-of-range octets as literals (regex checks digit count, not range)', () => {
    // 256/999 exceed the valid IPv4 octet range but match \d{1,3}; current
    // behaviour accepts them. Noted as a caveat in GLM_REPORT.md.
    expect(connectionAgentEndpointDisplay(aliasConnection(['256.1.1.1']))).toBe('256.1.1.1');
    expect(connectionAgentEndpointDisplay(aliasConnection(['999.999.999.999']))).toBe(
      '999.999.999.999',
    );
  });

  it('rejects too few octets', () => {
    // '1.2.3' is not an IPv4 literal -> no alias -> name == address -> null.
    expect(connectionAgentEndpointDisplay(aliasConnection(['1.2.3']))).toBeNull();
  });

  it('rejects too many octets', () => {
    expect(connectionAgentEndpointDisplay(aliasConnection(['1.2.3.4.5']))).toBeNull();
  });

  it('rejects non-numeric octets and adjacent characters', () => {
    expect(connectionAgentEndpointDisplay(aliasConnection(['a.b.c.d']))).toBeNull();
    expect(connectionAgentEndpointDisplay(aliasConnection(['1.2.3.4a']))).toBeNull();
    expect(connectionAgentEndpointDisplay(aliasConnection(['a1.2.3.4']))).toBeNull();
  });

  it('skips whitespace-only aliases before the first valid literal', () => {
    // Empty/whitespace aliases are skipped by trim(), then the first valid
    // literal wins.
    expect(connectionAgentEndpointDisplay(aliasConnection(['   ', '', '10.0.0.5']))).toBe(
      '10.0.0.5',
    );
  });

  it('trims surrounding whitespace from a candidate before matching', () => {
    // The candidate is trimmed first, so '  10.0.0.5  ' is a valid literal.
    expect(connectionAgentEndpointDisplay(aliasConnection(['  10.0.0.5  ']))).toBe('10.0.0.5');
  });

  it('returns the first valid literal after skipping invalid and excluded aliases', () => {
    const connection = aliasConnection([
      'hostname.example', // not an IPv4 literal
      '1.2.3', // too few octets
      'not-an-ip', // not an IPv4 literal
      '10.0.0.5', // first valid -> wins
      '10.0.0.6', // would also be valid but later
    ]);
    expect(connectionAgentEndpointDisplay(connection)).toBe('10.0.0.5');
  });

  it('excludes aliases equal to name, address, or hostname (case-insensitive)', () => {
    // 'node-1' matches name 'Node-1' (lowercased), '10.0.0.1' == address,
    // '10.0.0.2' == hostname. reportIp is intentionally unset so the alias
    // scan runs; only '10.0.0.9' survives.
    const connection = connectionFixture({
      name: 'Node-1',
      address: '10.0.0.1',
      agentIdentity: { hostname: '10.0.0.2' },
      hostAliases: ['node-1', '10.0.0.1', '10.0.0.2', '10.0.0.9'],
    });
    expect(connectionAgentEndpointDisplay(connection)).toBe('10.0.0.9');
  });

  it('prefers reportIp over any host alias', () => {
    // Precedence: reportIp short-circuits before the alias scan.
    expect(
      connectionAgentEndpointDisplay(
        connectionFixture({
          agentIdentity: { reportIp: '192.168.1.50' },
          hostAliases: ['10.0.0.5'],
        }),
      ),
    ).toBe('192.168.1.50');
  });

  it('falls back to hostname when distinct from name and no alias is available', () => {
    expect(
      connectionAgentEndpointDisplay(
        connectionFixture({ name: 'host-1', agentIdentity: { hostname: 'real-host' } }),
      ),
    ).toBe('real-host');
  });

  it('falls back to address when distinct from name and no hostname/alias is available', () => {
    expect(
      connectionAgentEndpointDisplay(connectionFixture({ name: 'host-1', address: '10.0.0.99' })),
    ).toBe('10.0.0.99');
  });

  it('returns null when name equals address and nothing else is available', () => {
    expect(
      connectionAgentEndpointDisplay(connectionFixture({ name: 'host-1', address: 'host-1' })),
    ).toBeNull();
  });
});

// ---- fleetSignalClassName ---------------------------------------------------

describe('fleetSignalClassName', () => {
  const PREFIX =
    'inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-medium whitespace-nowrap ';

  const expectedByTone: Record<FleetGovernanceSignalTone, string> = {
    ok: `${PREFIX}border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-200`,
    info: `${PREFIX}border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-200`,
    warning: `${PREFIX}border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200`,
    critical: `${PREFIX}border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-200`,
    muted: `${PREFIX}border-border bg-surface-alt text-muted`,
  };

  it.each(['ok', 'info', 'warning', 'critical', 'muted'] as FleetGovernanceSignalTone[])(
    'renders the exact className for the %s tone',
    (tone) => {
      expect(fleetSignalClassName(tone)).toBe(expectedByTone[tone]);
    },
  );

  it('always embeds the shared layout prefix', () => {
    for (const tone of [
      'ok',
      'info',
      'warning',
      'critical',
      'muted',
    ] as FleetGovernanceSignalTone[]) {
      expect(fleetSignalClassName(tone).startsWith(PREFIX)).toBe(true);
    }
  });

  it('produces a distinct class string per tone', () => {
    const tones = ['ok', 'info', 'warning', 'critical', 'muted'] as FleetGovernanceSignalTone[];
    const classNames = new Set(tones.map((tone) => fleetSignalClassName(tone)));
    expect(classNames.size).toBe(tones.length);
  });
});

// ---- configDriftFromFleet (private) via fleetGovernanceSignalsForConnection -

describe('configDriftFromFleet (via fleetGovernanceSignalsForConnection)', () => {
  it('returns an explicit configDrift verbatim, ignoring configRollout', () => {
    // configRollout 'configured' would derive status 'current' (label
    // 'Config current'); the explicit 'drifted' status must win.
    const signal = signalByKey(
      connectionFixture({
        fleet: fleetFixture({
          configRollout: 'configured',
          configDrift: { status: 'drifted', reason: 'override mismatch' },
        }),
      }),
      'config-drift',
    );
    expect(signal).toMatchObject({
      label: 'Config drift',
      detail: 'override mismatch',
      tone: 'warning',
    });
  });

  it.each(['configured', 'reported'] as ConnectionFleetConfigRollout[])(
    'derives status "current" from configRollout %s when no explicit configDrift is set',
    (configRollout) => {
      const signal = signalByKey(
        connectionFixture({ fleet: fleetFixture({ configRollout }) }),
        'config-drift',
      );
      expect(signal).toMatchObject({ label: 'Config current', tone: 'ok' });
      expect(signal?.detail).toBe('Configuration is current.');
    },
  );

  it('derives status "paused" from configRollout paused', () => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ configRollout: 'paused' }) }),
      'config-drift',
    );
    expect(signal).toMatchObject({ label: 'Config paused', tone: 'muted' });
    expect(signal?.detail).toBe('Configuration rollout is paused.');
  });

  it('derives status "unknown" from configRollout unknown', () => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ configRollout: 'unknown' }) }),
      'config-drift',
    );
    expect(signal).toMatchObject({ label: 'Config unknown', tone: 'warning' });
    expect(signal?.detail).toBe('Configuration drift state is unknown.');
  });
});

// ---- rolloutFromFleet (private) --------------------------------------------

describe('rolloutFromFleet (via fleetGovernanceSignalsForConnection)', () => {
  it('returns an explicit rollout verbatim, ignoring configRollout', () => {
    // configRollout 'configured' would derive 'current'; explicit 'blocked'
    // must win.
    const signal = signalByKey(
      connectionFixture({
        fleet: fleetFixture({
          configRollout: 'configured',
          rollout: { status: 'blocked', reason: 'halted by operator' },
        }),
      }),
      'rollout',
    );
    expect(signal).toMatchObject({
      label: 'Rollout blocked',
      detail: 'halted by operator',
      tone: 'critical',
    });
  });

  it.each(['configured', 'reported'] as ConnectionFleetConfigRollout[])(
    'derives status "current" / stage "applied" from configRollout %s',
    (configRollout) => {
      const signal = signalByKey(
        connectionFixture({ fleet: fleetFixture({ configRollout }) }),
        'rollout',
      );
      expect(signal).toMatchObject({ label: 'Rollout current', tone: 'ok' });
      expect(signal?.detail).toBe('The rollout state is current.');
    },
  );

  it('derives status "paused" / stage "paused" from configRollout paused', () => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ configRollout: 'paused' }) }),
      'rollout',
    );
    expect(signal).toMatchObject({ label: 'Rollout paused', tone: 'warning' });
    expect(signal?.detail).toBe('The staged rollout is paused.');
  });

  it('derives status "unknown" from configRollout unknown', () => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ configRollout: 'unknown' }) }),
      'rollout',
    );
    expect(signal).toMatchObject({ label: 'Rollout unknown', tone: 'warning' });
    expect(signal?.detail).toBe('Pulse has not classified staged rollout state yet.');
  });
});

// ---- credentialHealthFromFleet (private) -----------------------------------

describe('credentialHealthFromFleet (via fleetGovernanceSignalsForConnection)', () => {
  it('returns an explicit credentialHealth verbatim, ignoring credentialStatus', () => {
    // credentialStatus 'verified' would derive 'verified'; explicit 'expired'
    // must win.
    const signal = signalByKey(
      connectionFixture({
        fleet: fleetFixture({
          credentialStatus: 'verified',
          credentialHealth: { status: 'expired' },
        }),
      }),
      'credential-health',
    );
    expect(signal).toMatchObject({ label: 'Credentials expired', tone: 'critical' });
  });

  it.each([
    ['verified', 'Credentials verified', 'ok'],
    ['invalid', 'Credentials invalid', 'critical'],
    ['paused', 'Credentials paused', 'muted'],
    ['unknown', 'Credentials unknown', 'warning'],
  ] as const satisfies ReadonlyArray<
    [ConnectionFleetCredentialStatus, string, FleetGovernanceSignalTone]
  >)('derives the credential-health signal from credentialStatus %s', (status, label, tone) => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ credentialStatus: status }) }),
      'credential-health',
    );
    expect(signal).toMatchObject({ label, tone });
  });

  it('surfaces credential-health for non-agent API sources too', () => {
    // credentialHealth is source-agnostic: it appears even when agent-fleet
    // signals are gated off for a pull-based source.
    const signal = signalByKey(
      connectionFixture({
        type: 'pbs',
        fleet: fleetFixture({ credentialStatus: 'invalid' }),
      }),
      'credential-health',
    );
    expect(signal).toMatchObject({ label: 'Credentials invalid', tone: 'critical' });
  });
});

// ---- commandPolicyFromFleet (private) --------------------------------------

describe('commandPolicyFromFleet (via fleetGovernanceSignalsForConnection)', () => {
  it('returns an explicit commandPolicy verbatim, ignoring remoteControl', () => {
    // remoteControl 'disabled' would derive an info 'Remote control disabled';
    // the explicit blocked status must win.
    const signal = signalByKey(
      connectionFixture({
        fleet: fleetFixture({
          remoteControl: 'disabled',
          commandPolicy: { status: 'blocked', enforcement: 'blocked' },
        }),
      }),
      'command-policy',
    );
    expect(signal).toMatchObject({ label: 'Remote control blocked', tone: 'critical' });
  });

  it('flags an explicit drifted policy with desired=disabled/applied=enabled as critical', () => {
    // commandPolicySignal: enforcement 'drifted' + desired disabled & applied
    // enabled -> critical (the dangerous direction).
    const signal = signalByKey(
      connectionFixture({
        fleet: fleetFixture({
          remoteControl: 'enabled',
          commandPolicy: {
            status: 'enabled',
            desired: 'disabled',
            applied: 'enabled',
            enforcement: 'drifted',
            reason: 'policy reverted by agent',
          },
        }),
      }),
      'command-policy',
    );
    expect(signal).toMatchObject({
      label: 'Command policy mismatch',
      detail: 'policy reverted by agent',
      tone: 'critical',
    });
  });

  it('flags an explicit drifted policy in the safe direction as warning', () => {
    // desired enabled / applied disabled is the less dangerous drift -> warning.
    const signal = signalByKey(
      connectionFixture({
        fleet: fleetFixture({
          commandPolicy: {
            status: 'disabled',
            desired: 'enabled',
            applied: 'disabled',
            enforcement: 'drifted',
          },
        }),
      }),
      'command-policy',
    );
    expect(signal).toMatchObject({ label: 'Command policy mismatch', tone: 'warning' });
  });

  it.each([
    ['enabled', 'Remote control enabled', 'info'],
    ['disabled', 'Remote control disabled', 'info'],
    ['not-applicable', 'No remote control', 'muted'],
  ] as const satisfies ReadonlyArray<
    [ConnectionFleetRemoteControl, string, FleetGovernanceSignalTone]
  >)(
    'derives the command-policy signal from remoteControl %s (enforcement not-applicable)',
    (remoteControl, label, tone) => {
      const signal = signalByKey(
        connectionFixture({ fleet: fleetFixture({ remoteControl }) }),
        'command-policy',
      );
      expect(signal).toMatchObject({ label, tone });
    },
  );

  it('derives a pending signal from remoteControl unknown (enforcement pending short-circuits the status)', () => {
    // commandPolicyFromFleet maps remoteControl 'unknown' to enforcement
    // 'pending'; commandPolicySignal treats enforcement 'pending' as the
    // decisive branch, so the label is 'Command policy pending' rather than
    // 'Remote control unknown'.
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ remoteControl: 'unknown' }) }),
      'command-policy',
    );
    expect(signal).toMatchObject({ label: 'Command policy pending', tone: 'warning' });
  });
});

// ---- enrollmentSignal (private) --------------------------------------------

describe('enrollmentSignal (via fleetGovernanceSignalsForConnection)', () => {
  it.each([
    ['configured', 'Configured', 'ok'],
    ['enrolled', 'Enrolled', 'ok'],
    ['paused', 'Paused', 'muted'],
  ] as const satisfies ReadonlyArray<
    [ConnectionFleetEnrollmentState, string, FleetGovernanceSignalTone]
  >)('maps enrollment state %s to its signal', (state, label, tone) => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ enrollmentState: state }) }),
      'enrollment',
    );
    expect(signal).toMatchObject({ key: 'enrollment', label, tone });
  });

  it('uses the agent-specific detail for pending agent connections', () => {
    const signal = signalByKey(
      connectionFixture({ type: 'agent', fleet: fleetFixture({ enrollmentState: 'pending' }) }),
      'enrollment',
    );
    expect(signal).toMatchObject({ label: 'Enrollment pending', tone: 'warning' });
    expect(signal?.detail).toBe('Pulse has not received the first agent report yet.');
  });

  it('uses the generic detail for pending non-agent sources', () => {
    const signal = signalByKey(
      connectionFixture({ type: 'pbs', fleet: fleetFixture({ enrollmentState: 'pending' }) }),
      'enrollment',
    );
    expect(signal).toMatchObject({ label: 'Enrollment pending', tone: 'warning' });
    expect(signal?.detail).toBe('Pulse has not confirmed this source yet.');
  });
});

// ---- livenessSignal (private) ----------------------------------------------

describe('livenessSignal (via fleetGovernanceSignalsForConnection)', () => {
  it.each([
    ['active', 'Live', 'ok'],
    ['paused', 'Paused', 'muted'],
    ['stale', 'Stale', 'warning'],
    ['pending', 'Pending', 'warning'],
    ['unauthorized', 'Unauthorized', 'critical'],
    ['unreachable', 'Unreachable', 'critical'],
  ] as const satisfies ReadonlyArray<
    [ConnectionFleetLivenessState, string, FleetGovernanceSignalTone]
  >)('maps liveness state %s to its signal', (state, label, tone) => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ livenessState: state }) }),
      'liveness',
    );
    expect(signal).toMatchObject({ key: 'liveness', label, tone });
  });
});

// ---- versionSignal (private) -----------------------------------------------

describe('versionSignal (via fleetGovernanceSignalsForConnection)', () => {
  // versionSignal is only emitted for agent-fleet connection types.
  it.each([
    ['behind', 'Version behind', 'warning'],
    ['current', 'Version current', 'ok'],
    ['unknown', 'Version unknown', 'muted'],
    ['not-applicable', 'No agent version', 'muted'],
  ] as const satisfies ReadonlyArray<
    [ConnectionFleetVersionDrift, string, FleetGovernanceSignalTone]
  >)('maps version drift %s to its signal', (state, label, tone) => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ versionDrift: state }) }),
      'version',
    );
    expect(signal).toMatchObject({ key: 'version', label, tone });
  });
});

// ---- fleetGovernanceSignalsForConnection (orchestrator) --------------------

describe('fleetGovernanceSignalsForConnection', () => {
  it('falls back to the default fleet governance when connection.fleet is undefined', () => {
    // DEFAULT_FLEET_GOVERNANCE maps every field to its unknown/pending
    // baseline; for an agent that yields a consistent set of derived signals.
    const connection = connectionFixture({ fleet: undefined });
    const signals = fleetGovernanceSignalsForConnection(connection);
    const labels = signals.map((signal) => signal.label);
    expect(labels).toEqual(
      expect.arrayContaining([
        'Enrollment pending',
        'Pending',
        'Credentials unknown',
        'Config unknown',
        'Rollout unknown',
        'Version unknown',
        'Update unknown',
        'Adapter unknown',
        'No remote control',
      ]),
    );
  });

  it('gates agent-fleet signals on for docker connections', () => {
    const keys = fleetGovernanceSignalsForConnection(
      connectionFixture({
        type: 'docker',
        fleet: fleetFixture({ versionDrift: 'behind', updateStatus: 'update-available' }),
      }),
    ).map((signal) => signal.key);
    expect(keys).toContain('version');
    expect(keys).toContain('config-drift');
    expect(keys).toContain('rollout');
    expect(keys).toContain('command-policy');
  });

  it('gates agent-fleet signals on for kubernetes connections', () => {
    const keys = fleetGovernanceSignalsForConnection(
      connectionFixture({ type: 'kubernetes', fleet: fleetFixture() }),
    ).map((signal) => signal.key);
    expect(keys).toContain('version');
    expect(keys).toContain('command-policy');
  });

  it('gates agent-fleet signals off for pull-based API sources', () => {
    const keys = fleetGovernanceSignalsForConnection(
      connectionFixture({ type: 'pve', fleet: fleetFixture() }),
    ).map((signal) => signal.key);
    expect(keys).toEqual(['enrollment', 'liveness', 'credential-health', 'adapter']);
  });

  it('emits signals in the documented order for an agent with a failing module', () => {
    const keys = fleetGovernanceSignalsForConnection(
      connectionFixture({
        agentModules: [
          {
            name: 'kubernetes',
            enabled: true,
            state: 'starting',
            updatedAt: '2026-07-09T12:00:00Z',
          },
        ],
        fleet: fleetFixture(),
      }),
    ).map((signal) => signal.key);
    expect(keys).toEqual([
      'enrollment',
      'liveness',
      'credential-health',
      'module-health',
      'config-drift',
      'rollout',
      'version',
      'updates',
      'adapter',
      'command-policy',
    ]);
  });

  it('omits the module-health signal when no enabled module is failing', () => {
    const keys = fleetGovernanceSignalsForConnection(
      connectionFixture({
        agentModules: [
          {
            name: 'host',
            enabled: true,
            state: 'running',
            updatedAt: '2026-07-09T12:00:00Z',
          },
        ],
        fleet: fleetFixture(),
      }),
    ).map((signal) => signal.key);
    expect(keys).not.toContain('module-health');
  });

  it('always emits the adapter signal with the declared health regardless of type', () => {
    const adapterStates: ReadonlyArray<ConnectionFleetAdapterHealth> = [
      'healthy',
      'degraded',
      'blocked',
      'paused',
      'unknown',
    ];
    for (const adapterHealth of adapterStates) {
      const signal = signalByKey(
        connectionFixture({ type: 'pbs', fleet: fleetFixture({ adapterHealth }) }),
        'adapter',
      );
      expect(signal).toBeDefined();
    }
  });

  it('maps each update status to its updates signal for agent sources', () => {
    const updateStates: ReadonlyArray<ConnectionFleetUpdateStatus> = [
      'update-available',
      'checking',
      'updating',
      'failed',
      'disabled',
      'current',
      'unknown',
      'not-applicable',
    ];
    for (const updateStatus of updateStates) {
      const signal = signalByKey(
        connectionFixture({
          fleet: fleetFixture({ updateStatus }),
          agentUpdate:
            updateStatus === 'failed'
              ? { state: 'error', autoUpdate: true, lastError: 'boom' }
              : undefined,
        }),
        'updates',
      );
      expect(signal).toBeDefined();
      expect(signal?.key).toBe('updates');
    }
  });
});
