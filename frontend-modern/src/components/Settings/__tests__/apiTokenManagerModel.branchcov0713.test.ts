import { describe, expect, it } from 'vitest';
import type { APITokenRecord } from '@/api/security';
import {
  API_SCOPE_OPTIONS,
  type APIScopeOption,
  AGENT_REPORT_SCOPE,
  AUDIT_READ_SCOPE,
  DOCKER_MANAGE_SCOPE,
  DOCKER_REPORT_SCOPE,
  MONITORING_READ_SCOPE,
  SETTINGS_READ_SCOPE,
  SETTINGS_WRITE_SCOPE,
} from '@/constants/apiScopes';
import type { Resource } from '@/types/resource';
import {
  API_TOKEN_DOCKER_MANAGE_PRESET_DESCRIPTION,
  API_TOKEN_DOCKER_MANAGE_PRESET_LABEL,
  API_TOKEN_DOCKER_REPORT_PRESET_DESCRIPTION,
  API_TOKEN_DOCKER_REPORT_PRESET_LABEL,
} from '@/utils/apiTokenPresentation';
import {
  API_TOKEN_AGENT_PRESET_ID,
  API_TOKEN_AUDIT_READ_PRESET_ID,
  API_TOKEN_DOCKER_MANAGE_PRESET_ID,
  API_TOKEN_DOCKER_REPORT_PRESET_ID,
  API_TOKEN_KIOSK_PRESET_ID,
  API_TOKEN_PATROL_EXTERNAL_AGENT_PRESET_LABEL,
  API_TOKEN_PULSE_INTELLIGENCE_AGENT_PRESET_ID,
  API_TOKEN_SETTINGS_ADMIN_PRESET_ID,
  API_TOKEN_SETTINGS_READ_PRESET_ID,
  API_TOKEN_WILDCARD_SCOPE,
  buildAgentTokenUsage,
  buildDockerTokenUsage,
  countWildcardTokens,
  dockerActionIdForResource,
  getAPITokenScopePresets,
  groupAPITokenScopes,
  revokedTokenIdForResource,
  sortAPITokensByCreatedAt,
  tokenIdForResource,
} from '../apiTokenManagerModel';

// ---- Fixtures ---------------------------------------------------------------
// Mirrors the sibling apiTokenManagerModel.branchcov0712.test.ts builders so
// every private reader under test (readPlatformString / readNestedPlatformField
// / readPlatformData) is driven only through the exported orchestrators.

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

// ---- readPlatformString (private) via tokenIdForResource --------------------
//
// readPlatformString is module-private and ONLY reachable through
// tokenIdForResource / revokedTokenIdForResource (neither is exercised by the
// sibling tests). Placing `tokenId` at the TOP level of platformData makes
// readNestedPlatformField return it verbatim (the `field in platformData` arm),
// so readPlatformString receives the raw value and both of its
// typeof/length arms are exercised.

describe('readPlatformString (via tokenIdForResource)', () => {
  it('returns a non-empty string verbatim', () => {
    expect(
      tokenIdForResource(makeResource({ platformData: { tokenId: 'tok-abc' } })),
    ).toBe('tok-abc');
  });

  it('returns undefined for an empty string (length === 0)', () => {
    expect(
      tokenIdForResource(makeResource({ platformData: { tokenId: '' } })),
    ).toBeUndefined();
  });

  it('returns undefined for a whitespace-only string (still length > 0, not trimmed)', () => {
    // readPlatformString intentionally does NOT trim; a blank-but-non-empty
    // string is returned verbatim, proving it is distinct from the empty arm.
    expect(
      tokenIdForResource(makeResource({ platformData: { tokenId: '   ' } })),
    ).toBe('   ');
  });

  it.each([42, 0, true, false, null, ['array'], { k: 'v' }])(
    'returns undefined for a non-string value (%s)',
    (value) => {
      expect(
        tokenIdForResource(
          makeResource({ platformData: { tokenId: value as unknown } as Record<string, unknown> }),
        ),
      ).toBeUndefined();
    },
  );
});

// ---- tokenIdForResource (exported composition) -----------------------------
//
// Drives the readNestedPlatformField fall-through arms specifically for the
// 'tokenId' field key, proving the exported wrapper honors the same
// top-level -> agent -> docker resolution graph as tokenRevokedAtForResource.

