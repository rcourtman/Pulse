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
  toggleDarkMode: () => void;
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
        bodyClass="space-y-5"
      >
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex items-center gap-3">
            {/* Animated theme icon */}
            <div class={`relative p-2.5 rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 transition-all duration-300`}>
              <div class="relative w-5 h-5">
                <Sun class={`absolute inset-0 w-5 h-5 text-slate-500 transition-all duration-300 ${props.darkMode() ? 'opacity-0 rotate-90 scale-50' : 'opacity-100 rotate-0 scale-100'
                  }`} strokeWidth={2} />
                <Moon class={`absolute inset-0 w-5 h-5 text-slate-500 transition-all duration-300 ${props.darkMode() ? 'opacity-100 rotate-0 scale-100' : 'opacity-0 -rotate-90 scale-50'
                  }`} strokeWidth={2} />
              </div>
            </div>
            <div class="text-sm text-slate-600 dark:text-slate-400">
              <p class="font-medium text-slate-900 dark:text-slate-100">
                {props.darkMode() ? 'Dark mode' : 'Light mode'}
              </p>
              <p class="text-xs text-slate-500 dark:text-slate-400">
                Toggle to match your environment. Pulse remembers this preference on each browser.
              </p>
            </div>
          </div>
          <Toggle
            checked={props.darkMode()}
            containerClass="self-end sm:self-auto"
            onChange={(event) => {
              const desired = (event.currentTarget as HTMLInputElement).checked;
              if (desired !== props.darkMode()) {
                props.toggleDarkMode();
              }
            }}
          />
        </div>

        {/* Temperature Unit Selector */}
        <div class="flex flex-col gap-3 pt-4 border-t border-slate-200 dark:border-slate-700 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex items-center gap-3">
            <div class="p-2.5 rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800">
              <Thermometer class="w-5 h-5 text-slate-500" strokeWidth={2} />
            </div>
            <div class="text-sm text-slate-600 dark:text-slate-400">
              <p class="font-medium text-slate-900 dark:text-slate-100">
                Temperature unit
              </p>
              <p class="text-xs text-slate-500 dark:text-slate-400">
                Display temperatures in Celsius or Fahrenheit
              </p>
            </div>
          </div>
          <div class="flex items-center gap-1 self-end sm:self-auto bg-slate-100 dark:bg-slate-800 rounded-md p-1">
            <button
              type="button"
              class={`min-h-10 sm:min-h-9 min-w-10 px-3 py-2 text-sm rounded-md transition-all ${temperatureStore.unit() === 'celsius'
                ? 'bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 shadow-sm'
                : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'
                }`}
              onClick={() => temperatureStore.setUnit('celsius')}
            >
              °C
            </button>
            <button
              type="button"
              class={`min-h-10 sm:min-h-9 min-w-10 px-3 py-2 text-sm rounded-md transition-all ${temperatureStore.unit() === 'fahrenheit'
                ? 'bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 shadow-sm'
                : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'
                }`}
              onClick={() => temperatureStore.setUnit('fahrenheit')}
            >
              °F
            </button>
          </div>
        </div>

        {/* Full-width Mode Toggle */}
        <div class="flex flex-col gap-3 pt-4 border-t border-slate-200 dark:border-slate-700 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex items-center gap-3">
            <div class="p-2.5 rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800">
              <Maximize2 class="w-5 h-5 text-slate-500" strokeWidth={2} />
            </div>
            <div class="text-sm text-slate-600 dark:text-slate-400">
              <p class="font-medium text-slate-900 dark:text-slate-100">
                Full-width mode
              </p>
              <p class="text-xs text-slate-500 dark:text-slate-400">
                Expand content to use all available screen width on large monitors
              </p>
            </div>
          </div>
          <Toggle
            checked={layoutStore.isFullWidth()}
            containerClass="self-end sm:self-auto"
            onChange={() => layoutStore.toggle()}
          />
        </div>
      </SettingsPanel>

      {/* Navigation + Privacy Card */}
      <SettingsPanel
        title="Navigation and privacy"
        description="Control migration helpers and local-only metrics collection."
        icon={<Sliders class="w-5 h-5" strokeWidth={2} />}
        bodyClass="space-y-5"
      >
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="flex-1 space-y-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-slate-900 dark:text-slate-100">
                Disable legacy URL redirects
              </span>
              <Show when={props.disableLegacyRouteRedirectsLocked()}>
                <span
                  class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300"
                  title="Locked by environment variable PULSE_DISABLE_LEGACY_ROUTE_REDIRECTS"
                >
                  ENV
                </span>
              </Show>
            </div>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              When enabled, Pulse will not redirect old bookmarks like <code class="px-1 py-0.5 rounded bg-slate-200 dark:bg-slate-700">/services</code> or <code class="px-1 py-0.5 rounded bg-slate-200 dark:bg-slate-700">/kubernetes</code>.
              This helps surface stale bookmarks immediately.
            </p>
          </div>
          <Toggle
            checked={props.disableLegacyRouteRedirects()}
            containerClass="self-end sm:self-auto"
            disabled={props.disableLegacyRouteRedirectsLocked() || props.savingLegacyRedirects()}
            onChange={() => props.handleDisableLegacyRouteRedirectsChange(!props.disableLegacyRouteRedirects())}
          />
        </div>

        <div class="flex flex-col gap-3 pt-4 border-t border-slate-200 dark:border-slate-700 sm:flex-row sm:items-start sm:justify-between">
          <div class="flex-1 space-y-1">
            <span class="text-sm font-medium text-slate-900 dark:text-slate-100">
              Reduce Pro prompts
            </span>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              Hides proactive upgrade prompts (for example, the relay onboarding card). Paywalls still appear if you try to use a gated feature.
            </p>
          </div>
          <Toggle
            checked={props.reduceProUpsellNoise()}
            containerClass="self-end sm:self-auto"
            disabled={props.savingReduceUpsells()}
            onChange={() => props.handleReduceProUpsellNoiseChange(!props.reduceProUpsellNoise())}
          />
        </div>

        <div class="flex flex-col gap-3 pt-4 border-t border-slate-200 dark:border-slate-700 sm:flex-row sm:items-start sm:justify-between">
          <div class="flex-1 space-y-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-slate-900 dark:text-slate-100">
                Disable local upgrade metrics
              </span>
              <Show when={props.disableLocalUpgradeMetricsLocked()}>
                <span
                  class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300"
                  title="Locked by environment variable PULSE_DISABLE_LOCAL_UPGRADE_METRICS"
                >
                  ENV
                </span>
              </Show>
            </div>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              Records local-only events like "paywall viewed" and "trial started" to help debug and improve upgrade flows. These events are stored locally and are not exported to third parties.
            </p>
          </div>
          <Toggle
            checked={props.disableLocalUpgradeMetrics()}
            containerClass="self-end sm:self-auto"
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
        bodyClass="space-y-5"
      >
        <div class="space-y-2">
          <p class="text-sm text-slate-600 dark:text-slate-400">
            Shorter intervals provide near-real-time updates at the cost of higher API and CPU
            usage on each node. Set a longer interval to reduce load on busy clusters.
          </p>
          <p class="text-xs text-slate-500 dark:text-slate-400">
            Current cadence: {props.pvePollingInterval()} seconds (
            {props.pvePollingInterval() >= 60
              ? `${(props.pvePollingInterval() / 60).toFixed(
                props.pvePollingInterval() % 60 === 0 ? 0 : 1
              )} minute${props.pvePollingInterval() / 60 === 1 ? '' : 's'}`
              : 'under a minute'}
            ).
          </p>
        </div>

        <div class="space-y-4">
          {/* Preset buttons */}
          <div class="grid gap-2 sm:grid-cols-3">
            <For each={PVE_POLLING_PRESETS}>
              {(option) => (
                <button
                  type="button"
                  class={`min-h-10 sm:min-h-10 rounded-md border px-3 py-2.5 text-left text-sm transition-colors ${props.pvePollingSelection() === option.value
                    ? 'border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900 dark:text-blue-100'
                    : 'border-slate-200 bg-white text-slate-700 hover:border-blue-400 hover:text-blue-600 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-200'
                    } ${props.pvePollingEnvLocked() ? 'opacity-60 cursor-not-allowed' : ''}`}
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
              class={`min-h-10 sm:min-h-10 rounded-md border px-3 py-2.5 text-left text-sm transition-colors ${props.pvePollingSelection() === 'custom'
                ? 'border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900 dark:text-blue-100'
                : 'border-slate-200 bg-white text-slate-700 hover:border-blue-400 hover:text-blue-600 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-200'
                } ${props.pvePollingEnvLocked() ? 'opacity-60 cursor-not-allowed' : ''}`}
              disabled={props.pvePollingEnvLocked()}
              onClick={() => {
                if (props.pvePollingEnvLocked()) return;
                props.setPVEPollingSelection('custom');
                props.setPVEPollingInterval(props.pvePollingCustomSeconds());
                props.setHasUnsavedChanges(true);
              }}
            >
              Custom interval
            </button>
          </div>

          {/* Custom interval input */}
          <Show when={props.pvePollingSelection() === 'custom'}>
            <div class="space-y-2 rounded-md border border-dashed border-slate-300 p-4 dark:border-slate-600">
              <label class="text-xs font-medium text-slate-700 dark:text-slate-200">
                Custom polling interval (10-3600 seconds)
              </label>
              <input
                type="number"
                min={PVE_POLLING_MIN_SECONDS}
                max={PVE_POLLING_MAX_SECONDS}
                value={props.pvePollingCustomSeconds()}
                class="w-full min-h-10 sm:min-h-10 rounded-md border border-slate-300 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-slate-600 dark:bg-slate-800"
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
              <p class="text-[0.68rem] text-slate-500 dark:text-slate-400">
                Applies to all PVE clusters and standalone nodes.
              </p>
            </div>
          </Show>

          {/* Env override warning */}
          <Show when={props.pvePollingEnvLocked()}>
            <div class="flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-800 dark:bg-amber-900 dark:text-amber-200">
              <svg
                class="h-4 w-4 flex-shrink-0"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <circle cx="12" cy="16" r="0.5" />
              </svg>
              <span>Managed via environment variable PVE_POLLING_INTERVAL.</span>
            </div>
          </Show>
        </div>
      </SettingsPanel>
    </div>
  );
};
