import type {
  Resource,
  ResourcePolicy,
  ResourceRedactionHint,
  ResourceRoutingScope,
  ResourceSensitivity,
} from '@/types/resource';
import { requiresGovernedResourceDisplay } from '@/types/resource';

type PolicyBadgePresentation = {
  label: string;
  title: string;
  className: string;
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
  'inline-flex items-center rounded-sm px-1.5 py-0.5 text-[10px] font-medium whitespace-nowrap';

const sensitivityPresentation: Record<
  ResourceSensitivity,
  Pick<PolicyBadgePresentation, 'label' | 'title' | 'className'>
> = {
  public: {
    label: 'Public',
    title: 'Resource data is classified as public.',
    className: `${badgeBaseClass} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300`,
  },
  internal: {
    label: 'Internal',
    title: 'Resource data is classified for internal use.',
    className: `${badgeBaseClass} bg-slate-200 text-slate-700 dark:bg-slate-800 dark:text-slate-300`,
  },
  sensitive: {
    label: 'Sensitive',
    title: 'Resource data requires sensitivity-aware handling.',
    className: `${badgeBaseClass} bg-amber-100 text-amber-700 dark:bg-amber-900/25 dark:text-amber-300`,
  },
  restricted: {
    label: 'Restricted',
    title: 'Resource data is tightly restricted and requires guarded handling.',
    className: `${badgeBaseClass} bg-rose-100 text-rose-700 dark:bg-rose-900/25 dark:text-rose-300`,
  },
};

const routingPresentation: Record<
  ResourceRoutingScope,
  Pick<PolicyBadgePresentation, 'label' | 'title' | 'className'>
> = {
  'cloud-summary': {
    label: 'Cloud Summary',
    title: 'This resource may use cloud summarization within policy limits.',
    className: `${badgeBaseClass} bg-sky-100 text-sky-700 dark:bg-sky-900/25 dark:text-sky-300`,
  },
  'local-first': {
    label: 'Local First',
    title: 'This resource should prefer local handling before cloud escalation.',
    className: `${badgeBaseClass} bg-indigo-100 text-indigo-700 dark:bg-indigo-900/25 dark:text-indigo-300`,
  },
  'local-only': {
    label: 'Local Only',
    title: 'This resource must remain within the local boundary.',
    className: `${badgeBaseClass} bg-teal-100 text-teal-700 dark:bg-teal-900/25 dark:text-teal-300`,
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

export const hasDefaultResourcePolicyPosture = (policy?: ResourcePolicy): boolean =>
  Boolean(
    policy &&
    policy.sensitivity === 'internal' &&
    policy.routing.scope === 'cloud-summary' &&
    (policy.routing.redact?.length ?? 0) === 0,
  );

export const getResourceSensitivityLabel = (sensitivity?: ResourceSensitivity): string =>
  sensitivity ? sensitivityPresentation[sensitivity].label : 'Unclassified';

export const getResourceRoutingScopeLabel = (scope?: ResourceRoutingScope): string =>
  scope ? routingPresentation[scope].label : 'Unrouted';

export const getResourceRedactionHintLabel = (hint?: ResourceRedactionHint): string =>
  hint ? (redactionLabels[hint] ?? hint) : 'Unclassified';

export const getResourcePolicyRedactionLabels = (policy?: ResourcePolicy): string[] =>
  (policy?.routing.redact ?? []).map((hint) => getResourceRedactionHintLabel(hint));

const getConciseGovernedDisplaySummary = (summary: string): string => {
  const trimmed = summary.trim();
  if (!trimmed) return '';
  if (!trimmed.includes(';')) {
    return trimmed;
  }

  const parts = trimmed
    .split(';')
    .map((part) => part.trim())
    .filter((part) => part.length > 0);
  if (parts.length === 0) {
    return '';
  }

  const baseLabel = parts[0].replace(/\s+resource$/i, '').trim() || parts[0];
  const statusPart = parts.find((part, index) => index > 0 && /^status\s+/i.test(part));
  const status = statusPart?.replace(/^status\s+/i, '').trim();

  return status ? `${baseLabel} (${status})` : baseLabel;
};

export const getResourcePolicyGovernedSummary = (
  resource?: ResourcePolicyDisplayResource | null,
): string => {
  if (!resource) return '';

  const policy = resource.policy;
  if (!policy || !requiresGovernedResourceDisplay(policy)) {
    return resource.displayName?.trim() || resource.name?.trim() || '';
  }

  return resource.aiSafeSummary?.trim() || 'redacted by policy';
};

export const getResourcePolicyDisplayLabel = (
  resource?: ResourcePolicyDisplayResource | null,
): string => {
  if (!resource) return '';

  const policy = resource.policy;
  if (!policy) {
    return resource.displayName?.trim() || resource.name?.trim() || '';
  }
  if (requiresGovernedResourceDisplay(policy)) {
    return getConciseGovernedDisplaySummary(getResourcePolicyGovernedSummary(resource));
  }

  return resource.displayName?.trim() || resource.name?.trim() || '';
};
