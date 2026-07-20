import { describe, expect, it } from 'vitest';

import type {
  ResourceRedactionHint,
  ResourceRoutingScope,
  ResourceSensitivity,
} from '@/types/resource';
import {
  getResourcePolicyDisplayLabel,
  getResourcePolicyGovernedSummary,
  getResourcePolicyTableBadges,
  getResourceRedactionHintLabel,
  getResourceRoutingScopeLabel,
  getResourceSensitivityLabel,
  hasDefaultResourcePolicyPosture,
  shouldShowResourceAlternateName,
} from '@/utils/resourcePolicyPresentation';
import type { ResourcePolicyDisplayResource } from '@/utils/resourcePolicyPresentation';

/**
 * Branch-coverage supplement for `resourcePolicyPresentation.ts`.
 *
 * The existing sibling test (`resourcePolicyPresentation.test.ts`) exercises the
 * happy paths. This file targets the defensive / fallback arms of each named
 * function: undefined inputs, non-blocking postures, empty/whitespace strings,
 * the `?? hint` redaction fallback, the `?? 0` redact-length arm, the
 * sensitivity-vs-routing primary selection, and the concise-summary status
 * absence / all-empty-parts / no-semicolon branches.
 *
 * NOTE: `getConciseGovernedDisplaySummary` is a module-private (non-exported)
 * helper, so it is covered indirectly through `getResourcePolicyDisplayLabel`,
 * which is the only call site — matching the existing test's approach.
 */

const governedPolicy = (
  overrides: {
    sensitivity?: ResourceSensitivity;
    scope?: ResourceRoutingScope;
    redact?: ResourceRedactionHint[];
  } = {},
): NonNullable<ResourcePolicyDisplayResource['policy']> => ({
  sensitivity: overrides.sensitivity ?? 'restricted',
  routing: {
    scope: overrides.scope ?? 'local-only',
    redact: overrides.redact,
  },
});

// The static type marks `name` and `displayName` as required, but the source
// functions defendively optional-chain (`?.trim()`) on both. This helper lets
// us hand in deliberately-partial resources to exercise those fallback arms.
const asResource = (r: Partial<ResourcePolicyDisplayResource>): ResourcePolicyDisplayResource =>
  r as unknown as ResourcePolicyDisplayResource;

describe('getResourceSensitivityLabel — branch coverage', () => {
  it('returns the canonical label for every known sensitivity', () => {
    expect(getResourceSensitivityLabel('public')).toBe('Public');
    expect(getResourceSensitivityLabel('internal')).toBe('Internal');
    expect(getResourceSensitivityLabel('sensitive')).toBe('Sensitive');
    expect(getResourceSensitivityLabel('restricted')).toBe('Restricted');
  });

  it('returns "Unclassified" when no sensitivity is supplied (falsy arm)', () => {
    expect(getResourceSensitivityLabel(undefined)).toBe('Unclassified');
  });
});

describe('getResourceRoutingScopeLabel — branch coverage', () => {
  it('returns the canonical label for every known routing scope', () => {
    expect(getResourceRoutingScopeLabel('cloud-summary')).toBe('Cloud Summary');
    expect(getResourceRoutingScopeLabel('local-first')).toBe('Local First');
    expect(getResourceRoutingScopeLabel('local-only')).toBe('Local Only');
  });

  it('returns "Unrouted" when no scope is supplied (falsy arm)', () => {
    expect(getResourceRoutingScopeLabel(undefined)).toBe('Unrouted');
  });
});

describe('getResourceRedactionHintLabel — branch coverage', () => {
  it('returns the canonical label for every known redaction hint', () => {
    expect(getResourceRedactionHintLabel('hostname')).toBe('Hostname');
    expect(getResourceRedactionHintLabel('ip-address')).toBe('IP Address');
    expect(getResourceRedactionHintLabel('platform-id')).toBe('Platform ID');
    expect(getResourceRedactionHintLabel('alias')).toBe('Alias');
    expect(getResourceRedactionHintLabel('path')).toBe('Path');
  });

  it('returns "Unclassified" when no hint is supplied (falsy arm)', () => {
    expect(getResourceRedactionHintLabel(undefined)).toBe('Unclassified');
  });

  it('falls back to the raw hint string when the hint is not in the label map (?? hint arm)', () => {
    expect(getResourceRedactionHintLabel('custom-redaction' as ResourceRedactionHint)).toBe(
      'custom-redaction',
    );
  });
});

