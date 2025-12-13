import { Component, Show, For, Accessor, Setter } from 'solid-js';
import { Card } from '@/components/shared/Card';
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

interface BackupsSettingsPanelProps {
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

export const BackupsSettingsPanel: Component<BackupsSettingsPanelProps> = (props) => {
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
              <Clock class="w-5 h-5 text-blue-600 dark:text-blue-300" strokeWidth={2} />
            </div>
            <SectionHeader
              title="Backups"
              description="Backup polling and configuration management"
              size="sm"
              class="flex-1"
            />
          </div>
        </div>

        <div class="p-6 space-y-8">
          {/* Backup Polling Section */}
          <section class="space-y-3">
            <h4 class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
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
            <p class="text-xs text-gray-600 dark:text-gray-400">
              Control how often Pulse queries Proxmox backup tasks, datastore contents, and guest
              snapshots. Longer intervals reduce disk activity and API load.
            </p>

            <div class="space-y-3">
              {/* Enable toggle */}
              <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-gray-100">
                    Enable backup polling
                  </p>
                  <p class="text-xs text-gray-600 dark:text-gray-400">
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
                  <div class="w-11 h-6 bg-gray-200 peer-focus:outline-none rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600 peer-disabled:opacity-50"></div>
                </label>
              </div>

              {/* Polling interval options */}
              <Show when={props.backupPollingEnabled()}>
                <div class="space-y-3 rounded-md border border-gray-200 dark:border-gray-600 p-3">
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <label class="text-sm font-medium text-gray-900 dark:text-gray-100">
                        Polling interval
                      </label>
                      <p class="text-xs text-gray-600 dark:text-gray-400">
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
                      class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 disabled:opacity-50"
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
                      <label class="text-xs font-medium text-gray-700 dark:text-gray-300">
                        Custom interval (minutes)
                      </label>
                      <div class="flex items-center gap-3">
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
                          class="w-24 px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 disabled:opacity-50"
                        />
                        <span class="text-xs text-gray-500 dark:text-gray-400">
                          1 - {BACKUP_INTERVAL_MAX_MINUTES} minutes (~7 days max)
                        </span>
                      </div>
                    </div>
                  </Show>
                </div>
              </Show>

              {/* Env override warning */}
              <Show when={props.backupPollingEnvLocked()}>
                <div class="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 dark:border-amber-700 dark:bg-amber-900/20 p-3 text-xs text-amber-700 dark:text-amber-200">
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
            <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
              <div class="flex items-start gap-3">
                <div class="flex-shrink-0 w-10 h-10 bg-blue-100 dark:bg-blue-900/30 rounded-lg flex items-center justify-center">
                  <svg
                    class="w-5 h-5 text-blue-600 dark:text-blue-400"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10"
                    />
                  </svg>
                </div>
                <div class="flex-1">
                  <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-1">
                    Export Configuration
                  </h4>
                  <p class="text-xs text-gray-600 dark:text-gray-400 mb-3">
                    Download an encrypted backup of all nodes and settings
                  </p>
                  <button
                    type="button"
                    onClick={() => {
                      // Default to custom passphrase if no auth is configured
                      props.setUseCustomPassphrase(!props.securityStatus()?.hasAuthentication);
                      props.setShowExportDialog(true);
                    }}
                    class="px-3 py-1.5 bg-blue-600 text-white text-sm rounded-md hover:bg-blue-700 transition-colors inline-flex items-center gap-2"
                  >
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
                      />
                    </svg>
                    Export Backup
                  </button>
                </div>
              </div>
            </div>

            {/* Import Section */}
            <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4">
              <div class="flex items-start gap-3">
                <div class="flex-shrink-0 w-10 h-10 bg-gray-100 dark:bg-gray-700 rounded-lg flex items-center justify-center">
                  <svg
                    class="w-5 h-5 text-gray-600 dark:text-gray-400"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
                    />
                  </svg>
                </div>
                <div class="flex-1">
                  <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-1">
                    Restore Configuration
                  </h4>
                  <p class="text-xs text-gray-600 dark:text-gray-400 mb-3">
                    Upload a backup file to restore nodes and settings
                  </p>
                  <button
                    type="button"
                    onClick={() => props.setShowImportDialog(true)}
                    class="px-3 py-1.5 bg-gray-600 text-white text-sm rounded-md hover:bg-gray-700 transition-colors inline-flex items-center gap-2"
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

          {/* Important Notes */}
          <div class="mt-4 p-3 bg-amber-50 dark:bg-amber-900/20 rounded-lg border border-amber-200 dark:border-amber-800">
            <div class="flex gap-2">
              <svg
                class="w-4 h-4 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5"
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
              <div class="text-xs text-amber-700 dark:text-amber-300">
                <p class="font-medium mb-1">Important Notes</p>
                <ul class="space-y-0.5 text-amber-600 dark:text-amber-400">
                  <li>• Backups contain encrypted credentials and sensitive data</li>
                  <li>• Use a strong passphrase to protect your backup</li>
                  <li>• Store backup files securely and never share the passphrase</li>
                </ul>
              </div>
            </div>
          </div>
        </div>
      </Card>
    </div>
  );
};
