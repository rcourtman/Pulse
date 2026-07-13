import { describe, expect, it } from 'vitest';
import type {
  ActionAuditRecord,
  ActionEvidenceClass,
  ActionPolicyAuthorityFactor,
  ActionPolicyAuthorityKind,
  ActionVerificationTruthStatus,
} from '@/types/actionAudit';
import {
  formatActionName,
  formatPolicyAuthority,
  getActionInboxStatePresentation,
  getActionResourcePresentation,
  sortOpenActionsForReview,
  verificationTruthLabel,
} from '../actionPresentation';

const makeAuthority = (
  kind: ActionPolicyAuthorityKind,
  overrides: Partial<ActionPolicyAuthorityFactor> = {},
): ActionPolicyAuthorityFactor => ({
  kind,
  sourceId: `authority:${kind}`,
  status: 'consulted',
  scope: { orgId: 'org-1', resourceId: 'proxmox:node:pve-1', capabilityName: 'install_os_updates' },
  reasonCodes: ['capability_auto_elevated'],
  ...overrides,
});

describe('formatActionName', () => {
  it('returns the curated human title for the install_os_updates capability', () => {
    expect(formatActionName('install_os_updates')).toBe('Install operating system updates');
  });

  it('returns the curated human title for the clean_package_cache capability', () => {
    expect(formatActionName('clean_package_cache')).toBe('Clear downloaded package data');
  });

  it.each([
    ['snake_case capability is humanized', 'restart_service', 'Restart Service'],
    ['dotted capability is humanized', 'restart.service', 'Restart Service'],
    ['dashed capability is humanized', 'restart-service', 'Restart Service'],
    ['mixed separators collapse to single spaces', 'restart._-service', 'Restart Service'],
    ['repeated separators collapse to a single space', 'restart___service', 'Restart Service'],
    [
      'multi-word capability title-cases each word',
      'restart_long_running_service',
      'Restart Long Running Service',
    ],
    ['already-spaced phrase is still title-cased', 'restart service', 'Restart Service'],
    [
      'camelCase token has no separator and only first letter uppercased',
      'restartService',
      'RestartService',
    ],
    [
      'leading and trailing separators are trimmed before title-casing',
      '_restart_service_',
      'Restart Service',
    ],
    ['single word is title-cased', 'reboot', 'Reboot'],
    ['single character is uppercased', 'x', 'X'],
  ] as const)('regex path: %s', (_name, input, expected) => {
    expect(formatActionName(input)).toBe(expected);
  });

  it('treats capability names that merely start with the curated key as generic regex input', () => {
    expect(formatActionName('install_os_updates_v2')).toBe('Install Os Updates V2');
    expect(formatActionName('clean_package_cache_extra')).toBe('Clean Package Cache Extra');
  });

  it('returns an empty string for empty input', () => {
    expect(formatActionName('')).toBe('');
  });

  it.each(['_', '.', '-', '_._', '-_.', '___'])(
    'returns an empty string for an all-separator input %j',
    (input) => {
      expect(formatActionName(input)).toBe('');
    },
  );

  it('throws a TypeError when called with null despite the declared string type', () => {
    expect(() => formatActionName(null as unknown as string)).toThrow(TypeError);
  });

  it('throws a TypeError when called with undefined despite the declared string type', () => {
    expect(() => formatActionName(undefined as unknown as string)).toThrow(TypeError);
  });
});

describe('Actions inbox presentation', () => {
  it.each([
    ['pending_approval', 'Approval required', 'warning', 'border-l-amber-500'],
    ['planned', 'Ready to review', 'muted', 'border-l-slate-400'],
    ['approved', 'Ready to run', 'info', 'border-l-sky-500'],
    ['executing', 'Running', 'info', 'border-l-blue-500'],
    ['failed', 'Failed', 'danger', 'border-l-red-500'],
    ['completed', 'Completed', 'success', 'border-l-emerald-500'],
    ['rejected', 'Rejected', 'muted', 'border-l-slate-400'],
    ['expired', 'Expired', 'muted', 'border-l-slate-400'],
  ] as const)(
    'presents %s as an operator-facing queue state',
    (state, label, tone, accentClass) => {
      expect(getActionInboxStatePresentation(state)).toEqual({ label, tone, accentClass });
    },
  );

  it.each([
    ['app-container-b66c935311856ba4', 'App container', '…856ba4'],
    ['vm-7f8b2b6cd98c2089', 'Virtual machine', '…8c2089'],
    ['agent-acdfdee2953587fb', 'Host agent', '…3587fb'],
    ['docker:container:edge', 'edge', 'Docker container'],
    ['proxmox:node:pve-1', 'pve-1', 'Proxmox node'],
    ['', 'Unknown resource', ''],
  ] as const)('turns resource id %j into a compact label', (resourceId, label, detail) => {
    expect(getActionResourcePresentation(resourceId)).toEqual({ label, detail });
  });

  it('keeps an unrecognized resource id intact', () => {
    expect(getActionResourcePresentation('storage-pool')).toEqual({
      label: 'storage-pool',
      detail: '',
    });
  });

  it('sorts decisions before runnable and executing work without mutating the API response', () => {
    const actions = [
      { id: 'running', state: 'executing', updatedAt: '2026-07-13T12:03:00Z' },
      { id: 'approved', state: 'approved', updatedAt: '2026-07-13T12:02:00Z' },
      { id: 'approval-old', state: 'pending_approval', updatedAt: '2026-07-13T12:00:00Z' },
      { id: 'approval-new', state: 'pending_approval', updatedAt: '2026-07-13T12:01:00Z' },
    ] as ActionAuditRecord[];

    expect(sortOpenActionsForReview(actions).map((action) => action.id)).toEqual([
      'approval-new',
      'approval-old',
      'approved',
      'running',
    ]);
    expect(actions.map((action) => action.id)).toEqual([
      'running',
      'approved',
      'approval-old',
      'approval-new',
    ]);
  });
});

