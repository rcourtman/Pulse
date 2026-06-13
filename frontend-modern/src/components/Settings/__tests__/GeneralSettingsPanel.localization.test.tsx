import { fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DEFAULT_LOCALE, getActiveLocale, setActiveLocale } from '@/i18n';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { GeneralSettingsPanel, type GeneralSettingsPanelProps } from '../GeneralSettingsPanel';

function renderGeneralSettingsPanel() {
  const [themePreference, setThemePreference] = createSignal<'light' | 'dark' | 'system'>('system');
  const [pvePollingInterval, setPVEPollingInterval] = createSignal(30);
  const [pvePollingSelection, setPVEPollingSelection] = createSignal<number | 'custom'>(30);
  const [pvePollingCustomSeconds, setPVEPollingCustomSeconds] = createSignal(30);
  const [, setHasUnsavedChanges] = createSignal(false);
  const [telemetryEnabled] = createSignal(true);
  const [disableDockerUpdateActions] = createSignal(false);

  const props: GeneralSettingsPanelProps = {
    darkMode: () => false,
    themePreference,
    setThemePreference,
    pvePollingInterval,
    setPVEPollingInterval,
    pvePollingSelection,
    setPVEPollingSelection,
    pvePollingCustomSeconds,
    setPVEPollingCustomSeconds,
    pvePollingEnvLocked: () => false,
    setHasUnsavedChanges,
    telemetryEnabled,
    telemetryEnabledLocked: () => false,
    savingTelemetry: () => false,
    handleTelemetryEnabledChange: vi.fn(),
    telemetryPreview: () => null,
    telemetryPreviewEnabled: telemetryEnabled,
    telemetryPreviewPayload: () => '',
    loadingTelemetryPreview: () => false,
    resettingTelemetryInstallID: () => false,
    handleLoadTelemetryPreview: vi.fn(),
    handleCopyTelemetryPreview: vi.fn(),
    handleResetTelemetryInstallID: vi.fn(),
    disableDockerUpdateActions,
    disableDockerUpdateActionsLocked: () => false,
    savingDockerUpdateActions: () => false,
    handleDisableDockerUpdateActionsChange: vi.fn(),
  };

  const result = render(() => <GeneralSettingsPanel {...props} />);

  return result;
}

describe('GeneralSettingsPanel localization', () => {
  beforeEach(() => {
    localStorage.removeItem(STORAGE_KEYS.LOCALE_PREFERENCE);
    setActiveLocale(DEFAULT_LOCALE);
  });

  afterEach(() => {
    localStorage.removeItem(STORAGE_KEYS.LOCALE_PREFERENCE);
    setActiveLocale(DEFAULT_LOCALE);
  });

  it('surfaces the shared locale preference in Appearance settings', () => {
    renderGeneralSettingsPanel();

    expect(screen.getByText('Appearance')).toBeInTheDocument();
    expect(screen.getByText('Language')).toBeInTheDocument();
    expect(
      screen.getByText(/Commands, resource names, and API fields stay unchanged/),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'English' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Deutsch' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Español' })).toBeInTheDocument();
  });

  it('persists locale selection and immediately re-renders localized settings copy', () => {
    renderGeneralSettingsPanel();

    fireEvent.click(screen.getByRole('button', { name: 'Deutsch' }));

    expect(getActiveLocale()).toBe('de');
    expect(localStorage.getItem(STORAGE_KEYS.LOCALE_PREFERENCE)).toBe('de');
    expect(screen.getByText('Darstellung')).toBeInTheDocument();
    expect(screen.getByText('Sprache')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Deutsch' })).toHaveAttribute('aria-pressed', 'true');
  });
});
