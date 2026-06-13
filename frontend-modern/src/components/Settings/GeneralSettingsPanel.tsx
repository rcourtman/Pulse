import { Component, Show, Accessor, Setter } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Button } from '@/components/shared/Button';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { Toggle } from '@/components/shared/Toggle';
import { EnvironmentLockBadge } from '@/components/shared/EnvironmentLockBadge';
import { FilterButtonGroup, type FilterOption } from '@/components/shared/FilterButtonGroup';
import type { TelemetryPreviewResponse } from '@/api/settings';
import { DockerRuntimeSettingsCard } from './DockerRuntimeSettingsCard';
import Sun from 'lucide-solid/icons/sun';
import Moon from 'lucide-solid/icons/moon';
import Languages from 'lucide-solid/icons/languages';
import Thermometer from 'lucide-solid/icons/thermometer';
import Maximize2 from 'lucide-solid/icons/maximize-2';
import {
  activeLocale,
  setLocalePreference,
  SUPPORTED_LOCALE_LABELS,
  SUPPORTED_LOCALES,
  t,
  type SupportedLocale,
} from '@/i18n';
import { temperatureStore } from '@/utils/temperature';
import { layoutStore } from '@/utils/layout';
import {
  PVE_POLLING_MAX_SECONDS,
  PVE_POLLING_MIN_SECONDS,
  getPvePollingCadenceSummary,
  getPvePollingCustomOption,
  getPvePollingPresetOptions,
} from '@/utils/systemSettingsPresentation';
import { PRIVACY_DOC_URL } from '@/utils/docsLinks';

import Laptop from 'lucide-solid/icons/laptop';

const getThemePreferenceOptions = (): FilterOption<'light' | 'dark' | 'system'>[] => [
  { value: 'light', label: t('settings.general.theme.option.light'), icon: Sun },
  { value: 'dark', label: t('settings.general.theme.option.dark'), icon: Moon },
  { value: 'system', label: t('settings.general.theme.option.system'), icon: Laptop },
];

const getLocalePreferenceOptions = (): FilterOption<SupportedLocale>[] =>
  SUPPORTED_LOCALES.map((locale) => ({
    value: locale,
    label: SUPPORTED_LOCALE_LABELS[locale],
  }));

const TEMPERATURE_UNIT_OPTIONS: FilterOption<'celsius' | 'fahrenheit'>[] = [
  { value: 'celsius', label: 'Celsius' },
  { value: 'fahrenheit', label: 'Fahrenheit' },
];

const TELEMETRY_ENV_VAR = 'PULSE_TELEMETRY';
const PVE_POLLING_INTERVAL_ENV_VAR = 'PVE_POLLING_INTERVAL';

const getPvePollingOptions = (): FilterOption<number | 'custom'>[] => [
  ...getPvePollingPresetOptions(),
  getPvePollingCustomOption(),
];

export interface GeneralSettingsPanelProps {
  darkMode: Accessor<boolean>;
  themePreference: Accessor<'light' | 'dark' | 'system'>;
  setThemePreference: (pref: 'light' | 'dark' | 'system') => void;
  pvePollingInterval: Accessor<number>;
  setPVEPollingInterval: Setter<number>;
  pvePollingSelection: Accessor<number | 'custom'>;
  setPVEPollingSelection: Setter<number | 'custom'>;
  pvePollingCustomSeconds: Accessor<number>;
  setPVEPollingCustomSeconds: Setter<number>;
  pvePollingEnvLocked: () => boolean;
  setHasUnsavedChanges: Setter<boolean>;

  telemetryEnabled: Accessor<boolean>;
  telemetryEnabledLocked: () => boolean;
  savingTelemetry: Accessor<boolean>;
  handleTelemetryEnabledChange: (enabled: boolean) => Promise<void>;
  telemetryPreview: Accessor<TelemetryPreviewResponse | null>;
  telemetryPreviewEnabled: Accessor<boolean>;
  telemetryPreviewPayload: Accessor<string>;
  loadingTelemetryPreview: Accessor<boolean>;
  resettingTelemetryInstallID: Accessor<boolean>;
  handleLoadTelemetryPreview: () => Promise<void>;
  handleCopyTelemetryPreview: () => Promise<void>;
  handleResetTelemetryInstallID: () => Promise<void>;

