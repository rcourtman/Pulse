import { describe, expect, it } from 'vitest';
import type { APITokenRecord } from '@/api/security';
import type { Resource } from '@/types/resource';
import { DOCKER_REPORT_SCOPE } from '@/constants/apiScopes';
import {
  API_TOKEN_PULSE_INTELLIGENCE_AGENT_PRESET_ID,
  agentActionIdForResource,
  buildAgentTokenUsage,
  buildDockerTokenUsage,
  dockerActionIdForResource,
  getAPITokenDialogName,
  getAPITokenHint,
  getAPITokenScopePresets,
  hasAgentScopeResource,
  matchesScopePreset,
  tokenRevokedAtForResource,
} from '../apiTokenManagerModel';

// ---- Fixtures ---------------------------------------------------------------
// Mirrors the sibling APITokenManager.test.tsx fixture builders so the private
// helpers under test (readPlatformData / readNestedPlatformField /
// readPlatformNumber / normalizePresetScopes / appendUsageEntry) are driven
// through the single exported orchestrators that compose them.

const makeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'Resource One',
  displayName: 'Resource One',
  platformId: 'agent-1',
  platformType: 'agent',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  tags: [],
  ...overrides,
});

const makeToken = (overrides: Partial<APITokenRecord> = {}): APITokenRecord => ({
  id: 'token-1',
  name: 'Runtime token',
  prefix: 'pulse',
  suffix: '1234',
  createdAt: '2026-03-12T10:00:00.000Z',
  lastUsedAt: '2026-03-12T11:00:00.000Z',
  scopes: [DOCKER_REPORT_SCOPE],
  ...overrides,
});

// ---- readPlatformNumber (private) via tokenRevokedAtForResource -------------
//
// readPlatformNumber is module-private and only reachable through
// tokenRevokedAtForResource. Placing `tokenRevokedAt` at the TOP level of
// platformData makes readNestedPlatformField return it verbatim (the
// `field in platformData` arm), so readPlatformNumber receives the raw value
// and every typeof/Number.isFinite arm is exercised.

describe('readPlatformNumber (via tokenRevokedAtForResource)', () => {
  it('returns a finite number verbatim', () => {
    expect(
      tokenRevokedAtForResource(
        makeResource({ platformData: { tokenRevokedAt: 1_700_000_000_000 } }),
      ),
    ).toBe(1_700_000_000_000);
  });

  it('returns 0 for a finite zero (not treated as missing)', () => {
    expect(tokenRevokedAtForResource(makeResource({ platformData: { tokenRevokedAt: 0 } }))).toBe(
      0,
    );
  });

  it('returns a negative finite number verbatim', () => {
    expect(tokenRevokedAtForResource(makeResource({ platformData: { tokenRevokedAt: -42 } }))).toBe(
      -42,
    );
  });

  it.each([Number.NaN, Number.POSITIVE_INFINITY, Number.NEGATIVE_INFINITY])(
    'returns undefined for a number that is not finite (%s)',
    (value) => {
      expect(
        tokenRevokedAtForResource(makeResource({ platformData: { tokenRevokedAt: value } })),
      ).toBeUndefined();
    },
  );

  it.each(['1700000000000', '', [], { ms: 1 }, true, null])(
    'returns undefined for a non-number value (%s)',
    (value) => {
      expect(
        tokenRevokedAtForResource(
          makeResource({ platformData: { tokenRevokedAt: value } as Record<string, unknown> }),
        ),
      ).toBeUndefined();
    },
  );
});

// ---- readPlatformData (private) via tokenRevokedAtForResource ---------------
//
// readPlatformData is module-private; its two arms (platformData absent ->
// undefined; present -> unwrap-and-return) are both hit through
// tokenRevokedAtForResource.

describe('readPlatformData (via tokenRevokedAtForResource)', () => {
  it('returns undefined when platformData is absent (no platformData key)', () => {
    const resource = makeResource();
    expect(resource.platformData).toBeUndefined();
    expect(tokenRevokedAtForResource(resource)).toBeUndefined();
  });

  it('unwraps a populated platformData record so nested fields are reachable', () => {
    expect(
      tokenRevokedAtForResource(
        makeResource({ platformData: { tokenRevokedAt: 555, unrelated: true } }),
      ),
    ).toBe(555);
  });
});

