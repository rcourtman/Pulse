import { describe, expect, it } from 'vitest';
import type {
  ActionAuditApprovalPolicy,
  ActionAuditRecord,
  ActionExecutionStatus,
  ActionPolicyAuthorityFactor,
  ActionPolicyReasonCode,
  ActionVerificationTruthStatus,
} from '@/types/actionAudit';
import { getAPTActionPresentation } from '../aptActionPresentation';

interface MakeAuditOptions {
  capabilityName?: string;
  summary?: string;
  execution?: ActionExecutionStatus;
  verification?: ActionVerificationTruthStatus;
  reasonCodes?: ActionPolicyReasonCode[];
  approvalPolicy?: ActionAuditApprovalPolicy;
  requiresApproval?: boolean;
  params?: Record<string, unknown>;
  withResult?: boolean;
}

const makeAudit = ({
  capabilityName = 'install_os_updates',
  summary,
  execution = 'inconclusive',
  verification = 'inconclusive',
  reasonCodes = ['capability_auto_elevated'],
  approvalPolicy,
  requiresApproval,
  params,
  withResult = true,
}: MakeAuditOptions = {}): ActionAuditRecord => {
  const policy: ActionAuditApprovalPolicy =
    approvalPolicy ?? (reasonCodes.includes('capability_auto_elevated') ? 'admin' : 'none');
  const needsApproval = requiresApproval ?? reasonCodes.includes('capability_auto_elevated');
  const authority: ActionPolicyAuthorityFactor = {
    kind: 'capability_registry',
    sourceId: `capability-registry:${capabilityName}`,
    status: 'consulted',
    scope: { orgId: 'org-1', resourceId: 'proxmox:node:pve-1', capabilityName },
    reasonCodes,
  };
  return {
    id: 'apt-action',
    createdAt: '2026-07-18T10:00:00Z',
    updatedAt: '2026-07-18T10:01:00Z',
    state: 'completed',
    request: {
      requestId: 'apt-request',
      resourceId: 'proxmox:node:pve-1',
      capabilityName,
      params,
      reason: 'Resolve the current Patrol finding',
      requestedBy: 'pulse_patrol',
    },
    plan: {
      actionId: 'apt-action',
      requestId: 'apt-request',
      allowed: true,
      requiresApproval: needsApproval,
      approvalPolicy: policy,
      rollbackAvailable: false,
      policyDecision: {
        version: 1,
        status: 'resolved',
        decisionId: 'decision',
        actionId: 'apt-action',
        scope: { orgId: 'org-1', resourceId: 'proxmox:node:pve-1', capabilityName },
        approvalRequirement: {
          version: 1,
          floor: policy,
          quorum: 1,
          disallowRequester: false,
        },
        planningAllowed: true,
        requiresApproval: needsApproval,
        authorities: [authority],
      },
    },
    result: withResult
      ? {
          success: execution === 'succeeded',
          actionResultV2: {
            version: 2,
            execution: { status: execution, summary },
            verification: { status: verification, evidenceClass: 'agent_attested' },
            compensation: { support: 'unavailable', status: 'not_available' },
          },
        }
      : undefined,
    verificationOutcome: { status: 'unknown' },
  };
};

