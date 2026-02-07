import { beforeEach, describe, expect, it } from 'vitest';
import {
  markSystemSettingsLoadedWithDefaults,
  shouldDisableLegacyRouteRedirects,
  shouldHideDockerUpdateActions,
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
    });

    expect(shouldHideDockerUpdateActions()).toBe(true);
    expect(shouldDisableLegacyRouteRedirects()).toBe(true);
  });

  it('resets route and docker feature flags to safe defaults', () => {
    updateSystemSettingsFromResponse({
      autoUpdateEnabled: false,
      disableDockerUpdateActions: true,
      disableLegacyRouteRedirects: true,
    });

    markSystemSettingsLoadedWithDefaults();
    expect(shouldHideDockerUpdateActions()).toBe(false);
    expect(shouldDisableLegacyRouteRedirects()).toBe(false);
  });
});
