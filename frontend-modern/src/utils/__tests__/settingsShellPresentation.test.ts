import { describe, expect, it } from 'vitest';
import {
  getSettingsConfigurationLoadingState,
  getSettingsLoadingState,
  getSettingsSearchEmptyState,
  getSettingsUnsavedChangesBanner,
  SETTINGS_SHELL_COPY,
} from '@/utils/settingsShellPresentation';

describe('settingsShellPresentation', () => {
  it('returns canonical settings shell framing copy', () => {
    expect(SETTINGS_SHELL_COPY).toEqual({
      navigationAriaLabel: 'Settings navigation',
      navigationTitle: 'Settings',
      searchPlaceholder: 'Search settings...',
      searchShortcutHint: 'Any key',
      mobileBackLabel: 'Settings',
      collapseSidebarLabel: 'Collapse settings navigation',
      expandSidebarLabel: 'Expand settings navigation',
    });
    expect(getSettingsUnsavedChangesBanner()).toEqual({
      title: 'Unsaved changes',
      description: 'Your changes will be lost if you navigate away.',
      saveLabel: 'Save Changes',
      discardLabel: 'Discard',
    });
  });

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