// ---- readNestedPlatformField (private) via tokenRevokedAtForResource --------
//
// readNestedPlatformField is module-private; every arm is driven via
// tokenRevokedAtForResource by relocating `tokenRevokedAt` through the
// platformData graph (top-level -> agent -> docker -> absent).

describe('readNestedPlatformField (via tokenRevokedAtForResource)', () => {
  it('returns the field directly from platformData when present at the top level', () => {
    expect(tokenRevokedAtForResource(makeResource({ platformData: { tokenRevokedAt: 111 } }))).toBe(
      111,
    );
  });

  it('falls through to platformData.agent.<field> when not at the top level', () => {
    expect(
      tokenRevokedAtForResource(makeResource({ platformData: { agent: { tokenRevokedAt: 222 } } })),
    ).toBe(222);
  });

  it('falls through to platformData.docker.<field> when neither top-level nor agent has it', () => {
    expect(
      tokenRevokedAtForResource(
        makeResource({ platformData: { docker: { tokenRevokedAt: 333 } } }),
      ),
    ).toBe(333);
  });

  it('skips a non-object agent value and still resolves from docker', () => {
    // agent is a truthy string, so `typeof agent === 'object'` is false and the
    // agent branch is skipped; docker carries the field.
    expect(
      tokenRevokedAtForResource(
        makeResource({
          platformData: { agent: 'not-an-object', docker: { tokenRevokedAt: 444 } },
        }),
      ),
    ).toBe(444);
  });

  it('returns undefined when agent is an object without the field and docker is absent', () => {
    expect(
      tokenRevokedAtForResource(makeResource({ platformData: { agent: { unrelated: true } } })),
    ).toBeUndefined();
  });

  it('returns undefined when docker is a non-object value (object guard skips it)', () => {
    expect(
      tokenRevokedAtForResource(makeResource({ platformData: { docker: 'oops' } })),
    ).toBeUndefined();
  });

  it('returns undefined when the field is nowhere in the platformData graph', () => {
    expect(
      tokenRevokedAtForResource(
        makeResource({
          platformData: { agent: { other: 1 }, docker: { other: 2 } },
        }),
      ),
    ).toBeUndefined();
  });

  it('returns undefined when platformData itself is undefined', () => {
    expect(tokenRevokedAtForResource(makeResource({ platformData: undefined }))).toBeUndefined();
  });
});

// ---- tokenRevokedAtForResource (exported composition) ----------------------

describe('tokenRevokedAtForResource', () => {
  it('composes the three private readers to surface a revokedAt timestamp', () => {
    expect(
      tokenRevokedAtForResource(
        makeResource({
          platformData: { docker: { tokenId: 'tok', tokenRevokedAt: 9_999 } },
        }),
      ),
    ).toBe(9_999);
  });
});

// ---- hasAgentScopeResource --------------------------------------------------

describe('hasAgentScopeResource', () => {
  it('returns false for a docker-host even when it carries an agent facet (early return)', () => {
    const dockerHostWithAgent = makeResource({
      type: 'docker-host',
      platformType: 'docker',
      agent: { agentId: 'would-otherwise-match' },
    });
    expect(hasAgentScopeResource(dockerHostWithAgent)).toBe(false);
  });

  it.each(['agent', 'pbs', 'pmg'] as const)(
    'returns true for the canonical agent-scoped type %s',
    (type) => {
      expect(hasAgentScopeResource(makeResource({ type }))).toBe(true);
    },
  );

  it('returns true for a non-canonical type that carries an agent facet', () => {
    // 'vm' is not agent/pbs/pmg/docker-host, but resourceHasAgentFacet is true
    // because resource.agent is set.
    expect(
      hasAgentScopeResource(
        makeResource({ type: 'vm', platformType: 'proxmox-pve', agent: { agentId: 'vm-agent' } }),
      ),
    ).toBe(true);
  });

  it('returns false for a non-canonical type with no agent facet', () => {
    expect(
      hasAgentScopeResource(
        makeResource({ type: 'storage', platformType: 'proxmox-pve', agent: undefined }),
      ),
    ).toBe(false);
  });
});

// ---- getAPITokenHint --------------------------------------------------------

