/**
 * System Settings Store
 *
 * Provides reactive access to server-wide system settings.
 * Used to control features like Docker update buttons based on server configuration.
 */

import { createSignal } from 'solid-js';
import {
  SettingsAPI,
  type RuntimeBrandingResponse,
  type RuntimeDisplayResponse,
} from '@/api/settings';
import { logger } from '@/utils/logger';

// Server-side setting to hide Docker update buttons while still detecting updates
const [disableDockerUpdateActions, setDisableDockerUpdateActions] = createSignal(false);
// Server-side compatibility setting for proactive commercial prompts
const [reduceProUpsellNoise, setReduceProUpsellNoise] = createSignal(false);
const [runtimeBranding, setRuntimeBranding] = createSignal<RuntimeBrandingResponse>({
  enabled: false,
  displayName: '',
  logoDataUrl: '',
});

// Track if settings have been loaded
const [systemSettingsLoaded, setSystemSettingsLoaded] = createSignal(false);

/**
 * Update the shell flags from the authenticated runtime display projection.
 */
export function updateRuntimeDisplayFromResponse(display: RuntimeDisplayResponse): void {
  setDisableDockerUpdateActions(display.disableDockerUpdateActions ?? false);
  setReduceProUpsellNoise(display.reduceProUpsellNoise ?? false);
  setSystemSettingsLoaded(true);
  logger.debug('System settings updated from runtime display response', {
    disableDockerUpdateActions: display.disableDockerUpdateActions,
    reduceProUpsellNoise: display.reduceProUpsellNoise,
  });
}

export async function loadRuntimeBranding(): Promise<void> {
  try {
    updateRuntimeBrandingFromResponse(await SettingsAPI.getRuntimeBranding());
  } catch (err) {
    logger.warn('Failed to load runtime branding, using Pulse defaults', err);
    clearRuntimeBranding();
  }
}

export function updateRuntimeBrandingFromResponse(branding: RuntimeBrandingResponse): void {
  const enabled = branding.enabled === true;
  setRuntimeBranding({
    enabled,
    displayName:
      enabled && typeof branding.displayName === 'string' ? branding.displayName.trim() : '',
    logoDataUrl: enabled && typeof branding.logoDataUrl === 'string' ? branding.logoDataUrl : '',
  });
}

export function clearRuntimeBranding(): void {
  setRuntimeBranding({ enabled: false, displayName: '', logoDataUrl: '' });
}

/**
 * Check if Docker update actions (buttons) should be hidden.
 * Returns true if the server has configured to hide update buttons.
 */
export function shouldHideDockerUpdateActions(): boolean {
  return disableDockerUpdateActions();
}

export function shouldReduceProUpsellNoise(): boolean {
  return reduceProUpsellNoise();
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
  setReduceProUpsellNoise(false);
  setSystemSettingsLoaded(true);
  logger.debug('System settings marked as loaded with defaults');
}

/**
 * Update the local state when settings change (e.g., from Settings page).
 */
export function updateDockerUpdateActionsSetting(disabled: boolean): void {
  setDisableDockerUpdateActions(disabled);
}

export function updateReduceProUpsellNoiseSetting(enabled: boolean): void {
  setReduceProUpsellNoise(enabled);
}

export { runtimeBranding };
