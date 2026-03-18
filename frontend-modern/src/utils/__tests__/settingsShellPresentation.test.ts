import { describe, expect, it } from 'vitest';
import {
  getSettingsConfigurationLoadingState,
  getSettingsLoadingState,
  getSettingsSearchEmptyState,
} from '@/utils/settingsShellPresentation';

describe('settingsShellPresentation', () => {
  it('returns canonical settings search empty-state copy', () => {
    expect(getSettingsSearchEmptyState('alerts')).toEqual({
      text: 'No settings found for "alerts"',
    });
  });

  it('returns canonical settings loading copy', () => {
    expect(getSettingsLoadingState()).toEqual({
      text: 'Loading settings...',
    });
    expect(getSettingsConfigurationLoadingState()).toEqual({
      text: 'Loading configuration...',
    });
  });
});
