import { createSignal, createMemo, createRoot } from 'solid-js';
import { LicenseAPI, type LicenseEntitlements } from '@/api/license';
import { eventBus } from '@/stores/events';
import { logger } from '@/utils/logger';

// Reactive signals for entitlements (canonical gating source).
const [entitlements, setEntitlements] = createSignal<LicenseEntitlements | null>(null);
const [loading, setLoading] = createSignal(false);
const [loaded, setLoaded] = createSignal(false);

/**
 * Load the entitlements payload from the server.
 *
 * This is the canonical feature-gating source (do not gate on tier strings).
 */
export async function loadLicenseStatus(force = false): Promise<void> {
    if (loaded() && !force) return;

    setLoading(true);
    try {
        const next = await LicenseAPI.getEntitlements();
        setEntitlements(next);
        setLoaded(true);
        logger.debug('[licenseStore] Entitlements loaded', { tier: next.tier, sub_state: next.subscription_state });
    } catch (err) {
        logger.error('[licenseStore] Failed to load entitlements', err);
        // Best-effort fallback to avoid breaking the UI.
        setEntitlements({
            capabilities: ['update_alerts', 'sso', 'ai_patrol'],
            limits: [],
            // Match backend behavior when no license is present.
            subscription_state: 'expired',
            upgrade_reasons: [],
            tier: 'free',
            hosted_mode: false,
        });
        setLoaded(true);
    } finally {
        setLoading(false);
    }
}

/**
 * Start a Pro trial for the current org, then refresh entitlements.
 */
export async function startProTrial(): Promise<void> {
    const res = await LicenseAPI.startTrial();
    if (!res.ok) {
        throw new Error(`Failed to start trial (status ${res.status})`);
    }
    await loadLicenseStatus(true);
}

/**
 * Helper to check if the current license is Pulse Pro (any paid tier).
 */
export const isPro = createRoot(() =>
    createMemo(() => {
        const current = entitlements();
        return Boolean(current && current.tier !== 'free');
    })
);

/**
 * Check if a specific feature is enabled by the current license.
 */
export function hasFeature(feature: string): boolean {
    const current = entitlements();
    if (!current) return false;
    return current.capabilities.includes(feature);
}

export function isMultiTenantEnabled(): boolean {
    return hasFeature('multi_tenant');
}

export function isHostedModeEnabled(): boolean {
    return Boolean(entitlements()?.hosted_mode);
}

export function getUpgradeReason(key: string) {
    const current = entitlements();
    if (!current?.upgrade_reasons?.length) return undefined;
    return current.upgrade_reasons.find((reason) => reason.key === key);
}

export function getUpgradeActionUrl(key: string): string | undefined {
    return getUpgradeReason(key)?.action_url;
}

export function getFirstUpgradeActionUrl(): string | undefined {
    const current = entitlements();
    return current?.upgrade_reasons?.[0]?.action_url;
}

export function getUpgradeActionUrlOrFallback(key: string): string {
    return getUpgradeActionUrl(key) || getFirstUpgradeActionUrl() || `/pricing?feature=${encodeURIComponent(key)}`;
}

export function getLimit(key: string) {
    const current = entitlements();
    if (!current?.limits?.length) return undefined;
    return current.limits.find((limit) => limit.key === key);
}

/**
 * Max free range in days — must match backend maxFreeDuration (7 * 24 * time.Hour).
 * Any range exceeding this requires the long_term_metrics feature.
 */
const MAX_FREE_DAYS = 7;

function parseRangeDays(range: string): number {
    const match = range.match(/^(\d+)(h|d)$/);
    if (!match) return 0;
    const val = parseInt(match[1], 10);
    return match[2] === 'd' ? val : val / 24;
}

/**
 * Check if a time range requires Pro and the user doesn't have it.
 * Ranges exceeding 7 days require the long_term_metrics feature.
 */
export function isRangeLocked(range: string): boolean {
    return parseRangeDays(range) > MAX_FREE_DAYS && !hasFeature('long_term_metrics');
}

// Ensure org-scoped entitlements do not leak across tenant switches.
eventBus.on('org_switched', () => {
    setEntitlements(null);
    setLoaded(false);
    void loadLicenseStatus(true);
});

/**
 * Get the full license status.
 */
export { entitlements, entitlements as licenseStatus, loading as licenseLoading, loaded as licenseLoaded };
