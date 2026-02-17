import { beforeEach, describe, expect, it } from 'vitest';
import {
  markSystemSettingsLoadedWithDefaults,
  shouldDisableLegacyRouteRedirects,
  shouldDisableLocalUpgradeMetrics,
  shouldHideDockerUpdateActions,
  shouldReduceProUpsellNoise,
  shouldShowClassicPlatformShortcuts,
  updateSystemSettingsFromResponse,
} from '@/stores/systemSettings';

describe('systemSettings store', () => {
  beforeEach(() => {
    markSystemSettingsLoadedWithDefaults();
  });

  it('applies route and docker feature flags from API response', () => {
    updateSystemSettingsFromResponse({
      autoUpdateEnabled: false,
      disableDockerUpdateActions: true,
      disableLegacyRouteRedirects: true,
      showClassicPlatformShortcuts: false,
      reduceProUpsellNoise: true,
      disableLocalUpgradeMetrics: true,
    });

    expect(shouldHideDockerUpdateActions()).toBe(true);
    expect(shouldDisableLegacyRouteRedirects()).toBe(true);
    expect(shouldShowClassicPlatformShortcuts()).toBe(false);
    expect(shouldReduceProUpsellNoise()).toBe(true);
    expect(shouldDisableLocalUpgradeMetrics()).toBe(true);
  });

  it('resets route and docker feature flags to safe defaults', () => {
    updateSystemSettingsFromResponse({
      autoUpdateEnabled: false,
      disableDockerUpdateActions: true,
      disableLegacyRouteRedirects: true,
      showClassicPlatformShortcuts: false,
      reduceProUpsellNoise: true,
      disableLocalUpgradeMetrics: true,
    });

    markSystemSettingsLoadedWithDefaults();
    expect(shouldHideDockerUpdateActions()).toBe(false);
    expect(shouldDisableLegacyRouteRedirects()).toBe(false);
    expect(shouldShowClassicPlatformShortcuts()).toBe(true);
    expect(shouldReduceProUpsellNoise()).toBe(false);
    expect(shouldDisableLocalUpgradeMetrics()).toBe(false);
  });
});
