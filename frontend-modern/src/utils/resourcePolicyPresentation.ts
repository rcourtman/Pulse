import type {
  Resource,
  ResourcePolicy,
  ResourceRedactionHint,
  ResourceRoutingScope,
  ResourceSensitivity,
} from '@/types/resource';
import type { IntelligencePolicyPostureSummary } from '@/types/aiIntelligence';

type PolicyBadgePresentation = {
  label: string;
  title: string;
  className: string;
};

export type ResourcePolicyCountSummary = {
  label: string;
  count: number;
};

export type ResourcePolicyDisplayResource = Pick<
  Resource,
  'name' | 'displayName' | 'policy' | 'aiSafeSummary'
>;

export const RESOURCE_POLICY_SENSITIVITY_ORDER: ResourceSensitivity[] = [
  'public',
  'internal',
  'sensitive',
  'restricted',
];

export const RESOURCE_POLICY_ROUTING_ORDER: ResourceRoutingScope[] = [
  'cloud-summary',
  'local-first',
  'local-only',
];

export const RESOURCE_POLICY_REDACTION_ORDER: ResourceRedactionHint[] = [
  'hostname',
  'ip-address',
  'platform-id',
  'alias',
  'path',
];

const badgeBaseClass =
  'inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium whitespace-nowrap';

const sensitivityPresentation: Record<
  ResourceSensitivity,
  Pick<PolicyBadgePresentation, 'label' | 'title' | 'className'>
> = {
  public: {
    label: 'Public',
    title: 'Resource data is classified as public.',
    className: `${badgeBaseClass} bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300`,
  },
  internal: {
    label: 'Internal',
    title: 'Resource data is classified for internal use.',
    className: `${badgeBaseClass} bg-slate-200 text-slate-700 dark:bg-slate-800 dark:text-slate-300`,
  },
  sensitive: {
    label: 'Sensitive',
    title: 'Resource data requires sensitivity-aware handling.',
    className: `${badgeBaseClass} bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300`,
  },
  restricted: {
    label: 'Restricted',
    title: 'Resource data is tightly restricted and requires guarded handling.',
    className: `${badgeBaseClass} bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300`,
  },
};

const routingPresentation: Record<
  ResourceRoutingScope,
  Pick<PolicyBadgePresentation, 'label' | 'title' | 'className'>
> = {
  'cloud-summary': {
    label: 'Cloud Summary',
    title: 'This resource may use cloud summarization within policy limits.',
    className: `${badgeBaseClass} bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300`,
  },
  'local-first': {
    label: 'Local First',
    title: 'This resource should prefer local handling before cloud escalation.',
    className: `${badgeBaseClass} bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300`,
  },
  'local-only': {
    label: 'Local Only',
    title: 'This resource must remain within the local boundary.',
    className: `${badgeBaseClass} bg-violet-100 text-violet-700 dark:bg-violet-900 dark:text-violet-300`,
  },
};

const redactionLabels: Record<ResourceRedactionHint, string> = {
  hostname: 'Hostname',
  'ip-address': 'IP Address',
  'platform-id': 'Platform ID',
  alias: 'Alias',
  path: 'Path',
};

export const getResourcePolicyBadges = (policy?: ResourcePolicy): PolicyBadgePresentation[] => {
  if (!policy) return [];
  return [sensitivityPresentation[policy.sensitivity], routingPresentation[policy.routing.scope]];
};

export const getResourceSensitivityLabel = (sensitivity?: ResourceSensitivity): string =>
  sensitivity ? sensitivityPresentation[sensitivity].label : 'Unclassified';

export const getResourceRoutingScopeLabel = (scope?: ResourceRoutingScope): string =>
  scope ? routingPresentation[scope].label : 'Unrouted';

export const getResourceRedactionHintLabel = (hint?: ResourceRedactionHint): string =>
  hint ? redactionLabels[hint] ?? hint : 'Unclassified';

export const getResourcePolicyRedactionLabels = (policy?: ResourcePolicy): string[] =>
  (policy?.routing.redact ?? []).map((hint) => getResourceRedactionHintLabel(hint));

const buildCountSummaries = <T extends string>(
  counts: Partial<Record<T, number>> | undefined,
  order: readonly T[],
  labelFn: (value: T) => string,
  includeZeroCounts: boolean,
): ResourcePolicyCountSummary[] => {
  if (!counts) return [];

  const summaries: ResourcePolicyCountSummary[] = [];
  for (const value of order) {
    const count = counts[value] ?? 0;
    if (!includeZeroCounts && count <= 0) {
      continue;
    }
    summaries.push({
      label: labelFn(value),
      count,
    });
  }
  return summaries;
};

export const getResourcePolicySensitivitySummaries = (
  posture?: IntelligencePolicyPostureSummary | null,
): ResourcePolicyCountSummary[] =>
  buildCountSummaries(
    posture?.sensitivity_counts,
    RESOURCE_POLICY_SENSITIVITY_ORDER,
    getResourceSensitivityLabel,
    true,
  );

export const getResourcePolicyRoutingSummaries = (
  posture?: IntelligencePolicyPostureSummary | null,
): ResourcePolicyCountSummary[] =>
  buildCountSummaries(
    posture?.routing_counts,
    RESOURCE_POLICY_ROUTING_ORDER,
    getResourceRoutingScopeLabel,
    true,
  );

export const getResourcePolicyRedactionSummaries = (
  posture?: IntelligencePolicyPostureSummary | null,
): ResourcePolicyCountSummary[] =>
  buildCountSummaries(
    posture?.redaction_counts,
    RESOURCE_POLICY_REDACTION_ORDER,
    getResourceRedactionHintLabel,
    false,
  );

export const getResourcePolicyDisplayLabel = (
  resource?: ResourcePolicyDisplayResource | null,
): string => {
  if (!resource) return '';

  const summary = resource.aiSafeSummary?.trim() ?? '';
  const policy = resource.policy;
  if (!policy) {
    return resource.displayName?.trim() || resource.name?.trim() || '';
  }
  const requiresGovernedDisplay =
    (policy.routing.scope === 'local-only' || (policy.routing.redact?.length ?? 0) > 0);

  if (requiresGovernedDisplay) {
    return summary || 'redacted by policy';
  }

  return resource.displayName?.trim() || resource.name?.trim() || '';
};

export const shouldShowResourceAlternateName = (
  resource?: ResourcePolicyDisplayResource | null,
): boolean => {
  if (!resource?.displayName || !resource.name) return false;

  if (
    resource.policy &&
    (resource.policy.routing.scope === 'local-only' || (resource.policy.routing.redact?.length ?? 0) > 0)
  ) {
    return false;
  }

  return resource.displayName.trim().toLowerCase() !== resource.name.trim().toLowerCase();
};
