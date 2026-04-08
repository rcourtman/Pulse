import {
  isExternalUpgradeHref,
  resolveUpgradeDestination,
  type UpgradeDestination,
} from '@/utils/upgradeNavigation';

const DEFAULT_PUBLIC_PRICING_URL =
  'https://pulserelay.pro/pricing?utm_source=pulse&utm_medium=app&utm_campaign=upgrade';
export const SELF_HOSTED_PURCHASE_START_PATH = '/auth/license-purchase-start';

export const SELF_HOSTED_PRO_BILLING_ROUTE = '/settings/system/billing';
export const SELF_HOSTED_PRO_BILLING_PLAN_ROUTE = `${SELF_HOSTED_PRO_BILLING_ROUTE}/plan`;
export const SELF_HOSTED_PRO_BILLING_USAGE_ROUTE = `${SELF_HOSTED_PRO_BILLING_ROUTE}/usage`;
export const SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID = 'pulse-pro-plan';
export const SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID = 'pulse-pro-usage';
export const SELF_HOSTED_PRO_BILLING_RECOVERY_SECTION_ID = 'pulse-pro-recovery';
export const SELF_HOSTED_PRO_BILLING_DETAILS_QUERY_PARAM = 'details';
export const SELF_HOSTED_PRO_BILLING_USAGE_DETAILS_QUERY_PARAM =
  SELF_HOSTED_PRO_BILLING_DETAILS_QUERY_PARAM;
export const SELF_HOSTED_PRO_BILLING_PLAN_DETAILS_QUERY_PARAM =
  SELF_HOSTED_PRO_BILLING_DETAILS_QUERY_PARAM;
export const SELF_HOSTED_PRO_BILLING_PLAN_INTENT_QUERY_PARAM = 'intent';
export const SELF_HOSTED_PRO_BILLING_PURCHASE_QUERY_PARAM = 'purchase';
export const SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL = 'counting-rules';
export const SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL = 'recovery';
export const SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT = 'max_monitored_systems';
export const SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED = 'activated';
export const SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED = 'cancelled';
export const SELF_HOSTED_PRO_BILLING_PURCHASE_EXPIRED = 'expired';
export const SELF_HOSTED_PRO_BILLING_PURCHASE_FAILED = 'failed';

export type SelfHostedBillingSection = 'plan' | 'usage';
export type SelfHostedBillingUsageDetail = typeof SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL;
export type SelfHostedBillingPlanDetail = typeof SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL;
export type SelfHostedBillingDetail = SelfHostedBillingUsageDetail | SelfHostedBillingPlanDetail;
export type SelfHostedBillingPlanIntent = typeof SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT;
export type SelfHostedBillingPurchaseArrival =
  | typeof SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED
  | typeof SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED
  | typeof SELF_HOSTED_PRO_BILLING_PURCHASE_EXPIRED
  | typeof SELF_HOSTED_PRO_BILLING_PURCHASE_FAILED;

export const SELF_HOSTED_PRO_BILLING_PLAN_HREF = SELF_HOSTED_PRO_BILLING_PLAN_ROUTE;
export const SELF_HOSTED_PRO_BILLING_USAGE_HREF = SELF_HOSTED_PRO_BILLING_USAGE_ROUTE;
export const SELF_HOSTED_PRO_BILLING_PLAN_RECOVERY_HREF = `${SELF_HOSTED_PRO_BILLING_PLAN_ROUTE}?${SELF_HOSTED_PRO_BILLING_PLAN_DETAILS_QUERY_PARAM}=${SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL}`;
export const SELF_HOSTED_PRO_BILLING_USAGE_COUNTING_RULES_HREF = `${SELF_HOSTED_PRO_BILLING_USAGE_ROUTE}?${SELF_HOSTED_PRO_BILLING_USAGE_DETAILS_QUERY_PARAM}=${SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL}`;
export const SELF_HOSTED_PRO_BILLING_PLAN_MONITORED_SYSTEM_UPGRADE_HREF = `${SELF_HOSTED_PRO_BILLING_PLAN_ROUTE}?${SELF_HOSTED_PRO_BILLING_PLAN_INTENT_QUERY_PARAM}=${SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT}`;

const IN_PRODUCT_PRICING_DESTINATIONS: Record<string, string> = {
  max_monitored_systems: SELF_HOSTED_PRO_BILLING_PLAN_MONITORED_SYSTEM_UPGRADE_HREF,
  cloud: '/cloud',
};

const INTERNAL_HREF_BASE = 'https://pulse.invalid';

function normalizeSettingsLikePath(pathname: string): string {
  const normalized = pathname.trim();
  if (!normalized) return pathname;
  if (normalized.length > 1 && normalized.endsWith('/')) {
    return normalized.replace(/\/+$/, '');
  }
  return normalized;
}

function normalizeSearch(search: string): string {
  return search.startsWith('?') ? search.slice(1) : search;
}

function normalizeHash(hash: string): string {
  const normalized = hash.trim();
  return normalized.startsWith('#') ? normalized.slice(1) : normalized;
}

