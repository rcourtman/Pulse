import type {
  ResourcePolicy,
  ResourceRedactionHint,
  ResourceRoutingScope,
  ResourceSensitivity,
} from '@/types/resource';

type PolicyBadgePresentation = {
  label: string;
  title: string;
  className: string;
};

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

export const getResourcePolicyRedactionLabels = (policy?: ResourcePolicy): string[] =>
  (policy?.routing.redact ?? []).map((hint) => redactionLabels[hint] ?? hint);

