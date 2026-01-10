import { createSignal, createMemo } from 'solid-js';
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
            features: [],
        });
        setLoaded(true);
    } finally {
        setLoading(false);
    }
}

/**
 * Helper to check if the current license is Pulse Pro (any paid tier).
 */
export const isPro = createMemo(() => {
    const current = licenseStatus();
    return Boolean(current?.valid && current.tier !== 'free');
});

/**
 * @deprecated Use isPro() or hasFeature() instead. Kept for backwards compatibility.
 */
export const isEnterprise = isPro;

/**
 * Check if a specific feature is enabled by the current license.
 */
export function hasFeature(feature: string): boolean {
    const current = licenseStatus();
    if (!current?.valid) return false;
    return current.features.includes(feature);
}

/**
 * Get the full license status.
 */
export { licenseStatus, loading as licenseLoading, loaded as licenseLoaded };
