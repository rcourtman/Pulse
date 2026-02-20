import { Component, Show, For, Accessor, Setter } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { SectionHeader } from '@/components/shared/SectionHeader';
import Clock from 'lucide-solid/icons/clock';

const BACKUP_INTERVAL_OPTIONS = [
  { value: 0, label: 'Default (~90s)' },
  { value: 300, label: '5 minutes' },
  { value: 900, label: '15 minutes' },
  { value: 1800, label: '30 minutes' },
  { value: 3600, label: '1 hour' },
  { value: 21600, label: '6 hours' },
  { value: 86400, label: '24 hours' },
];

const BACKUP_INTERVAL_MAX_MINUTES = 7 * 24 * 60; // 7 days

export interface RecoverySettingsPanelProps {
  backupPollingEnabled: Accessor<boolean>;
  setBackupPollingEnabled: Setter<boolean>;
  backupPollingInterval: Accessor<number>;
  setBackupPollingInterval: Setter<number>;
  backupPollingCustomMinutes: Accessor<number>;
  setBackupPollingCustomMinutes: Setter<number>;
  backupPollingUseCustom: Accessor<boolean>;
  setBackupPollingUseCustom: Setter<boolean>;
  backupPollingEnvLocked: () => boolean;
  backupIntervalSelectValue: () => string;
  backupIntervalSummary: () => string;
  setHasUnsavedChanges: Setter<boolean>;

  // Export/Import handlers
  showExportDialog: Accessor<boolean>;
  setShowExportDialog: Setter<boolean>;
  showImportDialog: Accessor<boolean>;
  setShowImportDialog: Setter<boolean>;
  setUseCustomPassphrase: Setter<boolean>;
  securityStatus: Accessor<{ hasAuthentication: boolean } | null>;
}