function billingSearch(search: string): URLSearchParams {
  return new URLSearchParams(normalizeSearch(search));
}

function isSelfHostedBillingPath(pathname: string): boolean {
  const normalized = normalizeSettingsLikePath(pathname);
  return (
    normalized === SELF_HOSTED_PRO_BILLING_ROUTE ||
    normalized === SELF_HOSTED_PRO_BILLING_PLAN_ROUTE ||
    normalized === SELF_HOSTED_PRO_BILLING_USAGE_ROUTE
  );
}

function normalizeFeatureKey(feature: string | null | undefined): string | undefined {
  const normalized = feature?.trim();
  return normalized ? normalized : undefined;
}

export function getSelfHostedPurchaseStartUrl(
  feature?: string | null,
  searchParams?: URLSearchParams,
): string {
  const url = new URL(SELF_HOSTED_PURCHASE_START_PATH, INTERNAL_HREF_BASE);
  if (searchParams) {
    for (const [key, value] of searchParams.entries()) {
      const normalizedValue = value.trim();
      if (normalizedValue) {
        url.searchParams.set(key, normalizedValue);
      }
    }
  }

  const normalizedFeature = normalizeFeatureKey(feature);
  if (normalizedFeature) {
    url.searchParams.set('feature', normalizedFeature);
  }
  return `${url.pathname}${url.search}`;
}

export function resolveSelfHostedPurchaseStartDestination(
  feature?: string | null,
  searchParams?: URLSearchParams,
): UpgradeDestination {
  return resolveUpgradeDestination(getSelfHostedPurchaseStartUrl(feature, searchParams), {
    hardNavigation: true,
    newTab: true,
    preserveOpener: true,
  });
}

export function getInProductPricingDestination(
  feature: string | null | undefined,
): string | undefined {
  const normalizedFeature = normalizeFeatureKey(feature);
  if (!normalizedFeature) return undefined;
  return IN_PRODUCT_PRICING_DESTINATIONS[normalizedFeature];
}

export function getPublicPricingUrl(feature?: string | null): string {
  const url = new URL(DEFAULT_PUBLIC_PRICING_URL);
  const normalizedFeature = normalizeFeatureKey(feature);
  if (normalizedFeature) {
    url.searchParams.set('feature', normalizedFeature);
  }
  return url.toString();
}

export function getUpgradeFallbackDestination(feature?: string | null): string {
  return getInProductPricingDestination(feature) || getSelfHostedPurchaseStartUrl(feature);
}

export function getSelfHostedBillingUsageDetail(
  search: string,
): SelfHostedBillingUsageDetail | null {
  const detail = billingSearch(search)
    .get(SELF_HOSTED_PRO_BILLING_USAGE_DETAILS_QUERY_PARAM)
    ?.trim();
  return detail === SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL ? detail : null;
}

export function getSelfHostedBillingPlanDetail(search: string): SelfHostedBillingPlanDetail | null {
  const detail = billingSearch(search)
    .get(SELF_HOSTED_PRO_BILLING_PLAN_DETAILS_QUERY_PARAM)
    ?.trim();
  return detail === SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL ? detail : null;
}

export function getSelfHostedBillingPlanIntent(search: string): SelfHostedBillingPlanIntent | null {
  const intent = billingSearch(search).get(SELF_HOSTED_PRO_BILLING_PLAN_INTENT_QUERY_PARAM)?.trim();
  return intent === SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT ? intent : null;
}

export function getSelfHostedBillingPurchaseArrival(
  search: string,
): SelfHostedBillingPurchaseArrival | null {
  const purchase = billingSearch(search).get(SELF_HOSTED_PRO_BILLING_PURCHASE_QUERY_PARAM)?.trim();
  switch (purchase) {
    case SELF_HOSTED_PRO_BILLING_PURCHASE_ACTIVATED:
    case SELF_HOSTED_PRO_BILLING_PURCHASE_CANCELLED:
    case SELF_HOSTED_PRO_BILLING_PURCHASE_EXPIRED:
    case SELF_HOSTED_PRO_BILLING_PURCHASE_FAILED:
      return purchase;
    default:
      return null;
  }
}

export function getSelfHostedBillingHref(
  section: SelfHostedBillingSection,
  options: {
    detail?: SelfHostedBillingDetail | null;
    intent?: SelfHostedBillingPlanIntent | null;
    purchase?: SelfHostedBillingPurchaseArrival | null;
  } = {},
): string {
  const baseRoute =
    section === 'usage' ? SELF_HOSTED_PRO_BILLING_USAGE_ROUTE : SELF_HOSTED_PRO_BILLING_PLAN_ROUTE;
  const params = new URLSearchParams();

  if (section === 'usage' && options.detail === SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL) {
    params.set(
      SELF_HOSTED_PRO_BILLING_USAGE_DETAILS_QUERY_PARAM,
      SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL,
    );
  }

  if (section === 'plan' && options.intent === SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT) {
    params.set(
      SELF_HOSTED_PRO_BILLING_PLAN_INTENT_QUERY_PARAM,
      SELF_HOSTED_PRO_BILLING_MONITORED_SYSTEM_INTENT,
    );
  }

  if (section === 'plan' && options.detail === SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL) {
    params.set(
      SELF_HOSTED_PRO_BILLING_PLAN_DETAILS_QUERY_PARAM,
      SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL,
    );
  }

  if (options.purchase) {
    params.set(SELF_HOSTED_PRO_BILLING_PURCHASE_QUERY_PARAM, options.purchase);
  }

  const search = params.toString();
  return `${baseRoute}${search ? `?${search}` : ''}`;
}

