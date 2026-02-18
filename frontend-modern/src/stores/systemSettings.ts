/**
 * System Settings Store
 * 
 * Provides reactive access to server-wide system settings.
 * Used to control features like Docker update buttons based on server configuration.
 */

import { createSignal } from 'solid-js';
import { SettingsAPI } from '@/api/settings';
import { logger } from '@/utils/logger';
import type { SystemConfig } from '@/types/config';

// Server-side setting to hide Docker update buttons while still detecting updates
const [disableDockerUpdateActions, setDisableDockerUpdateActions] = createSignal(false);
// Server-side setting to disable all legacy frontend route redirects
const [disableLegacyRouteRedirects, setDisableLegacyRouteRedirects] = createSignal(false);
// Server-side setting to reduce proactive Pro prompts (paywalls still appear when accessing gated features)
const [reduceProUpsellNoise, setReduceProUpsellNoise] = createSignal(false);
// Server-side setting to disable local-only upgrade UX metrics collection
const [disableLocalUpgradeMetrics, setDisableLocalUpgradeMetrics] = createSignal(false);

// Track if settings have been loaded
const [systemSettingsLoaded, setSystemSettingsLoaded] = createSignal(false);

/**
 * Update the system settings store from an existing settings object.
 * Call this after you've already fetched system settings (e.g., for theme loading).
 */
export function updateSystemSettingsFromResponse(settings: SystemConfig): void {
    setDisableDockerUpdateActions(settings.disableDockerUpdateActions ?? false);
    setDisableLegacyRouteRedirects(settings.disableLegacyRouteRedirects ?? false);
    setReduceProUpsellNoise(settings.reduceProUpsellNoise ?? false);
    setDisableLocalUpgradeMetrics(settings.disableLocalUpgradeMetrics ?? false);
    setSystemSettingsLoaded(true);
    logger.debug('System settings updated from response', {
        disableDockerUpdateActions: settings.disableDockerUpdateActions,
        disableLegacyRouteRedirects: settings.disableLegacyRouteRedirects,
        reduceProUpsellNoise: settings.reduceProUpsellNoise,
        disableLocalUpgradeMetrics: settings.disableLocalUpgradeMetrics,
    });
}

/**
 * Load system settings from the server.
 * Use this only when you need settings but haven't fetched them elsewhere.
 * Prefer `updateSystemSettingsFromResponse` when you already have the settings.
 */
export async function loadSystemSettings(): Promise<void> {
    try {
        const settings = await SettingsAPI.getSystemSettings();
        updateSystemSettingsFromResponse(settings);
    } catch (err) {
        logger.warn('Failed to load system settings, using defaults', err);
        // Use safe defaults
        setDisableDockerUpdateActions(false);
        setDisableLegacyRouteRedirects(false);
        setReduceProUpsellNoise(false);
        setDisableLocalUpgradeMetrics(false);
        setSystemSettingsLoaded(true);
    }
}

/**
 * Check if Docker update actions (buttons) should be hidden.
 * Returns true if the server has configured to hide update buttons.
 */
export function shouldHideDockerUpdateActions(): boolean {
    return disableDockerUpdateActions();
}

/**
 * Check if legacy frontend route redirects should be disabled globally.
 */
export function shouldDisableLegacyRouteRedirects(): boolean {
    return disableLegacyRouteRedirects();
}

export function shouldReduceProUpsellNoise(): boolean {
    return reduceProUpsellNoise();
}

export function shouldDisableLocalUpgradeMetrics(): boolean {
    return disableLocalUpgradeMetrics();
}

/**
 * Check if system settings have been loaded from the server.
 */
export function areSystemSettingsLoaded(): boolean {
    return systemSettingsLoaded();
}

/**
 * Mark settings as loaded with default values.
 * Call this when settings fail to load but the app should continue working.
 */
export function markSystemSettingsLoadedWithDefaults(): void {
    setDisableDockerUpdateActions(false);
    setDisableLegacyRouteRedirects(false);
    setReduceProUpsellNoise(false);
    setDisableLocalUpgradeMetrics(false);
    setSystemSettingsLoaded(true);
    logger.debug('System settings marked as loaded with defaults');
}

/**
 * Update the local state when settings change (e.g., from Settings page).
 */
export function updateDockerUpdateActionsSetting(disabled: boolean): void {
    setDisableDockerUpdateActions(disabled);
}

export function updateLegacyRouteRedirectsSetting(disabled: boolean): void {
    setDisableLegacyRouteRedirects(disabled);
}

export function updateReduceProUpsellNoiseSetting(enabled: boolean): void {
    setReduceProUpsellNoise(enabled);
}

export function updateDisableLocalUpgradeMetricsSetting(disabled: boolean): void {
    setDisableLocalUpgradeMetrics(disabled);
}
