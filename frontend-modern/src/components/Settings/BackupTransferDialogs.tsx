import { Accessor, Component, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { controlClass, formField, formHelpText, labelClass } from '@/components/shared/Form';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';

interface BackupTransferDialogsProps {
  securityStatus: Accessor<SecurityStatusInfo | null>;
  exportPassphrase: Accessor<string>;
  setExportPassphrase: (value: string) => void;
  useCustomPassphrase: Accessor<boolean>;
  setUseCustomPassphrase: (value: boolean) => void;
  importPassphrase: Accessor<string>;
  setImportPassphrase: (value: string) => void;
  importFile: Accessor<File | null>;
  setImportFile: (file: File | null) => void;
  showExportDialog: Accessor<boolean>;
  showImportDialog: Accessor<boolean>;
  showApiTokenModal: Accessor<boolean>;
  apiTokenInput: Accessor<string>;
  setApiTokenInput: (value: string) => void;
  handleExport: () => void;
  handleImport: () => void;
  closeExportDialog: () => void;
  closeImportDialog: () => void;
  closeApiTokenModal: () => void;
  handleApiTokenAuthenticate: () => void;
}

export const BackupTransferDialogs: Component<BackupTransferDialogsProps> = (props) => {
  return (
    <>
      <Show when={props.showExportDialog()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="Export configuration" size="md" class="mb-4" />

            <div class="space-y-4">
              <Show when={props.securityStatus()?.hasAuthentication}>
                <div class="bg-base rounded-md p-4 border border-border">
                  <div class="space-y-3">
                    <label class="flex items-start gap-3 cursor-pointer">
                      <input
                        type="radio"
                        checked={!props.useCustomPassphrase()}
                        onChange={() => {
                          props.setUseCustomPassphrase(false);
                          props.setExportPassphrase('');
                        }}
                        class="mt-1 text-blue-600 focus:ring-blue-500"
                      />
                      <div class="flex-1">
                        <div class="text-sm font-medium text-base-content">
                          Use your login password
                        </div>
                        <div class="text-xs text-muted mt-0.5">
                          Use the same password you use to log into Pulse (recommended)
                        </div>
                      </div>
                    </label>

                    <label class="flex items-start gap-3 cursor-pointer">
                      <input
                        type="radio"
                        checked={props.useCustomPassphrase()}
                        onChange={() => props.setUseCustomPassphrase(true)}
                        class="mt-1 text-blue-600 focus:ring-blue-500"
                      />
                      <div class="flex-1">
                        <div class="text-sm font-medium text-base-content">
                          Use a custom passphrase
                        </div>
                        <div class="text-xs text-muted mt-0.5">
                          Create a different passphrase for this backup
                        </div>
                      </div>
                    </label>
                  </div>
                </div>
              </Show>

              <div class={formField}>
                <label class={labelClass()}>
                  {props.securityStatus()?.hasAuthentication
                    ? props.useCustomPassphrase()
                      ? 'Custom Passphrase'
                      : 'Enter Your Login Password'
                    : 'Encryption Passphrase'}
                </label>
                <input
                  type="password"
                  value={props.exportPassphrase()}
                  onInput={(event) => props.setExportPassphrase(event.currentTarget.value)}
                  placeholder={
                    props.securityStatus()?.hasAuthentication
                      ? props.useCustomPassphrase()
                        ? 'Enter a strong passphrase'
                        : 'Enter your Pulse login password'
                      : 'Enter a strong passphrase for encryption'
                  }
                  class={controlClass()}
                />
                <Show
                  when={!props.securityStatus()?.hasAuthentication || props.useCustomPassphrase()}
                >
                  <p class={`${formHelpText} mt-1`}>
                    You'll need this passphrase to restore the backup.
                  </p>
                </Show>
                <Show
                  when={props.securityStatus()?.hasAuthentication && !props.useCustomPassphrase()}
                >
                  <p class={`${formHelpText} mt-1`}>
                    You'll use this same password when restoring the backup
                  </p>
                </Show>
              </div>

              <div class="bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-800 rounded-md p-3">
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
                    <strong>Important:</strong> The backup contains node credentials but NOT
                    authentication settings. Each Pulse instance should configure its own login
                    credentials for security. Remember your{' '}
                    {props.useCustomPassphrase() || !props.securityStatus()?.hasAuthentication
                      ? 'passphrase'
                      : 'password'}{' '}
                    for restoring.
                  </div>
                </div>
              </div>

              <div class="flex justify-end space-x-3">
                <button
                  type="button"
                  onClick={props.closeExportDialog}
                  class="px-4 py-2 border border-border text-base-content rounded-md hover:bg-surface-hover"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={props.handleExport}
                  disabled={
                    !props.exportPassphrase() ||
                    (props.useCustomPassphrase() && props.exportPassphrase().length < 12)
                  }
                  class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Export
                </button>
              </div>
            </div>
          </Card>
        </div>
      </Show>

      <Show when={props.showApiTokenModal()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="API token required" size="md" class="mb-4" />

            <div class="space-y-4">
              <p class="text-sm text-muted">
                This Pulse instance requires an API token for export/import operations. Please
                enter the API token configured on the server.
              </p>

              <div class={formField}>
                <label class={labelClass()}>API Token</label>
                <input
                  type="password"
                  value={props.apiTokenInput()}
                  onInput={(event) => props.setApiTokenInput(event.currentTarget.value)}
                  placeholder="Enter API token"
                  class={controlClass()}
                />
              </div>

              <div class="text-xs text-muted rounded p-2">
                <p class="font-semibold mb-1">
                  Create or rotate API tokens in Settings → Security → API tokens.
                </p>
                <p>
                  Tokens are managed in the UI and stored in <code>api_tokens.json</code>.
                </p>
              </div>
            </div>

            <div class="flex justify-end space-x-2 mt-6">
              <button
                type="button"
                onClick={props.closeApiTokenModal}
                class="px-4 py-2 border border-border text-base-content rounded-md hover:bg-surface-hover"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={props.handleApiTokenAuthenticate}
                disabled={!props.apiTokenInput()}
                class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Authenticate
              </button>
            </div>
          </Card>
        </div>
      </Show>

      <Show when={props.showImportDialog()}>
        <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <Card padding="lg" class="max-w-md w-full">
            <SectionHeader title="Import configuration" size="md" class="mb-4" />

            <div class="space-y-4">
              <div class={formField}>
                <label class={labelClass()}>Configuration File</label>
                <input
                  type="file"
                  accept=".json"
                  onChange={(event) => {
                    const file = event.currentTarget.files?.[0] ?? null;
                    props.setImportFile(file);
                  }}
                  class={controlClass('cursor-pointer')}
                />
              </div>

              <div class={formField}>
                <label class={labelClass()}>Backup Password</label>
                <input
                  type="password"
                  value={props.importPassphrase()}
                  onInput={(event) => props.setImportPassphrase(event.currentTarget.value)}
                  placeholder="Enter the password used when creating this backup"
                  class={controlClass()}
                />
                <p class={`${formHelpText} mt-1`}>
                  This is usually your Pulse login password, unless you used a custom passphrase
                </p>
              </div>

              <div class="bg-yellow-50 dark:bg-yellow-900 border border-yellow-200 dark:border-yellow-800 rounded p-3">
                <p class="text-xs text-yellow-700 dark:text-yellow-300">
                  <strong>Warning:</strong> Importing will replace all current configuration. This
                  action cannot be undone.
                </p>
              </div>

              <div class="flex justify-end space-x-3">
                <button
                  type="button"
                  onClick={props.closeImportDialog}
                  class="px-4 py-2 border border-border text-base-content rounded-md hover:bg-surface-hover"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={props.handleImport}
                  disabled={!props.importPassphrase() || !props.importFile()}
                  class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Import
                </button>
              </div>
            </div>
          </Card>
        </div>
      </Show>
    </>
  );
};
