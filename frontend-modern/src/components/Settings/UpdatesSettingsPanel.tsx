import { Component, Show, For, Accessor, Setter } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { HelpIcon } from '@/components/shared/HelpIcon';
import {
  SelectionCardGroup,
  type SelectionCardOption,
} from '@/components/shared/SelectionCardGroup';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import CheckCircle from 'lucide-solid/icons/check-circle';
import ArrowRight from 'lucide-solid/icons/arrow-right';
import Package from 'lucide-solid/icons/package';
import type { UpdateInfo, VersionInfo, UpdatePlan } from '@/api/updates';
import { buildDockerImageTag, buildLinuxAmd64DownloadCommand } from '@/components/updateVersion';
import { UpdateInstallGuide } from '@/components/Settings/UpdateInstallGuide';
import {
  getUpdateChannelCardOptions,
  type UpdateChannelOptionValue,
} from '@/components/Settings/updatesSettingsModel';
import {
  getUpdateAvailabilityHeading,
  getUpdateBuildBadges,
  getUpdateCheckModeLabel,
  getUpdatePrimaryStatusLabel,
} from '@/utils/updatesPresentation';

export interface UpdatesSettingsPanelProps {
  versionInfo: Accessor<VersionInfo | null>;
  updateInfo: Accessor<UpdateInfo | null>;
  checkingForUpdates: Accessor<boolean>;
  updateChannel: Accessor<'stable' | 'rc'>;
  setUpdateChannel: Setter<'stable' | 'rc'>;
  autoUpdateEnabled: Accessor<boolean>;
  setAutoUpdateEnabled: Setter<boolean>;
  autoUpdateCheckInterval: Accessor<number>;
  setAutoUpdateCheckInterval: Setter<number>;
  autoUpdateTime: Accessor<string>;
  setAutoUpdateTime: Setter<string>;
  checkForUpdates: () => Promise<void>;
  setHasUnsavedChanges: Setter<boolean>;
  // Update installation props
  updatePlan: Accessor<UpdatePlan | null>;
  onInstallUpdate: () => void;
  isInstalling: Accessor<boolean>;
}