describe('getAPITokenHint', () => {
  it('returns the em-dash placeholder for null', () => {
    expect(getAPITokenHint(null)).toBe('—');
  });

  it('returns the em-dash placeholder for undefined', () => {
    expect(getAPITokenHint(undefined)).toBe('—');
  });

  it('combines prefix and suffix with an ellipsis when both are present', () => {
    expect(getAPITokenHint(makeToken({ prefix: 'pulse', suffix: '4321' }))).toBe('pulse…4321');
  });

  it('uses only the prefix when the suffix is blank', () => {
    expect(getAPITokenHint(makeToken({ prefix: 'pulse', suffix: '' }))).toBe('pulse…');
  });

  it('falls back to the em-dash when only a suffix is present (prefix is falsy)', () => {
    expect(getAPITokenHint(makeToken({ prefix: '', suffix: '9999' }))).toBe('—');
  });

  it('falls back to the em-dash when neither prefix nor suffix is present', () => {
    expect(getAPITokenHint(makeToken({ prefix: '', suffix: '' }))).toBe('—');
  });
});

// ---- getAPITokenDialogName --------------------------------------------------

describe('getAPITokenDialogName', () => {
  it('returns the trimmed name when the name has non-whitespace content', () => {
    expect(getAPITokenDialogName(makeToken({ name: '  Container automation  ' }))).toBe(
      'Container automation',
    );
  });

  it('falls through when the name is only whitespace (trim() is empty -> falsy)', () => {
    expect(getAPITokenDialogName(makeToken({ name: '   ', prefix: 'pulse', suffix: '1234' }))).toBe(
      'pulse…1234',
    );
  });

  it('exercises the optional-chain arm when name is undefined at runtime', () => {
    expect(
      getAPITokenDialogName(
        makeToken({ name: undefined as unknown as string, prefix: 'pulse', suffix: '' }),
      ),
    ).toBe('pulse…');
  });

  it('combines prefix and suffix when the name is blank and both are set', () => {
    expect(getAPITokenDialogName(makeToken({ name: '', prefix: 'pl', suffix: 'xy' }))).toBe(
      'pl…xy',
    );
  });

  it('returns "untitled token" when name, prefix, and suffix are all blank', () => {
    expect(getAPITokenDialogName(makeToken({ name: '', prefix: '', suffix: '' }))).toBe(
      'untitled token',
    );
  });
});

// ---- normalizePresetScopes (private) via getAPITokenScopePresets -----------
//
// normalizePresetScopes is module-private; its only caller is
// getAPITokenScopePresets, which feeds the normalized result to the Pulse
// Intelligence preset (and gates that preset on length > 0). Asserting that
// preset's `scopes` and presence exercises every normalizePresetScopes branch:
// trim(), filter(Boolean), Set dedup, and the scopes ?? [] fallback.

describe('normalizePresetScopes (via getAPITokenScopePresets)', () => {
  const PULSE_PRESET_ID = API_TOKEN_PULSE_INTELLIGENCE_AGENT_PRESET_ID;

  it('adds no Pulse Intelligence preset when the required-scopes arg defaults to []', () => {
    const presets = getAPITokenScopePresets();
    expect(presets.find((preset) => preset.id === PULSE_PRESET_ID)).toBeUndefined();
    expect(presets).toHaveLength(7);
  });

  it('adds no Pulse Intelligence preset when every provided scope is blank/whitespace', () => {
    const presets = getAPITokenScopePresets(['', '   ', '\t']);
    expect(presets.find((preset) => preset.id === PULSE_PRESET_ID)).toBeUndefined();
    expect(presets).toHaveLength(7);
  });

  it('trims, deduplicates, and drops empties, preserving first-seen order', () => {
    const presets = getAPITokenScopePresets(['  b  ', 'b', '', 'a']);
    const pulsePreset = presets.find((preset) => preset.id === PULSE_PRESET_ID);
    expect(pulsePreset?.scopes).toStrictEqual(['b', 'a']);
    expect(presets).toHaveLength(8);
  });

  it('keeps a single non-empty scope after normalization', () => {
    const presets = getAPITokenScopePresets(['  monitoring:read  ']);
    expect(presets.find((preset) => preset.id === PULSE_PRESET_ID)?.scopes).toStrictEqual([
      'monitoring:read',
    ]);
  });
});

// ---- agentActionIdForResource -----------------------------------------------

