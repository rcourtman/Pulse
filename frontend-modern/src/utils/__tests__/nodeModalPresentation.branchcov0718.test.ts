import { describe, expect, it } from 'vitest';
import {
  buildNodeModalMonitoringPayload,
  getNodeModalDefaultFormData,
  getNodeModalTestResultPresentation,
  getNodeMonitoringCoverageCopy,
  getNodeTokenIdPlaceholder,
  getTemperatureMonitoringLockedCopy,
  type NodeModalFormData,
} from '@/utils/nodeModalPresentation';

// Branch-coverage companion to nodeModalPresentation.test.ts.
//
// The sibling suite already pins every switch arm of getNodeProductName,
// getNodeEndpointPlaceholder, getNodeEndpointHelp, getNodeGuestUrlPlaceholder,
// getNodeUsernamePlaceholder and getNodeUsernameHelp; pins both arms of each
// pmg predicate inside getNodeModalDefaultFormData (via toMatchObject on a
// subset of fields); covers the pve/pbs toggle-propagation arms of
// buildNodeModalMonitoringPayload; and exercises the success/warning/error
// outcomes of getNodeModalTestResultPresentation via partial matches.
//
// This file targets only the residual surface:
//   - getTemperatureMonitoringLockedCopy (wholly uncovered export)
//   - getNodeTokenIdPlaceholder('pmg')  (untested `case 'pmg'` arm)
//   - getNodeMonitoringCoverageCopy('pbs') (third input routing to the non-pmg arm)
//   - getNodeModalTestResultPresentation with undefined/null/''/unrecognized status
//     (the default arm of the switch on its full optional-input space) plus
//     full-string pins of every panelClass/textClass so the dark-mode classes
//     cannot silently regress
//   - buildNodeModalMonitoringPayload pmg toggle propagation and the pve
//     monitorPhysicalDisks/physicalDiskPollingMinutes assignment arms
//   - getNodeModalDefaultFormData via toStrictEqual on every field so the
//     fields the sibling ignores (tokenName, tokenValue, fingerprint, the
//     pbs/pmg-only monitor flags when called for pve, etc.) are locked down

// ===========================================================================
// getTemperatureMonitoringLockedCopy — previously uncovered export.
// ===========================================================================

describe('getTemperatureMonitoringLockedCopy branch coverage', () => {
  it('returns the canonical environment-override lock message verbatim', () => {
    // Single-return literal; pin the exact wording (including the env-var
    // token ENABLE_TEMPERATURE_MONITORING) so a copy edit surfaces loudly.
    expect(getTemperatureMonitoringLockedCopy()).toBe(
      'Locked by environment variables. Remove the override (ENABLE_TEMPERATURE_MONITORING) and restart Pulse to manage it in the UI.',
    );
  });
});

// ===========================================================================
// getNodeTokenIdPlaceholder — pmg switch arm (sibling only tested pve + pbs).
// ===========================================================================

describe('getNodeTokenIdPlaceholder branch coverage', () => {
  it("returns the pmg-specific token id placeholder (untested `case 'pmg'` arm)", () => {
    expect(getNodeTokenIdPlaceholder('pmg')).toBe('pulse-monitor@pmg!pulse-token');
  });
});

// ===========================================================================
// getNodeMonitoringCoverageCopy — pbs input (sibling only asserted pve + pmg).
// ===========================================================================

describe('getNodeMonitoringCoverageCopy branch coverage', () => {
  it('returns the generic non-pmg coverage copy for pbs (false arm of the pmg predicate)', () => {
    // The sibling test covers this return via 'pve'; pinning the pbs input
    // too locks down the third node type that routes here. The exact string
    // is asserted verbatim so the em-dash + 'PBS job activity' phrasing is
    // protected against silent copy drift.
    expect(getNodeMonitoringCoverageCopy('pbs')).toBe(
      'Pulse automatically tracks all supported resources for this node — virtual machines, containers, storage usage, backups, and PBS job activity — so you always get full visibility without extra configuration.',
    );
  });
});

// ===========================================================================
// getNodeModalTestResultPresentation — default arm with the full optional
// input space (undefined / null / '' / unrecognized) plus full-string pins
// of every panelClass and textClass.
// ===========================================================================