export const RecoverySettingsPanel: Component<RecoverySettingsPanelProps> = (props) => {
  return (
    <div class="space-y-6">
      <SettingsPanel
        title="Recovery"
        description="Manage backup/snapshot polling and configuration export/import."
        icon={<Clock class="w-5 h-5" strokeWidth={2} />}
        bodyClass="space-y-8"
      >
        {/* Backup Polling Section */}
        <section class="space-y-3">
          <h4 class="flex items-center gap-2 text-sm font-medium text-slate-700 dark:text-slate-300">
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
            >
              <circle cx="12" cy="12" r="9" stroke-width="2" />
              <path
                d="M12 7v5l3 3"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
            Backup polling
          </h4>
          <p class="text-xs text-slate-600 dark:text-slate-400">
            Control how often Pulse queries Proxmox backup tasks, datastore contents, and guest
            snapshots. Longer intervals reduce disk activity and API load.
          </p>

          <div class="space-y-3">
            {/* Enable toggle */}
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p class="text-sm font-medium text-slate-900 dark:text-slate-100">
                  Enable backup polling
                </p>
                <p class="text-xs text-slate-600 dark:text-slate-400">
                  Required for dashboard backup status, storage snapshots, and alerting.
                </p>
              </div>
              <label class="relative inline-flex items-center cursor-pointer">
                <input
                  type="checkbox"
                  class="sr-only peer"
                  checked={props.backupPollingEnabled()}
                  disabled={props.backupPollingEnvLocked()}
                  onChange={(e) => {
                    props.setBackupPollingEnabled(e.currentTarget.checked);
                    if (!props.backupPollingEnvLocked()) {
                      props.setHasUnsavedChanges(true);
                    }
                  }}
                />
                <div class="w-11 h-6 bg-slate-200 peer-focus:outline-none rounded-full peer dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
              </label>
            </div>

            {/* Polling interval options */}
            <Show when={props.backupPollingEnabled()}>
              <div class="space-y-3 rounded-md border border-slate-200 dark:border-slate-600 p-3">
                <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <label class="text-sm font-medium text-slate-900 dark:text-slate-100">
                      Polling interval
                    </label>
                    <p class="text-xs text-slate-600 dark:text-slate-400">
                      {props.backupIntervalSummary()}
                    </p>
                  </div>
                  <select
                    value={props.backupIntervalSelectValue()}
                    disabled={props.backupPollingEnvLocked()}
                    onChange={(e) => {
                      const value = e.currentTarget.value;
                      if (value === 'custom') {
                        props.setBackupPollingUseCustom(true);
                        const minutes = Math.max(1, props.backupPollingCustomMinutes());
                        props.setBackupPollingInterval(minutes * 60);
                      } else {
                        props.setBackupPollingUseCustom(false);
                        const seconds = parseInt(value, 10);
                        if (!Number.isNaN(seconds)) {
                          props.setBackupPollingInterval(seconds);
                          if (seconds > 0) {
                            props.setBackupPollingCustomMinutes(
                              Math.max(1, Math.round(seconds / 60))
                            );
                          }
                        }
                      }
                      if (!props.backupPollingEnvLocked()) {
                        props.setHasUnsavedChanges(true);
                      }
                    }}
                    class="w-full sm:w-auto min-h-10 sm:min-h-9 px-3 py-2.5 text-sm border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 disabled:opacity-50"
                  >
                    <For each={BACKUP_INTERVAL_OPTIONS}>
                      {(option) => <option value={String(option.value)}>{option.label}</option>}
                    </For>
                    <option value="custom">Custom interval...</option>
                  </select>
                </div>

                {/* Custom interval input */}
                <Show when={props.backupIntervalSelectValue() === 'custom'}>
                  <div class="space-y-2">
                    <label class="text-xs font-medium text-slate-700 dark:text-slate-300">
                      Custom interval (minutes)
                    </label>
                    <div class="flex flex-col sm:flex-row sm:items-center gap-3">
                      <input
                        type="number"
                        min="1"
                        max={BACKUP_INTERVAL_MAX_MINUTES}
                        value={props.backupPollingCustomMinutes()}
                        disabled={props.backupPollingEnvLocked()}
                        onInput={(e) => {
                          const value = Number(e.currentTarget.value);
                          if (Number.isNaN(value)) {
                            return;
                          }
                          const clamped = Math.max(
                            1,
                            Math.min(BACKUP_INTERVAL_MAX_MINUTES, Math.floor(value))
                          );
                          props.setBackupPollingCustomMinutes(clamped);
                          props.setBackupPollingInterval(clamped * 60);
                          if (!props.backupPollingEnvLocked()) {
                            props.setHasUnsavedChanges(true);
                          }
                        }}
                        class="w-full sm:w-24 min-h-10 sm:min-h-9 px-3 py-2.5 text-sm border border-slate-300 dark:border-slate-600 rounded-md bg-white dark:bg-slate-800 disabled:opacity-50"
                      />
                      <span class="text-xs text-slate-500 dark:text-slate-400">
                        1 - {BACKUP_INTERVAL_MAX_MINUTES} minutes (~7 days max)
                      </span>
                    </div>
                  </div>
                </Show>
              </div>
            </Show>

            {/* Env override warning */}
            <Show when={props.backupPollingEnvLocked()}>
              <div class="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 dark:border-amber-700 dark:bg-amber-900 p-3 text-xs text-amber-700 dark:text-amber-200">
                <svg
                  class="w-4 h-4 flex-shrink-0 mt-0.5"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <div>
                  <p class="font-medium">Environment override detected</p>
                  <p class="mt-1">
                    The <code class="font-mono">ENABLE_BACKUP_POLLING</code> or{' '}
                    <code class="font-mono">BACKUP_POLLING_INTERVAL</code> environment variables
                    are set. Remove them and restart Pulse to manage backup polling here.
                  </p>
                </div>
              </div>
            </Show>
          </div>
        </section>

        {/* Backup & Restore Section */}
        <SectionHeader
          title="Backup & restore"
          description="Backup your node configurations and credentials or restore from a previous backup."
          size="md"
          class="mb-4"
        />

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Export Section */}
          <div class="group border border-slate-200 dark:border-slate-700 rounded-md p-5 bg-blue-50 dark:bg-blue-900 hover:border-blue-300 dark:hover:border-blue-700 transition-all duration-200">
            <div class="flex items-start gap-4">
              <div class="flex-shrink-0 w-12 h-12 bg-blue-500 rounded-md flex items-center justify-center shadow-sm shadow-blue-500">
                {/* Archive/Download Box Icon */}
                <svg
                  class="w-6 h-6 text-white"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
                  />
                </svg>
              </div>
              <div class="flex-1 min-w-0">
                <h4 class="text-base font-semibold text-slate-900 dark:text-slate-100 mb-1">
                  Export Configuration
                </h4>
                <p class="text-sm text-slate-600 dark:text-slate-400 mb-3">
                  Create an encrypted backup package
                </p>

                {/* Feature list */}
                <ul class="space-y-1 mb-4">
                  <li class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                    <svg class="w-3.5 h-3.5 text-emerald-400" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                    </svg>
                    <span>All node connections & credentials</span>
                  </li>
                  <li class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                    <svg class="w-3.5 h-3.5 text-emerald-400" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                    </svg>
                    <span>Alert thresholds & overrides</span>
                  </li>
                  <li class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                    <svg class="w-3.5 h-3.5 text-emerald-400" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                    </svg>
                    <span>AES-256 encryption</span>
                  </li>
                </ul>

                <button
                  type="button"
                  onClick={() => {
                    // Default to custom passphrase if no auth is configured
                    props.setUseCustomPassphrase(!props.securityStatus()?.hasAuthentication);
                    props.setShowExportDialog(true);
                  }}
                  class="w-full sm:w-auto min-h-10 sm:min-h-9 px-4 py-2.5 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-all duration-200 inline-flex items-center justify-center gap-2 shadow-sm hover:shadow-sm"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
                    />
                  </svg>
                  Create Backup
                </button>
              </div>
            </div>
          </div>

          {/* Import Section */}
          <div class="group border border-slate-200 dark:border-slate-700 rounded-md p-5 bg-slate-50 dark:bg-slate-800 hover:border-slate-400 dark:hover:border-slate-600 transition-all duration-200">
            <div class="flex items-start gap-4">
              <div class="flex-shrink-0 w-12 h-12 bg-slate-500 rounded-md flex items-center justify-center shadow-sm shadow-gray-500">
                {/* Upload/Restore Icon */}
                <svg
                  class="w-6 h-6 text-white"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
                  />
                </svg>
              </div>
              <div class="flex-1 min-w-0">
                <h4 class="text-base font-semibold text-slate-900 dark:text-slate-100 mb-1">
                  Restore Configuration
                </h4>
                <p class="text-sm text-slate-600 dark:text-slate-400 mb-3">
                  Import from an encrypted backup file
                </p>

                {/* Feature list */}
                <ul class="space-y-1 mb-4">
                  <li class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                    <svg class="w-3.5 h-3.5 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                    </svg>
                    <span>Merge or replace existing config</span>
                  </li>
                  <li class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                    <svg class="w-3.5 h-3.5 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                    </svg>
                    <span>Validates backup before applying</span>
                  </li>
                  <li class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                    <svg class="w-3.5 h-3.5 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                    </svg>
                    <span>Requires original passphrase</span>
                  </li>
                </ul>

                <button
                  type="button"
                  onClick={() => props.setShowImportDialog(true)}
                  class="w-full sm:w-auto min-h-10 sm:min-h-9 px-4 py-2.5 bg-slate-600 text-white text-sm font-medium rounded-md hover:bg-slate-700 transition-all duration-200 inline-flex items-center justify-center gap-2 shadow-sm hover:shadow-sm"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
                    />
                  </svg>
                  Restore Backup
                </button>
              </div>
            </div>
          </div>
        </div>

        {/* Security Tips */}
        <div class="mt-6 p-4 bg-amber-50 dark:bg-amber-900 rounded-md border border-amber-200 dark:border-amber-800">
          <div class="flex gap-3">
            {/* Shield Icon */}
            <div class="flex-shrink-0 w-10 h-10 bg-amber-100 dark:bg-amber-900 rounded-md flex items-center justify-center">
              <svg
                class="w-5 h-5 text-amber-600 dark:text-amber-400"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
                />
              </svg>
            </div>
            <div class="flex-1">
              <p class="text-sm font-medium text-amber-800 dark:text-amber-200 mb-2">Security Tips</p>
              <ul class="space-y-2">
                <li class="flex items-start gap-2 text-xs text-amber-700 dark:text-amber-300">
                  <svg class="w-4 h-4 text-amber-500 flex-shrink-0 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z" clip-rule="evenodd" />
                  </svg>
                  <span>Recovery exports contain encrypted credentials and sensitive data</span>
                </li>
                <li class="flex items-start gap-2 text-xs text-amber-700 dark:text-amber-300">
                  <svg class="w-4 h-4 text-amber-500 flex-shrink-0 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M18 8a6 6 0 01-7.743 5.743L10 14l-1 1-1 1H6v2H2v-4l4.257-4.257A6 6 0 1118 8zm-6-4a1 1 0 100 2 2 2 0 012 2 1 1 0 102 0 4 4 0 00-4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>Use a strong passphrase (12+ characters, mix of letters, numbers, symbols)</span>
                </li>
                <li class="flex items-start gap-2 text-xs text-amber-700 dark:text-amber-300">
                  <svg class="w-4 h-4 text-amber-500 flex-shrink-0 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
                    <path d="M10 2a5 5 0 00-5 5v2a2 2 0 00-2 2v5a2 2 0 002 2h10a2 2 0 002-2v-5a2 2 0 00-2-2V7a5 5 0 00-5-5zm3 7V7a3 3 0 10-6 0v2h6z" />
                  </svg>
                  <span>Store backup files securely and never share the passphrase</span>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </SettingsPanel>
    </div>
  );
};