describe('hasDefaultResourcePolicyPosture — branch coverage', () => {
  it('returns false when no policy is supplied (short-circuit before fields)', () => {
    expect(hasDefaultResourcePolicyPosture(undefined)).toBe(false);
  });

  it('returns false when sensitivity is not "internal"', () => {
    expect(
      hasDefaultResourcePolicyPosture({
        sensitivity: 'restricted',
        routing: { scope: 'cloud-summary' },
      }),
    ).toBe(false);
  });

  it('returns false when scope is not "cloud-summary" even with internal sensitivity', () => {
    expect(
      hasDefaultResourcePolicyPosture({
        sensitivity: 'internal',
        routing: { scope: 'local-first' },
      }),
    ).toBe(false);
  });

  it('returns false when redact is a non-empty array (exercises redact?.length with defined array)', () => {
    expect(
      hasDefaultResourcePolicyPosture({
        sensitivity: 'internal',
        routing: { scope: 'cloud-summary', redact: ['hostname'] },
      }),
    ).toBe(false);
  });

  it('returns true when redact is an empty array (defined but length 0)', () => {
    expect(
      hasDefaultResourcePolicyPosture({
        sensitivity: 'internal',
        routing: { scope: 'cloud-summary', redact: [] },
      }),
    ).toBe(true);
  });

  it('returns true for the canonical default posture (redact undefined hits ?? 0)', () => {
    expect(
      hasDefaultResourcePolicyPosture({
        sensitivity: 'internal',
        routing: { scope: 'cloud-summary' },
      }),
    ).toBe(true);
  });
});

describe('getResourcePolicyTableBadges — branch coverage', () => {
  it('returns an empty array when no policy is supplied', () => {
    expect(getResourcePolicyTableBadges(undefined)).toEqual([]);
  });

  it('returns an empty array for a non-blocking public posture', () => {
    expect(
      getResourcePolicyTableBadges({
        sensitivity: 'public',
        routing: { scope: 'cloud-summary' },
      }),
    ).toEqual([]);
  });

  it('uses the sensitivity badge as primary when scope is not local-only (restricted + cloud-summary)', () => {
    const badges = getResourcePolicyTableBadges({
      sensitivity: 'restricted',
      routing: { scope: 'cloud-summary' },
    });
    expect(badges).toHaveLength(1);
    expect(badges[0]?.label).toBe('Restricted');
    // No redactions -> redactionTitle is undefined and filtered out of the title.
    expect(badges[0]?.title).toBe(
      'Restricted: Resource data is tightly restricted and requires guarded handling. Cloud Summary: This resource may use cloud summarization within policy limits.',
    );
    expect(badges[0]?.title).not.toContain('Redacts');
  });

  it('uses the sensitivity badge as primary when restricted with local-first scope and redactions', () => {
    const badges = getResourcePolicyTableBadges({
      sensitivity: 'restricted',
      routing: { scope: 'local-first', redact: ['hostname', 'path'] },
    });
    expect(badges).toHaveLength(1);
    expect(badges[0]?.label).toBe('Restricted');
    expect(badges[0]?.title).toContain('Local First');
    expect(badges[0]?.title).toContain('Redacts Hostname, Path.');
  });

  it('uses the routing badge as primary when scope is local-only even with non-restricted sensitivity', () => {
    const badges = getResourcePolicyTableBadges({
      sensitivity: 'sensitive',
      routing: { scope: 'local-only', redact: ['alias'] },
    });
    expect(badges).toHaveLength(1);
    expect(badges[0]?.label).toBe('Local Only');
    expect(badges[0]?.title).toContain('Sensitive');
    expect(badges[0]?.title).toContain('Redacts Alias.');
  });
});

describe('getResourcePolicyGovernedSummary — branch coverage', () => {
  it('returns empty string when no resource is supplied', () => {
    expect(getResourcePolicyGovernedSummary(undefined)).toBe('');
    expect(getResourcePolicyGovernedSummary(null)).toBe('');
  });

  it('falls back to displayName/name when policy is absent', () => {
    expect(
      getResourcePolicyGovernedSummary({
        name: 'host-1',
        displayName: 'Host One',
      }),
    ).toBe('Host One');
  });

  it('falls back to name when displayName is absent and policy is not governed', () => {
    expect(
      getResourcePolicyGovernedSummary(
        asResource({
          name: 'host-1',
          policy: { sensitivity: 'internal', routing: { scope: 'cloud-summary' } },
        }),
      ),
    ).toBe('host-1');
  });

  it('returns empty string when neither displayName nor name is present and policy is not governed', () => {
    expect(
      getResourcePolicyGovernedSummary(
        asResource({
          name: '',
          displayName: '   ',
          policy: { sensitivity: 'internal', routing: { scope: 'cloud-summary' } },
        }),
      ),
    ).toBe('');
  });

  it('returns the trimmed aiSafeSummary for a governed resource', () => {
    expect(
      getResourcePolicyGovernedSummary(
        asResource({
          name: 'n',
          policy: governedPolicy(),
          aiSafeSummary: '  governed summary text  ',
        }),
      ),
    ).toBe('governed summary text');
  });

  it('returns "redacted by policy" for a governed resource with an empty aiSafeSummary', () => {
    expect(
      getResourcePolicyGovernedSummary(
        asResource({
          name: 'n',
          policy: governedPolicy(),
          aiSafeSummary: '   ',
        }),
      ),
    ).toBe('redacted by policy');
  });

  it('returns "redacted by policy" for a governed resource with no aiSafeSummary at all', () => {
    expect(
      getResourcePolicyGovernedSummary(
        asResource({
          name: 'n',
          policy: governedPolicy(),
        }),
      ),
    ).toBe('redacted by policy');
  });
});