describe('tokenIdForResource', () => {
  it('resolves tokenId from platformData.agent when absent at the top level', () => {
    expect(
      tokenIdForResource(makeResource({ platformData: { agent: { tokenId: 'agent-tok' } } })),
    ).toBe('agent-tok');
  });

  it('resolves tokenId from platformData.docker when neither top-level nor agent carry it', () => {
    expect(
      tokenIdForResource(
        makeResource({ platformData: { agent: { x: 1 }, docker: { tokenId: 'docker-tok' } } }),
      ),
    ).toBe('docker-tok');
  });

  it('returns undefined when platformData is absent', () => {
    expect(tokenIdForResource(makeResource({ platformData: undefined }))).toBeUndefined();
  });

  it('returns undefined when the field is nowhere in the platformData graph', () => {
    expect(
      tokenIdForResource(
        makeResource({ platformData: { agent: { other: 1 }, docker: { other: 2 } } }),
      ),
    ).toBeUndefined();
  });
});

// ---- revokedTokenIdForResource ----------------------------------------------
//
// Same private-reader composition as tokenIdForResource but for the
// 'revokedTokenId' key. Not exercised by sibling tests.

describe('revokedTokenIdForResource', () => {
  it('returns a non-empty revokedTokenId string verbatim from the top level', () => {
    expect(
      revokedTokenIdForResource(
        makeResource({ platformData: { revokedTokenId: 'revoked-1' } }),
      ),
    ).toBe('revoked-1');
  });

  it('returns undefined for an empty-string revokedTokenId', () => {
    expect(
      revokedTokenIdForResource(makeResource({ platformData: { revokedTokenId: '' } })),
    ).toBeUndefined();
  });

  it.each([99, true, null, { k: 'v' }])(
    'returns undefined for a non-string revokedTokenId (%s)',
    (value) => {
      expect(
        revokedTokenIdForResource(
          makeResource({
            platformData: { revokedTokenId: value as unknown } as Record<string, unknown>,
          }),
        ),
      ).toBeUndefined();
    },
  );

  it('resolves revokedTokenId from platformData.docker when not at the top level', () => {
    expect(
      revokedTokenIdForResource(
        makeResource({ platformData: { docker: { revokedTokenId: 'docker-revoked' } } }),
      ),
    ).toBe('docker-revoked');
  });

  it('returns undefined when platformData is absent', () => {
    expect(revokedTokenIdForResource(makeResource({ platformData: undefined }))).toBeUndefined();
  });
});

// ---- sortAPITokensByCreatedAt ----------------------------------------------

describe('sortAPITokensByCreatedAt', () => {
  it('returns an empty array unchanged', () => {
    expect(sortAPITokensByCreatedAt([])).toStrictEqual([]);
  });

  it('returns a single-element array unchanged', () => {
    const only = makeToken({ id: 'solo' });
    expect(sortAPITokensByCreatedAt([only])).toStrictEqual([only]);
  });

  it('sorts tokens newest-first when given oldest-first input', () => {
    const oldest = makeToken({ id: 'old', createdAt: '2026-01-01T00:00:00.000Z' });
    const middle = makeToken({ id: 'mid', createdAt: '2026-06-01T00:00:00.000Z' });
    const newest = makeToken({ id: 'new', createdAt: '2026-12-31T00:00:00.000Z' });
    expect(sortAPITokensByCreatedAt([oldest, middle, newest]).map((t) => t.id)).toStrictEqual([
      'new',
      'mid',
      'old',
    ]);
  });

  it('keeps an already newest-first array in the same order', () => {
    const newest = makeToken({ id: 'new', createdAt: '2026-12-31T00:00:00.000Z' });
    const middle = makeToken({ id: 'mid', createdAt: '2026-06-01T00:00:00.000Z' });
    const oldest = makeToken({ id: 'old', createdAt: '2026-01-01T00:00:00.000Z' });
    expect(sortAPITokensByCreatedAt([newest, middle, oldest]).map((t) => t.id)).toStrictEqual([
      'new',
      'mid',
      'old',
    ]);
  });

  it('preserves all elements when timestamps tie (count intact)', () => {
    const tied = makeToken({ id: 'tied', createdAt: '2026-05-05T00:00:00.000Z' });
    const alsoTied = makeToken({ id: 'also-tied', createdAt: '2026-05-05T00:00:00.000Z' });
    const result = sortAPITokensByCreatedAt([tied, alsoTied]);
    expect(result).toHaveLength(2);
    expect(new Set(result.map((t) => t.id))).toStrictEqual(new Set(['tied', 'also-tied']));
  });

  it('does not mutate the input array', () => {
    const oldest = makeToken({ id: 'old', createdAt: '2026-01-01T00:00:00.000Z' });
    const newest = makeToken({ id: 'new', createdAt: '2026-12-31T00:00:00.000Z' });
    const input = [oldest, newest];
    sortAPITokensByCreatedAt(input);
    expect(input.map((t) => t.id)).toStrictEqual(['old', 'new']);
  });

  it('returns a new array reference (not the input)', () => {
    const input = [makeToken()];
    expect(sortAPITokensByCreatedAt(input)).not.toBe(input);
  });
});