describe('getNodeModalTestResultPresentation branch coverage', () => {
  it('routes an undefined status to the default error presentation', () => {
    // switch(undefined) misses both named cases and hits the default arm.
    // The sibling test never calls the function without an argument, so the
    // optional-parameter handling on the default arm is otherwise unexercised.
    expect(getNodeModalTestResultPresentation()).toStrictEqual({
      panelClass:
        'mx-6 p-3 rounded-md text-sm bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 text-red-800 dark:text-red-200',
      textClass: 'text-red-800 dark:text-red-200',
      icon: 'error',
    });
  });

  it('routes a null status to the default error presentation', () => {
    // switch(null) also hits the default arm; observable through the red
    // icon and the dark-mode panelClass together.
    expect(getNodeModalTestResultPresentation(null)).toStrictEqual({
      panelClass:
        'mx-6 p-3 rounded-md text-sm bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 text-red-800 dark:text-red-200',
      textClass: 'text-red-800 dark:text-red-200',
      icon: 'error',
    });
  });

  it('routes an empty-string status to the default error presentation', () => {
    // '' is neither 'success' nor 'warning' — default arm.
    const out = getNodeModalTestResultPresentation('');
    expect(out.icon).toBe('error');
    expect(out.panelClass).toBe(
      'mx-6 p-3 rounded-md text-sm bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 text-red-800 dark:text-red-200',
    );
    expect(out.textClass).toBe('text-red-800 dark:text-red-200');
  });

  it('routes an unrecognized status string to the default error presentation', () => {
    // Any value other than 'success'/'warning' falls through to default; the
    // returned icon is the 'error' sentinel even though the input string
    // itself is preserved nowhere on the output.
    const out = getNodeModalTestResultPresentation('connection-refused');
    expect(out.icon).toBe('error');
    expect(out.panelClass).toContain('bg-red-50');
    expect(out.panelClass).toContain('dark:bg-red-900');
    expect(out.textClass).toBe('text-red-800 dark:text-red-200');
  });

  it('returns the exact success presentation with every dark-mode class pinned', () => {
    // The sibling test only used stringContaining('bg-green-50'); this pins
    // the whole panelClass string (including dark:green-900, the border
    // classes, and the green-200 dark textClass) so a tailwind refactor
    // cannot silently drop a class.
    expect(getNodeModalTestResultPresentation('success')).toStrictEqual({
      panelClass:
        'mx-6 p-3 rounded-md text-sm bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-800 text-green-800 dark:text-green-200',
      textClass: 'text-green-800 dark:text-green-200',
      icon: 'success',
    });
  });

  it('returns the exact warning presentation with every dark-mode class pinned', () => {
    expect(getNodeModalTestResultPresentation('warning')).toStrictEqual({
      panelClass:
        'mx-6 p-3 rounded-md text-sm bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 text-amber-800 dark:text-amber-200',
      textClass: 'text-amber-800 dark:text-amber-200',
      icon: 'warning',
    });
  });

  it('returns the exact error presentation with every dark-mode class pinned', () => {
    // 'error' is not a named case — it falls through to default; the sibling
    // test only asserts icon + stringContaining('bg-red-50'). This pins the
    // full panelClass and the previously-unasserted textClass.
    expect(getNodeModalTestResultPresentation('error')).toStrictEqual({
      panelClass:
        'mx-6 p-3 rounded-md text-sm bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 text-red-800 dark:text-red-200',
      textClass: 'text-red-800 dark:text-red-200',
      icon: 'error',
    });
  });
});

// ===========================================================================
// buildNodeModalMonitoringPayload — pmg toggle propagation (sibling only used
// default form data for pmg) plus the pve monitorPhysicalDisks=true arm with
// a custom physicalDiskPollingMinutes (sibling left both at their defaults).
// ===========================================================================

describe('buildNodeModalMonitoringPayload branch coverage', () => {
  it('propagates every toggled pmg scope boolean through the pmg case arm', () => {
    // The sibling pmg test only reads back the default form data; this
    // overrides all four pmg surfaces (two flipped to false, two flipped to
    // true) so each assignment in the pmg case runs against a non-default
    // value and the output is pinned via toStrictEqual (no extra keys leak).
    const form: NodeModalFormData = {
      ...getNodeModalDefaultFormData('pmg'),
      monitorMailStats: false,
      monitorQueues: true,
      monitorQuarantine: false,
      monitorDomainStats: true,
    };
    expect(buildNodeModalMonitoringPayload('pmg', form)).toStrictEqual({
      monitorMailStats: false,
      monitorQueues: true,
      monitorQuarantine: false,
      monitorDomainStats: true,
    });
  });

  it('propagates monitorPhysicalDisks=true and a custom physicalDiskPollingMinutes through the pve arm', () => {
    // The sibling pve toggle test left monitorPhysicalDisks at its false
    // default and physicalDiskPollingMinutes at 5; this flips the disk flag
    // to true and overrides the polling cadence to 30 so both assignment
    // arms run with non-default values in a single toStrictEqual payload.
    const form: NodeModalFormData = {
      ...getNodeModalDefaultFormData('pve'),
      monitorPhysicalDisks: true,
      physicalDiskPollingMinutes: 30,
    };
    expect(buildNodeModalMonitoringPayload('pve', form)).toStrictEqual({
      monitorVMs: true,
      monitorContainers: true,
      monitorStorage: true,
      monitorBackups: true,
      monitorPhysicalDisks: true,
      physicalDiskPollingMinutes: 30,
    });
  });
});

