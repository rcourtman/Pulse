import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type {
  Connection,
  ConnectionState,
  ConnectionFleetConfigDriftStatus,
  ConnectionFleetCredentialHealthStatus,
  ConnectionFleetGovernance,
  ConnectionFleetRolloutStatus,
} from '@/api/connections';
import {
  agentAttachmentProblem,
  connectionAgentEndpointDisplay,
  connectionAgentIdentitySummary,
  connectionAgentVersionPresentation,
  fleetGovernanceSignalsForConnection,
  lastActivityTextFromLastSeen,
  type FleetGovernanceSignal,
  type FleetGovernanceSignalKey,
  type FleetGovernanceSignalTone,
} from '../connectionsTableModel';

// ---- Fixtures ---------------------------------------------------------------
// Mirrors the sibling coverage test so the private signal builders below are
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

// The pinned instant every relative-age assertion is computed against.
const PINNED_NOW = new Date('2026-07-11T12:00:00.000Z');

// ---- lastActivityTextFromLastSeen ------------------------------------------

describe('lastActivityTextFromLastSeen', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it.each([undefined, null, ''].map((value) => [value] as [string | null | undefined]))(
    'returns "No activity yet" for a falsy lastSeen (%s)',
    (lastSeen) => {
      vi.setSystemTime(PINNED_NOW);
      expect(lastActivityTextFromLastSeen(lastSeen)).toBe('No activity yet');
    },
  );

  it.each(['not-a-date', '   '] as const)(
    'returns "Unknown" for a truthy but unparseable lastSeen (%s)',
    (lastSeen) => {
      // A whitespace-only string is truthy, so it bypasses the !lastSeen guard
      // and falls through to Date.parse -> NaN -> "Unknown".
      vi.setSystemTime(PINNED_NOW);
      expect(lastActivityTextFromLastSeen(lastSeen)).toBe('Unknown');
    },
  );

  it('reports 0s ago for a timestamp at exactly the current instant', () => {
    vi.setSystemTime(PINNED_NOW);
    expect(lastActivityTextFromLastSeen('2026-07-11T12:00:00.000Z')).toBe('0s ago');
  });

  it('clamps a future timestamp to 0s ago instead of a negative age', () => {
    vi.setSystemTime(PINNED_NOW);
    expect(lastActivityTextFromLastSeen('2026-07-11T13:00:00.000Z')).toBe('0s ago');
  });

  it.each([
    ['2026-07-11T11:59:30.000Z', '30s ago'],
    ['2026-07-11T11:59:00.000Z', '1m ago'],
    ['2026-07-11T11:55:00.000Z', '5m ago'],
    ['2026-07-11T11:00:00.000Z', '1h ago'],
    ['2026-07-11T09:00:00.000Z', '3h ago'],
    ['2026-07-10T12:00:00.000Z', '1d ago'],
    ['2026-07-08T12:00:00.000Z', '3d ago'],
  ] as const satisfies ReadonlyArray<[string, string]>)(
    'formats %s as %s relative to the pinned now',
    (lastSeen, expected) => {
      vi.setSystemTime(PINNED_NOW);
      expect(lastActivityTextFromLastSeen(lastSeen)).toBe(expected);
    },
  );
});

// ---- agentAttachmentProblem -------------------------------------------------

