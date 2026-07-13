import type {
  ActionAuditRecord,
  ActionResourceReference,
  ActionAuditState,
  ActionEvidenceClass,
  ActionPolicyAuthorityFactor,
  ActionPolicyReasonCode,
  ActionVerificationTruthStatus,
} from '@/types/actionAudit';
import type { MetadataBadgeTone } from '@/components/shared/MetadataBadge';

export const formatActionName = (value: string): string =>
  value === 'install_os_updates'
    ? 'Install operating system updates'
    : value === 'clean_package_cache'
      ? 'Clear downloaded package data'
      : value
          .replace(/[._-]+/g, ' ')
          .trim()
          .replace(/\b\w/g, (letter) => letter.toUpperCase());

export interface ActionInboxStatePresentation {
  accentClass: string;
  label: string;
  tone: MetadataBadgeTone;
}

const ACTION_INBOX_STATE_PRESENTATION: Record<ActionAuditState, ActionInboxStatePresentation> = {
  planned: { accentClass: 'border-l-slate-400', label: 'Ready to review', tone: 'muted' },
  pending_approval: {
    accentClass: 'border-l-amber-500',
    label: 'Approval required',
    tone: 'warning',
  },
  approved: { accentClass: 'border-l-sky-500', label: 'Ready to run', tone: 'info' },
  executing: { accentClass: 'border-l-blue-500', label: 'Running', tone: 'info' },
  completed: { accentClass: 'border-l-emerald-500', label: 'Completed', tone: 'success' },
  failed: { accentClass: 'border-l-red-500', label: 'Failed', tone: 'danger' },
  rejected: { accentClass: 'border-l-slate-400', label: 'Rejected', tone: 'muted' },
  expired: { accentClass: 'border-l-slate-400', label: 'Expired', tone: 'muted' },
};

export const getActionInboxStatePresentation = (
  state: ActionAuditState,
): ActionInboxStatePresentation => ACTION_INBOX_STATE_PRESENTATION[state];

const OPEN_ACTION_PRIORITY: Record<ActionAuditState, number> = {
  pending_approval: 0,
  planned: 1,
  approved: 1,
  executing: 2,
  failed: 3,
  completed: 4,
  rejected: 4,
  expired: 4,
};

export const sortOpenActionsForReview = (
  actions: readonly ActionAuditRecord[],
): ActionAuditRecord[] =>
  [...actions].sort((left, right) => {
    const priority = OPEN_ACTION_PRIORITY[left.state] - OPEN_ACTION_PRIORITY[right.state];
    if (priority !== 0) return priority;
    return Date.parse(right.updatedAt) - Date.parse(left.updatedAt);
  });

export interface ActionApprovalBadgePresentation {
  count: number;
  label: string;
}

export const getActionApprovalBadgePresentation = (
  count: number,
): ActionApprovalBadgePresentation | null => {
  if (!Number.isFinite(count) || count <= 0) return null;
  const normalized = Math.floor(count);
  return {
    count: normalized,
    label: `${normalized} ${normalized === 1 ? 'action awaits' : 'actions await'} approval`,
  };
};

export interface ActionResourcePresentation {
  detail: string;
  label: string;
}

const OPAQUE_RESOURCE_SUFFIX = /^(?:[a-f\d]{12,}|[a-z\d]{16,})$/i;

const compactOpaqueSuffix = (value: string): string => `…${value.slice(-6)}`;

const DASHED_RESOURCE_PREFIXES: Array<{ prefix: string; label: string }> = [
  { prefix: 'app-container', label: 'App container' },
  { prefix: 'agent', label: 'Host agent' },
  { prefix: 'vm', label: 'Virtual machine' },
];

const CANONICAL_RESOURCE_KINDS: Record<string, string> = {
  'docker:container': 'Docker container',
  'proxmox:node': 'Proxmox node',
  'proxmox:vm': 'Proxmox virtual machine',
  'proxmox:lxc': 'Proxmox container',
};

const RESOURCE_TYPE_LABELS: Record<string, string> = {
  'app-container': 'App container',
  agent: 'Host agent',
  container: 'Container',
  lxc: 'Proxmox container',
  node: 'Node',
  vm: 'Virtual machine',
};

const resourceTypeLabel = (resourceType: string): string =>
  RESOURCE_TYPE_LABELS[resourceType.trim().toLowerCase()] || formatActionName(resourceType);

export const getActionResourcePresentation = (
  resourceId: string,
  resource?: ActionResourceReference,
): ActionResourcePresentation => {
  const normalized = resourceId.trim();
  if (!normalized) return { label: 'Unknown resource', detail: '' };

  const authoritativeName = resource?.name?.trim();
  if (authoritativeName) {
    return {
      label: authoritativeName,
      detail: resourceTypeLabel(resource?.type ?? ''),
    };
  }

  const canonicalParts = normalized.split(':').filter(Boolean);
  if (canonicalParts.length > 1) {
    const name = canonicalParts.at(-1) || normalized;
    const kindKey = canonicalParts.slice(0, -1).join(':').toLowerCase();
    const kind = CANONICAL_RESOURCE_KINDS[kindKey] || formatActionName(kindKey);
    return OPAQUE_RESOURCE_SUFFIX.test(name)
      ? { label: kind, detail: compactOpaqueSuffix(name) }
      : { label: name, detail: kind };
  }

  for (const candidate of DASHED_RESOURCE_PREFIXES) {
    const prefix = `${candidate.prefix}-`;
    if (!normalized.toLowerCase().startsWith(prefix)) continue;
    const suffix = normalized.slice(prefix.length);
    return {
      label: candidate.label,
      detail: OPAQUE_RESOURCE_SUFFIX.test(suffix) ? compactOpaqueSuffix(suffix) : suffix,
    };
  }

  return { label: normalized, detail: '' };
};

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

export const verificationTruthLabel = (
  status: ActionVerificationTruthStatus,
  evidenceClass?: ActionEvidenceClass,
): string => {
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
