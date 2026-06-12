import { describe, expect, it } from 'vitest';
import {
  getSettingsConfigurationLoadingState,
  getSettingsLoadingState,
  getSettingsSearchEmptyState,
  getSettingsShellCopy,
  getSettingsUnsavedChangesBanner,
  SETTINGS_SHELL_COPY,
} from '@/utils/settingsShellPresentation';

describe('settingsShellPresentation', () => {
  it('returns canonical settings shell framing copy', () => {
    const expectedEnglishCopy = {
      navigationAriaLabel: 'Settings navigation',
      navigationTitle: 'Settings',
      searchPlaceholder: 'Search settings...',
      searchShortcutHint: undefined,
      mobileBackLabel: 'Settings',
      collapseSidebarLabel: 'Collapse settings navigation',
      expandSidebarLabel: 'Expand settings navigation',
    };
    expect(SETTINGS_SHELL_COPY).toEqual(expectedEnglishCopy);
    expect(getSettingsShellCopy('en')).toEqual(expectedEnglishCopy);
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
    expect(getSettingsSearchEmptyState('alertas', 'es')).toEqual({
      text: 'No se encontraron ajustes para "alertas"',
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

  it('returns German settings shell copy from the first-wave catalog', () => {
    expect(getSettingsShellCopy('de')).toMatchObject({
      navigationAriaLabel: 'Einstellungsnavigation',
      navigationTitle: 'Einstellungen',
      searchPlaceholder: 'Einstellungen suchen...',
    });
    expect(getSettingsUnsavedChangesBanner('de')).toMatchObject({
      title: 'Nicht gespeicherte Aenderungen',
      saveLabel: 'Aenderungen speichern',
    });
    expect(getSettingsLoadingState('de')).toEqual({
      text: 'Einstellungen werden geladen...',
    });
    expect(getSettingsConfigurationLoadingState('de')).toEqual({
      text: 'Konfiguration wird geladen...',
    });
  });
});
