import { Component, Show, Accessor, Setter } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { HelpIcon } from '@/components/shared/HelpIcon';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import CheckCircle from 'lucide-solid/icons/check-circle';
import ArrowRight from 'lucide-solid/icons/arrow-right';
import Package from 'lucide-solid/icons/package';
import Download from 'lucide-solid/icons/download';
import type { UpdateInfo, VersionInfo, UpdatePlan } from '@/api/updates';
import {
  buildDockerImageTag,
  buildLinuxAmd64DownloadCommand,
} from '@/components/updateVersion';

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
  // Update installation props
  updatePlan: Accessor<UpdatePlan | null>;
  onInstallUpdate: () => void;
  isInstalling: Accessor<boolean>;
}

export const UpdatesSettingsPanel: Component<UpdatesSettingsPanelProps> = (props) => {
  const latestVersion = () => props.updateInfo()?.latestVersion;
  const dockerImageTag = () => buildDockerImageTag(latestVersion());
  const systemdDownloadCommand = () => buildLinuxAmd64DownloadCommand(latestVersion());

  return (
    <div class="space-y-6">
      <SettingsPanel
        title="Updates"
        description="Manage version checks and automatic update preferences."
        icon={<RefreshCw class="w-5 h-5" strokeWidth={2} />}
        bodyClass="space-y-6"
      >
        <section class="space-y-4">
          <div class="space-y-4">
            {/* Version Status Section */}
            <div class="rounded-md border border-slate-200 dark:border-slate-700 overflow-hidden">
              {/* Version Grid */}
              <div class={`grid gap-px ${props.updateInfo()?.available ? 'sm:grid-cols-3' : 'sm:grid-cols-2'}`}>
                {/* Current Version */}
                <div class="bg-slate-50 dark:bg-slate-800 p-4">
                  <div class="flex items-start gap-3">
                    <div class="p-2 bg-blue-100 dark:bg-blue-900 rounded-md">
                      <Package class="w-5 h-5 text-blue-600 dark:text-blue-400" />
                    </div>
                    <div class="flex-1 min-w-0">
                      <p class="text-xs font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">
                        Current Version
                      </p>
                      <p class="mt-1 text-lg font-bold text-slate-900 dark:text-slate-100 truncate">
                        {props.versionInfo()?.version || 'Loading...'}
                      </p>
                      <div class="mt-1.5 flex flex-wrap items-center gap-1.5">
                        <Show when={props.versionInfo()?.isDevelopment}>
                          <span class="inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300">
                            Development
                          </span>
                        </Show>
                        <Show when={props.versionInfo()?.isDocker}>
                          <span class="inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">
                            Docker
                          </span>
                        </Show>
                        <Show when={props.versionInfo()?.isSourceBuild}>
                          <span class="inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-slate-100 text-slate-700 dark:bg-slate-700 dark:text-slate-300">
                            Source
                          </span>
                        </Show>
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
                        Update Ready
                      </span>
                    </div>
                  </div>
                </Show>

                {/* Latest Version / Status */}
                <div class={`p-4 ${props.updateInfo()?.available
                  ? 'bg-green-50 dark:bg-green-900'
                  : 'bg-slate-50 dark:bg-slate-800'
                  }`}>
                  <div class="flex items-start gap-3">
                    <div class={`p-2 rounded-md ${props.updateInfo()?.available
                      ? 'bg-green-100 dark:bg-green-800'
                      : 'bg-slate-100 dark:bg-slate-700'
                      }`}>
                      <Show when={props.updateInfo()?.available} fallback={
                        <CheckCircle class="w-5 h-5 text-slate-500 dark:text-slate-400" />
                      }>
                        <CheckCircle class="w-5 h-5 text-green-600 dark:text-green-400" />
                      </Show>
                    </div>
                    <div class="flex-1 min-w-0">
                      <p class="text-xs font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">
                        {props.updateInfo()?.available ? 'Available' : 'Status'}
                      </p>
                      <Show when={props.updateInfo()?.available} fallback={
                        <p class="mt-1 text-lg font-bold text-slate-900 dark:text-slate-100">
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
              <div class="bg-white dark:bg-slate-800 border-t border-slate-200 dark:border-slate-700 px-4 py-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <p class="text-xs text-slate-500 dark:text-slate-400">
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
                    props.versionInfo()?.isSourceBuild
                  }
                  class={`self-end sm:self-auto min-h-10 sm:min-h-9 px-4 py-2.5 text-sm rounded-md transition-colors flex items-center gap-2 ${props.versionInfo()?.isSourceBuild
                    ? 'bg-slate-100 dark:bg-slate-700 text-slate-400 dark:text-slate-500 cursor-not-allowed'
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

            {/* Docker installation notice - enhanced with copy-able commands */}
            <Show when={props.versionInfo()?.isDocker && !props.updateInfo()?.available}>
              <div class="p-4 bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md space-y-3">
                <div class="flex items-center gap-2">
                  <svg class="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M13.983 11.078h2.119a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.119a.186.186 0 00-.185.186v1.887c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 00.186-.186V3.574a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.888c0 .102.082.186.185.186m0 2.716h2.118a.187.187 0 00.186-.186V6.29a.186.186 0 00-.186-.185h-2.118a.186.186 0 00-.185.185v1.888c0 .102.082.185.185.186m-2.93 0h2.12a.186.186 0 00.184-.186V6.29a.185.185 0 00-.185-.185H8.1a.185.185 0 00-.185.185v1.888c0 .102.083.185.185.186m-2.964 0h2.119a.186.186 0 00.185-.186V6.29a.186.186 0 00-.185-.185H5.136a.186.186 0 00-.186.185v1.888c0 .102.084.185.186.186m5.893 2.715h2.118a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.118a.186.186 0 00-.185.186v1.887c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.186v1.887c0 .102.083.185.185.185m-2.964 0h2.119a.185.185 0 00.185-.185V9.006a.185.185 0 00-.185-.186h-2.119a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.186.186 0 00-.186.186v1.887c0 .102.084.185.186.185m-.001 2.716h2.118a.185.185 0 00.185-.185v-1.888a.185.185 0 00-.185-.185H2.136a.185.185 0 00-.186.185v1.888c0 .102.084.185.186.185m23.063-3.167a.509.509 0 00-.376-.25.431.431 0 00-.116-.01.431.431 0 00-.114.01 3.6 3.6 0 00-1.618.877c-.186.166-.356.36-.509.577a6.6 6.6 0 00-1.117-1.474 6.6 6.6 0 00-9.336 0 6.6 6.6 0 00-1.938 4.684 6.6 6.6 0 001.938 4.684 6.6 6.6 0 004.668 1.938 6.6 6.6 0 004.668-1.938 6.6 6.6 0 001.938-4.684 6.6 6.6 0 00-.185-1.41 3.6 3.6 0 001.587-.904.509.509 0 00.134-.459" />
                  </svg>
                  <p class="text-sm font-medium text-blue-800 dark:text-blue-200">
                    Docker Installation
                  </p>
                </div>
                <p class="text-xs text-blue-700 dark:text-blue-300">
                  Updates are managed through Docker. Use these commands to check for and apply updates:
                </p>
                <div class="space-y-2">
                  <div class="relative group">
                    <code class="block p-2.5 bg-slate-900 dark:bg-slate-950 rounded-md text-xs font-mono text-blue-400 border border-slate-700">
                      docker pull rcourtman/pulse:latest && docker restart pulse
                    </code>
                    <button
                      type="button"
                      onClick={() => navigator.clipboard.writeText('docker pull rcourtman/pulse:latest && docker restart pulse')}
                      class="absolute top-1.5 right-1.5 p-1 rounded bg-slate-700 hover:bg-slate-600 text-slate-300 opacity-60 hover:opacity-100 transition-opacity"
                      title="Copy to clipboard"
                    >
                      <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                      </svg>
                    </button>
                  </div>
                  <p class="text-[10px] text-blue-600 dark:text-blue-400">
                    Or with Docker Compose: <code class="px-1 py-0.5 bg-blue-100 dark:bg-blue-800 rounded text-[10px]">docker-compose pull && docker-compose up -d</code>
                  </p>
                </div>
              </div>
            </Show>

            {/* Source build notice */}
            <Show when={props.versionInfo()?.isSourceBuild}>
              <div class="p-3 bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md">
                <p class="text-xs text-blue-800 dark:text-blue-200">
                  <strong>Built from source:</strong> Pull the latest code from git and rebuild to
                  update.
                </p>
              </div>
            </Show>

            {/* Warning message */}
            <Show when={Boolean(props.updateInfo()?.warning)}>
              <div class="p-3 bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-700 rounded-md">
                <p class="text-xs text-amber-800 dark:text-amber-200">
                  {props.updateInfo()?.warning}
                </p>
              </div>
            </Show>

            {/* Update available */}
            <Show when={props.updateInfo()?.available}>
              <div class="rounded-md border border-green-200 dark:border-green-700 overflow-hidden bg-green-50 dark:bg-green-900">
                {/* Header */}
                <div class="px-5 py-4 border-b border-green-200 dark:border-green-800 bg-green-100 dark:bg-green-800">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div class="flex items-center gap-3">
                      <div class="p-2 bg-green-100 dark:bg-green-900 rounded-md">
                        <svg class="w-5 h-5 text-green-700 dark:text-green-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                          <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                        </svg>
                      </div>
                      <div>
                        <h4 class="text-base font-semibold text-green-900 dark:text-green-100">
                          Update Available
                        </h4>
                        <p class="text-xs text-green-700 dark:text-green-300">
                          Version {props.updateInfo()?.latestVersion} is ready to install
                        </p>
                      </div>
                    </div>

                    {/* Automated Install Button */}
                    <Show when={props.updatePlan()?.canAutoUpdate}>
                      <button
                        type="button"
                        onClick={props.onInstallUpdate}
                        disabled={props.isInstalling()}
                        class={`w-full justify-center sm:w-auto px-4 py-2.5 rounded-md text-sm font-medium transition-all flex items-center gap-2 ${props.isInstalling()
                          ? 'bg-green-400 dark:bg-green-600 text-white cursor-not-allowed'
                          : 'bg-green-600 hover:bg-green-700 text-white'
                          }`}
                      >
                        <Show when={props.isInstalling()} fallback={
                          <>
                            <Download class="w-4 h-4" />
                            Install Update
                          </>
                        }>
                          <div class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></div>
                          Installing...
                        </Show>
                      </button>
                    </Show>
                  </div>
                </div>

                {/* Installation Steps */}
                <div class="p-5 space-y-4">
                  {/* Manual Steps Header */}
                  <Show when={!props.updatePlan()?.canAutoUpdate}>
                    <div class="text-sm font-medium text-green-800 dark:text-green-200 mb-3">
                      Follow these steps to update manually:
                    </div>
                  </Show>

                  <Show when={props.updatePlan()?.canAutoUpdate}>
                    <div class="text-sm text-green-700 dark:text-green-300 mb-3">
                      Click "Install Update" above for automatic installation, or update manually:
                    </div>
                  </Show>

                  {/* ProxmoxVE LXC Installation */}
                  <Show when={props.versionInfo()?.deploymentType === 'proxmoxve'}>
                    <div class="space-y-3">
                      <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
                        <span class="flex items-center justify-center w-6 h-6 rounded-full bg-green-200 dark:bg-green-800 text-xs font-bold text-green-700 dark:text-green-300">1</span>
                        Open your Pulse LXC console
                      </div>
                      <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
                        <span class="flex items-center justify-center w-6 h-6 rounded-full bg-green-200 dark:bg-green-800 text-xs font-bold text-green-700 dark:text-green-300">2</span>
                        Run the update command:
                      </div>
                      <div class="ml-0 sm:ml-8 relative group">
                        <code class="block p-3 bg-slate-900 dark:bg-slate-950 rounded-md text-sm font-mono text-green-400 border border-slate-700">
                          update
                        </code>
                        <button
                          type="button"
                          onClick={() => navigator.clipboard.writeText('update')}
                          class="absolute right-2 top-2 inline-flex min-h-9 min-w-9 items-center justify-center rounded bg-slate-700 p-2 text-slate-300 opacity-70 transition-opacity hover:bg-slate-600 hover:opacity-100"
                          title="Copy to clipboard"
                        >
                          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                          </svg>
                        </button>
                      </div>
                      <p class="ml-0 sm:ml-8 text-xs text-green-600 dark:text-green-400">
                        The script will automatically download and install the latest version.
                      </p>
                    </div>
                  </Show>

                  {/* Docker Installation */}
                  <Show when={props.versionInfo()?.deploymentType === 'docker' || (!props.versionInfo()?.deploymentType && props.versionInfo()?.isDocker)}>
                    <div class="space-y-3">
                      <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
                        <span class="flex items-center justify-center w-6 h-6 rounded-full bg-green-200 dark:bg-green-800 text-xs font-bold text-green-700 dark:text-green-300">1</span>
                        Pull the latest image
                      </div>
                      <div class="ml-0 sm:ml-8 relative group">
                        <code class="block p-3 bg-slate-900 dark:bg-slate-950 rounded-md text-sm font-mono text-green-400 border border-slate-700">
                          docker pull rcourtman/pulse:{dockerImageTag()}
                        </code>
                        <button
                          type="button"
                          onClick={() => navigator.clipboard.writeText(`docker pull rcourtman/pulse:${dockerImageTag()}`)}
                          class="absolute right-2 top-2 inline-flex min-h-9 min-w-9 items-center justify-center rounded bg-slate-700 p-2 text-slate-300 opacity-70 transition-opacity hover:bg-slate-600 hover:opacity-100"
                          title="Copy to clipboard"
                        >
                          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                          </svg>
                        </button>
                      </div>

                      <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
                        <span class="flex items-center justify-center w-6 h-6 rounded-full bg-green-200 dark:bg-green-800 text-xs font-bold text-green-700 dark:text-green-300">2</span>
                        Restart the container
                      </div>
                      <div class="ml-0 sm:ml-8 relative group">
                        <code class="block p-3 bg-slate-900 dark:bg-slate-950 rounded-md text-sm font-mono text-green-400 border border-slate-700">
                          docker restart pulse
                        </code>
                        <button
                          type="button"
                          onClick={() => navigator.clipboard.writeText('docker restart pulse')}
                          class="absolute right-2 top-2 inline-flex min-h-9 min-w-9 items-center justify-center rounded bg-slate-700 p-2 text-slate-300 opacity-70 transition-opacity hover:bg-slate-600 hover:opacity-100"
                          title="Copy to clipboard"
                        >
                          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                          </svg>
                        </button>
                      </div>
                      <p class="ml-0 sm:ml-8 text-xs text-green-600 dark:text-green-400">
                        Or use Docker Compose: <code class="px-1.5 py-0.5 bg-slate-200 dark:bg-slate-700 rounded text-xs">docker-compose pull && docker-compose up -d</code>
                      </p>
                    </div>
                  </Show>

                  {/* Systemd/Manual Installation */}
                  <Show when={props.versionInfo()?.deploymentType === 'systemd' || props.versionInfo()?.deploymentType === 'manual'}>
                    <div class="space-y-3">
                      <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
                        <span class="flex items-center justify-center w-6 h-6 rounded-full bg-green-200 dark:bg-green-800 text-xs font-bold text-green-700 dark:text-green-300">1</span>
                        Stop the service
                      </div>
                      <div class="ml-0 sm:ml-8 relative group">
                        <code class="block p-3 bg-slate-900 dark:bg-slate-950 rounded-md text-sm font-mono text-green-400 border border-slate-700">
                          sudo systemctl stop pulse
                        </code>
                        <button
                          type="button"
                          onClick={() => navigator.clipboard.writeText('sudo systemctl stop pulse')}
                          class="absolute right-2 top-2 inline-flex min-h-9 min-w-9 items-center justify-center rounded bg-slate-700 p-2 text-slate-300 opacity-70 transition-opacity hover:bg-slate-600 hover:opacity-100"
                          title="Copy to clipboard"
                        >
                          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                          </svg>
                        </button>
                      </div>

                      <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
                        <span class="flex items-center justify-center w-6 h-6 rounded-full bg-green-200 dark:bg-green-800 text-xs font-bold text-green-700 dark:text-green-300">2</span>
                        Download and extract the new version
                      </div>
                      <div class="ml-0 sm:ml-8 relative group">
                        <code class="block p-3 bg-slate-900 dark:bg-slate-950 rounded-md text-sm font-mono text-green-400 border border-slate-700 whitespace-pre-wrap break-all">
                          {systemdDownloadCommand()}
                        </code>
                        <button
                          type="button"
                          onClick={() => navigator.clipboard.writeText(systemdDownloadCommand())}
                          class="absolute right-2 top-2 inline-flex min-h-9 min-w-9 items-center justify-center rounded bg-slate-700 p-2 text-slate-300 opacity-70 transition-opacity hover:bg-slate-600 hover:opacity-100"
                          title="Copy to clipboard"
                        >
                          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                          </svg>
                        </button>
                      </div>

                      <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
                        <span class="flex items-center justify-center w-6 h-6 rounded-full bg-green-200 dark:bg-green-800 text-xs font-bold text-green-700 dark:text-green-300">3</span>
                        Start the service
                      </div>
                      <div class="ml-0 sm:ml-8 relative group">
                        <code class="block p-3 bg-slate-900 dark:bg-slate-950 rounded-md text-sm font-mono text-green-400 border border-slate-700">
                          sudo systemctl start pulse
                        </code>
                        <button
                          type="button"
                          onClick={() => navigator.clipboard.writeText('sudo systemctl start pulse')}
                          class="absolute right-2 top-2 inline-flex min-h-9 min-w-9 items-center justify-center rounded bg-slate-700 p-2 text-slate-300 opacity-70 transition-opacity hover:bg-slate-600 hover:opacity-100"
                          title="Copy to clipboard"
                        >
                          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                          </svg>
                        </button>
                      </div>
                    </div>
                  </Show>

                  {/* Development Installation */}
                  <Show when={props.versionInfo()?.deploymentType === 'development'}>
                    <div class="space-y-3">
                      <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
                        <span class="flex items-center justify-center w-6 h-6 rounded-full bg-green-200 dark:bg-green-800 text-xs font-bold text-green-700 dark:text-green-300">1</span>
                        Pull the latest changes
                      </div>
                      <div class="ml-0 sm:ml-8 relative group">
                        <code class="block p-3 bg-slate-900 dark:bg-slate-950 rounded-md text-sm font-mono text-green-400 border border-slate-700">
                          git pull origin main
                        </code>
                        <button
                          type="button"
                          onClick={() => navigator.clipboard.writeText('git pull origin main')}
                          class="absolute right-2 top-2 inline-flex min-h-9 min-w-9 items-center justify-center rounded bg-slate-700 p-2 text-slate-300 opacity-70 transition-opacity hover:bg-slate-600 hover:opacity-100"
                          title="Copy to clipboard"
                        >
                          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                          </svg>
                        </button>
                      </div>

                      <div class="flex items-center gap-2 text-sm font-medium text-green-800 dark:text-green-200">
                        <span class="flex items-center justify-center w-6 h-6 rounded-full bg-green-200 dark:bg-green-800 text-xs font-bold text-green-700 dark:text-green-300">2</span>
                        Rebuild and restart
                      </div>
                      <div class="ml-0 sm:ml-8 relative group">
                        <code class="block p-3 bg-slate-900 dark:bg-slate-950 rounded-md text-sm font-mono text-green-400 border border-slate-700">
                          make build && make run
                        </code>
                        <button
                          type="button"
                          onClick={() => navigator.clipboard.writeText('make build && make run')}
                          class="absolute right-2 top-2 inline-flex min-h-9 min-w-9 items-center justify-center rounded bg-slate-700 p-2 text-slate-300 opacity-70 transition-opacity hover:bg-slate-600 hover:opacity-100"
                          title="Copy to clipboard"
                        >
                          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                          </svg>
                        </button>
                      </div>
                    </div>
                  </Show>
                </div>

                {/* Release notes footer */}
                <Show when={props.updateInfo()?.releaseNotes}>
                  <div class="px-5 py-3 border-t border-green-200 dark:border-green-800 bg-white dark:bg-slate-800">
                    <details class="group">
                      <summary class="flex items-center gap-2 text-sm font-medium text-green-700 dark:text-green-300 cursor-pointer hover:text-green-800 dark:hover:text-green-200 transition-colors">
                        <svg class="w-4 h-4 transition-transform group-open:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
                        </svg>
                        View Release Notes
                      </summary>
                      <pre class="mt-3 p-4 text-xs text-slate-700 dark:text-slate-300 whitespace-pre-wrap font-mono bg-slate-100 dark:bg-slate-800 rounded-md border border-slate-200 dark:border-slate-700 max-h-64 overflow-y-auto">
                        {props.updateInfo()?.releaseNotes}
                      </pre>
                    </details>
                  </div>
                </Show>
              </div>
            </Show>

            {/* Update settings */}
            <div class="border-t border-slate-200 dark:border-slate-700 pt-6 space-y-5">
              <h4 class="flex items-center gap-2 text-sm font-medium text-slate-700 dark:text-slate-300">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
                Update Preferences
                <HelpIcon contentId="updates.pulse.channel" size="xs" />
              </h4>

              {/* Update Channel */}
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <button
                  type="button"
                  onClick={() => {
                    props.setUpdateChannel('stable');
                    props.setHasUnsavedChanges(true);
                  }}
                  disabled={props.versionInfo()?.isSourceBuild}
                  class={`p-4 rounded-md border-2 transition-all text-left disabled:opacity-50 disabled:cursor-not-allowed ${props.updateChannel() === 'stable'
                    ? 'border-green-500 bg-green-50 dark:bg-green-900'
                    : 'border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600'
                    }`}
                >
                  <div class="flex items-center gap-3">
                    <div class={`p-2 rounded-md ${props.updateChannel() === 'stable'
                      ? 'bg-green-100 dark:bg-green-800'
                      : 'bg-slate-100 dark:bg-slate-800'
                      }`}>
                      <svg class={`w-5 h-5 ${props.updateChannel() === 'stable'
                        ? 'text-green-600 dark:text-green-400'
                        : 'text-slate-500 dark:text-slate-400'
                        }`} fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                      </svg>
                    </div>
                    <div>
                      <p class={`text-sm font-semibold ${props.updateChannel() === 'stable'
                        ? 'text-green-900 dark:text-green-100'
                        : 'text-slate-900 dark:text-slate-100'
                        }`}>Stable</p>
                      <p class="text-xs text-slate-500 dark:text-slate-400">
                        Production-ready releases
                      </p>
                    </div>
                  </div>
                </button>

                <button
                  type="button"
                  onClick={() => {
                    props.setUpdateChannel('rc');
                    props.setHasUnsavedChanges(true);
                  }}
                  disabled={props.versionInfo()?.isSourceBuild}
                  class={`p-4 rounded-md border-2 transition-all text-left disabled:opacity-50 disabled:cursor-not-allowed ${props.updateChannel() === 'rc'
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-900'
                    : 'border-slate-200 dark:border-slate-700 hover:border-slate-300 dark:hover:border-slate-600'
                    }`}
                >
                  <div class="flex items-center gap-3">
                    <div class={`p-2 rounded-md ${props.updateChannel() === 'rc'
                      ? 'bg-blue-100 dark:bg-blue-800'
                      : 'bg-slate-100 dark:bg-slate-800'
                      }`}>
                      <svg class={`w-5 h-5 ${props.updateChannel() === 'rc'
                        ? 'text-blue-600 dark:text-blue-400'
                        : 'text-slate-500 dark:text-slate-400'
                        }`} fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z" />
                      </svg>
                    </div>
                    <div>
                      <p class={`text-sm font-semibold ${props.updateChannel() === 'rc'
                        ? 'text-blue-900 dark:text-blue-100'
                        : 'text-slate-900 dark:text-slate-100'
                        }`}>Release Candidate</p>
                      <p class="text-xs text-slate-500 dark:text-slate-400">
                        Preview upcoming features
                      </p>
                    </div>
                  </div>
                </button>
              </div>

              {/* Auto Update Toggle */}
              <div class="p-4 rounded-md border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800">
                <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                  <div class="flex items-center gap-3">
                    <div class="p-2 bg-blue-100 dark:bg-blue-900 rounded-md">
                      <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                    </div>
                    <div>
                      <label class="text-sm font-medium text-slate-900 dark:text-slate-100">
                        Automatic Update Checks
                      </label>
                      <p class="text-xs text-slate-600 dark:text-slate-400">
                        Periodically check for new versions (installation is always manual)
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
                      disabled={props.versionInfo()?.isSourceBuild}
                      class="sr-only peer"
                    />
                    <div class="w-11 h-6 bg-slate-200 peer-focus:outline-none rounded-full peer dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
                  </label>
                </div>

                {/* Auto update options (shown when enabled) */}
                <Show when={props.autoUpdateEnabled()}>
                  <div class="mt-4 pt-4 border-t border-slate-200 dark:border-slate-700 grid grid-cols-1 sm:grid-cols-2 gap-4">
                    {/* Check Interval */}
                    <div class="space-y-2">
                      <label class="text-xs font-medium text-slate-700 dark:text-slate-300">
                        Check Interval
                      </label>
                      <select
                        value={props.autoUpdateCheckInterval()}
                        onChange={(e) => {
                          props.setAutoUpdateCheckInterval(parseInt(e.currentTarget.value));
                          props.setHasUnsavedChanges(true);
                        }}
                        class="w-full px-3 py-2 text-sm border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800"
                      >
                        <option value="6">Every 6 hours</option>
                        <option value="12">Every 12 hours</option>
                        <option value="24">Daily</option>
                        <option value="168">Weekly</option>
                      </select>
                    </div>

                    {/* Check Time */}
                    <div class="space-y-2">
                      <label class="text-xs font-medium text-slate-700 dark:text-slate-300">
                        Preferred Time
                      </label>
                      <input
                        type="time"
                        value={props.autoUpdateTime()}
                        onChange={(e) => {
                          props.setAutoUpdateTime(e.currentTarget.value);
                          props.setHasUnsavedChanges(true);
                        }}
                        class="w-full px-3 py-2 text-sm border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800"
                      />
                    </div>
                  </div>
                </Show>
              </div>
            </div>
          </div>
        </section>
      </SettingsPanel>
    </div>
  );
};
