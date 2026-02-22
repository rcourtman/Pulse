import { Component, Show, For, Accessor, Setter } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Toggle } from '@/components/shared/Toggle';
import Sliders from 'lucide-solid/icons/sliders-horizontal';
import Activity from 'lucide-solid/icons/activity';
import Sun from 'lucide-solid/icons/sun';
import Moon from 'lucide-solid/icons/moon';
import Thermometer from 'lucide-solid/icons/thermometer';
import Maximize2 from 'lucide-solid/icons/maximize-2';
import { temperatureStore } from '@/utils/temperature';
import { layoutStore } from '@/utils/layout';

import Laptop from 'lucide-solid/icons/laptop';

const PVE_POLLING_MIN_SECONDS = 10;
const PVE_POLLING_MAX_SECONDS = 3600;
const PVE_POLLING_PRESETS = [
  { label: 'Realtime (10s)', value: 10 },
  { label: 'Balanced (30s)', value: 30 },
  { label: 'Low (60s)', value: 60 },
  { label: 'Very low (5m)', value: 300 },
];

interface GeneralSettingsPanelProps {
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

  disableLegacyRouteRedirects: Accessor<boolean>;
  disableLegacyRouteRedirectsLocked: () => boolean;
  savingLegacyRedirects: Accessor<boolean>;
  handleDisableLegacyRouteRedirectsChange: (disabled: boolean) => Promise<void>;

  reduceProUpsellNoise: Accessor<boolean>;
  savingReduceUpsells: Accessor<boolean>;
  handleReduceProUpsellNoiseChange: (enabled: boolean) => Promise<void>;

  disableLocalUpgradeMetrics: Accessor<boolean>;
  disableLocalUpgradeMetricsLocked: () => boolean;
  savingUpgradeMetrics: Accessor<boolean>;
  handleDisableLocalUpgradeMetricsChange: (disabled: boolean) => Promise<void>;
}

