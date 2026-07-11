/**
 * Branch-coverage tests for settingsNavigationModel.
 *
 * Focuses exclusively on branches NOT already exercised by the sibling
 * `settingsRouting.test.ts` suite, for these target functions:
 *   - deriveTabFromQuery        (exported)
 *   - deriveTabFromPath         (exported)
 *   - isRouteableSettingsPath   (exported)
 *   - isRouteableSettingsLocation (exported)
 *   - normalizeSettingsPath     (NOT exported — a module-local const, exercised
 *                               indirectly through resolveCanonicalSettingsPath
 *                               and isRouteableSettingsPath, which are the
 *                               closest exported callers)
 */
import { describe, expect, it } from 'vitest';
import {
  DEFAULT_SETTINGS_TAB,
  deriveTabFromPath,
  deriveTabFromQuery,
  isRouteableSettingsLocation,
  isRouteableSettingsPath,
  resolveCanonicalSettingsPath,
  type SettingsTab,
} from '../settingsNavigationModel';

// ---- deriveTabFromQuery: uncovered switch arms -----------------------------
// The sibling suite already covers the `infrastructure`, `availability`,
// `monitoring-availability`, `system-ai`, `patrol`, `assistant`, `discovery`,
// `system-relay`, `system-billing`, `diagnostics`, `reporting`, `logs`,
// `system-recovery`, `organization-overview`, `organization-billing`,
// `security-overview`, `data-handling`, `resource-privacy` cases and the OAuth
// callback + unknown/default arms. Here we cover every remaining switch case.

describe('deriveTabFromQuery — uncovered switch cases', () => {
  const cases: Array<[string, SettingsTab]> = [
    ['?tab=system-updates', 'system-updates'],
    ['?tab=system-network', 'system-network'],
    ['?tab=system-general', 'system-general'],
    ['?tab=pulse-intelligence', 'system-ai'],
    ['?tab=provider-models', 'system-ai'],
    ['?tab=provider', 'system-ai'],
    ['?tab=system-ai-patrol', 'system-ai-patrol'],
    ['?tab=system-ai-assistant', 'system-ai-assistant'],
    // NOTE: `system-ai-discovery` and `discovery` intentionally map to
    // `system-ai-assistant` here (see GLM_REPORT.md suspected bug).
    ['?tab=system-ai-discovery', 'system-ai-assistant'],
    ['?tab=support-diagnostics', 'support-diagnostics'],
    ['?tab=support-reporting', 'support-reporting'],
    ['?tab=support-logs', 'support-logs'],
    ['?tab=api', 'api'],
    ['?tab=organization-access', 'organization-access'],
    ['?tab=organization-sharing', 'organization-sharing'],
    ['?tab=organization-billing-admin', 'organization-billing-admin'],
    ['?tab=security-data-handling', 'security-data-handling'],
    ['?tab=security-auth', 'security-auth'],
    ['?tab=security-sso', 'security-sso'],
    ['?tab=security-roles', 'security-roles'],
    ['?tab=security-users', 'security-users'],
    ['?tab=security-audit', 'security-audit'],
    ['?tab=security-webhooks', 'security-webhooks'],
  ];

  for (const [query, expectedTab] of cases) {
    it(`maps ${query} to the '${expectedTab}' tab`, () => {
      expect(deriveTabFromQuery(query)).toBe(expectedTab);
    });
  }
});

// ---- deriveTabFromQuery: guard / normalization branches --------------------

describe('deriveTabFromQuery — guard and normalization branches', () => {
  it('returns null for an empty search string (no tab param)', () => {
    expect(deriveTabFromQuery('')).toBeNull();
  });

  it('returns null when the search carries no tab parameter', () => {
    expect(deriveTabFromQuery('?foo=bar&baz=qux')).toBeNull();
  });

  it('returns null when the tab parameter value is empty', () => {
    expect(deriveTabFromQuery('?tab=')).toBeNull();
  });

  it('returns null when the tab parameter is only whitespace', () => {
    expect(deriveTabFromQuery('?tab=   ')).toBeNull();
  });

  it('lowercases the tab value before matching (case-insensitive)', () => {
    expect(deriveTabFromQuery('?tab=SYSTEM-AI')).toBe('system-ai');
  });

  it('trims surrounding whitespace from the tab value before matching', () => {
    expect(deriveTabFromQuery('?tab=  system-ai  ')).toBe('system-ai');
  });

  it('honors the OAuth-callback guard ahead of an otherwise-valid tab', () => {
    // isAISettingsOAuthCallbackQuery(search) short-circuits to 'system-ai'
    // even when a different tab is also present.
    expect(deriveTabFromQuery('?ai_oauth_success=true&tab=organization-overview')).toBe(
      'system-ai',
    );
  });

  it('honors the OAuth-error guard ahead of an otherwise-valid tab', () => {
    expect(deriveTabFromQuery('?ai_oauth_error=denied&tab=security-auth')).toBe('system-ai');
  });
});

