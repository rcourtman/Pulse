import { Component, Show, For, Accessor, Setter } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { Toggle } from '@/components/shared/Toggle';
import Sliders from 'lucide-solid/icons/sliders-horizontal';
import Activity from 'lucide-solid/icons/activity';

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
      <Card
        padding="none"
        class="overflow-hidden border border-gray-200 dark:border-gray-700"
        border={false}
      >
        <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
              <Sliders class="w-5 h-5 text-blue-600 dark:text-blue-300" strokeWidth={2} />
            </div>
            <SectionHeader
              title="General"
              description="Appearance and display preferences"
              size="sm"
              class="flex-1"
            />
          </div>
        </div>
        <div class="p-6 space-y-5">
          <div class="flex items-center justify-between gap-3">
            <div class="text-sm text-gray-600 dark:text-gray-400">
              <p class="font-medium text-gray-900 dark:text-gray-100">Dark mode</p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                Toggle to match your environment. Pulse remembers this preference on each browser.
              </p>
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
        </div>
      </Card>

      {/* Monitoring Cadence Card */}
      <Card
        padding="none"
        class="overflow-hidden border border-gray-200 dark:border-gray-700"
        border={false}
      >
        <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
              <Activity class="w-5 h-5 text-blue-600 dark:text-blue-300" strokeWidth={2} />
            </div>
            <SectionHeader
              title="Monitoring cadence"
              description="Control how frequently Pulse polls Proxmox VE nodes."
              size="sm"
              class="flex-1"
            />
          </div>
        </div>
        <div class="p-6 space-y-5">
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
                    class={`rounded-lg border px-3 py-2 text-left text-sm transition-colors ${
                      props.pvePollingSelection() === option.value
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
                class={`rounded-lg border px-3 py-2 text-left text-sm transition-colors ${
                  props.pvePollingSelection() === 'custom'
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
        </div>
      </Card>
    </div>
  );
};