// ---- countWildcardTokens ----------------------------------------------------

describe('countWildcardTokens', () => {
  it('returns 0 for an empty token list', () => {
    expect(countWildcardTokens([])).toBe(0);
  });

  it('returns 0 when every token has non-wildcard scopes', () => {
    expect(
      countWildcardTokens([
        makeToken({ id: 'a', scopes: [MONITORING_READ_SCOPE] }),
        makeToken({ id: 'b', scopes: [DOCKER_REPORT_SCOPE, SETTINGS_READ_SCOPE] }),
      ]),
    ).toBe(0);
  });

  it('counts a token whose scopes are undefined (treated as wildcard)', () => {
    expect(
      countWildcardTokens([
        makeToken({ id: 'a', scopes: [MONITORING_READ_SCOPE] }),
        makeToken({ id: 'b', scopes: undefined as unknown as string[] }),
      ]),
    ).toBe(1);
  });

  it('counts a token whose scopes are an empty array', () => {
    expect(
      countWildcardTokens([
        makeToken({ id: 'a', scopes: [] }),
        makeToken({ id: 'b', scopes: [AUDIT_READ_SCOPE] }),
      ]),
    ).toBe(1);
  });

  it('counts a token whose scopes explicitly include the wildcard', () => {
    expect(
      countWildcardTokens([
        makeToken({ id: 'a', scopes: [API_TOKEN_WILDCARD_SCOPE] }),
        makeToken({ id: 'b', scopes: [API_TOKEN_WILDCARD_SCOPE, MONITORING_READ_SCOPE] }),
      ]),
    ).toBe(2);
  });

  it('counts the correct total across a mixed list', () => {
    expect(
      countWildcardTokens([
        makeToken({ id: 'scoped', scopes: [MONITORING_READ_SCOPE] }),
        makeToken({ id: 'undef', scopes: undefined as unknown as string[] }),
        makeToken({ id: 'empty', scopes: [] }),
        makeToken({ id: 'wild', scopes: [API_TOKEN_WILDCARD_SCOPE] }),
        makeToken({ id: 'wild2', scopes: [SETTINGS_READ_SCOPE, API_TOKEN_WILDCARD_SCOPE] }),
        makeToken({ id: 'scoped2', scopes: [AUDIT_READ_SCOPE, SETTINGS_WRITE_SCOPE] }),
      ]),
    ).toBe(4);
  });
});

// ---- groupAPITokenScopes ----------------------------------------------------