describe('agentAttachmentProblem', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('appends the relative-age suffix for a stale agent with a known lastSeen', () => {
    vi.setSystemTime(PINNED_NOW);
    const problem = agentAttachmentProblem(
      connectionFixture({
        state: 'stale',
        lastSeen: '2026-07-11T08:00:00.000Z',
        stateReason: 'no keepalive',
      }),
    );
    expect(problem).toEqual({
      label: 'Agent offline · 4h ago',
      detail: 'no keepalive',
      tone: 'warning',
    });
  });

  it('omits the suffix when lastSeen is null (No activity yet) and uses the fallback detail', () => {
    vi.setSystemTime(PINNED_NOW);
    const problem = agentAttachmentProblem(
      connectionFixture({ state: 'stale', lastSeen: null, stateReason: '' }),
    );
    expect(problem?.label).toBe('Agent offline');
    expect(problem?.detail).toBe(
      'The Pulse Agent on this host has not reported recently. Proxmox API metrics are unaffected.',
    );
    expect(problem?.tone).toBe('warning');
  });

  it('omits the suffix when lastSeen is unparseable (Unknown)', () => {
    vi.setSystemTime(PINNED_NOW);
    const problem = agentAttachmentProblem(
      connectionFixture({ state: 'stale', lastSeen: 'garbage', stateReason: '' }),
    );
    expect(problem?.label).toBe('Agent offline');
  });

  it.each([
    [
      'unreachable',
      'Agent unreachable',
      'Pulse cannot currently reach the agent on this host.',
      'critical',
    ],
    ['unauthorized', 'Agent unauthorized', 'The Pulse Agent token is being rejected.', 'critical'],
    ['pending', 'Agent pending first report', 'The Pulse Agent has not reported yet.', 'warning'],
  ] as const satisfies ReadonlyArray<[ConnectionState, string, string, 'critical' | 'warning']>)(
    'uses the fallback detail for state %s when stateReason is empty',
    (state, label, detail, tone) => {
      vi.setSystemTime(PINNED_NOW);
      expect(agentAttachmentProblem(connectionFixture({ state, stateReason: '' }))).toEqual({
        label,
        detail,
        tone,
      });
    },
  );

  it.each([
    ['unreachable', 'Agent unreachable'],
    ['unauthorized', 'Agent unauthorized'],
    ['pending', 'Agent pending first report'],
  ] as const satisfies ReadonlyArray<[ConnectionState, string]>)(
    'uses the explicit stateReason for state %s when one is provided',
    (state, label) => {
      vi.setSystemTime(PINNED_NOW);
      const problem = agentAttachmentProblem(
        connectionFixture({ state, stateReason: 'explicit operator reason' }),
      );
      expect(problem?.label).toBe(label);
      expect(problem?.detail).toBe('explicit operator reason');
    },
  );

  it.each(['active', 'paused'] as const)(
    'returns undefined for the non-problem state %s',
    (state) => {
      vi.setSystemTime(PINNED_NOW);
      expect(agentAttachmentProblem(connectionFixture({ state }))).toBeUndefined();
    },
  );
});

// ---- connectionAgentVersionPresentation -------------------------------------