// ---- deriveTabFromPath: uncovered path arms --------------------------------
// The sibling suite already covers the organization access/sharing/billing and
// the billing plan/usage subroutes plus support diagnostics/reporting/logs.
// Here we cover every remaining path-prefix / includes arm and the fallback.

describe('deriveTabFromPath — uncovered path arms', () => {
  const cases: Array<[string, SettingsTab]> = [
    ['/settings', DEFAULT_SETTINGS_TAB],
    ['/settings/infrastructure', 'infrastructure-systems'],
    ['/settings/monitoring/availability', 'monitoring-availability'],
    ['/settings/pulse-intelligence/patrol', 'system-ai-patrol'],
    ['/settings/pulse-intelligence/assistant', 'system-ai-assistant'],
    // NOTE: the discovery route canonicalizes to the *assistant* path today
    // (resolveCanonicalSettingsPath maps PULSE_INTELLIGENCE_DISCOVERY_PREFIX to
    // settingsTabPath('system-ai-assistant')), so deriveTabFromPath observes
    // 'system-ai-assistant'. See GLM_REPORT.md suspected bug #1.
    ['/settings/pulse-intelligence/discovery', 'system-ai-assistant'],
    ['/settings/pulse-intelligence/provider', 'system-ai'],
    // Legacy /settings/system-ai canonicalizes to the provider prefix.
    ['/settings/system-ai', 'system-ai'],
    ['/settings/system-general', 'system-general'],
    ['/settings/system-network', 'system-network'],
    ['/settings/system-updates', 'system-updates'],
    ['/settings/system-recovery', 'system-recovery'],
    ['/settings/system-relay', 'system-relay'],
    // SUPPORT_PREFIX exact-value arm collapses to diagnostics.
    ['/settings/support', 'support-diagnostics'],
    ['/settings/security/api', 'api'],
    ['/settings/security-overview', 'security-overview'],
    ['/settings/security-data-handling', 'security-data-handling'],
    ['/settings/security-auth', 'security-auth'],
    ['/settings/security-sso', 'security-sso'],
    ['/settings/security-roles', 'security-roles'],
    ['/settings/security-users', 'security-users'],
    ['/settings/security-audit', 'security-audit'],
    ['/settings/security-webhooks', 'security-webhooks'],
    ['/settings/organization/billing-admin', 'organization-billing-admin'],
  ];

  for (const [path, expectedTab] of cases) {
    it(`maps ${path} to the '${expectedTab}' tab`, () => {
      expect(deriveTabFromPath(path)).toBe(expectedTab);
    });
  }

  it('falls back to the default tab for an unrecognized settings path', () => {
    expect(deriveTabFromPath('/settings/unknown')).toBe(DEFAULT_SETTINGS_TAB);
  });

  it('falls back to the default tab for a non-settings path', () => {
    // resolveCanonicalSettingsPath returns null -> the normalizeSettingsPath
    // fallback is used -> no arm matches -> default tab.
    expect(deriveTabFromPath('/totally/elsewhere')).toBe(DEFAULT_SETTINGS_TAB);
  });

  it("matches the loose '/settings/system-ai' includes arm for an uncanonicalized path", () => {
    // '/settings/system-ai-foo' is not retired and not canonicalized, so it
    // passes through resolveCanonicalSettingsPath unchanged and then hits the
    // broad `canonicalPath.includes('/settings/system-ai')` arm.
    expect(deriveTabFromPath('/settings/system-ai-foo')).toBe('system-ai');
  });
});

// ---- normalizeSettingsPath (indirect, via exported callers) ----------------
// normalizeSettingsPath is a module-local const and is not exported. Its three
// branches (empty/falsy -> '/settings'; trailing-slash strip; passthrough) are
// observed through resolveCanonicalSettingsPath and isRouteableSettingsPath.

describe('normalizeSettingsPath (exercised via resolveCanonicalSettingsPath)', () => {
  it("normalizes an empty string to '/settings' before canonical resolution", () => {
    // '' trims to '' -> '/settings' -> resolved to the default tab path.
    expect(resolveCanonicalSettingsPath('')).toBe('/settings/infrastructure');
    // Equivalent to passing '/settings' directly.
    expect(resolveCanonicalSettingsPath('')).toBe(resolveCanonicalSettingsPath('/settings'));
  });

  it("defends against a null runtime input by treating it as '' (the `path || ''` arm)", () => {
    expect(resolveCanonicalSettingsPath(null as unknown as string)).toBe('/settings/infrastructure');
  });

  it("defends against an undefined runtime input by treating it as '' (the `path || ''` arm)", () => {
    expect(resolveCanonicalSettingsPath(undefined as unknown as string)).toBe(
      '/settings/infrastructure',
    );
  });

  it('strips a single trailing slash from a settings path', () => {
    expect(resolveCanonicalSettingsPath('/settings/system-general/')).toBe(
      '/settings/system-general',
    );
  });

  it('strips multiple trailing slashes from a settings path', () => {
    expect(resolveCanonicalSettingsPath('/settings/system-general//')).toBe(
      '/settings/system-general',
    );
  });

  it("does not strip a lone '/' (length === 1) and therefore resolves it to null", () => {
    // '/' has length 1 so the `length > 1 && endsWith('/')` arm is skipped;
    // the value stays '/', which is neither '/settings' nor under '/settings/'.
    expect(resolveCanonicalSettingsPath('/')).toBeNull();
  });
});

