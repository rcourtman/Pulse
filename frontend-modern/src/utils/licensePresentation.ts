import type { BillingState, HostedOrganizationSummary } from '@/api/billingAdmin';
import type { CommercialMigrationStatus, LicenseStatus } from '@/api/license';
import { CLOUD_PLAN_LABELS } from '@/utils/cloudPlans';

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

export interface LicenseLoadingStateCopy {
  text: string;
}

export interface LicenseInlineNotice {
  tone: string;
  title: string;
  body: string;
}

export interface BillingAdminOrganizationBadge {
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

export const formatLicensePlanVersion = (value?: string | null): string | null => {
  const normalized = (value || '').trim();
  if (!normalized) return null;
  const canonical = CLOUD_PLAN_LABELS[normalized.toLowerCase()];
  if (canonical) return canonical;
  return formatTitleCase(normalized);
};

export const getCommercialMigrationActionText = (action?: string): string => {
  switch (action) {
    case 'retry_activation':
      return 'Retry activation from this instance.';
    case 'use_v6_activation_key':
      return 'Use the current v6 activation key for this purchase.';
    case 'enter_supported_v5_key':
      return 'Retry with the original v5 Pro/Lifetime key from this instance.';
    default:
      return 'Review the activation state from this instance before trying again.';
  }
};

export const getCommercialMigrationNotice = (
  migration?: CommercialMigrationStatus,
): LicenseInlineNotice | null => {
  if (!migration?.state) return null;

  const actionText = getCommercialMigrationActionText(migration.recommended_action);
  const blockedText = 'A new Pro trial stays blocked until this is resolved.';

  if (migration.state === 'pending') {
    let body =
      'Pulse detected a paid v5 license, but the automatic v6 exchange did not complete yet.';
    switch (migration.reason) {
      case 'exchange_rate_limited':
        body = 'Pulse detected a paid v5 license, but the v6 exchange is rate-limited right now.';
        break;
      case 'exchange_conflict':
        body =
          'Pulse detected a paid v5 license, but another v6 activation handoff is still settling.';
        break;
      case 'exchange_unavailable':
      default:
        break;
    }

    return {
      tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
      title: 'v5 license migration pending',
      body: `${body} ${actionText} ${blockedText}`,
    };
  }

  let body = 'Pulse detected a paid v5 license, but it could not be migrated automatically.';
  switch (migration.reason) {
    case 'exchange_invalid':
      body = 'Pulse detected a paid v5 license, but that key was rejected during v6 migration.';
      break;
    case 'exchange_malformed':
      body = 'Pulse detected a v5-looking key, but it is malformed and cannot be migrated.';
      break;
    case 'exchange_revoked':
      body =
        'Pulse detected a paid v5 license, but that key is no longer eligible for automatic migration.';
      break;
    case 'exchange_non_migratable':
      body = 'Pulse detected a paid v5 license, but it is not eligible for automatic v6 migration.';
      break;
    case 'exchange_unsupported':
      body = 'Pulse detected a key that is not a supported v5 Pro/Lifetime migration input.';
      break;
    default:
      break;
  }

  return {
    tone: 'border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 text-red-900 dark:text-red-100',
    title: 'v5 license migration needs attention',
    body: `${body} ${actionText} ${blockedText}`,
  };
};

export const getTrialActivationNotice = (result?: string | null): LicenseInlineNotice | null => {
  switch ((result || '').trim().toLowerCase()) {
    case 'activated':
      return {
        tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
        title: 'Trial activated',
        body: 'Pulse activated the Pro trial for this instance. The entitlement state below is live.',
      };
    case 'invalid':
      return {
        tone: 'border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 text-red-900 dark:text-red-100',
        title: 'Activation link invalid',
        body: 'That activation handoff is invalid or expired. Start the hosted checkout flow again from this Pulse instance.',
      };
    case 'replayed':
      return {
        tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
        title: 'Activation link already used',
        body: 'This checkout handoff was already redeemed. Use the current entitlement state below or start a new checkout if needed.',
      };
    case 'unavailable':
      return {
        tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
        title: 'Activation unavailable',
        body: 'Pulse could not finish activation right now. Retry the return link from checkout or start the flow again from this instance.',
      };
    case 'ineligible':
      return {
        tone: 'border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 text-red-900 dark:text-red-100',
        title: 'Trial not available',
        body: 'This organization is not eligible for another Pro trial. Review the current license state below or upgrade instead.',
      };
    default:
      return null;
  }
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

export const getLicenseStatusLoadingState = (): LicenseLoadingStateCopy => ({
  text: 'Loading license status...',
});

export const getNoActiveProLicenseState = (): LicenseLoadingStateCopy => ({
  text: 'No Pro license is active.',
});

export const getOrganizationBillingLicenseStatusLabel = (
  status?: Pick<LicenseStatus, 'valid' | 'in_grace_period'> | null,
): string => {
  if (!status?.valid) return 'No License';
  return status.in_grace_period ? 'Grace Period' : 'Active';
};

export const getBillingAdminTrialStatus = (
  state?: Pick<BillingState, 'subscription_state' | 'trial_started_at' | 'trial_ends_at'> | null,
): string => {
  if (!state) return 'Loading...';

  const subscriptionState = (state.subscription_state || '').toLowerCase();
  if (subscriptionState !== 'trial' && !state.trial_ends_at && !state.trial_started_at) {
    return 'No trial';
  }

  const started = formatUnixSeconds(state.trial_started_at);
  const ends = formatUnixSeconds(state.trial_ends_at);
  if (subscriptionState === 'trial') {
    return `Trial (ends ${ends})`;
  }
  return `Trial (started ${started}, ends ${ends})`;
};

export const getBillingAdminOrganizationBadges = (
  organization: Pick<HostedOrganizationSummary, 'soft_deleted' | 'suspended'>,
): BillingAdminOrganizationBadge[] => {
  const badges: BillingAdminOrganizationBadge[] = [];
  if (organization.soft_deleted) {
    badges.push({
      label: 'soft-deleted',
      badgeClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200',
    });
  }
  if (organization.suspended && !organization.soft_deleted) {
    badges.push({
      label: 'suspended',
      badgeClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200',
    });
  }
  return badges;
};

export const getBillingAdminStateUpdateSuccessMessage = (
  nextState: 'suspended' | 'active',
): string =>
  nextState === 'suspended' ? 'Organization billing suspended' : 'Organization billing activated';

export const BILLING_ADMIN_EMPTY_STATE = 'No organizations found.';

function formatUnixSeconds(value?: number | null): string {
  if (!value || value <= 0) return 'N/A';
  const date = new Date(value * 1000);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString();
}