  disableDockerUpdateActions: Accessor<boolean>;
  disableDockerUpdateActionsLocked: () => boolean;
  savingDockerUpdateActions: Accessor<boolean>;
  handleDisableDockerUpdateActionsChange: (disabled: boolean) => Promise<void>;
}

export const GeneralSettingsPanel: Component<GeneralSettingsPanelProps> = (props) => {
  const handlePVEPollingSelection = (value: number | 'custom') => {
    if (props.pvePollingEnvLocked()) return;

    props.setPVEPollingSelection(value);
    props.setPVEPollingInterval(value === 'custom' ? props.pvePollingCustomSeconds() : value);
    props.setHasUnsavedChanges(true);
  };

  return (
    <div class="space-y-6">
      {/* Appearance Card */}
      <SettingsPanel
        title={t('settings.general.appearance.title')}
        noPadding
        bodyClass="divide-y divide-border"
      >
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-4 sm:p-6">
          <div class="flex items-center gap-3 min-w-0">
            {/* Animated theme icon */}
            <div
              class={`shrink-0 relative p-2.5 rounded-md border border-border bg-surface transition-all duration-300`}
            >
              <div class="relative w-5 h-5">
                <Sun
                  class={`absolute inset-0 w-5 h-5 text-slate-500 transition-all duration-300 ${props.darkMode() ? 'opacity-0 rotate-90 scale-50' : 'opacity-100 rotate-0 scale-100'}`}
                  strokeWidth={2}
                />
                <Moon
                  class={`absolute inset-0 w-5 h-5 text-slate-500 transition-all duration-300 ${props.darkMode() ? 'opacity-100 rotate-0 scale-100' : 'opacity-0 -rotate-90 scale-50'}`}
                  strokeWidth={2}
                />
              </div>
            </div>
            <div class="text-sm text-muted min-w-0">
              <p class="font-medium text-base-content truncate">
                {t('settings.general.theme.title')}
              </p>
              <p class="text-xs text-muted line-clamp-2">
                {t('settings.general.theme.description')}
              </p>
            </div>
          </div>
          <FilterButtonGroup
            class="w-full sm:w-auto sm:shrink-0 max-w-full"
            options={getThemePreferenceOptions()}
            value={props.themePreference()}
            onChange={props.setThemePreference}
            variant="settings"
          />
        </div>

        {/* Language Selector */}
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-4 sm:p-6">
          <div class="flex items-center gap-3 min-w-0">
            <div class="shrink-0 p-2.5 rounded-md border border-border bg-surface">
              <Languages class="w-5 h-5 text-slate-500" strokeWidth={2} />
            </div>
            <div class="text-sm text-muted min-w-0">
              <p class="font-medium text-base-content truncate">
                {t('settings.general.language.title')}
              </p>
              <p class="text-xs text-muted line-clamp-2">
                {t('settings.general.language.description')}
              </p>
            </div>
          </div>
          <FilterButtonGroup
            class="w-full sm:w-auto sm:shrink-0 max-w-full"
            options={getLocalePreferenceOptions()}
            value={activeLocale()}
            onChange={setLocalePreference}
            variant="settings"
            ariaLabel={t('settings.general.language.ariaLabel')}
          />
        </div>

        {/* Temperature Unit Selector */}
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-4 sm:p-6">
          <div class="flex items-center gap-3 min-w-0">
            <div class="shrink-0 p-2.5 rounded-md border border-border bg-surface">
              <Thermometer class="w-5 h-5" strokeWidth={2} />
            </div>
            <div class="text-sm text-muted min-w-0">
              <p class="font-medium text-base-content truncate">
                {t('settings.general.temperature.title')}
              </p>
              <p class="text-xs text-muted line-clamp-2">
                {t('settings.general.temperature.description')}
              </p>
            </div>
          </div>
          <FilterButtonGroup
            class="w-full sm:w-auto sm:shrink-0 max-w-full"
            options={TEMPERATURE_UNIT_OPTIONS}
            value={temperatureStore.unit()}
            onChange={(value) => temperatureStore.setUnit(value)}
            variant="settings"
          />
        </div>

        {/* Full-width Mode Toggle */}
        <div class="flex items-center justify-between gap-4 p-4 sm:p-6">
          <div class="flex items-center gap-3 min-w-0">
            <div class="shrink-0 p-2.5 rounded-md border border-border bg-surface">
              <Maximize2 class="w-5 h-5 text-slate-500" strokeWidth={2} />
            </div>
            <div class="text-sm text-muted min-w-0">
              <p class="font-medium text-base-content truncate">
                {t('settings.general.fullWidth.title')}
              </p>
              <p class="text-xs text-muted line-clamp-2">
                {t('settings.general.fullWidth.description')}
              </p>
            </div>
          </div>
          <Toggle
            checked={layoutStore.isFullWidth()}
            class="shrink-0"
            onChange={() => layoutStore.toggle()}
          />
        </div>
      </SettingsPanel>

      {/* Usage Data + Privacy Card */}
      <SettingsPanel
        title={t('settings.general.telemetry.section.title')}
        description={t('settings.general.telemetry.section.description')}
        noPadding
        bodyClass="divide-y divide-border"
      >
        <div class="p-4 sm:p-6 space-y-4">
          <div class="flex items-center justify-between gap-4">
            <div class="flex-1 min-w-0 space-y-1">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-base-content truncate">
                  {t('settings.general.telemetry.title')}
                </span>
                <Show when={props.telemetryEnabledLocked()}>
                  <EnvironmentLockBadge envVar={TELEMETRY_ENV_VAR} />
                </Show>
              </div>
              <p class="text-xs text-muted leading-relaxed">
                {t('settings.general.telemetry.description')}{' '}
                <ExternalTextLink href={PRIVACY_DOC_URL} variant="muted">
                  {t('settings.general.telemetry.fullDetails')}
                </ExternalTextLink>
              </p>
            </div>
            <Toggle
              checked={props.telemetryEnabled()}
              class="shrink-0"
              disabled={props.telemetryEnabledLocked() || props.savingTelemetry()}
              onChange={() => props.handleTelemetryEnabledChange(!props.telemetryEnabled())}
            />
          </div>

          <div class="flex flex-wrap gap-3">
            <Button
              variant="secondary"
              size="settingsActionXs"
              disabled={props.loadingTelemetryPreview()}
              onClick={() => void props.handleLoadTelemetryPreview()}
            >
              {props.telemetryPreview()
                ? t('settings.general.telemetry.refreshPayload')
                : t('settings.general.telemetry.previewPayload')}
            </Button>
            <Button
              variant="secondary"
              size="settingsActionXs"
              disabled={props.resettingTelemetryInstallID()}
              onClick={() => void props.handleResetTelemetryInstallID()}
            >
              {t('settings.general.telemetry.resetId')}
            </Button>
            <Show when={props.telemetryPreviewPayload()}>
              <Button
                variant="secondary"
                size="settingsActionXs"
                onClick={() => void props.handleCopyTelemetryPreview()}
              >
                {t('settings.general.telemetry.copyJson')}
              </Button>
            </Show>
          </div>

          <Show when={props.telemetryPreviewPayload()}>
            <div class="rounded-md border border-border bg-surface-alt">
              <div class="flex flex-col gap-1 border-b border-border px-4 py-3">
                <p class="text-xs font-semibold uppercase tracking-wide text-muted">
                  {t('settings.general.telemetry.payloadTitle')}
                </p>
                <Show when={!props.telemetryPreviewEnabled()}>
                  <p class="text-xs text-muted leading-relaxed">
                    {t('settings.general.telemetry.disabledPreview')}
                  </p>
                </Show>
              </div>
              <pre
                aria-label={t('settings.general.telemetry.payloadAriaLabel')}
                class="overflow-x-auto px-4 py-3 text-xs leading-relaxed text-base-content"
              >
                {props.telemetryPreviewPayload()}
              </pre>
            </div>
          </Show>
        </div>
      </SettingsPanel>

      <DockerRuntimeSettingsCard
        disableDockerUpdateActions={props.disableDockerUpdateActions}
        disableDockerUpdateActionsLocked={props.disableDockerUpdateActionsLocked}
        savingDockerUpdateActions={props.savingDockerUpdateActions}
        handleDisableDockerUpdateActionsChange={props.handleDisableDockerUpdateActionsChange}
      />

      {/* Monitoring Cadence Card */}
      <SettingsPanel
        title={t('settings.general.monitoringCadence.section.title')}
        description={t('settings.general.monitoringCadence.section.description')}
        noPadding
        bodyClass="divide-y divide-border"
      >
        <div class="p-4 sm:p-6">
          <div class="space-y-4">
            <div class="space-y-2">
              <p class="text-[10px] font-bold uppercase tracking-wider text-muted">
                {getPvePollingCadenceSummary(props.pvePollingInterval())}
              </p>
              <p class="text-xs text-muted leading-relaxed max-w-3xl">
                {t('settings.general.monitoringCadence.description')}
              </p>
            </div>

            <div class="space-y-4 pt-2">
              {/* Preset buttons */}
              <FilterButtonGroup
                class="sm:grid-cols-2 xl:grid-cols-5"
                options={getPvePollingOptions()}
                value={props.pvePollingSelection()}
                onChange={handlePVEPollingSelection}
                variant="prominent"
                disabled={props.pvePollingEnvLocked()}
              />

              {/* Custom interval input */}
              <Show when={props.pvePollingSelection() === 'custom'}>
                <div class="mt-4 flex flex-col sm:flex-row sm:items-center gap-4 rounded-md border border-dashed border-border bg-surface-hover p-4 transition-all animate-in fade-in slide-in-from-top-1">
                  <div class="flex-1 min-w-0">
                    <label class="block text-sm font-medium text-base-content truncate">
                      {t('settings.general.monitoringCadence.custom.title')}
                    </label>
                    <p class="text-xs text-muted mt-0.5 line-clamp-2">
                      {t('settings.general.monitoringCadence.custom.description', {
                        min: PVE_POLLING_MIN_SECONDS,
                        max: PVE_POLLING_MAX_SECONDS,
                      })}
                    </p>
                  </div>
                  <input
                    type="number"
                    min={PVE_POLLING_MIN_SECONDS}
                    max={PVE_POLLING_MAX_SECONDS}
                    value={props.pvePollingCustomSeconds()}
                    class="w-full sm:w-32 min-h-10 rounded-md border border-border px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent dark:focus:ring-blue-400 shadow-sm"
                    disabled={props.pvePollingEnvLocked()}
                    onInput={(e) => {
                      if (props.pvePollingEnvLocked()) return;
                      const parsed = Math.floor(Number(e.currentTarget.value));
                      if (Number.isNaN(parsed)) {
                        return;
                      }
                      const clamped = Math.min(
                        PVE_POLLING_MAX_SECONDS,
                        Math.max(PVE_POLLING_MIN_SECONDS, parsed),
                      );
                      props.setPVEPollingCustomSeconds(clamped);
                      props.setPVEPollingInterval(clamped);
                      props.setHasUnsavedChanges(true);
                    }}
                  />
                </div>
              </Show>

              {/* Env override warning */}
              <Show when={props.pvePollingEnvLocked()}>
                <div class="flex items-center gap-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-200">
                  <svg
                    class="h-4 w-4 shrink-0 mt-0.5 self-start"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <circle cx="12" cy="12" r="10" />
                    <line x1="12" y1="8" x2="12" y2="12" />
                    <circle cx="12" cy="16" r="0.5" />
                  </svg>
                  <span class="leading-relaxed">
                    {t('settings.general.monitoringCadence.envLocked', {
                      envVar: PVE_POLLING_INTERVAL_ENV_VAR,
                    })}
                  </span>
                </div>
              </Show>
            </div>
          </div>
        </div>
      </SettingsPanel>
    </div>
  );
};
