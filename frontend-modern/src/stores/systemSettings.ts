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

// Track if settings have been loaded
const [systemSettingsLoaded, setSystemSettingsLoaded] = createSignal(false);

/**
 * Update the system settings store from an existing settings object.
 * Call this after you've already fetched system settings (e.g., for theme loading).
 */
export function updateSystemSettingsFromResponse(settings: SystemConfig): void {
    setDisableDockerUpdateActions(settings.disableDockerUpdateActions ?? false);
    setSystemSettingsLoaded(true);
    logger.debug('System settings updated from response', {
        disableDockerUpdateActions: settings.disableDockerUpdateActions
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
    setSystemSettingsLoaded(true);
    logger.debug('System settings marked as loaded with defaults');
}

/**
 * Update the local state when settings change (e.g., from Settings page).
 */
export function updateDockerUpdateActionsSetting(disabled: boolean): void {
    setDisableDockerUpdateActions(disabled);
}