describe('agentActionIdForResource', () => {
  it('returns the actionable agent id when an explicit agent id is resolvable', () => {
    expect(
      agentActionIdForResource(
        makeResource({
          id: 'fallback-id',
          platformData: { agent: { agentId: 'agent-007' } },
        }),
      ),
    ).toBe('agent-007');
  });

  it('falls back to resource.id when no agent id is resolvable anywhere', () => {
    expect(
      agentActionIdForResource(
        makeResource({ id: 'fallback-id', type: 'vm', platformType: 'proxmox-pve' }),
      ),
    ).toBe('fallback-id');
  });
});

// ---- dockerActionIdForResource ----------------------------------------------

describe('dockerActionIdForResource', () => {
  it('returns the docker runtime id when hostSourceId is resolvable', () => {
    expect(
      dockerActionIdForResource(
        makeResource({
          id: 'fallback-id',
          type: 'docker-host',
          platformType: 'docker',
          platformData: { docker: { hostSourceId: 'docker-runtime-1' } },
        }),
      ),
    ).toBe('docker-runtime-1');
  });

  it('falls back to resource.id when no docker runtime id is resolvable', () => {
    expect(
      dockerActionIdForResource(
        makeResource({ id: 'fallback-id', platformData: { agent: { agentId: 'a1' } } }),
      ),
    ).toBe('fallback-id');
  });
});

// ---- appendUsageEntry (private) via buildDockerTokenUsage / buildAgentTokenUsage
//
// appendUsageEntry is module-private; its three arms (first entry / duplicate
// item-id no-op / new item-id append) are driven through the two exported
// usage builders, which also exercise their own skip/iterate branches.

describe('appendUsageEntry (via buildDockerTokenUsage)', () => {
  it('returns an empty map for an empty resource list', () => {
    expect(buildDockerTokenUsage([])).toStrictEqual(new Map());
  });

  it('skips resources that have no tokenId', () => {
    const withoutToken = makeResource({
      id: 'no-token',
      type: 'docker-host',
      platformType: 'docker',
      displayName: 'No Token',
      platformData: { docker: { hostSourceId: 'rt-9' } },
    });
    expect(buildDockerTokenUsage([withoutToken])).toStrictEqual(new Map());
  });

  it('creates a count=1 entry for the first resource with a given tokenId', () => {
    const resource = makeResource({
      id: 'd-1',
      type: 'docker-host',
      platformType: 'docker',
      displayName: 'Docker One',
      platformData: { docker: { hostSourceId: 'rt-1', tokenId: 't1' } },
    });
    expect(buildDockerTokenUsage([resource])).toStrictEqual(
      new Map([['t1', { count: 1, items: [{ id: 'rt-1', label: 'Docker One' }] }]]),
    );
  });

  it('appends a distinct item and increments the count for a shared tokenId', () => {
    const a = makeResource({
      id: 'd-1',
      type: 'docker-host',
      platformType: 'docker',
      displayName: 'Docker One',
      platformData: { docker: { hostSourceId: 'rt-1', tokenId: 't1' } },
    });
    const b = makeResource({
      id: 'd-2',
      type: 'docker-host',
      platformType: 'docker',
      displayName: 'Docker Two',
      platformData: { docker: { hostSourceId: 'rt-2', tokenId: 't1' } },
    });
    expect(buildDockerTokenUsage([a, b])).toStrictEqual(
      new Map([
        [
          't1',
          {
            count: 2,
            items: [
              { id: 'rt-1', label: 'Docker One' },
              { id: 'rt-2', label: 'Docker Two' },
            ],
          },
        ],
      ]),
    );
  });

  it('no-ops when a second resource maps to an already-recorded item id (dedup)', () => {
    // Both resources resolve to dockerActionId 'rt-shared' (same hostSourceId),
    // so appendUsageEntry's duplicate-item guard keeps count at 1 and preserves
    // the first-seen label.
    const a = makeResource({
      id: 'd-1',
      type: 'docker-host',
      platformType: 'docker',
      displayName: 'Docker One',
      platformData: { docker: { hostSourceId: 'rt-shared', tokenId: 't1' } },
    });
    const b = makeResource({
      id: 'd-2',
      type: 'docker-host',
      platformType: 'docker',
      displayName: 'Docker Two',
      platformData: { docker: { hostSourceId: 'rt-shared', tokenId: 't1' } },
    });
    expect(buildDockerTokenUsage([a, b])).toStrictEqual(
      new Map([['t1', { count: 1, items: [{ id: 'rt-shared', label: 'Docker One' }] }]]),
    );
  });
});