export const GeneralSettingsPanel: Component<GeneralSettingsPanelProps> = (props) => {
  return (
    <div class="space-y-6">
      {/* Appearance Card */}
      <SettingsPanel
        title="General"
        description="Manage appearance, layout, and default monitoring cadence."
        icon={<Sliders class="w-5 h-5" strokeWidth={2} />}
        noPadding
        bodyClass="divide-y divide-border"
      >
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
          <div class="flex items-center gap-3 min-w-0">
            {/* Animated theme icon */}
            <div class={`shrink-0 relative p-2.5 rounded-md border border-border bg-surface transition-all duration-300`}>
              <div class="relative w-5 h-5">
                <Sun class={`absolute inset-0 w-5 h-5 text-slate-500 transition-all duration-300 ${props.darkMode() ? 'opacity-0 rotate-90 scale-50' : 'opacity-100 rotate-0 scale-100'}`} strokeWidth={2} />
                <Moon class={`absolute inset-0 w-5 h-5 text-slate-500 transition-all duration-300 ${props.darkMode() ? 'opacity-100 rotate-0 scale-100' : 'opacity-0 -rotate-90 scale-50'}`} strokeWidth={2} />
              </div>
            </div>
            <div class="text-sm text-muted min-w-0">
              <p class="font-medium text-base-content truncate">
                Theme preference
              </p>
              <p class="text-xs text-muted line-clamp-2">
                Choose light, dark, or sync with your system theme.
              </p>
            </div>
          </div>
          <div class="shrink-0 flex self-start sm:self-auto items-center gap-1 bg-surface-alt rounded-md p-1 ml-12 sm:ml-0">
            <button
              type="button"
              class={`flex items-center gap-1.5 min-h-10 sm:min-h-9 px-3 py-2 text-sm font-medium rounded-md transition-all ${props.themePreference() === 'light'
 ? 'bg-white dark:bg-slate-700 text-base-content shadow-sm'
 : 'text-muted hover: dark:hover:text-slate-200'
 }`}
              onClick={() => props.setThemePreference('light')}
            >
              <Sun class="w-4 h-4" strokeWidth={2.5} />
              <span class="hidden lg:inline">Light</span>
            </button>
            <button
              type="button"
              class={`flex items-center gap-1.5 min-h-10 sm:min-h-9 px-3 py-2 text-sm font-medium rounded-md transition-all ${props.themePreference() === 'dark'
 ? 'bg-white dark:bg-slate-700 text-base-content shadow-sm'
 : 'text-muted hover: dark:hover:text-slate-200'
 }`}
              onClick={() => props.setThemePreference('dark')}
            >
              <Moon class="w-4 h-4" strokeWidth={2.5} />
              <span class="hidden lg:inline">Dark</span>
            </button>
            <button
              type="button"
              class={`flex items-center gap-1.5 min-h-10 sm:min-h-9 px-3 py-2 text-sm font-medium rounded-md transition-all ${props.themePreference() === 'system'
 ? 'bg-white dark:bg-slate-700 text-base-content shadow-sm'
 : 'text-muted hover: dark:hover:text-slate-200'
 }`}
              onClick={() => props.setThemePreference('system')}
 >
 <Laptop class="w-4 h-4" strokeWidth={2.5} />
 <span class="hidden lg:inline">System</span>
 </button>
 </div>
 </div>

 {/* Temperature Unit Selector */}
 <div class="flex items-center justify-between gap-4 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
 <div class="flex items-center gap-3 min-w-0">
 <div class="shrink-0 p-2.5 rounded-md border border-border bg-surface">
 <Thermometer class="w-5 h-5" strokeWidth={2} />
 </div>
 <div class="text-sm text-muted min-w-0">
 <p class="font-medium text-base-content truncate">
 Temperature unit
 </p>
 <p class="text-xs text-muted line-clamp-2">
 Display temperatures in Celsius or Fahrenheit
 </p>
 </div>
 </div>
 <div class="shrink-0 flex items-center gap-1 bg-surface-alt rounded-md p-1">
 <button
 type="button"
 class={`min-h-10 sm:min-h-9 min-w-10 px-3 py-2 text-sm rounded-md transition-all ${temperatureStore.unit() ==='celsius'
 ? 'bg-white dark:bg-slate-700 text-base-content shadow-sm'
 : 'text-muted hover: dark:hover:text-slate-200'
 }`}
              onClick={() => temperatureStore.setUnit('celsius')}
            >
              °C
            </button>
            <button
              type="button"
              class={`min-h-10 sm:min-h-9 min-w-10 px-3 py-2 text-sm rounded-md transition-all ${temperatureStore.unit() === 'fahrenheit'
 ? 'bg-white dark:bg-slate-700 text-base-content shadow-sm'
 : 'text-muted hover: dark:hover:text-slate-200'
 }`}
              onClick={() => temperatureStore.setUnit('fahrenheit')}
            >
              °F
            </button>
          </div>
        </div>

        {/* Full-width Mode Toggle */}
        <div class="flex items-center justify-between gap-4 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
          <div class="flex items-center gap-3 min-w-0">
            <div class="shrink-0 p-2.5 rounded-md border border-border bg-surface">
              <Maximize2 class="w-5 h-5 text-slate-500" strokeWidth={2} />
            </div>
            <div class="text-sm text-muted min-w-0">
              <p class="font-medium text-base-content truncate">
                Full-width mode
              </p>
              <p class="text-xs text-muted line-clamp-2">
                Expand content to use all available screen width on large monitors
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

      {/* Navigation + Privacy Card */}
      <SettingsPanel
        title="Navigation and privacy"
        description="Control migration helpers and local-only metrics collection."
        icon={<Sliders class="w-5 h-5" strokeWidth={2} />}
        noPadding
        bodyClass="divide-y divide-border"
      >
        <div class="flex items-center justify-between gap-4 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
          <div class="flex-1 min-w-0 space-y-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-base-content truncate">
                Disable legacy URL redirects
              </span>
              <Show when={props.disableLegacyRouteRedirectsLocked()}>
                <span
                  class="shrink-0 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300"
                  title="Locked by environment variable PULSE_DISABLE_LEGACY_ROUTE_REDIRECTS"
                >
                  ENV
                </span>
              </Show>
            </div>
            <p class="text-xs text-muted line-clamp-2">
              When enabled, Pulse will not redirect old bookmarks like <code class="px-1 py-0.5 rounded bg-surface-hover">/services</code>.
            </p>
          </div>
          <Toggle
            checked={props.disableLegacyRouteRedirects()}
            class="shrink-0"
            disabled={props.disableLegacyRouteRedirectsLocked() || props.savingLegacyRedirects()}
            onChange={() => props.handleDisableLegacyRouteRedirectsChange(!props.disableLegacyRouteRedirects())}
          />
        </div>

        <div class="flex items-center justify-between gap-4 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
          <div class="flex-1 min-w-0 space-y-1">
            <div class="text-sm font-medium text-base-content truncate">
              Reduce Pro prompts
            </div>
            <p class="text-xs text-muted line-clamp-2">
              Hides proactive upgrade prompts (for example, the relay onboarding card). Paywalls still appear if you try to use a gated feature.
            </p>
          </div>
          <Toggle
            checked={props.reduceProUpsellNoise()}
            class="shrink-0"
            disabled={props.savingReduceUpsells()}
            onChange={() => props.handleReduceProUpsellNoiseChange(!props.reduceProUpsellNoise())}
          />
        </div>

        <div class="flex items-center justify-between gap-4 p-4 sm:p-6 hover:bg-surface-hover transition-colors">
          <div class="flex-1 min-w-0 space-y-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-base-content truncate">
                Disable local upgrade metrics
              </span>
              <Show when={props.disableLocalUpgradeMetricsLocked()}>
                <span
                  class="shrink-0 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300"
                  title="Locked by environment variable PULSE_DISABLE_LOCAL_UPGRADE_METRICS"
                >
                  ENV
                </span>
              </Show>
            </div>
            <p class="text-xs text-muted line-clamp-2">
              Records local-only events like "paywall viewed" and "trial started" to help debug and improve upgrade flows. These events are stored locally and are not exported to third parties.
            </p>
          </div>
          <Toggle
            checked={props.disableLocalUpgradeMetrics()}
            class="shrink-0"
            disabled={props.disableLocalUpgradeMetricsLocked() || props.savingUpgradeMetrics()}
            onChange={() => props.handleDisableLocalUpgradeMetricsChange(!props.disableLocalUpgradeMetrics())}
          />
        </div>
      </SettingsPanel>

      {/* Monitoring Cadence Card */}
      <SettingsPanel
        title="Monitoring cadence"
        description="Control how frequently Pulse polls Proxmox VE nodes."
        icon={<Activity class="w-5 h-5" strokeWidth={2} />}
        noPadding
        bodyClass="divide-y divide-border"
      >
        <div class="p-4 sm:p-6 hover:bg-surface-hover transition-colors">
          <div class="space-y-4">
            <div class="space-y-2">
              <p class="text-[10px] font-bold uppercase tracking-wider text-muted">
                Current cadence: {props.pvePollingInterval()} seconds (
                {props.pvePollingInterval() >= 60
                  ? `${(props.pvePollingInterval() / 60).toFixed(
                    props.pvePollingInterval() % 60 === 0 ? 0 : 1
                  )} minute${props.pvePollingInterval() / 60 === 1 ? '' : 's'}`
                  : 'under a minute'}
                )
              </p>
              <p class="text-xs text-muted leading-relaxed max-w-3xl">
                Shorter intervals provide near-real-time updates at the cost of higher API and CPU
                usage on each node. Set a longer interval to reduce load on busy clusters.
              </p>
            </div>

            <div class="space-y-4 pt-2">
              {/* Preset buttons */}
              <div class="grid gap-2 sm:grid-cols-4">
                <For each={PVE_POLLING_PRESETS}>
                  {(option) => (
                    <button
                      type="button"
                      class={`min-h-10 sm:min-h-10 rounded-md border px-3 py-2.5 text-center text-sm font-medium transition-colors ${props.pvePollingSelection() === option.value ? 'border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900 dark:text-blue-200' : 'border-border bg-surface text-base-content hover:border-border hover:bg-surface-hover' } ${props.pvePollingEnvLocked() ? 'opacity-60 cursor-not-allowed' : ''}`}
                      disabled={props.pvePollingEnvLocked()}
                      onClick={() => {
                        if (props.pvePollingEnvLocked()) return;
                        props.setPVEPollingSelection(option.value);
                        props.setPVEPollingInterval(option.value);
                        props.setHasUnsavedChanges(true);
                      }}
                    >
                      {option.label}
                    </button>
                  )}
                </For>
                <button
                  type="button"
                  class={`min-h-10 sm:min-h-10 rounded-md border px-3 py-2.5 text-center text-sm font-medium transition-colors ${props.pvePollingSelection() === 'custom' ? 'border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900 dark:text-blue-200' : 'border-border bg-surface text-base-content hover:border-border hover:bg-surface-hover' } ${props.pvePollingEnvLocked() ? 'opacity-60 cursor-not-allowed' : ''}`}
                  disabled={props.pvePollingEnvLocked()}
                  onClick={() => {
                    if (props.pvePollingEnvLocked()) return;
                    props.setPVEPollingSelection('custom');
                    props.setPVEPollingInterval(props.pvePollingCustomSeconds());
                    props.setHasUnsavedChanges(true);
                  }}
                >
                  Custom
                </button>
              </div>

              {/* Custom interval input */}
              <Show when={props.pvePollingSelection() === 'custom'}>
                <div class="mt-4 flex flex-col sm:flex-row sm:items-center gap-4 rounded-md border border-dashed border-border bg-surface-hover p-4 transition-all animate-in fade-in slide-in-from-top-1">
                  <div class="flex-1 min-w-0">
                    <label class="block text-sm font-medium text-base-content truncate">
                      Custom polling interval
                    </label>
                    <p class="text-xs text-muted mt-0.5 line-clamp-2">
                      Enter seconds ({PVE_POLLING_MIN_SECONDS}-{PVE_POLLING_MAX_SECONDS}). Applies to all clusters.
                    </p>
                  </div>
                  <input
                    type="number"
                    min={PVE_POLLING_MIN_SECONDS}
                    max={PVE_POLLING_MAX_SECONDS}
                    value={props.pvePollingCustomSeconds()}
                    class="w-full sm:w-32 min-h-10 rounded-md border border-border px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent dark:bg-slate-900 dark:text-slate-100 dark:focus:ring-blue-400 shadow-sm"
                    disabled={props.pvePollingEnvLocked()}
                    onInput={(e) => {
                      if (props.pvePollingEnvLocked()) return;
                      const parsed = Math.floor(Number(e.currentTarget.value));
                      if (Number.isNaN(parsed)) {
                        return;
                      }
                      const clamped = Math.min(
                        PVE_POLLING_MAX_SECONDS,
                        Math.max(PVE_POLLING_MIN_SECONDS, parsed)
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
                  <svg class="h-4 w-4 shrink-0 mt-0.5 self-start" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" />
                    <line x1="12" y1="8" x2="12" y2="12" />
                    <circle cx="12" cy="16" r="0.5" />
                  </svg>
                  <span class="leading-relaxed">Managed via environment variable <strong>ENV_PVE_POLLING_INTERVAL</strong>.</span>
                </div>
              </Show>
            </div>
          </div>
        </div>
      </SettingsPanel>
    </div>
  );
};