describe('formatPolicyAuthority', () => {
  it.each([
    ['capability_registry', 'Capability safety policy'],
    ['tenant_patrol_policy', 'Patrol policy for this organization'],
    ['resource_operator_policy', 'Policy for this resource'],
  ] as const)('returns the curated label for kind %s', (kind, expected) => {
    expect(formatPolicyAuthority(makeAuthority(kind))).toBe(expected);
  });

  it('ignores non-kind fields when selecting the label', () => {
    const consulted = makeAuthority('capability_registry', {
      status: 'consulted',
      reasonCodes: [],
    });
    const unavailable = makeAuthority('capability_registry', {
      status: 'unavailable',
      sourceId: 'other-source',
      approvalFloor: 'admin',
    });
    expect(formatPolicyAuthority(consulted)).toBe('Capability safety policy');
    expect(formatPolicyAuthority(unavailable)).toBe('Capability safety policy');
  });

  it('returns undefined when factor.kind is an unrecognized value, because the switch has no default arm', () => {
    const malformed = {
      ...makeAuthority('capability_registry'),
      kind: 'not_a_real_authority_kind',
    } as unknown as ActionPolicyAuthorityFactor;
    expect(formatPolicyAuthority(malformed)).toBeUndefined();
  });

  it('returns undefined when factor is an empty object, because kind is undefined and the switch has no default', () => {
    expect(formatPolicyAuthority({} as unknown as ActionPolicyAuthorityFactor)).toBeUndefined();
  });
});

describe('verificationTruthLabel', () => {
  it.each([
    ['confirmed', undefined, 'Confirmation lacks an evidence source'],
    ['confirmed', 'none' as ActionEvidenceClass, 'Confirmation lacks an evidence source'],
  ] as const)(
    'reports a confirmed status without independent/agent evidence (%s, %s)',
    (status, evidenceClass, expected) => {
      expect(verificationTruthLabel(status, evidenceClass)).toBe(expected);
    },
  );

  it('credits an independent observer for a confirmed outcome', () => {
    expect(verificationTruthLabel('confirmed', 'independent')).toBe(
      'Confirmed by independent observer',
    );
  });

  it('credits the executing agent for a confirmed outcome', () => {
    expect(verificationTruthLabel('confirmed', 'agent_attested')).toBe(
      'Confirmed by executing agent',
    );
  });

  it('routes an unrecognized evidence class to the "lacks an evidence source" arm without throwing', () => {
    expect(verificationTruthLabel('confirmed', 'bogus' as unknown as ActionEvidenceClass)).toBe(
      'Confirmation lacks an evidence source',
    );
  });

  it.each([
    ['contradicted', 'Outcome contradicted'],
    ['inconclusive', 'Outcome inconclusive'],
    ['not_attempted', 'Outcome not verified'],
  ] as const)('returns the curated label for status %s', (status, expected) => {
    expect(verificationTruthLabel(status)).toBe(expected);
  });

  it('ignores evidenceClass for non-confirmed statuses', () => {
    expect(verificationTruthLabel('contradicted', 'independent')).toBe('Outcome contradicted');
    expect(verificationTruthLabel('inconclusive', 'agent_attested')).toBe('Outcome inconclusive');
    expect(verificationTruthLabel('not_attempted', 'none')).toBe('Outcome not verified');
  });

  it('returns undefined when status is unrecognized, because the switch has no default arm', () => {
    expect(
      verificationTruthLabel('bogus' as unknown as ActionVerificationTruthStatus, 'independent'),
    ).toBeUndefined();
  });
});
