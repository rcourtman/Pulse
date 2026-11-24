import { Component, Show, For, Accessor, Setter, createUniqueId } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Button } from '@/components/shared/Button';
import { EnvironmentLockBadge } from '@/components/shared/EnvironmentLockBadge';
import { FormSelect } from '@/components/shared/FormSelect';
import { TogglePrimitive } from '@/components/shared/Toggle';
import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
import { getShippedDocUrl } from '@/utils/docsLinks';
import {
  BACKUP_INTERVAL_MAX_MINUTES,
  BACKUP_INTERVAL_OPTIONS,
} from '@/utils/systemSettingsPresentation';

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
  setShowExportDialog: (open: boolean) => void;
  showImportDialog: Accessor<boolean>;
  setShowImportDialog: (open: boolean) => void;
}

export const RecoverySettingsPanel: Component<RecoverySettingsPanelProps> = (props) => {
  const backupPollingTitleId = `settings-recovery-${createUniqueId()}-backup-polling-title`;

  return (
    <div class="space-y-4 sm:space-y-6">
      {/* Pulse Configuration Backup & Restore */}
      <SettingsPanel
        title="Configuration backup & restore"
        description="Create an encrypted configuration export for the selected workspace, not a full-server backup."
        noPadding
      >
        <div class="p-4 sm:p-6 space-y-4">
          <p class="text-sm text-muted">
            History, TrueNAS/vSphere connections, local login credentials and agent enrollment state
            are not included. Check the{' '}
            <ExternalTextLink href={getShippedDocUrl('MIGRATION.md')}>
              migration guide
            </ExternalTextLink>{' '}
            for the full scope and full-state recovery.
          </p>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Export Section */}
            <div class="group border border-border rounded-md p-5 bg-blue-50 dark:bg-blue-900/25 hover:border-blue-300 dark:hover:border-blue-700 transition-all duration-200">
              <div class="flex items-start gap-4">
                <div class="shrink-0 w-12 h-12 bg-blue-500 rounded-md flex items-center justify-center shadow-xs shadow-blue-500">
                  <svg
                    class="w-6 h-6 text-white"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                    stroke-width="2"
                    aria-hidden="true"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
                    />
                  </svg>
                </div>
                <div class="flex-1 min-w-0">
                  <h4 class="text-base font-semibold text-base-content mb-1">
                    Export Configuration
                  </h4>
                  <p class="text-sm text-muted mb-3">Create an encrypted backup package</p>

                  <ul class="space-y-1 mb-4">
                    <li class="flex items-center gap-2 text-xs text-muted">
                      <svg
                        class="w-3.5 h-3.5 text-emerald-400 shrink-0"
                        fill="currentColor"
                        viewBox="0 0 20 20"
                        aria-hidden="true"
                      >
                        <path
                          fill-rule="evenodd"
                          d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                          clip-rule="evenodd"
                        />
                      </svg>
                      <span>Proxmox connections & credentials</span>
                    </li>
                    <li class="flex items-center gap-2 text-xs text-muted">
                      <svg
                        class="w-3.5 h-3.5 text-emerald-400 shrink-0"
                        fill="currentColor"
                        viewBox="0 0 20 20"
                        aria-hidden="true"
                      >
                        <path
                          fill-rule="evenodd"
                          d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                          clip-rule="evenodd"
                        />
                      </svg>
                      <span>Alert thresholds & overrides</span>
                    </li>
                    <li class="flex items-center gap-2 text-xs text-muted">
                      <svg
                        class="w-3.5 h-3.5 text-emerald-400 shrink-0"
                        fill="currentColor"
                        viewBox="0 0 20 20"
                        aria-hidden="true"
                      >
                        <path
                          fill-rule="evenodd"
                          d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                          clip-rule="evenodd"
                        />
                      </svg>
                      <span>AES-256 encryption</span>
                    </li>
                  </ul>

                  <Button
                    type="button"
                    variant="primary"
                    size="settingsAction"
                    class="w-full gap-2 sm:w-auto"
                    onClick={() => props.setShowExportDialog(true)}
                  >
                    <svg
                      class="w-4 h-4"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
                      />
                    </svg>
                    Create Backup
                  </Button>
                </div>
              </div>
            </div>

            {/* Import Section */}
            <div class="group border border-border rounded-md p-5 bg-surface-alt hover:border-border transition-all duration-200">
              <div class="flex items-start gap-4">
                <div class="shrink-0 w-12 h-12 bg-slate-500 rounded-md flex items-center justify-center shadow-xs shadow-gray-500">
                  <svg
                    class="w-6 h-6 text-white"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                    stroke-width="2"
                    aria-hidden="true"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
                    />
                  </svg>
                </div>
                <div class="flex-1 min-w-0">
                  <h4 class="text-base font-semibold text-base-content mb-1">
                    Restore Configuration
                  </h4>
                  <p class="text-sm text-muted mb-3">
                    Import from an encrypted configuration file. Back up the destination first.
                  </p>

                  <ul class="space-y-1 mb-4">
                    <li class="flex items-center gap-2 text-xs text-muted">
                      <svg
                        class="w-3.5 h-3.5 text-blue-500 shrink-0"
                        fill="currentColor"
                        viewBox="0 0 20 20"
                        aria-hidden="true"
                      >
                        <path
                          fill-rule="evenodd"
                          d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                          clip-rule="evenodd"
                        />
                      </svg>
                      <span>Replaces included settings and API-token records</span>
                    </li>
                    <li class="flex items-center gap-2 text-xs text-muted">
                      <svg
                        class="w-3.5 h-3.5 text-blue-500 shrink-0"
                        fill="currentColor"
                        viewBox="0 0 20 20"
                        aria-hidden="true"
                      >
                        <path
                          fill-rule="evenodd"
                          d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                          clip-rule="evenodd"
                        />
                      </svg>
                      <span>Validates backup before applying</span>
                    </li>
                    <li class="flex items-center gap-2 text-xs text-muted">
                      <svg
                        class="w-3.5 h-3.5 text-blue-500 shrink-0"
                        fill="currentColor"
                        viewBox="0 0 20 20"
                        aria-hidden="true"
                      >
                        <path
                          fill-rule="evenodd"
                          d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                          clip-rule="evenodd"
                        />
                      </svg>
                      <span>Requires original passphrase</span>
                    </li>
                  </ul>

                  <Button
                    type="button"
                    variant="secondary"
                    size="settingsAction"
                    class="w-full gap-2 sm:w-auto"
                    onClick={() => props.setShowImportDialog(true)}
                  >
                    <svg
                      class="w-4 h-4"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
                      />
                    </svg>
                    Restore Backup
                  </Button>
                </div>
              </div>
            </div>
          </div>

          <p class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5 text-xs text-amber-800 dark:border-amber-800 dark:bg-amber-900/25 dark:text-amber-200">
            <strong>Security note:</strong> Recovery exports contain encrypted credentials. Use a
            unique passphrase of 12 or more characters and store it separately from the export file.
          </p>
        </div>
      </SettingsPanel>

      {/* Proxmox Backup Polling Section (separated clearly from Pulse Config Export/Restore) */}
      <SettingsPanel
        title="Proxmox backup polling"
        description="Control how often Pulse queries Proxmox VE backup tasks, PBS datastores, and guest snapshots. Separate from Pulse configuration backups above."
        noPadding
      >
        <div class="p-4 sm:p-6">
          <section class="space-y-3">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p id={backupPollingTitleId} class="text-sm font-medium text-base-content">
                  Enable backup polling
                </p>
                <p class="text-xs text-muted">
                  Required for workload backup status, storage snapshots, and alerting. Longer
                  intervals reduce disk activity and API load.
                </p>
              </div>
              <TogglePrimitive
                checked={props.backupPollingEnabled()}
                disabled={props.backupPollingEnvLocked()}
                ariaLabelledBy={backupPollingTitleId}
                onChange={(e) => {
                  props.setBackupPollingEnabled(e.currentTarget.checked);
                  if (!props.backupPollingEnvLocked()) {
                    props.setHasUnsavedChanges(true);
                  }
                }}
              />
            </div>

            {/* Polling interval options */}
            <Show when={props.backupPollingEnabled()}>
              <div class="space-y-3 rounded-md border border-border p-3">
                <FormSelect
                  id="recovery-backup-polling-interval"
                  label={
                    <span>
                      <span class="block text-sm font-medium text-base-content">
                        Polling interval
                      </span>
                      <span aria-hidden="true" class="block text-xs font-normal text-muted">
                        {props.backupIntervalSummary()}
                      </span>
                    </span>
                  }
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
                            Math.max(1, Math.round(seconds / 60)),
                          );
                        }
                      }
                    }
                    if (!props.backupPollingEnvLocked()) {
                      props.setHasUnsavedChanges(true);
                    }
                  }}
                  fieldBaseClass="flex"
                  fieldClass="flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
                  selectBaseClass="w-full sm:w-auto min-h-10 sm:min-h-9 px-3 py-2.5 text-sm border border-border rounded-md bg-surface text-base-content disabled:opacity-50"
                >
                  <For each={BACKUP_INTERVAL_OPTIONS}>
                    {(option) => <option value={String(option.value)}>{option.label}</option>}
                  </For>
                  <option value="custom">Custom interval...</option>
                </FormSelect>

                {/* Custom interval input */}
                <Show when={props.backupIntervalSelectValue() === 'custom'}>
                  <div class="space-y-2">
                    <label
                      for="backup-custom-interval-minutes"
                      class="text-xs font-medium text-base-content"
                    >
                      Custom interval (minutes)
                    </label>
                    <div class="flex flex-col sm:flex-row sm:items-center gap-3">
                      <input
                        id="backup-custom-interval-minutes"
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
                            Math.min(BACKUP_INTERVAL_MAX_MINUTES, Math.floor(value)),
                          );
                          props.setBackupPollingCustomMinutes(clamped);
                          props.setBackupPollingInterval(clamped * 60);
                          if (!props.backupPollingEnvLocked()) {
                            props.setHasUnsavedChanges(true);
                          }
                        }}
                        class="w-full sm:w-24 min-h-10 sm:min-h-9 px-3 py-2.5 text-sm border border-border rounded-md bg-surface text-base-content disabled:opacity-50"
                      />
                      <span class="text-xs text-muted">
                        1 - {BACKUP_INTERVAL_MAX_MINUTES} minutes (~7 days max)
                      </span>
                    </div>
                  </div>
                </Show>
              </div>
            </Show>

            {/* Env override warning */}
            <Show when={props.backupPollingEnvLocked()}>
              <div class="flex flex-wrap items-center gap-2 rounded-md border border-amber-200 bg-amber-50 dark:border-amber-700 dark:bg-amber-900/25 p-3 text-xs text-amber-700 dark:text-amber-200">
                <EnvironmentLockBadge envVar="ENABLE_BACKUP_POLLING / BACKUP_POLLING_INTERVAL" />
                <p>
                  Remove <code class="font-mono">ENABLE_BACKUP_POLLING</code> or{' '}
                  <code class="font-mono">BACKUP_POLLING_INTERVAL</code> and restart Pulse to manage
                  backup polling here.
                </p>
              </div>
            </Show>
          </section>
        </div>
      </SettingsPanel>
    </div>
  );
};
