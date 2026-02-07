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
        <div class="flex items-center justify-between gap-4">
          <div class="flex items-center gap-3">
            {/* Animated theme icon */}
            <div class={`relative p-2.5 rounded-xl transition-all duration-300 ${props.darkMode()
              ? 'bg-gray-600 shadow-lg shadow-gray-500/20'
              : 'bg-gray-400 shadow-lg shadow-gray-500/20'
              }`}>
              <div class="relative w-5 h-5">
                <Sun class={`absolute inset-0 w-5 h-5 text-white transition-all duration-300 ${props.darkMode() ? 'opacity-0 rotate-90 scale-50' : 'opacity-100 rotate-0 scale-100'
                  }`} strokeWidth={2} />
                <Moon class={`absolute inset-0 w-5 h-5 text-white transition-all duration-300 ${props.darkMode() ? 'opacity-100 rotate-0 scale-100' : 'opacity-0 -rotate-90 scale-50'
                  }`} strokeWidth={2} />
              </div>
            </div>
            <div class="text-sm text-gray-600 dark:text-gray-400">
              <p class="font-medium text-gray-900 dark:text-gray-100">
                {props.darkMode() ? 'Dark mode' : 'Light mode'}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                Toggle to match your environment. Pulse remembers this preference on each browser.
              </p>
            </div>
          </div>
          <Toggle
            checked={props.darkMode()}
            onChange={(event) => {
              const desired = (event.currentTarget as HTMLInputElement).checked;
              if (desired !== props.darkMode()) {
                props.toggleDarkMode();
              }
            }}
          />
        </div>

        {/* Temperature Unit Selector */}
        <div class="flex items-center justify-between gap-4 pt-4 border-t border-gray-200 dark:border-gray-700">
          <div class="flex items-center gap-3">
            <div class="p-2.5 rounded-xl bg-gray-500 shadow-lg shadow-gray-500/20">
              <Thermometer class="w-5 h-5 text-white" strokeWidth={2} />
            </div>
            <div class="text-sm text-gray-600 dark:text-gray-400">
              <p class="font-medium text-gray-900 dark:text-gray-100">
                Temperature unit
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                Display temperatures in Celsius or Fahrenheit
              </p>
            </div>
          </div>
          <div class="flex items-center gap-1 bg-gray-100 dark:bg-gray-800 rounded-lg p-1">
            <button
              type="button"
              class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${temperatureStore.unit() === 'celsius'
                ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
                }`}
              onClick={() => temperatureStore.setUnit('celsius')}
            >
              °C
            </button>
            <button
              type="button"
              class={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${temperatureStore.unit() === 'fahrenheit'
                ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 shadow-sm'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
                }`}
              onClick={() => temperatureStore.setUnit('fahrenheit')}
            >
              °F
            </button>
          </div>
        </div>

        {/* Full-width Mode Toggle */}
        <div class="flex items-center justify-between gap-4 pt-4 border-t border-gray-200 dark:border-gray-700">
          <div class="flex items-center gap-3">
            <div class={`p-2.5 rounded-xl transition-all duration-300 ${layoutStore.isFullWidth()
              ? 'bg-blue-500 shadow-lg shadow-blue-500/25'
              : 'bg-gray-400 shadow-lg shadow-gray-500/25'
              }`}>
              <Maximize2 class="w-5 h-5 text-white" strokeWidth={2} />
            </div>
            <div class="text-sm text-gray-600 dark:text-gray-400">
              <p class="font-medium text-gray-900 dark:text-gray-100">
                Full-width mode
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                Expand content to use all available screen width on large monitors
              </p>
            </div>
          </div>
          <Toggle
            checked={layoutStore.isFullWidth()}
            onChange={() => layoutStore.toggle()}
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
          <p class="text-sm text-gray-600 dark:text-gray-400">
            Shorter intervals provide near-real-time updates at the cost of higher API and CPU
            usage on each node. Set a longer interval to reduce load on busy clusters.
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">
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
                  class={`rounded-lg border px-3 py-2 text-left text-sm transition-colors ${props.pvePollingSelection() === option.value
                    ? 'border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900/30 dark:text-blue-100'
                    : 'border-gray-200 bg-white text-gray-700 hover:border-blue-400 hover:text-blue-600 dark:border-gray-600 dark:bg-gray-900/50 dark:text-gray-200'
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
              class={`rounded-lg border px-3 py-2 text-left text-sm transition-colors ${props.pvePollingSelection() === 'custom'
                ? 'border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-400 dark:bg-blue-900/30 dark:text-blue-100'
                : 'border-gray-200 bg-white text-gray-700 hover:border-blue-400 hover:text-blue-600 dark:border-gray-600 dark:bg-gray-900/50 dark:text-gray-200'
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
            <div class="space-y-2 rounded-md border border-dashed border-gray-300 p-4 dark:border-gray-600">
              <label class="text-xs font-medium text-gray-700 dark:text-gray-200">
                Custom polling interval (10-3600 seconds)
              </label>
              <input
                type="number"
                min={PVE_POLLING_MIN_SECONDS}
                max={PVE_POLLING_MAX_SECONDS}
                value={props.pvePollingCustomSeconds()}
                class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-900/60"
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
              <p class="text-[0.68rem] text-gray-500 dark:text-gray-400">
                Applies to all PVE clusters and standalone nodes.
              </p>
            </div>
          </Show>

          {/* Env override warning */}
          <Show when={props.pvePollingEnvLocked()}>
            <div class="flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-800 dark:bg-amber-900/30 dark:text-amber-200">
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