describe('connectionAgentVersionPresentation', () => {
  it('renders the amber "Update available" badge when both versions and the flag are set', () => {
    const presentation = connectionAgentVersionPresentation(
      connectionFixture({
        agentVersion: '6.0.0',
        expectedAgentVersion: '6.0.3',
        agentUpdateAvailable: true,
      }),
    );
    expect(presentation).toMatchObject({
      badgeLabel: 'Update available',
      detail: '6.0.0 -> 6.0.3',
      title: 'Pulse Agent update available: 6.0.0 -> 6.0.3',
    });
    expect(presentation?.badgeClassName).toContain('amber');
  });

  it('trims whitespace from both versions before composing the update detail', () => {
    const presentation = connectionAgentVersionPresentation(
      connectionFixture({
        agentVersion: '  6.0.0  ',
        expectedAgentVersion: ' 6.0.3 ',
        agentUpdateAvailable: true,
      }),
    );
    expect(presentation?.detail).toBe('6.0.0 -> 6.0.3');
    expect(presentation?.title).toBe('Pulse Agent update available: 6.0.0 -> 6.0.3');
  });

  it('falls through to the version-target badge when the flag is set but the current version is blank', () => {
    // agentUpdateAvailable is true, but currentVersion trims to '' so the
    // update branch short-circuits; the plain agent-version branch also needs
    // a current version, so the expected-version ("Version target") branch wins.
    const presentation = connectionAgentVersionPresentation(
      connectionFixture({
        agentVersion: '   ',
        expectedAgentVersion: '6.0.3',
        agentUpdateAvailable: true,
      }),
    );
    expect(presentation?.badgeLabel).toBe('Version target');
    expect(presentation?.detail).toBe('6.0.3');
  });

  it('renders the agent-version badge when the flag is set but the expected version is blank', () => {
    const presentation = connectionAgentVersionPresentation(
      connectionFixture({
        agentVersion: '6.0.0',
        expectedAgentVersion: '',
        agentUpdateAvailable: true,
      }),
    );
    expect(presentation?.badgeLabel).toBe('Agent version');
    expect(presentation?.detail).toBe('6.0.0');
  });

  it('renders the neutral agent-version badge when no update is flagged', () => {
    const presentation = connectionAgentVersionPresentation(
      connectionFixture({
        agentVersion: '6.0.0',
        expectedAgentVersion: '6.0.3',
        agentUpdateAvailable: false,
      }),
    );
    expect(presentation).toMatchObject({ badgeLabel: 'Agent version', detail: '6.0.0' });
    expect(presentation?.badgeClassName).not.toContain('amber');
  });

  it('renders the version-target badge when only the expected version is known', () => {
    const presentation = connectionAgentVersionPresentation(
      connectionFixture({ agentVersion: undefined, expectedAgentVersion: '6.0.3' }),
    );
    expect(presentation).toMatchObject({
      badgeLabel: 'Version target',
      detail: '6.0.3',
      title: 'Pulse Agent target version 6.0.3',
    });
  });

  it('returns null when neither version is present and no update is flagged', () => {
    expect(
      connectionAgentVersionPresentation(
        connectionFixture({
          agentVersion: undefined,
          expectedAgentVersion: undefined,
          agentUpdateAvailable: false,
        }),
      ),
    ).toBeNull();
  });

  it('returns null when agentUpdateAvailable is unset and no versions are known', () => {
    // agentUpdateAvailable is optional; undefined is falsy and must not surface a badge.
    expect(
      connectionAgentVersionPresentation(
        connectionFixture({ agentVersion: undefined, expectedAgentVersion: undefined }),
      ),
    ).toBeNull();
  });
});

// ---- connectionAgentEndpointDisplay (remaining branches) -------------------

describe('connectionAgentEndpointDisplay (remaining branches)', () => {
  it('skips a hostname that equals the name case-insensitively and falls through to a distinct address', () => {
    const endpoint = connectionAgentEndpointDisplay(
      connectionFixture({
        name: 'Host-1',
        address: '10.0.0.7',
        agentIdentity: { hostname: 'host-1' },
      }),
    );
    // hostname 'host-1' lowercased equals name 'Host-1' lowercased -> skipped;
    // address '10.0.0.7' is distinct -> returned.
    expect(endpoint).toBe('10.0.0.7');
  });

  it('returns null when no reportIp, alias, hostname, or address is available', () => {
    expect(
      connectionAgentEndpointDisplay(
        connectionFixture({
          name: 'host-1',
          address: '',
          agentIdentity: undefined,
          hostAliases: [],
        }),
      ),
    ).toBeNull();
  });
});

// ---- prettifyPlatform (private) via connectionAgentIdentitySummary ----------
//
// prettifyPlatform is module-private and only reachable through
// connectionAgentHostProfileLabel -> connectionAgentIdentitySummary. An identity
// carrying only `platform` (no hostProfile/osName/osVersion) reduces the summary
// to the prettified platform value, exercising every switch arm.

