import type {
  ActionEvidenceClass,
  ActionPolicyAuthorityFactor,
  ActionPolicyReasonCode,
  ActionVerificationTruthStatus,
} from '@/types/actionAudit';

export const formatActionName = (value: string): string =>
  value
    .replace(/[._-]+/g, ' ')
    .trim()
    .replace(/\b\w/g, (letter) => letter.toUpperCase());

export const formatPolicyAuthority = (factor: ActionPolicyAuthorityFactor): string => {
  switch (factor.kind) {
    case 'capability_registry':
      return 'Capability safety policy';
    case 'tenant_patrol_policy':
      return 'Patrol policy for this organization';
    case 'resource_operator_policy':
      return 'Policy for this resource';
  }
};

const POLICY_REASON_LABELS: Record<ActionPolicyReasonCode, string> = {
  capability_approval_none: 'No approval floor',
  capability_approval_admin: 'Administrator approval required',
  capability_approval_mfa: 'Verified approval required',
  capability_dry_run_only: 'Dry run only',
  capability_auto_never: 'Automatic execution not allowed',
  capability_auto_low_risk: 'Eligible for low-risk automation',
  capability_auto_elevated: 'Eligible for elevated automation',
  tenant_policy_unavailable: 'Organization policy unavailable',
  tenant_emergency_stop: 'Emergency stop is active',
  tenant_mode_monitor: 'Patrol is in Watch only',
  tenant_mode_assisted: 'Patrol is in Safe fixes',
  tenant_mode_full: 'Patrol is in Autopilot',
  tenant_mode_unknown: 'Patrol mode is unknown',
  tenant_full_mode_locked: 'Autopilot acknowledgement is not active',
  tenant_full_mode_unlocked: 'Autopilot acknowledgement is active',
  resource_policy_unavailable: 'Resource policy unavailable',
  resource_policy_missing: 'No resource policy is recorded',
  resource_never_auto_remediate: 'Automatic remediation is blocked for this resource',
  resource_capability_allowed: 'This action is allowed for the resource',
  resource_capability_not_allowed: 'This action is not allowed for the resource',
  resource_window_open: 'Resource action window is open',
  resource_window_closed: 'Resource action window is closed',
};

export const formatPolicyReason = (reason: ActionPolicyReasonCode): string =>
  POLICY_REASON_LABELS[reason];

export const formatEvidenceClass = (value: ActionEvidenceClass): string => {
  switch (value) {
    case 'independent':
      return 'Independent observer';
    case 'agent_attested':
      return 'Executing agent';
    case 'none':
      return 'No evidence source';
  }
};

export const verificationTruthLabel = (status: ActionVerificationTruthStatus, evidenceClass?: ActionEvidenceClass): string => {
  switch (status) {
    case 'confirmed':
      return evidenceClass === 'independent'
        ? 'Confirmed by independent observer'
        : evidenceClass === 'agent_attested'
          ? 'Confirmed by executing agent'
          : 'Confirmation lacks an evidence source';
    case 'contradicted':
      return 'Outcome contradicted';
    case 'inconclusive':
      return 'Outcome inconclusive';
    case 'not_attempted':
      return 'Outcome not verified';
  }
};