export const UpdatesSettingsPanel: Component<UpdatesSettingsPanelProps> = (props) => {
  const latestVersion = () => props.updateInfo()?.latestVersion;
  const dockerImageTag = () => buildDockerImageTag(latestVersion());
  const systemdDownloadCommand = () => buildLinuxAmd64DownloadCommand(latestVersion());
  const isPreviewChannel = () => props.updateChannel() === 'rc';
  const autoUpdateLocked = () => Boolean(props.versionInfo()?.isSourceBuild || isPreviewChannel());
  const updateChannelOptions = (): SelectionCardOption<UpdateChannelOptionValue>[] =>
    getUpdateChannelCardOptions(props.versionInfo()).map((option) => ({
      ...option,
      icon: (iconProps) =>
        option.value === 'stable' ? (
          <svg
            class={`w-5 h-5 ${iconProps.active ? 'text-green-600 dark:text-green-400' : 'text-muted'}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
            />
          </svg>
        ) : (
          <svg
            class={`w-5 h-5 ${iconProps.active ? 'text-blue-600 dark:text-blue-400' : 'text-muted'}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z"
            />
          </svg>
        ),
    }));

  return (
    <SettingsPanel
      title="Updates"
      description="Manage version checks and automatic update preferences."
      icon={<RefreshCw class="w-5 h-5" strokeWidth={2} />}
      noPadding
      bodyClass="divide-y divide-border"
    >
        <div class="p-4 sm:p-6">
          <div class="space-y-4">
            {/* Version Status Section */}
            <div class="rounded-md border border-border overflow-hidden">
              {/* Version Grid */}
              <div
                class={`grid gap-px ${props.updateInfo()?.available ? 'sm:grid-cols-3' : 'sm:grid-cols-2'}`}
              >
                {/* Current Version */}
                <div class="bg-surface-alt p-4">
                  <div class="flex items-start gap-3">
                    <div class="p-2 bg-blue-100 dark:bg-blue-900 rounded-md">
                      <Package class="w-5 h-5 text-blue-600 dark:text-blue-400" />
                    </div>
                    <div class="flex-1 min-w-0">
                      <p class="text-xs font-medium uppercase tracking-wide text-muted">
                        Current Version
                      </p>
                      <p class="mt-1 text-lg font-bold text-base-content truncate">
                        {props.versionInfo()?.version || 'Loading...'}
                      </p>
                      <div class="mt-1.5 flex flex-wrap items-center gap-1.5">
                        <For each={getUpdateBuildBadges(props.versionInfo())}>
                          {(badge) => <span class={badge.className}>{badge.label}</span>}
                        </For>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Update Status / Arrow */}
                <Show when={props.updateInfo()?.available}>
                  <div class="bg-green-50 dark:bg-green-900 p-4 flex items-center justify-center">
                    <div class="flex flex-col items-center gap-1.5">
                      <div class="flex items-center gap-2 text-green-600 dark:text-green-400">
                        <ArrowRight class="w-5 h-5" />
                      </div>
                      <span class="text-xs font-semibold text-green-700 dark:text-green-300 uppercase tracking-wide">
                        {getUpdatePrimaryStatusLabel(true)}
                      </span>
                    </div>
                  </div>
                </Show>

                {/* Latest Version / Status */}
                <div
                  class={`p-4 ${
                    props.updateInfo()?.available
                      ? 'bg-green-50 dark:bg-green-900'
                      : 'bg-surface-alt'
                  }`}
                >
                  <div class="flex items-start gap-3">
                    <div
                      class={`p-2 rounded-md ${
                        props.updateInfo()?.available
                          ? 'bg-green-100 dark:bg-green-800'
                          : 'bg-surface-hover'
                      }`}
                    >
                      <Show
                        when={props.updateInfo()?.available}
                        fallback={<CheckCircle class="w-5 h-5 text-muted" />}
                      >
                        <CheckCircle class="w-5 h-5 text-green-600 dark:text-green-400" />
                      </Show>
                    </div>
                    <div class="flex-1 min-w-0">
                      <p class="text-xs font-medium uppercase tracking-wide text-muted">
                        {getUpdateAvailabilityHeading(Boolean(props.updateInfo()?.available))}
                      </p>
                      <Show
                        when={props.updateInfo()?.available}
                        fallback={
                          <p class="mt-1 text-lg font-bold text-base-content">
                            {getUpdatePrimaryStatusLabel(false)}
                          </p>
                        }
                      >
                        <p class="mt-1 text-lg font-bold text-green-700 dark:text-green-300">
                          {props.updateInfo()?.latestVersion}
                        </p>
                        <Show when={props.updateInfo()?.releaseDate}>
                          <p class="mt-0.5 text-xs text-green-600 dark:text-green-400">
                            Released{' '}
                            {new Date(props.updateInfo()!.releaseDate).toLocaleDateString()}
                          </p>
                        </Show>
                      </Show>
                    </div>
                  </div>
                </div>
              </div>

              {/* Check for updates button */}
              <div class="bg-surface border-t border-border px-4 py-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <p class="text-xs text-muted">{getUpdateCheckModeLabel(props.autoUpdateEnabled())}</p>
                <button
                  type="button"
                  onClick={props.checkForUpdates}
                  disabled={props.checkingForUpdates() || props.versionInfo()?.isSourceBuild}
                  class={`self-end sm:self-auto min-h-10 sm:min-h-9 px-4 py-2.5 text-sm rounded-md transition-colors flex items-center gap-2 ${
                    props.versionInfo()?.isSourceBuild
                      ? 'bg-surface-hover text-muted cursor-not-allowed'
                      : 'bg-blue-600 text-white hover:bg-blue-700'
                  }`}
                >
                  {props.checkingForUpdates() ? (
                    <>
                      <div class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></div>
                      Checking...
                    </>
                  ) : (
                    <>
                      <RefreshCw class="w-4 h-4" />
                      Check Now
                    </>
                  )}
                </button>
              </div>
            </div>

            <UpdateInstallGuide
              versionInfo={props.versionInfo()}
              updateInfo={props.updateInfo()}
              updatePlan={props.updatePlan()}
              isInstalling={props.isInstalling()}
              dockerImageTag={dockerImageTag()}
              systemdDownloadCommand={systemdDownloadCommand()}
              onInstallUpdate={props.onInstallUpdate}
            />
          </div>
        </div>

        <div class="p-4 sm:p-6">
          <div class="space-y-5">
            <h4 class="flex items-center gap-2 text-sm font-medium text-base-content">
              <svg
                class="w-4 h-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
                />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                />
              </svg>
              Update Preferences
              <HelpIcon contentId="updates.pulse.channel" size="xs" />
            </h4>

            {/* Update Channel */}
            <SelectionCardGroup
              class="sm:grid-cols-2"
              options={updateChannelOptions()}
              value={props.updateChannel()}
              onChange={(value) => {
                if (value === 'rc') {
                  props.setAutoUpdateEnabled(false);
                }
                props.setUpdateChannel(value);
                props.setHasUnsavedChanges(true);
              }}
              variant="detail"
            />

            <Show when={isPreviewChannel()}>
              <div class="rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-100">
                <p class="font-medium">RC is a manual preview channel.</p>
                <p class="mt-1 text-xs text-amber-800 dark:text-amber-200">
                  Use this on staging or internal validation environments. Automatic
                  stable updates stay disabled on RC so preview installs do not drift
                  between channels unattended.
                </p>
              </div>
            </Show>

            {/* Auto Update Toggle */}
            <div class="p-4 rounded-md border border-border bg-surface-alt">
              <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div class="flex items-center gap-3">
                  <div class="p-2 bg-blue-100 dark:bg-blue-900 rounded-md">
                    <svg
                      class="w-5 h-5 text-blue-600 dark:text-blue-400"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                  </div>
                  <div>
                    <label class="text-sm font-medium text-base-content">
                      Automatic Stable Updates
                    </label>
                    <p class="text-xs text-muted">
                      Supported host installs can automatically apply stable releases.
                      RC preview validation always stays manual.
                    </p>
                  </div>
                </div>
                <label class="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    data-testid="updates-auto-check-toggle"
                    checked={props.autoUpdateEnabled()}
                    onChange={(e) => {
                      props.setAutoUpdateEnabled(e.currentTarget.checked);
                      props.setHasUnsavedChanges(true);
                    }}
                    disabled={autoUpdateLocked()}
                    class="sr-only peer"
                  />
                  <div class="w-11 h-6 bg-surface-alt peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
                </label>
              </div>

              <Show when={isPreviewChannel()}>
                <p class="mt-3 text-xs text-amber-700 dark:text-amber-300">
                  Automatic stable updates are unavailable while the RC preview
                  channel is selected.
                </p>
              </Show>

              {/* Auto update options (shown when enabled) */}
              <Show when={props.autoUpdateEnabled() && !isPreviewChannel()}>
                <div class="mt-4 pt-4 border-t border-border grid grid-cols-1 sm:grid-cols-2 gap-4">
                  {/* Check Interval */}
                  <div class="space-y-2">
                    <label class="text-xs font-medium text-base-content">Check Interval</label>
                    <select
                      value={props.autoUpdateCheckInterval()}
                      onChange={(e) => {
                        props.setAutoUpdateCheckInterval(parseInt(e.currentTarget.value));
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-2 text-sm border border-border rounded-md bg-surface"
                    >
                      <option value="6">Every 6 hours</option>
                      <option value="12">Every 12 hours</option>
                      <option value="24">Daily</option>
                      <option value="168">Weekly</option>
                    </select>
                  </div>

                  {/* Check Time */}
                  <div class="space-y-2">
                    <label class="text-xs font-medium text-base-content">Preferred Time</label>
                    <input
                      type="time"
                      value={props.autoUpdateTime()}
                      onChange={(e) => {
                        props.setAutoUpdateTime(e.currentTarget.value);
                        props.setHasUnsavedChanges(true);
                      }}
                      class="w-full px-3 py-2 text-sm border border-border rounded-md bg-surface"
                    />
                  </div>
                </div>
              </Show>
            </div>
          </div>
        </div>
    </SettingsPanel>
  );
};
