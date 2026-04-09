import type { BillingState, HostedOrganizationSummary } from '@/api/billingAdmin';
import type {
  CommercialMigrationStatus,
  LicenseStatus,
  MonitoredSystemContinuityStatus,
} from '@/api/license';
import { CLOUD_PLAN_LABELS } from '@/utils/cloudPlans';
import {
  formatMonitoredSystemUsageUnavailableMessage,
  getMonitoredSystemLimitUnavailableReason,
  isMonitoredSystemLimitUsageAvailable,
  type MonitoredSystemLimitUsageStatus,
} from '@/utils/monitoredSystemPresentation';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';

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

const NON_DISPLAYABLE_FEATURES = new Set(['multi_user', 'white_label', 'unlimited']);

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

export interface LicenseActionNotice extends LicenseInlineNotice {
  actionLabel: string;
}

export interface BillingAdminOrganizationBadge {
  label: string;
  badgeClass: string;
}

export interface SelfHostedActivationNoticeCopy {
  title: string;
  body: string;
}

export interface SelfHostedRecoveryPresentation {
  disclosureLabel: string;
  disclosureDescription: string;
  fieldLabel: string;
  fieldPlaceholder: string;
  helpTextBeforeTerms: string;
  helpTextAfterTerms: string;
  termsLabel: string;
  activateIdleLabel: string;
  activatePendingLabel: string;
  clearIdleLabel: string;
  clearPendingLabel: string;
  legacyNotice: SelfHostedActivationNoticeCopy;
}

const GRANDFATHERED_V5_PLAN_LABELS: Record<string, string> = {
  v5_lifetime_grandfathered: 'V5 Lifetime Grandfathered',
  v5_pro_monthly_grandfathered: 'V5 Pro Monthly (Grandfathered)',
  v5_pro_annual_grandfathered: 'V5 Pro Annual (Grandfathered)',
};

export const getLicenseTierLabel = (tier?: string | null): string => {
  const normalized = (tier || '').trim().toLowerCase();
  if (!normalized) return 'Unknown';
  return TIER_LABELS[normalized] || titleCaseDelimitedLabel(normalized);
};

export const getLicenseFeatureLabel = (feature?: string | null): string => {
  const normalized = (feature || '').trim().toLowerCase();
  if (!normalized) return 'Unknown';
  return FEATURE_LABELS[normalized] || titleCaseDelimitedLabel(normalized);
};

export const isDisplayableLicenseFeature = (feature?: string | null): boolean => {
  const normalized = (feature || '').trim().toLowerCase();
  if (!normalized) return false;
  return !NON_DISPLAYABLE_FEATURES.has(normalized);
};

export const getFeatureMinTierLabel = (feature?: string | null): string => {
  const normalized = (feature || '').trim().toLowerCase();
  if (!normalized) return 'Pro';
  return FEATURE_MIN_TIER_LABELS[normalized] || 'Pro';
};

export const formatLicensePlanVersion = (value?: string | null): string | null => {
  const normalized = (value || '').trim();
  if (!normalized) return null;
  const grandfathered = GRANDFATHERED_V5_PLAN_LABELS[normalized.toLowerCase()];
  if (grandfathered) return grandfathered;
  const canonical = CLOUD_PLAN_LABELS[normalized.toLowerCase()];
  if (canonical) return canonical;
  return titleCaseDelimitedLabel(normalized);
};

export const getGrandfatheredPriceContinuityNotice = (
  planVersion?: string | null,
  subscriptionState?: string | null,
): LicenseInlineNotice | null => {
  const normalizedPlan = (planVersion || '').trim().toLowerCase();
  if (
    normalizedPlan !== 'v5_pro_monthly_grandfathered' &&
    normalizedPlan !== 'v5_pro_annual_grandfathered'
  ) {
    return null;
  }

  const normalizedState = (subscriptionState || '').trim().toLowerCase();
  if (normalizedState !== 'active' && normalizedState !== 'grace') {
    return null;
  }

  return {
    tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
    title: 'Grandfathered v5 pricing',
    body: 'This migrated v5 Pro subscription keeps its existing recurring price until you cancel. If you cancel and return later, current v6 pricing applies.',
  };
};