// ===========================================================================
// getNodeModalDefaultFormData — full toStrictEqual pin per node type. The
// sibling test only toMatchObject'd a subset of fields; this locks every
// field (including tokenName/tokenValue/fingerprint empty strings, the pve
// monitorVMs/Containers/Storage/Backups=true defaults, and the
// clusterEndpointOverrides empty record) so a default flip cannot hide.
// ===========================================================================

describe('getNodeModalDefaultFormData branch coverage', () => {
  it('returns the complete canonical pve form shape with every field pinned', () => {
    expect(getNodeModalDefaultFormData('pve')).toStrictEqual({
      name: '',
      host: '',
      guestURL: '',
      authType: 'token',
      setupMode: 'auto',
      user: '',
      password: '',
      tokenName: '',
      tokenValue: '',
      fingerprint: '',
      verifySSL: true,
      monitorVMs: true,
      monitorContainers: true,
      monitorStorage: true,
      monitorBackups: true,
      monitorPhysicalDisks: false,
      physicalDiskPollingMinutes: 5,
      monitorDatastores: true,
      monitorSyncJobs: true,
      monitorVerifyJobs: true,
      monitorPruneJobs: true,
      monitorGarbageJobs: true,
      monitorMailStats: true,
      monitorQueues: true,
      monitorQuarantine: true,
      monitorDomainStats: false,
      clusterEndpointOverrides: {},
    });
  });

  it('returns the complete canonical pbs form shape with every field pinned', () => {
    // pbs takes the same non-pmg arms of both ternaries as pve (authType
    // 'token', setupMode 'auto'); pinning it separately guards against a
    // future per-nodeType default flip.
    expect(getNodeModalDefaultFormData('pbs')).toStrictEqual({
      name: '',
      host: '',
      guestURL: '',
      authType: 'token',
      setupMode: 'auto',
      user: '',
      password: '',
      tokenName: '',
      tokenValue: '',
      fingerprint: '',
      verifySSL: true,
      monitorVMs: true,
      monitorContainers: true,
      monitorStorage: true,
      monitorBackups: true,
      monitorPhysicalDisks: false,
      physicalDiskPollingMinutes: 5,
      monitorDatastores: true,
      monitorSyncJobs: true,
      monitorVerifyJobs: true,
      monitorPruneJobs: true,
      monitorGarbageJobs: true,
      monitorMailStats: true,
      monitorQueues: true,
      monitorQuarantine: true,
      monitorDomainStats: false,
      clusterEndpointOverrides: {},
    });
  });

  it('returns the complete canonical pmg form shape with both pmg-ternary arms pinned', () => {
    // pmg is the only input that takes the true arm of both
    // `nodeType === 'pmg'` ternaries (authType 'password', setupMode
    // 'manual'); this full-shape assertion fires both arms in one go.
    expect(getNodeModalDefaultFormData('pmg')).toStrictEqual({
      name: '',
      host: '',
      guestURL: '',
      authType: 'password',
      setupMode: 'manual',
      user: '',
      password: '',
      tokenName: '',
      tokenValue: '',
      fingerprint: '',
      verifySSL: true,
      monitorVMs: true,
      monitorContainers: true,
      monitorStorage: true,
      monitorBackups: true,
      monitorPhysicalDisks: false,
      physicalDiskPollingMinutes: 5,
      monitorDatastores: true,
      monitorSyncJobs: true,
      monitorVerifyJobs: true,
      monitorPruneJobs: true,
      monitorGarbageJobs: true,
      monitorMailStats: true,
      monitorQueues: true,
      monitorQuarantine: true,
      monitorDomainStats: false,
      clusterEndpointOverrides: {},
    });
  });
});