describe('prettifyPlatform (via connectionAgentIdentitySummary)', () => {
  const platformOnly = (platform: string): Connection =>
    connectionFixture({ agentIdentity: { platform } });

  it.each([
    ['linux', 'Linux'],
    ['windows', 'Windows'],
    ['darwin', 'macOS'],
    ['macos', 'macOS'],
    ['freebsd', 'FreeBSD'],
  ] as const satisfies ReadonlyArray<[string, string]>)(
    'prettifies the canonical platform %s to %s',
    (platform, expected) => {
      expect(connectionAgentIdentitySummary(platformOnly(platform))).toBe(expected);
    },
  );

  it('normalizes case before matching (LINUX -> Linux, macOS stays macOS)', () => {
    expect(connectionAgentIdentitySummary(platformOnly('LINUX'))).toBe('Linux');
    expect(connectionAgentIdentitySummary(platformOnly('macOS'))).toBe('macOS');
  });

  it('returns the trimmed original casing for an unknown platform (default switch arm)', () => {
    // '  Arch Linux  ' lowercases to 'arch linux' which is not a canonical
    // case; the default arm returns the trimmed ORIGINAL casing.
    expect(connectionAgentIdentitySummary(platformOnly('  Arch Linux  '))).toBe('Arch Linux');
  });

  it.each(['', '   '])('returns null for a blank platform (%s)', (platform) => {
    expect(connectionAgentIdentitySummary(platformOnly(platform))).toBeNull();
  });

  it('returns null when platform is undefined and no other identity field is set', () => {
    expect(
      connectionAgentIdentitySummary(connectionFixture({ agentIdentity: { platform: undefined } })),
    ).toBeNull();
  });
});

// ---- connectionAgentHostProfileLabel (private) via connectionAgentIdentitySummary

describe('connectionAgentHostProfileLabel (via connectionAgentIdentitySummary)', () => {
  it('returns the manifest family for a known host profile', () => {
    expect(
      connectionAgentIdentitySummary(
        connectionFixture({ agentIdentity: { hostProfile: 'unraid' } }),
      ),
    ).toBe('Unraid');
  });

  it('resolves a host-profile alias token to its family', () => {
    expect(
      connectionAgentIdentitySummary(
        connectionFixture({ agentIdentity: { hostProfile: 'unraid-os' } }),
      ),
    ).toBe('Unraid');
  });

  it('falls back to prettifyPlatform when the host profile is not in the manifest', () => {
    // 'centos' has no manifest family and is not a canonical platform case, so
    // the default arm of prettifyPlatform returns the trimmed value verbatim.
    expect(
      connectionAgentIdentitySummary(
        connectionFixture({ agentIdentity: { hostProfile: 'centos' } }),
      ),
    ).toBe('centos');
  });

  it('combines the host-profile family with an os version when both are present', () => {
    expect(
      connectionAgentIdentitySummary(
        connectionFixture({ agentIdentity: { hostProfile: 'unraid', osVersion: '7.1.0' } }),
      ),
    ).toBe('Unraid 7.1.0');
  });

  it('returns the osName alone when osVersion is missing and hostProfile is absent', () => {
    // hostProfile is null here, so the summary cannot be coming from the
    // hostProfile branch; this isolates the `if (osName) return osName` arm.
    expect(
      connectionAgentIdentitySummary(connectionFixture({ agentIdentity: { osName: 'Custom OS' } })),
    ).toBe('Custom OS');
  });

  it('returns null when every identity field is blank', () => {
    expect(
      connectionAgentIdentitySummary(
        connectionFixture({
          agentIdentity: { platform: '   ', osName: '', osVersion: '  ', hostProfile: '' },
        }),
      ),
    ).toBeNull();
  });
});

// ---- configDriftSignal (private) via fleetGovernanceSignalsForConnection ----
//
// An explicit fleet.configDrift overrides the derived path, so passing
// { status } with no reason exercises every switch case of configDriftSignal
// AND the `state.reason || <fallback>` falsy arm for each.

