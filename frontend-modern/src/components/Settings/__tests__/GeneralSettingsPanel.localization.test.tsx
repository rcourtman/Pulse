import { fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DEFAULT_LOCALE, getActiveLocale, setActiveLocale } from '@/i18n';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { GeneralSettingsPanel, type GeneralSettingsPanelProps } from '../GeneralSettingsPanel';

function renderGeneralSettingsPanel(overrides: Partial<GeneralSettingsPanelProps> = {}) {
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
    ...overrides,
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
    expect(screen.getByText('Usage data and privacy')).toBeInTheDocument();
    expect(screen.getByText('Outbound usage telemetry')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Preview payload' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reset ID' })).toBeInTheDocument();
    expect(screen.getByText('Monitoring cadence')).toBeInTheDocument();
    expect(screen.getByText('Current cadence: 30 seconds (under a minute)')).toBeInTheDocument();
    expect(screen.getByText('Docker / Podman updates')).toBeInTheDocument();
    expect(screen.getByText('PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=true')).toBeInTheDocument();
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
    expect(screen.getByText('Nutzungsdaten und Datenschutz')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Payload anzeigen' })).toBeInTheDocument();
    expect(screen.getByText('Monitoring-Takt')).toBeInTheDocument();
    expect(
      screen.getByText('Aktueller Takt: 30 Sekunden (unter einer Minute)'),
    ).toBeInTheDocument();
    expect(screen.getByText('Docker / Podman-Updates')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Deutsch' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('re-renders the same general settings journey in Spanish without translating identifiers', () => {
    renderGeneralSettingsPanel();

    fireEvent.click(screen.getByRole('button', { name: 'Español' }));

    expect(getActiveLocale()).toBe('es');
    expect(screen.getByText('Apariencia')).toBeInTheDocument();
    expect(screen.getByText('Idioma')).toBeInTheDocument();
    expect(screen.getByText('Datos de uso y privacidad')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Vista previa del payload' })).toBeInTheDocument();
    expect(screen.getByText('Cadencia de supervisión')).toBeInTheDocument();
    expect(
      screen.getByText('Cadencia actual: 30 segundos (menos de un minuto)'),
    ).toBeInTheDocument();
    expect(screen.getByText('Actualizaciones de Docker / Podman')).toBeInTheDocument();
    expect(screen.getByText(/API y CPU/)).toBeInTheDocument();
    expect(screen.getByText('PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=true')).toBeInTheDocument();
  });

  it('localizes telemetry preview state while preserving JSON as a machine-facing token', () => {
    renderGeneralSettingsPanel({
      telemetryPreview: () => ({ enabled: false }) as never,
      telemetryPreviewEnabled: () => false,
      telemetryPreviewPayload: () => '{"install_id":"abc"}',
    });

    fireEvent.click(screen.getByRole('button', { name: 'Español' }));

    expect(screen.getByRole('button', { name: 'Actualizar payload' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Copiar JSON' })).toBeInTheDocument();
    expect(screen.getByText('Payload de heartbeat actual')).toBeInTheDocument();
    expect(screen.getByLabelText('Vista previa del payload de telemetría')).toHaveTextContent(
      '"install_id":"abc"',
    );
  });
});
