import { createSignal, createMemo, createRoot } from 'solid-js';
import { LicenseAPI, type LicenseStatus } from '@/api/license';
import { logger } from '@/utils/logger';

// Reactive signals for license status
const [licenseStatus, setLicenseStatus] = createSignal<LicenseStatus | null>(null);
const [loading, setLoading] = createSignal(false);
const [loaded, setLoaded] = createSignal(false);

/**
 * Load the license status from the server.
 */
export async function loadLicenseStatus(force = false): Promise<void> {
    if (loaded() && !force) return;

    setLoading(true);
    try {
        const status = await LicenseAPI.getStatus();
        setLicenseStatus(status);
        setLoaded(true);
        logger.debug('[licenseStore] License status loaded', { tier: status.tier, valid: status.valid });
    } catch (err) {
        logger.error('[licenseStore] Failed to load license status', err);
        // Fallback to free tier on error to avoid breaking UI
        setLicenseStatus({
            valid: false,
            tier: 'free',
            is_lifetime: false,
            days_remaining: 0,
            features: ['update_alerts', 'sso', 'ai_patrol'],
        });
        setLoaded(true);
    } finally {
        setLoading(false);
    }
}

/**
 * Helper to check if the current license is Pulse Pro (any paid tier).
 */
export const isPro = createRoot(() =>
    createMemo(() => {
        const current = licenseStatus();
        return Boolean(current?.valid && current.tier !== 'free');
    })
);

/**
 * Check if a specific feature is enabled by the current license.
 * Free tier features (e.g., ai_patrol) are available even without a valid Pro license.
 */
export function hasFeature(feature: string): boolean {
    const current = licenseStatus();
    if (!current) return false;
    return current.features.includes(feature);
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

/**
 * Get the full license status.
 */
export { licenseStatus, loading as licenseLoading, loaded as licenseLoaded };