describe('normalizeSettingsPath (exercised via isRouteableSettingsPath)', () => {
  it("treats an empty path as routeable because it normalizes to '/settings'", () => {
    expect(isRouteableSettingsPath('')).toBe(true);
  });

  it('treats a trailing-slash routeable path as routeable after normalization', () => {
    expect(isRouteableSettingsPath('/settings/system-general/')).toBe(true);
  });
});

// ---- isRouteableSettingsPath: uncovered branches ---------------------------

describe('isRouteableSettingsPath — uncovered branches', () => {
  it('returns false for a non-settings path (canonical resolution is null)', () => {
    // Covers the `canonicalPath ? ROUTEABLE_SETTINGS_PATHS.has(...) : false`
    // false arm driven by a null canonical resolution.
    expect(isRouteableSettingsPath('/totally/elsewhere')).toBe(false);
  });

  it('returns true for the monitoring prefix which canonicalizes to the availability path', () => {
    // '/settings/monitoring' resolves to '/settings/monitoring/availability',
    // which is in the routeable set — exercises canonical-resolve-then-check.
    expect(isRouteableSettingsPath('/settings/monitoring')).toBe(true);
  });
});

// ---- isRouteableSettingsLocation: uncovered branches -----------------------
// The sibling suite covers the infra `?add=agent`(true)/`?add=availability`
// (false) and availability `?add=target`(true)/`?add=availability`(false)
// pairs. Here we cover the default-search arm, the short-circuit-false arm, the
// remaining infra/availability guard arms, and the availability kind branches.

describe('isRouteableSettingsLocation — uncovered branches', () => {
  it('defaults the search argument to an empty string (routeable, no add guard)', () => {
    expect(isRouteableSettingsLocation('/settings/infrastructure')).toBe(true);
  });

  it('short-circuits to false when the path is not routeable, regardless of search', () => {
    expect(isRouteableSettingsLocation('/totally/elsewhere', '?add=agent')).toBe(false);
  });

  it('short-circuits to false for a retired settings path', () => {
    expect(isRouteableSettingsLocation('/settings/workloads', '')).toBe(false);
  });

  it('stays routeable on the infrastructure path with a non-add query parameter', () => {
    // params.has('add') is false -> the infrastructure add guard is skipped.
    expect(isRouteableSettingsLocation('/settings/infrastructure', '?agentUpdates=1')).toBe(true);
  });

  it('stays routeable on the infrastructure path with a recognized add step', () => {
    // deriveAddStepFromSearch('?add=pve') === 'pve' (not null) -> guard skipped.
    expect(isRouteableSettingsLocation('/settings/infrastructure', '?add=pve')).toBe(true);
  });

  it('rejects the infrastructure path when the add step is unrecognized', () => {
    // deriveAddStepFromSearch('?add=bogus') === null -> infra guard returns false.
    expect(isRouteableSettingsLocation('/settings/infrastructure', '?add=bogus')).toBe(false);
  });

  it('stays routeable on the availability path with no add parameter', () => {
    expect(isRouteableSettingsLocation('/settings/monitoring/availability', '')).toBe(true);
  });

  it('rejects the availability path when add is present but not the target value', () => {
    // shouldOpenAvailabilityTargetAddDialog is false because add !== 'target'.
    expect(isRouteableSettingsLocation('/settings/monitoring/availability', '?add=foo')).toBe(
      false,
    );
  });

  it('rejects the availability path when add=target but the target kind is invalid', () => {
    // targetKind 'bogus' normalizes to undefined -> shouldOpen...() false.
    expect(
      isRouteableSettingsLocation(
        '/settings/monitoring/availability',
        '?add=target&targetKind=bogus',
      ),
    ).toBe(false);
  });

  it('stays routeable on the availability path when add=target and the kind is valid', () => {
    // targetKind 'machine' is valid -> shouldOpen...() true -> guard skipped.
    expect(
      isRouteableSettingsLocation(
        '/settings/monitoring/availability',
        '?add=target&targetKind=machine',
      ),
    ).toBe(true);
  });
});