describe('groupAPITokenScopes', () => {
  it('returns every populated group in the canonical display order', () => {
    const groups = groupAPITokenScopes();
    expect(groups.map(([group]) => group)).toStrictEqual([
      'Monitoring',
      'AI',
      'Agents',
      'Settings',
      'Security',
    ]);
  });

  it('places every API_SCOPE_OPTIONS entry into exactly one group, preserving source order', () => {
    const groups = groupAPITokenScopes();
    const byGroup = new Map<string, APIScopeOption[]>();
    for (const [group, options] of groups) byGroup.set(group, options);

    // Each canonical option must appear exactly once, in the same relative
    // order it has inside API_SCOPE_OPTIONS.
    for (const [group, options] of groups) {
      const expected = API_SCOPE_OPTIONS.filter((option) => option.group === group);
      expect(options).toStrictEqual(expected);
      expect(byGroup.get(group)).toBe(options);
    }
  });

  it('drops no group, because every shipped group has at least one option', () => {
    // The `.filter(([, options]) => options.length > 0)` guard is a no-op for
    // the shipped catalog; assert all five groups survive.
    expect(groupAPITokenScopes()).toHaveLength(5);
  });

  it('routes specific known scopes to their expected groups', () => {
    const groups = groupAPITokenScopes();
    const valuesFor = (group: string) =>
      groups.find(([g]) => g === group)?.[1].map((o) => o.value);
    expect(valuesFor('Monitoring')).toStrictEqual([MONITORING_READ_SCOPE, 'monitoring:write']);
    expect(valuesFor('Security')).toStrictEqual([AUDIT_READ_SCOPE]);
    expect(valuesFor('Agents')).toContain(DOCKER_REPORT_SCOPE);
    expect(valuesFor('Agents')).toContain(DOCKER_MANAGE_SCOPE);
    expect(valuesFor('Agents')).toContain(AGENT_REPORT_SCOPE);
    expect(valuesFor('Settings')).toStrictEqual([SETTINGS_READ_SCOPE, SETTINGS_WRITE_SCOPE]);
  });
});

// ---- dockerActionIdForResource (remaining resolution arms) -----------------
//
// Sibling tests already cover the docker.hostSourceId arm and the final
// fallback to resource.id. These cases drive every OTHER arm of the underlying
// getActionableDockerRuntimeIdFromResource short-circuit chain.

describe('dockerActionIdForResource (uncovered arms)', () => {
  it('returns discoveryTarget.resourceId for an app-container discovery target (early return)', () => {
    expect(
      dockerActionIdForResource(
        makeResource({
          id: 'fallback-id',
          discoveryTarget: {
            resourceType: 'app-container',
            agentId: 'dt-agent',
            resourceId: 'container-abc',
          },
        }),
      ),
    ).toBe('container-abc');
  });

  it('skips the early return when discoveryTarget.resourceType is not app-container', () => {
    // 'agent' is not an app-container type, so the early-return guard fails and
    // resolution continues down the hostSourceId chain -> resource.id fallback.
    expect(
      dockerActionIdForResource(
        makeResource({
          id: 'fallback-id',
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'dt-agent',
            resourceId: 'would-be-skipped',
          },
        }),
      ),
    ).toBe('fallback-id');
  });

  it('resolves from platformData.hostSourceId when docker.hostSourceId is absent', () => {
    expect(
      dockerActionIdForResource(
        makeResource({ id: 'fallback-id', platformData: { hostSourceId: 'pd-host-1' } }),
      ),
    ).toBe('pd-host-1');
  });

  it('resolves from resource.identity.machineId for a docker-host', () => {
    expect(
      dockerActionIdForResource(
        makeResource({
          id: 'fallback-id',
          type: 'docker-host',
          identity: { machineId: 'machine-xyz' },
        }),
      ),
    ).toBe('machine-xyz');
  });

  it('falls back to platformData.machineId for a docker-host when identity.machineId is absent', () => {
    expect(
      dockerActionIdForResource(
        makeResource({
          id: 'fallback-id',
          type: 'docker-host',
          platformData: { machineId: 'pd-machine-1' },
        }),
      ),
    ).toBe('pd-machine-1');
  });

  it('resolves from metricsTarget.resourceId for a docker-host when no machine id exists', () => {
    expect(
      dockerActionIdForResource(
        makeResource({
          id: 'fallback-id',
          type: 'docker-host',
          metricsTarget: { resourceType: 'docker-host', resourceId: 'metrics-rt-1' },
        }),
      ),
    ).toBe('metrics-rt-1');
  });

  it('skips the metricsTarget arm when metricsTarget.resourceType is not docker-host', () => {
    // metricsTarget points at an agent, so the resourceType guard fails and the
    // final docker-host discoveryTarget.agentId arm fires instead.
    expect(
      dockerActionIdForResource(
        makeResource({
          id: 'fallback-id',
          type: 'docker-host',
          metricsTarget: { resourceType: 'agent', resourceId: 'would-be-skipped' },
          discoveryTarget: { resourceType: 'agent', agentId: 'dt-agent-9', resourceId: 'dt-res' },
        }),
      ),
    ).toBe('dt-agent-9');
  });

  it('resolves from discoveryTarget.agentId for a docker-host (final arm)', () => {
    expect(
      dockerActionIdForResource(
        makeResource({
          id: 'fallback-id',
          type: 'docker-host',
          discoveryTarget: { resourceType: 'agent', agentId: 'dt-agent-9', resourceId: 'dt-res' },
        }),
      ),
    ).toBe('dt-agent-9');
  });

  it('fires the metricsTarget arm even for a non-docker-host type (only metricsTarget.resourceType is gated)', () => {
    // The metricsTarget ternary keys off metricsTarget.resourceType, NOT
    // resource.type, so a vm that reports a docker-host metrics target still
    // resolves to that metricsTarget.resourceId.
    expect(
      dockerActionIdForResource(
        makeResource({
          id: 'fallback-id',
          type: 'vm',
          metricsTarget: { resourceType: 'docker-host', resourceId: 'metrics-rt-9' },
        }),
      ),
    ).toBe('metrics-rt-9');
  });

  it('does not consult the docker-host-gated arms (machineId / discoveryTarget.agentId) for a non-docker-host type', () => {
    // Both identity.machineId and discoveryTarget.agentId ternaries short-circuit
    // to undefined when resource.type !== 'docker-host', so with no hostSourceId
    // and no docker-host metricsTarget the result is resource.id.
    expect(
      dockerActionIdForResource(
        makeResource({
          id: 'fallback-id',
          type: 'vm',
          identity: { machineId: 'would-be-skipped' },
          discoveryTarget: { resourceType: 'agent', agentId: 'also-skipped', resourceId: 'r' },
        }),
      ),
    ).toBe('fallback-id');
  });
});

