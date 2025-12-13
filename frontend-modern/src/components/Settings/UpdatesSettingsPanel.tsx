import { Component, Show, Accessor, Setter } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import CheckCircle from 'lucide-solid/icons/check-circle';
import ArrowRight from 'lucide-solid/icons/arrow-right';
import Package from 'lucide-solid/icons/package';
import type { UpdateInfo, VersionInfo } from '@/api/updates';

interface UpdatesSettingsPanelProps {
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
}

export const UpdatesSettingsPanel: Component<UpdatesSettingsPanelProps> = (props) => {
  return (
    <div class="space-y-6">
      <Card
        padding="none"
        class="overflow-hidden border border-gray-200 dark:border-gray-700"
        border={false}
      >
        {/* Header */}
        <div class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
          <div class="flex items-center gap-3">
            <div class="p-2 bg-blue-100 dark:bg-blue-900/40 rounded-lg">
              <RefreshCw class="w-5 h-5 text-blue-600 dark:text-blue-300" strokeWidth={2} />
            </div>
            <SectionHeader
              title="Updates"
              description="Version checking and automatic update configuration"
              size="sm"
              class="flex-1"
            />
          </div>
        </div>

        <div class="p-6 space-y-6">
          <section class="space-y-4">
            <div class="space-y-4">
              {/* Version Status Section */}
              <div class="rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
                {/* Version Grid */}
                <div class={`grid gap-px ${props.updateInfo()?.available ? 'sm:grid-cols-3' : 'sm:grid-cols-2'}`}>
                  {/* Current Version */}
                  <div class="bg-gray-50 dark:bg-gray-800/60 p-4">
                    <div class="flex items-start gap-3">
                      <div class="p-2 bg-blue-100 dark:bg-blue-900/50 rounded-lg">
                        <Package class="w-5 h-5 text-blue-600 dark:text-blue-400" />
                      </div>
                      <div class="flex-1 min-w-0">
                        <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          Current Version
                        </p>
                        <p class="mt-1 text-lg font-bold text-gray-900 dark:text-gray-100 truncate">
                          {props.versionInfo()?.version || 'Loading...'}
                        </p>
                        <div class="mt-1.5 flex flex-wrap items-center gap-1.5">
                          <Show when={props.versionInfo()?.isDevelopment}>
                            <span class="inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300">
                              Development
                            </span>
                          </Show>
                          <Show when={props.versionInfo()?.isDocker}>
                            <span class="inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300">
                              Docker
                            </span>
                          </Show>
                          <Show when={props.versionInfo()?.isSourceBuild}>
                            <span class="inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300">
                              Source
                            </span>
                          </Show>
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Update Status / Arrow */}
                  <Show when={props.updateInfo()?.available}>
                    <div class="bg-gradient-to-r from-green-50 to-emerald-50 dark:from-green-900/30 dark:to-emerald-900/30 p-4 flex items-center justify-center">
                      <div class="flex flex-col items-center gap-1.5">
                        <div class="flex items-center gap-2 text-green-600 dark:text-green-400">
                          <ArrowRight class="w-5 h-5" />
                        </div>
                        <span class="text-xs font-semibold text-green-700 dark:text-green-300 uppercase tracking-wide">
                          Update Ready
                        </span>
                      </div>
                    </div>
                  </Show>

                  {/* Latest Version / Status */}
                  <div class={`p-4 ${props.updateInfo()?.available
                    ? 'bg-gradient-to-br from-green-50 to-emerald-50 dark:from-green-900/20 dark:to-emerald-900/20'
                    : 'bg-gray-50 dark:bg-gray-800/60'
                    }`}>
                    <div class="flex items-start gap-3">
                      <div class={`p-2 rounded-lg ${props.updateInfo()?.available
                        ? 'bg-green-100 dark:bg-green-900/50'
                        : 'bg-gray-100 dark:bg-gray-700'
                        }`}>
                        <Show when={props.updateInfo()?.available} fallback={
                          <CheckCircle class="w-5 h-5 text-gray-500 dark:text-gray-400" />
                        }>
                          <CheckCircle class="w-5 h-5 text-green-600 dark:text-green-400" />
                        </Show>
                      </div>
                      <div class="flex-1 min-w-0">
                        <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                          {props.updateInfo()?.available ? 'Available' : 'Status'}
                        </p>
                        <Show when={props.updateInfo()?.available} fallback={
                          <p class="mt-1 text-lg font-bold text-gray-900 dark:text-gray-100">
                            Up to date
                          </p>
                        }>
                          <p class="mt-1 text-lg font-bold text-green-700 dark:text-green-300">
                            {props.updateInfo()?.latestVersion}
                          </p>
                          <Show when={props.updateInfo()?.releaseDate}>
                            <p class="mt-0.5 text-xs text-green-600 dark:text-green-400">
                              Released {new Date(props.updateInfo()!.releaseDate).toLocaleDateString()}
                            </p>
                          </Show>
                        </Show>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Check for updates button */}
                <div class="bg-white dark:bg-gray-900/50 border-t border-gray-200 dark:border-gray-700 px-4 py-3 flex items-center justify-between">
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    <Show when={props.autoUpdateEnabled()}>
                      Auto-check enabled
                    </Show>
                    <Show when={!props.autoUpdateEnabled()}>
                      Manual checks only
                    </Show>
                  </p>
                  <button
                    type="button"
                    onClick={props.checkForUpdates}
                    disabled={
                      props.checkingForUpdates() ||
                      props.versionInfo()?.isDocker ||
                      props.versionInfo()?.isSourceBuild
                    }
                    class={`px-4 py-2 text-sm rounded-lg transition-colors flex items-center gap-2 ${props.versionInfo()?.isDocker || props.versionInfo()?.isSourceBuild
                      ? 'bg-gray-100 dark:bg-gray-700 text-gray-400 dark:text-gray-500 cursor-not-allowed'
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

              {/* Docker installation notice */}
              <Show when={props.versionInfo()?.isDocker && !props.updateInfo()?.available}>
                <div class="p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                  <p class="text-xs text-blue-800 dark:text-blue-200">
                    <strong>Docker Installation:</strong> Updates are managed through Docker. Pull
                    the latest image to update.
                  </p>
                </div>
              </Show>

              {/* Source build notice */}
              <Show when={props.versionInfo()?.isSourceBuild}>
                <div class="p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                  <p class="text-xs text-blue-800 dark:text-blue-200">
                    <strong>Built from source:</strong> Pull the latest code from git and rebuild to
                    update.
                  </p>
                </div>
              </Show>

              {/* Warning message */}
              <Show when={Boolean(props.updateInfo()?.warning)}>
                <div class="p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg">
                  <p class="text-xs text-amber-800 dark:text-amber-200">
                    {props.updateInfo()?.warning}
                  </p>
                </div>
              </Show>

              {/* Update available */}
              <Show when={props.updateInfo()?.available}>
                <div class="p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-xl space-y-3">
                  <div class="flex items-center gap-2">
                    <svg class="w-5 h-5 text-green-600 dark:text-green-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <h4 class="text-sm font-semibold text-green-900 dark:text-green-100">
                      How to install the update
                    </h4>
                  </div>

                  <div class="p-3 bg-green-100 dark:bg-green-900/40 rounded-lg space-y-2">
                    <Show when={props.versionInfo()?.deploymentType === 'proxmoxve'}>
                      <p class="text-xs text-green-700 dark:text-green-300">
                        Type{' '}
                        <code class="px-1 py-0.5 bg-green-200 dark:bg-green-800 rounded">update</code>{' '}
                        in the LXC console
                      </p>
                    </Show>
                    <Show when={props.versionInfo()?.deploymentType === 'docker'}>
                      <div class="text-xs text-green-700 dark:text-green-300 space-y-1">
                        <p>Run these commands:</p>
                        <code class="block p-1 bg-green-200 dark:bg-green-800 rounded text-xs">
                          docker pull rcourtman/pulse:latest
                          <br />
                          docker restart pulse
                        </code>
                      </div>
                    </Show>
                    <Show
                      when={
                        props.versionInfo()?.deploymentType === 'systemd' ||
                        props.versionInfo()?.deploymentType === 'manual'
                      }
                    >
                      <div class="text-xs text-green-700 dark:text-green-300 space-y-1">
                        <p>
                          Click the "Install Update" button below, or download and install manually:
                        </p>
                        <code class="block p-1 bg-green-200 dark:bg-green-800 rounded text-xs">
                          curl -LO
                          https://github.com/rcourtman/Pulse/releases/download/
                          {props.updateInfo()?.latestVersion}/pulse-{props.updateInfo()?.latestVersion}
                          -linux-amd64.tar.gz
                          <br />
                          sudo systemctl stop pulse
                          <br />
                          sudo tar -xzf pulse-{props.updateInfo()?.latestVersion}
                          -linux-amd64.tar.gz -C /usr/local/bin pulse
                          <br />
                          sudo systemctl start pulse
                        </code>
                      </div>
                    </Show>
                    <Show when={props.versionInfo()?.deploymentType === 'development'}>
                      <p class="text-xs text-green-700 dark:text-green-300">
                        Pull latest changes and rebuild
                      </p>
                    </Show>
                    <Show when={!props.versionInfo()?.deploymentType && props.versionInfo()?.isDocker}>
                      <p class="text-xs text-green-700 dark:text-green-300">
                        Pull the latest Pulse Docker image and recreate your container.
                      </p>
                    </Show>
                  </div>

                  {/* Release notes */}
                  <Show when={props.updateInfo()?.releaseNotes}>
                    <details class="mt-1">
                      <summary class="text-xs text-green-700 dark:text-green-300 cursor-pointer">
                        Release Notes
                      </summary>
                      <pre class="mt-2 text-xs text-green-600 dark:text-green-400 whitespace-pre-wrap font-mono bg-green-100 dark:bg-green-900/30 p-2 rounded">
                        {props.updateInfo()?.releaseNotes}
                      </pre>
                    </details>
                  </Show>
                </div>
              </Show>

              {/* Update settings */}
              <div class="border-t border-gray-200 dark:border-gray-600 pt-4 space-y-4">
                {/* Update Channel */}
                <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                      Update Channel
                    </label>
                    <p class="text-xs text-gray-600 dark:text-gray-400">
                      Choose between stable and release candidate versions
                    </p>
                  </div>
                  <select
                    value={props.updateChannel()}
                    onChange={(e) => {
                      props.setUpdateChannel(e.currentTarget.value as 'stable' | 'rc');
                      props.setHasUnsavedChanges(true);
                    }}
                    disabled={props.versionInfo()?.isDocker}
                    class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 disabled:opacity-50"
                  >
                    <option value="stable">Stable</option>
                    <option value="rc">Release Candidate</option>
                  </select>
                </div>

                {/* Auto Update Toggle */}
                <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                      Update Checks
                    </label>
                    <p class="text-xs text-gray-600 dark:text-gray-400">
                      Automatically check for updates (installation is manual)
                    </p>
                  </div>
                  <label class="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={props.autoUpdateEnabled()}
                      onChange={(e) => {
                        props.setAutoUpdateEnabled(e.currentTarget.checked);
                        props.setHasUnsavedChanges(true);
                      }}
                      disabled={props.versionInfo()?.isDocker}
                      class="sr-only peer"
                    />
                    <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
                  </label>
                </div>

                {/* Auto update options (shown when enabled) */}
                <Show when={props.autoUpdateEnabled()}>
                  <div class="space-y-4 rounded-md border border-gray-200 dark:border-gray-600 p-3">
                    {/* Check Interval */}
                    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                          Check Interval
                        </label>
                        <p class="text-xs text-gray-600 dark:text-gray-400">
                          How often to check for updates
                        </p>
                      </div>
                      <select
                        value={props.autoUpdateCheckInterval()}
                        onChange={(e) => {
                          props.setAutoUpdateCheckInterval(parseInt(e.currentTarget.value));
                          props.setHasUnsavedChanges(true);
                        }}
                        class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                      >
                        <option value="6">Every 6 hours</option>
                        <option value="12">Every 12 hours</option>
                        <option value="24">Daily</option>
                        <option value="168">Weekly</option>
                      </select>
                    </div>

                    {/* Check Time */}
                    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                          Check Time
                        </label>
                        <p class="text-xs text-gray-600 dark:text-gray-400">
                          Preferred time to check for updates
                        </p>
                      </div>
                      <input
                        type="time"
                        value={props.autoUpdateTime()}
                        onChange={(e) => {
                          props.setAutoUpdateTime(e.currentTarget.value);
                          props.setHasUnsavedChanges(true);
                        }}
                        class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800"
                      />
                    </div>
                  </div>
                </Show>
              </div>
            </div>
          </section>
        </div>
      </Card>
    </div>
  );
};
