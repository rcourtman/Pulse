const TIER_LABELS: Record<string, string> = {
  free: 'Community',
  relay: 'Relay',
  pro: 'Pro',
  pro_plus: 'Pro+',
  pro_annual: 'Pro Annual',
  lifetime: 'Lifetime',
  cloud: 'Cloud',
  msp: 'MSP',
  enterprise: 'Enterprise',
};

const FEATURE_LABELS: Record<string, string> = {
  ai_patrol: 'Pulse Patrol',
  ai_alerts: 'Pulse Alert Analysis',
  ai_autofix: 'Patrol Auto-Fix',
  kubernetes_ai: 'Kubernetes Insights',
  update_alerts: 'Update Alerts',
  sso: 'Basic SSO (OIDC)',
  advanced_sso: 'Advanced SSO (SAML/Multi-Provider)',
  rbac: 'Role-Based Access Control (RBAC)',
  audit_logging: 'Audit Logging',
  advanced_reporting: 'PDF/CSV Reporting',
  agent_profiles: 'Centralized Agent Profiles',
  relay: 'Pulse Relay (Remote Access)',
  mobile_app: 'Mobile App Access',
  push_notifications: 'Push Notifications',
  long_term_metrics: 'Extended Metric History',
  multi_user: 'Multi-User Mode',
  white_label: 'White-Label Branding',
  multi_tenant: 'Multi-Tenant Mode',
  unlimited: 'Unlimited Instances',
};

const FEATURE_MIN_TIER_LABELS: Record<string, string> = {
  relay: 'Relay',
  mobile_app: 'Relay',
  push_notifications: 'Relay',
  multi_tenant: 'MSP',
};

export interface LicenseSubscriptionStatusPresentation {
  label: string;
  badgeClass: string;
}

const formatTitleCase = (value: string) =>
  value.replace(/[_-]/g, ' ').replace(/\b\w/g, (match) => match.toUpperCase());

export const getLicenseTierLabel = (tier?: string | null): string => {
  const normalized = (tier || '').trim().toLowerCase();
  if (!normalized) return 'Unknown';
  return TIER_LABELS[normalized] || formatTitleCase(normalized);
};

export const getLicenseFeatureLabel = (feature?: string | null): string => {
  const normalized = (feature || '').trim().toLowerCase();
  if (!normalized) return 'Unknown';
  return FEATURE_LABELS[normalized] || formatTitleCase(normalized);
};

export const getFeatureMinTierLabel = (feature?: string | null): string => {
  const normalized = (feature || '').trim().toLowerCase();
  if (!normalized) return 'Pro';
  return FEATURE_MIN_TIER_LABELS[normalized] || 'Pro';
};

export const getLicenseSubscriptionStatusPresentation = (
  state?: string | null,
): LicenseSubscriptionStatusPresentation => {
  switch ((state || '').trim().toLowerCase()) {
    case 'trial':
      return {
        label: 'Trial',
        badgeClass: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
      };
    case 'active':
      return {
        label: 'Active',
        badgeClass: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
      };
    case 'grace':
      return {
        label: 'Grace Period',
        badgeClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
      };
    case 'suspended':
      return {
        label: 'Suspended',
        badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
      };
    case 'canceled':
    case 'expired':
      return {
        label: 'Expired',
        badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
      };
    default:
      return {
        label: 'Unknown',
        badgeClass: 'bg-surface-alt text-muted',
      };
  }
};