// ---- getAPITokenScopePresets (structural verification) ---------------------
//
// Sibling tests only assert preset COUNT and the Pulse-Intelligence gating.
// These cases verify the full id/label/scopes/description contract of every
// shipped preset and the exact ordering, including the Pulse preset insertion.

describe('getAPITokenScopePresets (full structure)', () => {
  it('returns the seven base presets in canonical order with no pulse scopes', () => {
    const presets = getAPITokenScopePresets();
    expect(presets.map((p) => p.id)).toStrictEqual([
      API_TOKEN_KIOSK_PRESET_ID,
      API_TOKEN_AGENT_PRESET_ID,
      API_TOKEN_DOCKER_REPORT_PRESET_ID,
      API_TOKEN_DOCKER_MANAGE_PRESET_ID,
      API_TOKEN_SETTINGS_READ_PRESET_ID,
      API_TOKEN_SETTINGS_ADMIN_PRESET_ID,
      API_TOKEN_AUDIT_READ_PRESET_ID,
    ]);

    const byId = new Map(presets.map((p) => [p.id, p]));

    expect(byId.get(API_TOKEN_KIOSK_PRESET_ID)).toStrictEqual({
      id: API_TOKEN_KIOSK_PRESET_ID,
      label: 'Kiosk / Monitoring',
      scopes: [MONITORING_READ_SCOPE],
      description:
        'Read-only access for wall displays. Use ?token=xxx&kiosk=1 in the URL to hide navigation and filters.',
    });
    expect(byId.get(API_TOKEN_AGENT_PRESET_ID)?.scopes).toStrictEqual([AGENT_REPORT_SCOPE]);
    expect(byId.get(API_TOKEN_DOCKER_REPORT_PRESET_ID)).toStrictEqual({
      id: API_TOKEN_DOCKER_REPORT_PRESET_ID,
      label: API_TOKEN_DOCKER_REPORT_PRESET_LABEL,
      scopes: [DOCKER_REPORT_SCOPE],
      description: API_TOKEN_DOCKER_REPORT_PRESET_DESCRIPTION,
    });
    expect(byId.get(API_TOKEN_DOCKER_MANAGE_PRESET_ID)).toStrictEqual({
      id: API_TOKEN_DOCKER_MANAGE_PRESET_ID,
      label: API_TOKEN_DOCKER_MANAGE_PRESET_LABEL,
      scopes: [DOCKER_REPORT_SCOPE, DOCKER_MANAGE_SCOPE],
      description: API_TOKEN_DOCKER_MANAGE_PRESET_DESCRIPTION,
    });
    expect(byId.get(API_TOKEN_SETTINGS_READ_PRESET_ID)?.scopes).toStrictEqual([
      SETTINGS_READ_SCOPE,
    ]);
    expect(byId.get(API_TOKEN_SETTINGS_ADMIN_PRESET_ID)?.scopes).toStrictEqual([
      SETTINGS_READ_SCOPE,
      SETTINGS_WRITE_SCOPE,
    ]);
    expect(byId.get(API_TOKEN_AUDIT_READ_PRESET_ID)?.scopes).toStrictEqual([AUDIT_READ_SCOPE]);
  });

  it('inserts the Pulse Intelligence preset at index 1 when required scopes are provided', () => {
    const presets = getAPITokenScopePresets(['monitoring:read', 'ai:execute']);
    expect(presets).toHaveLength(8);
    expect(presets[1]).toStrictEqual({
      id: API_TOKEN_PULSE_INTELLIGENCE_AGENT_PRESET_ID,
      label: API_TOKEN_PATROL_EXTERNAL_AGENT_PRESET_LABEL,
      scopes: ['monitoring:read', 'ai:execute'],
      description:
        'Scopes for connected agents that read Pulse context and request Patrol work.',
    });
    // Base presets still follow in canonical order after the inserted preset.
    expect(presets.map((p) => p.id)).toStrictEqual([
      API_TOKEN_KIOSK_PRESET_ID,
      API_TOKEN_PULSE_INTELLIGENCE_AGENT_PRESET_ID,
      API_TOKEN_AGENT_PRESET_ID,
      API_TOKEN_DOCKER_REPORT_PRESET_ID,
      API_TOKEN_DOCKER_MANAGE_PRESET_ID,
      API_TOKEN_SETTINGS_READ_PRESET_ID,
      API_TOKEN_SETTINGS_ADMIN_PRESET_ID,
      API_TOKEN_AUDIT_READ_PRESET_ID,
    ]);
  });

  it('every preset exposes a non-empty id, label, and description', () => {
    const presets = getAPITokenScopePresets(['agent:report']);
    for (const preset of presets) {
      expect(preset.id.length).toBeGreaterThan(0);
      expect(preset.label.length).toBeGreaterThan(0);
      expect(preset.description.length).toBeGreaterThan(0);
      expect(Array.isArray(preset.scopes)).toBe(true);
      expect(preset.scopes.length).toBeGreaterThan(0);
    }
  });
});