describe('getAPTActionPresentation — residual branch coverage', () => {
  describe('isAPTAction false arm — returns undefined (L107)', () => {
    it('returns undefined for a capability name that is neither APT capability', () => {
      expect(
        getAPTActionPresentation(makeAudit({ capabilityName: 'restart_service' })),
      ).toBeUndefined();
    });
  });

  describe('safetyPosture: Server policy controlled fallback (L144)', () => {
    it('falls through to Server policy controlled when no auto reason code is present', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({ reasonCodes: ['tenant_mode_monitor'] }),
      );
      expect(presentation?.safetyPosture).toBe('Server policy controlled');
    });

    it('still classifies as Server policy controlled when authorities carry an unrelated reason code', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({ reasonCodes: ['resource_window_closed'] }),
      );
      expect(presentation?.safetyPosture).toBe('Server policy controlled');
    });
  });

  describe('approvalPosture: Explicit approval required arm (L148-149)', () => {
    it('demands explicit approval when the plan requiresApproval but the floor is not admin', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          reasonCodes: ['tenant_mode_assisted'],
          approvalPolicy: 'mfa',
          requiresApproval: true,
        }),
      );
      expect(presentation?.approvalPosture).toBe('Explicit approval required');
    });

    it('also reaches the explicit-approval arm under a dry_run_only policy that still requires approval', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          reasonCodes: ['capability_dry_run_only'],
          approvalPolicy: 'dry_run_only',
          requiresApproval: true,
        }),
      );
      expect(presentation?.approvalPosture).toBe('Explicit approval required');
    });
  });

  describe('nextStep default arm with no result envelope at all (L119)', () => {
    it('keeps the initial server-policy nextStep when result is entirely absent', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({ reasonCodes: ['capability_auto_elevated'], withResult: false }),
      );
      expect(presentation?.nextStep).toBe(
        'Review the server policy and exact scope before making a decision.',
      );
      expect(presentation?.facts).toEqual([]);
    });
  });

  describe('nextStep generic-rescan fallback inside actionResultV2 (L134)', () => {
    it('renders the generic rescan nextStep for a healthy, non-confirmed, non-recovery update', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          summary:
            'APT package updates: phase=verify; 4 pending before, 4 pending after; package manager health: healthy; recovery required: false; reboot required: false',
          verification: 'contradicted',
        }),
      );
      expect(presentation?.nextStep).toBe(
        'Review the separate execution and verification results, then rescan before creating another plan.',
      );
    });

    it('also reaches the generic-rescan fallback for a cleanup whose rescan is not required', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          capabilityName: 'clean_package_cache',
          reasonCodes: ['capability_auto_low_risk'],
          summary:
            'APT package cache: phase=verify; 100 bytes before, 0 bytes after, 100 bytes reclaimed; rollback available: false; rescan required: false',
          verification: 'inconclusive',
        }),
      );
      expect(presentation?.nextStep).toBe(
        'Review the separate execution and verification results, then rescan before creating another plan.',
      );
    });
  });

  describe('nextStep confirmed arm for cleanup (L125)', () => {
    it('directs a confirmed cleanup to the measured-rescan nextStep rather than reboot wording', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          capabilityName: 'clean_package_cache',
          reasonCodes: ['capability_auto_low_risk'],
          summary:
            'APT package cache: phase=complete; 100 bytes before, 0 bytes after, 100 bytes reclaimed; rollback available: false; rescan required: false',
          execution: 'succeeded',
          verification: 'confirmed',
        }),
      );
      expect(presentation?.nextStep).toBe(
        'Review the measured space reclaimed. Create another cleanup plan only if a fresh scan still shows pressure.',
      );
    });
  });

  describe('formatPhase untaken arms reachable through the summary parsers', () => {
    it('renders the preflight phase for an update plan', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          summary:
            'APT package updates: phase=preflight; 4 pending before, 4 pending after; package manager health: healthy; recovery required: false; reboot required: false',
        }),
      );
      expect(presentation?.facts).toContainEqual({
        label: 'Last phase reached',
        value: 'Safety check before changes',
      });
    });

    it('renders the refresh phase for an update plan', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          summary:
            'APT package updates: phase=refresh; 4 pending before, 4 pending after; package manager health: healthy; recovery required: false; reboot required: false',
        }),
      );
      expect(presentation?.facts).toContainEqual({
        label: 'Last phase reached',
        value: 'Refresh update information',
      });
    });

    it('renders the preflight phase for a cleanup plan and a No rescan fact', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          capabilityName: 'clean_package_cache',
          reasonCodes: ['capability_auto_low_risk'],
          summary:
            'APT package cache: phase=preflight; 100 bytes before, 100 bytes after, 0 bytes reclaimed; rollback available: false; rescan required: false',
        }),
      );
      expect(presentation?.facts).toContainEqual({
        label: 'Last phase reached',
        value: 'Safety check before changes',
      });
      expect(presentation?.facts).toContainEqual({
        label: 'Fresh rescan required',
        value: 'No',
      });
    });

    it('renders the complete phase for a cleanup plan', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          capabilityName: 'clean_package_cache',
          reasonCodes: ['capability_auto_low_risk'],
          summary:
            'APT package cache: phase=complete; 100 bytes before, 0 bytes after, 100 bytes reclaimed; rollback available: false; rescan required: false',
        }),
      );
      expect(presentation?.facts).toContainEqual({
        label: 'Last phase reached',
        value: 'Completed workflow',
      });
    });

    it('renders the verify phase for a cleanup plan', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          capabilityName: 'clean_package_cache',
          reasonCodes: ['capability_auto_low_risk'],
          summary:
            'APT package cache: phase=verify; 100 bytes before, 0 bytes after, 100 bytes reclaimed; rollback available: false; rescan required: false',
        }),
      );
      expect(presentation?.facts).toContainEqual({
        label: 'Last phase reached',
        value: 'Check the result',
      });
    });

    it('renders No reboot required when reboot required is false on an update plan', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          summary:
            'APT package updates: phase=complete; 2 pending before, 0 pending after; package manager health: healthy; recovery required: false; reboot required: false',
          execution: 'succeeded',
          verification: 'confirmed',
        }),
      );
      expect(presentation?.facts).toContainEqual({
        label: 'Reboot required',
        value: 'No',
      });
    });

    it('renders No recovery required as a fact value distinct from the Yes arm', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          summary:
            'APT package updates: phase=install; 6 pending before, 0 pending after; package manager health: healthy; recovery required: false; reboot required: false',
          execution: 'succeeded',
          verification: 'confirmed',
        }),
      );
      expect(presentation?.facts).toContainEqual({
        label: 'Recovery required',
        value: 'No',
      });
    });
  });

  describe('parseCount boundary — zero counts are accepted, not treated as missing', () => {
    it('accepts zero pending before and zero pending after on an update plan', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          summary:
            'APT package updates: phase=complete; 0 pending before, 0 pending after; package manager health: healthy; recovery required: false; reboot required: false',
        }),
      );
      expect(presentation?.facts).toContainEqual({ label: 'Updates before', value: '0' });
      expect(presentation?.facts).toContainEqual({ label: 'Updates remaining', value: '0' });
    });

    it('accepts a zero-byte cleanup that reclaims nothing', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({
          capabilityName: 'clean_package_cache',
          reasonCodes: ['capability_auto_low_risk'],
          summary:
            'APT package cache: phase=complete; 0 bytes before, 0 bytes after, 0 bytes reclaimed; rollback available: false; rescan required: false',
        }),
      );
      expect(presentation?.facts).toContainEqual({
        label: 'Downloaded package data before',
        value: '0 B',
      });
      expect(presentation?.facts).toContainEqual({
        label: 'Space reclaimed',
        value: '0 B',
      });
    });
  });

  describe('hasExactEmptyParameters — undefined params are treated as valid empty', () => {
    it('treats an absent params field as valid empty parameters', () => {
      const presentation = getAPTActionPresentation(
        makeAudit({ reasonCodes: ['capability_auto_elevated'] }),
      );
      expect(presentation?.parametersValid).toBe(true);
      expect(presentation?.parameterAuthority).toContain('accepts no command, path');
    });
  });

  describe('title arms', () => {
    it('uses the install title for install_os_updates', () => {
      expect(getAPTActionPresentation(makeAudit())?.title).toBe('Install operating system updates');
    });

    it('uses the cleanup title for clean_package_cache', () => {
      expect(
        getAPTActionPresentation(
          makeAudit({ capabilityName: 'clean_package_cache', reasonCodes: ['capability_auto_low_risk'] }),
        )?.title,
      ).toBe('Clear downloaded package data');
    });
  });

  describe('authorityBoundary arms', () => {
    it('bounds the install capability away from package removal and reboot', () => {
      expect(getAPTActionPresentation(makeAudit())?.authorityBoundary).toBe(
        'The agent may refresh update information and install the complete approved update set. It cannot remove packages or reboot the host.',
      );
    });

    it('bounds the cleanup capability to downloaded package data only', () => {
      expect(
        getAPTActionPresentation(
          makeAudit({ capabilityName: 'clean_package_cache', reasonCodes: ['capability_auto_low_risk'] }),
        )?.authorityBoundary,
      ).toBe(
        'The agent may clear only downloaded package data on the pressured filesystem. It cannot choose paths, remove installed packages, or reboot the host.',
      );
    });
  });
});