describe('configDriftSignal (via fleetGovernanceSignalsForConnection)', () => {
  it.each([
    ['current', 'Config current', 'Desired and applied configuration fingerprints match.', 'ok'],
    [
      'drifted',
      'Config drift',
      'Desired and applied configuration fingerprints do not match.',
      'warning',
    ],
    [
      'pending',
      'Config pending',
      'Pulse is waiting for applied configuration confirmation.',
      'warning',
    ],
    ['paused', 'Config paused', 'Configuration rollout is paused for this source.', 'muted'],
    [
      'unknown',
      'Config unknown',
      'Pulse does not yet have enough config fingerprint data.',
      'warning',
    ],
    [
      'not-applicable',
      'No config drift',
      'This source is not governed by desired/applied config rollout.',
      'muted',
    ],
  ] as const satisfies ReadonlyArray<
    [ConnectionFleetConfigDriftStatus, string, string, FleetGovernanceSignalTone]
  >)(
    'uses the fallback detail when configDrift %s carries no reason',
    (status, label, detail, tone) => {
      const signal = signalByKey(
        connectionFixture({ fleet: fleetFixture({ configDrift: { status } }) }),
        'config-drift',
      );
      expect(signal).toMatchObject({ key: 'config-drift', label, detail, tone });
    },
  );

  it('honours an explicit reason over the fallback copy (pending case)', () => {
    const signal = signalByKey(
      connectionFixture({
        fleet: fleetFixture({ configDrift: { status: 'pending', reason: 'awaiting agent ack' } }),
      }),
      'config-drift',
    );
    expect(signal?.detail).toBe('awaiting agent ack');
    expect(signal?.tone).toBe('warning');
  });
});

// ---- rolloutSignal (private) ------------------------------------------------

describe('rolloutSignal (via fleetGovernanceSignalsForConnection)', () => {
  it.each([
    ['current', 'Rollout current', 'The rollout state is current.', 'ok'],
    ['pending', 'Rollout pending', 'The staged rollout is waiting for confirmation.', 'warning'],
    ['paused', 'Rollout paused', 'The staged rollout is paused.', 'warning'],
    [
      'blocked',
      'Rollout blocked',
      'The rollout is blocked by the current connection state.',
      'critical',
    ],
    ['unknown', 'Rollout unknown', 'Pulse has not classified staged rollout state yet.', 'warning'],
    ['not-applicable', 'No rollout', 'This source does not use staged rollout control.', 'muted'],
  ] as const satisfies ReadonlyArray<
    [ConnectionFleetRolloutStatus, string, string, FleetGovernanceSignalTone]
  >)(
    'uses the fallback detail when rollout %s carries no reason',
    (status, label, detail, tone) => {
      const signal = signalByKey(
        connectionFixture({ fleet: fleetFixture({ rollout: { status } }) }),
        'rollout',
      );
      expect(signal).toMatchObject({ key: 'rollout', label, detail, tone });
    },
  );

  it('honours an explicit reason over the fallback copy (pending case)', () => {
    const signal = signalByKey(
      connectionFixture({
        fleet: fleetFixture({ rollout: { status: 'pending', reason: 'canary batch paused' } }),
      }),
      'rollout',
    );
    expect(signal?.detail).toBe('canary batch paused');
    expect(signal?.tone).toBe('warning');
  });
});

// ---- credentialHealthSignal (private) ---------------------------------------
//
// credentialHealthSignal emits fixed detail copy (no reason fallback). The
// sibling coverage test already covers verified/invalid/expired/paused/unknown;
// these cover the two remaining statuses.

describe('credentialHealthSignal (via fleetGovernanceSignalsForConnection)', () => {
  it.each([
    [
      'expiring',
      'Credentials expiring',
      'The credential is approaching its configured expiration.',
      'warning',
    ],
    [
      'not-applicable',
      'No credentials',
      'This source does not use a stored credential path.',
      'muted',
    ],
  ] as const satisfies ReadonlyArray<
    [ConnectionFleetCredentialHealthStatus, string, string, FleetGovernanceSignalTone]
  >)('maps credential-health status %s to its fixed signal', (status, label, detail, tone) => {
    const signal = signalByKey(
      connectionFixture({ fleet: fleetFixture({ credentialHealth: { status } }) }),
      'credential-health',
    );
    expect(signal).toMatchObject({ key: 'credential-health', label, detail, tone });
  });
});