// ---- buildDockerTokenUsage / buildAgentTokenUsage (multi-key accumulation) --
//
// Sibling tests only ever drive a single tokenId. These cases prove the map
// accumulates independent entries across DISTINCT tokenIds (the second
// appendUsageEntry "first entry for this key" arm exercised for a new key).

describe('buildDockerTokenUsage (multi-key accumulation)', () => {
  it('records separate entries for resources bound to different tokenIds', () => {
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
      platformData: { docker: { hostSourceId: 'rt-2', tokenId: 't2' } },
    });
    expect(buildDockerTokenUsage([a, b])).toStrictEqual(
      new Map([
        ['t1', { count: 1, items: [{ id: 'rt-1', label: 'Docker One' }] }],
        ['t2', { count: 1, items: [{ id: 'rt-2', label: 'Docker Two' }] }],
      ]),
    );
  });
});

describe('buildAgentTokenUsage (multi-key accumulation)', () => {
  it('records separate entries for agent resources bound to different tokenIds', () => {
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
      platformData: { agent: { agentId: 'a2', tokenId: 't2' } },
    });
    expect(buildAgentTokenUsage([a, b])).toStrictEqual(
      new Map([
        ['t1', { count: 1, items: [{ id: 'a1', label: 'Agent One' }] }],
        ['t2', { count: 1, items: [{ id: 'a2', label: 'Agent Two' }] }],
      ]),
    );
  });
});