describe('getResourcePolicyDisplayLabel — branch coverage', () => {
  it('returns empty string when no resource is supplied', () => {
    expect(getResourcePolicyDisplayLabel(undefined)).toBe('');
    expect(getResourcePolicyDisplayLabel(null)).toBe('');
  });

  it('falls back to displayName when policy is absent', () => {
    expect(
      getResourcePolicyDisplayLabel({
        name: 'host-1',
        displayName: 'Host One',
      }),
    ).toBe('Host One');
  });

  it('falls back to name when displayName is absent and policy is absent', () => {
    expect(getResourcePolicyDisplayLabel(asResource({ name: 'host-1' }))).toBe('host-1');
  });

  it('returns empty string when neither name nor displayName is present and policy is absent', () => {
    expect(getResourcePolicyDisplayLabel({ name: '   ', displayName: '' })).toBe('');
  });

  it('falls back to displayName/name when policy is present but not governed', () => {
    expect(
      getResourcePolicyDisplayLabel({
        name: 'host-1',
        displayName: 'Host One',
        policy: { sensitivity: 'public', routing: { scope: 'cloud-summary' } },
      }),
    ).toBe('Host One');
  });

  // The following cases drive the private getConciseGovernedDisplaySummary helper.

  it('concise: returns the trimmed summary unchanged when it has no semicolon', () => {
    expect(
      getResourcePolicyDisplayLabel(
        asResource({
          name: 'n',
          policy: governedPolicy(),
          aiSafeSummary: '  plain governed label  ',
        }),
      ),
    ).toBe('plain governed label');
  });

  it('concise: strips a trailing " resource" suffix from the base label and appends a status', () => {
    expect(
      getResourcePolicyDisplayLabel(
        asResource({
          name: 'n',
          policy: governedPolicy(),
          aiSafeSummary: 'backup server resource; status online; sources pbs',
        }),
      ),
    ).toBe('backup server (online)');
  });

  it('concise: returns only the base label when no status part is present', () => {
    expect(
      getResourcePolicyDisplayLabel(
        asResource({
          name: 'n',
          policy: governedPolicy(),
          aiSafeSummary: 'web node; count 5; sources api',
        }),
      ),
    ).toBe('web node');
  });

  it('concise: returns empty string when every semicolon-delimited part is empty', () => {
    expect(
      getResourcePolicyDisplayLabel(
        asResource({
          name: 'n',
          policy: governedPolicy(),
          // governed summary trims to "; ;", which has no non-empty parts -> ''
          aiSafeSummary: ' ; ; ',
        }),
      ),
    ).toBe('');
  });

  it('concise: keeps the base label verbatim when the " resource" regex does not match', () => {
    expect(
      getResourcePolicyDisplayLabel(
        asResource({
          name: 'n',
          policy: governedPolicy(),
          aiSafeSummary: 'storage array; status degraded',
        }),
      ),
    ).toBe('storage array (degraded)');
  });
});

describe('shouldShowResourceAlternateName — branch coverage', () => {
  it('returns false when no resource is supplied', () => {
    expect(shouldShowResourceAlternateName(undefined)).toBe(false);
    expect(shouldShowResourceAlternateName(null)).toBe(false);
  });

  it('returns false when displayName is absent', () => {
    expect(shouldShowResourceAlternateName(asResource({ name: 'host-1' }))).toBe(false);
  });

  it('returns false when name is absent', () => {
    expect(shouldShowResourceAlternateName(asResource({ displayName: 'Host One' }))).toBe(false);
  });

  it('treats a whitespace-only displayName as present (guard does not trim) and thus differing from name', () => {
    // The guard `!resource?.displayName` does not trim, so '   ' is truthy and
    // passes; then ''.toLowerCase() !== 'host-1' -> true.
    expect(shouldShowResourceAlternateName({ name: 'host-1', displayName: '   ' })).toBe(true);
  });

  it('returns false when the policy requires governed handling', () => {
    expect(
      shouldShowResourceAlternateName({
        name: 'host-1',
        displayName: 'Host One',
        policy: governedPolicy(),
      }),
    ).toBe(false);
  });

  it('returns false when displayName and name match case-insensitively after trimming', () => {
    expect(
      shouldShowResourceAlternateName({
        name: 'HOST-1',
        displayName: '  host-1  ',
      }),
    ).toBe(false);
  });

  it('returns true when displayName and name differ', () => {
    expect(
      shouldShowResourceAlternateName({
        name: 'host-1',
        displayName: 'Primary Node',
      }),
    ).toBe(true);
  });
});