export const getMonitoredSystemContinuityNotice = (
  continuity?: MonitoredSystemContinuityStatus | null,
  limit?: MonitoredSystemLimitUsageStatus | null,
): LicenseInlineNotice | null => {
  if (!isMonitoredSystemLimitUsageAvailable(limit)) {
    const title =
      continuity?.capture_pending === true
        ? 'Migration continuity verification pending'
        : 'Monitored-system usage unavailable';
    return {
      tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
      title,
      body: formatMonitoredSystemUsageUnavailableMessage(
        getMonitoredSystemLimitUnavailableReason(limit),
      ),
    };
  }

  if (
    continuity &&
    typeof continuity.grandfathered_floor === 'number' &&
    continuity.grandfathered_floor > 0 &&
    continuity.effective_limit > continuity.plan_limit
  ) {
    return {
      tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
      title: 'Grandfathered monitored-system floor',
      body: `This migrated v5 installation keeps an effective monitored-system limit of ${continuity.effective_limit}. The current plan includes ${continuity.plan_limit}, and the observed legacy estate was grandfathered at ${continuity.grandfathered_floor}.`,
    };
  }

  return null;
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
        body: 'That activation handoff is invalid or expired. Return to Pulse Pro settings on this instance and start a fresh secure trial handoff.',
      };
    case 'replayed':
      return {
        tone: 'border-sky-200 dark:border-sky-900 bg-sky-50 dark:bg-sky-900 text-sky-900 dark:text-sky-100',
        title: 'Trial already activated',
        body: 'This activation handoff was already redeemed for this instance. Use the current entitlement state below as the source of truth.',
      };
    case 'unavailable':
      return {
        tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
        title: 'Activation unavailable',
        body: 'Pulse could not finish activation right now. Refresh the billing state below, then retry the return link or start the secure trial handoff again from this instance if needed.',
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

export const getPurchaseActivationNotice = (result?: string | null): LicenseInlineNotice | null => {
  switch ((result || '').trim().toLowerCase()) {
    case 'activated':
      return {
        tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
        title: 'Pulse Pro activated',
        body: 'Pulse finished checkout and activated this instance automatically. The plan state below is live.',
      };
    case 'cancelled':
      return {
        tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
        title: 'Checkout cancelled',
        body: 'Checkout was cancelled before completion. The current plan state below is unchanged until you start the upgrade again.',
      };
    case 'expired':
      return {
        tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
        title: 'Upgrade return expired',
        body: 'That secure checkout return link expired or was already used. Start the upgrade again from this instance if you still need it.',
      };
    case 'failed':
      return {
        tone: 'border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 text-red-900 dark:text-red-100',
        title: 'Activation needs attention',
        body: 'Checkout completed, but Pulse could not finish local activation automatically. Review the plan state below, then open recovery if you already have a key from this purchase.',
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

export const getTrialEndedProLicenseNotice = (): LicenseActionNotice => ({
  tone: 'border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 text-red-900 dark:text-red-100',
  title: 'Your Pro trial has ended',
  body: 'Upgrade to keep Pro features.',
  actionLabel: 'View Pro plans',
});

export const getInactiveProUpsellNotice = (): LicenseActionNotice => ({
  tone: 'border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200',
  title: 'Upgrade to Pro',
  body: 'Unlock Pulse Patrol, alert analysis, auto-fix, and more.',
  actionLabel: 'View Pro plans',
});

export const SELF_HOSTED_RECOVERY_PRESENTATION: SelfHostedRecoveryPresentation = {
  disclosureLabel: 'Redeem existing key',
  disclosureDescription:
    'Use this only if you already have a Pulse Pro key or need to recover a legacy self-hosted purchase on this instance.',
  fieldLabel: 'Pulse Pro Key',
  fieldPlaceholder: 'Paste your license key or activation key',
  helpTextBeforeTerms:
    'Paste the Pulse v6 activation key shown on the hosted checkout success page. A backup copy is also sent by email, but the hosted success page is the primary handoff. You can also paste a legacy Pulse v5 Pro/Lifetime license key and Pulse will exchange it automatically during activation when migration is available. By activating a license, you agree to the',
  helpTextAfterTerms: '.',
  termsLabel: 'Terms of Service',
  activateIdleLabel: 'Activate License',
  activatePendingLabel: 'Activating...',
  clearIdleLabel: 'Clear License',
  clearPendingLabel: 'Clearing...',
  legacyNotice: {
    title: 'Legacy v5 license detected',
    body: 'Pulse will try to exchange this key into the v6 activation model automatically. If the exchange cannot complete immediately, retry from this panel or use the self-serve retrieval flow to get the current v6 activation key.',
  },
};

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