describe('appendUsageEntry (via buildAgentTokenUsage)', () => {
  it('creates a count=1 entry for the first agent resource with a tokenId', () => {
    const resource = makeResource({
      id: 'a-1',
      type: 'agent',
      platformType: 'agent',
      displayName: 'Agent One',
      platformData: { agent: { agentId: 'a1', tokenId: 't1' } },
    });
    expect(buildAgentTokenUsage([resource])).toStrictEqual(
      new Map([['t1', { count: 1, items: [{ id: 'a1', label: 'Agent One' }] }]]),
    );
  });

  it('appends a distinct agent item and increments the count', () => {
    const a = makeResource({
      id: 'a-1',
      type: 'agent',
      platformType: 'agent',
      displayName: 'Agent One',
      platformData: { agent: { agentId: 'a1', tokenId: 't1' } },
    });
    const b = makeResource({
      id: 'a-2',
      type: 'agent',
      platformType: 'agent',
      displayName: 'Agent Two',
      platformData: { agent: { agentId: 'a2', tokenId: 't1' } },
    });
    expect(buildAgentTokenUsage([a, b])).toStrictEqual(
      new Map([
        [
          't1',
          {
            count: 2,
            items: [
              { id: 'a1', label: 'Agent One' },
              { id: 'a2', label: 'Agent Two' },
            ],
          },
        ],
      ]),
    );
  });

  it('dedups agent resources that resolve to the same actionable agent id', () => {
    const a = makeResource({
      id: 'a-1',
      type: 'agent',
      platformType: 'agent',
      displayName: 'Agent One',
      platformData: { agent: { agentId: 'shared', tokenId: 't1' } },
    });
    const b = makeResource({
      id: 'a-2',
      type: 'agent',
      platformType: 'agent',
      displayName: 'Agent Two',
      platformData: { agent: { agentId: 'shared', tokenId: 't1' } },
    });
    expect(buildAgentTokenUsage([a, b])).toStrictEqual(
      new Map([['t1', { count: 1, items: [{ id: 'shared', label: 'Agent One' }] }]]),
    );
  });

  it('skips agent resources that have no tokenId', () => {
    const withoutToken = makeResource({
      id: 'a-1',
      type: 'agent',
      platformType: 'agent',
      displayName: 'Agent One',
      platformData: { agent: { agentId: 'a1' } },
    });
    expect(buildAgentTokenUsage([withoutToken])).toStrictEqual(new Map());
  });
});

// ---- matchesScopePreset -----------------------------------------------------

describe('matchesScopePreset', () => {
  it('matches regardless of input order for an exact non-empty set', () => {
    expect(matchesScopePreset(['b', 'a'], ['a', 'b'])).toBe(true);
  });

  it('returns false when the selection is a strict subset of the preset', () => {
    expect(matchesScopePreset(['a'], ['a', 'b'])).toBe(false);
  });

  it('returns false when the selection is a strict superset of the preset', () => {
    expect(matchesScopePreset(['a', 'b', 'c'], ['a', 'b'])).toBe(false);
  });

  it('returns false for a disjoint set of the same length', () => {
    expect(matchesScopePreset(['a'], ['b'])).toBe(false);
  });

  it('returns true for an empty preset against an empty selection', () => {
    expect(matchesScopePreset([], [])).toBe(true);
  });

  it('returns true for an empty preset when the selection is just the wildcard', () => {
    expect(matchesScopePreset(['*'], [])).toBe(true);
  });

  it('returns false for an empty preset when the selection has a non-wildcard scope', () => {
    expect(matchesScopePreset(['monitoring:read'], [])).toBe(false);
  });

  it('ignores the wildcard when comparing against a non-empty preset that otherwise matches', () => {
    // The '*' is filtered out before the length/equality check, so ['a','*']
    // matches preset ['a'].
    expect(matchesScopePreset(['a', '*'], ['a'])).toBe(true);
  });

  it('still returns false when a wildcard is present but the remaining scope does not match', () => {
    expect(matchesScopePreset(['b', '*'], ['a'])).toBe(false);
  });
});