export function resolveSelfHostedBillingSection(
  pathname: string,
  search = '',
  hash = '',
): SelfHostedBillingSection {
  const normalizedPath = normalizeSettingsLikePath(pathname);

  if (normalizedPath === SELF_HOSTED_PRO_BILLING_USAGE_ROUTE) {
    return 'usage';
  }
  if (normalizedPath === SELF_HOSTED_PRO_BILLING_PLAN_ROUTE) {
    return 'plan';
  }
  if (normalizedPath !== SELF_HOSTED_PRO_BILLING_ROUTE) {
    return 'plan';
  }

  const normalizedHash = normalizeHash(hash);
  if (normalizedHash === SELF_HOSTED_PRO_BILLING_USAGE_SECTION_ID) {
    return 'usage';
  }
  if (normalizedHash === SELF_HOSTED_PRO_BILLING_PLAN_SECTION_ID) {
    return 'plan';
  }
  if (normalizedHash === SELF_HOSTED_PRO_BILLING_RECOVERY_SECTION_ID) {
    return 'plan';
  }
  if (getSelfHostedBillingUsageDetail(search) === SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL) {
    return 'usage';
  }
  if (getSelfHostedBillingPlanDetail(search) === SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL) {
    return 'plan';
  }
  return 'plan';
}

export function resolveCanonicalSelfHostedBillingHref(
  pathname: string,
  search = '',
  hash = '',
): string | null {
  const normalizedPath = normalizeSettingsLikePath(pathname);
  if (!isSelfHostedBillingPath(normalizedPath)) {
    return null;
  }

  const section = resolveSelfHostedBillingSection(normalizedPath, search, hash);
  const normalizedHash = normalizeHash(hash);
  const usageDetail =
    section === 'usage' &&
    getSelfHostedBillingUsageDetail(search) === SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL
      ? SELF_HOSTED_PRO_BILLING_COUNTING_RULES_DETAIL
      : null;
  const planIntent = section === 'plan' ? getSelfHostedBillingPlanIntent(search) : null;
  const planDetail =
    section === 'plan' &&
    (getSelfHostedBillingPlanDetail(search) === SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL ||
      normalizedHash === SELF_HOSTED_PRO_BILLING_RECOVERY_SECTION_ID)
      ? SELF_HOSTED_PRO_BILLING_RECOVERY_DETAIL
      : null;
  const purchase = section === 'plan' ? getSelfHostedBillingPurchaseArrival(search) : null;

  return getSelfHostedBillingHref(section, {
    detail: section === 'usage' ? usageDetail : planDetail,
    intent: planIntent,
    purchase,
  });
}

export function scopeSelfHostedBillingDestination(
  destination: UpgradeDestination,
  section: SelfHostedBillingSection,
  options: {
    detail?: SelfHostedBillingDetail | null;
    intent?: SelfHostedBillingPlanIntent | null;
    purchase?: SelfHostedBillingPurchaseArrival | null;
  } = {},
): UpgradeDestination {
  if (destination.external) {
    return destination;
  }

  const url = new URL(destination.href, INTERNAL_HREF_BASE);
  if (!isSelfHostedBillingPath(url.pathname)) {
    return destination;
  }

  return {
    ...destination,
    href: getSelfHostedBillingHref(section, options),
  };
}

export function getPricingRouteDestination(search: string): string {
  const params = new URLSearchParams(search.startsWith('?') ? search.slice(1) : search);
  const feature = params.get('feature');
  const inProductDestination = getInProductPricingDestination(feature);
  if (inProductDestination) {
    return inProductDestination;
  }

  if (normalizeFeatureKey(feature)) {
    return getSelfHostedPurchaseStartUrl(feature, params);
  }

  const url = new URL(DEFAULT_PUBLIC_PRICING_URL);
  for (const [key, value] of params.entries()) {
    const normalizedValue = value.trim();
    if (normalizedValue) {
      url.searchParams.set(key, normalizedValue);
    }
  }
  return url.toString();
}

export function isExternalPricingDestination(destination: string): boolean {
  return isExternalUpgradeHref(destination);
}

export function isSelfHostedPurchaseStartDestination(destination: string): boolean {
  if (isExternalUpgradeHref(destination)) {
    return false;
  }

  try {
    return new URL(destination, INTERNAL_HREF_BASE).pathname === SELF_HOSTED_PURCHASE_START_PATH;
  } catch {
    return false;
  }
}

export function handoffToExternalPricing(destination: string): void {
  window.location.replace(destination);
}
