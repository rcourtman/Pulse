import { Component, Show, createEffect, onMount } from 'solid-js';
import {
  formCheckbox,
  formControl,
  formField,
  formHelpText,
  formLabel,
  formSelect,
} from '@/components/shared/Form';
import { MonitoredSystemAdmissionPreview } from '../../MonitoredSystemAdmissionPreview';
import type { TrueNASConnection } from '@/api/truenas';
import type { TrueNASSettingsPanelState } from '../../useTrueNASSettingsPanelState';

const buttonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const primaryButtonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60';

export interface TrueNASCredentialSlotProps {
  state: TrueNASSettingsPanelState;
  editingConnection?: TrueNASConnection | null;
  onCancel: () => void;
  onSaved: () => void;
}

export const TrueNASCredentialSlot: Component<TrueNASCredentialSlotProps> = (props) => {
  let primed = false;

  onMount(() => {
    if (!props.state.dialogOpen()) {
      if (props.editingConnection) {
        props.state.openEditDialog(props.editingConnection);
      } else {
        props.state.openCreateDialog();
      }
    }
    primed = true;
  });

  createEffect(() => {
    const open = props.state.dialogOpen();
    if (primed && !open && !props.state.saving()) {
      props.onSaved();
    }
  });

  const handleCancel = () => {
    props.state.closeDialog();
    props.onCancel();
  };

  return (
    <div class="space-y-6">
      <Show when={props.state.featureDisabled()}>
        <div class="rounded-md border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-200">
          {props.state.featureDisabledMessage() ||
            'TrueNAS connections are disabled for this Pulse tier.'}
        </div>
      </Show>

      <Show when={!props.state.featureDisabled()}>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class={formField}>
            <span class={formLabel}>Name</span>
            <input
              class={formControl}
              value={props.state.form().name}
              onInput={(event) => props.state.updateForm({ name: event.currentTarget.value })}
              placeholder="tower"
            />
          </label>
          <label class={formField}>
            <span class={formLabel}>Host</span>
            <input
              class={formControl}
              value={props.state.form().host}
              onInput={(event) => props.state.updateForm({ host: event.currentTarget.value })}
              placeholder="truenas.local"
            />
          </label>
          <label class={formField}>
            <span class={formLabel}>Port</span>
            <input
              class={formControl}
              inputMode="numeric"
              value={props.state.form().port}
              onInput={(event) => props.state.updateForm({ port: event.currentTarget.value })}
              placeholder={props.state.form().useHttps ? '443' : '80'}
            />
          </label>
          <label class={formField}>
            <span class={formLabel}>Poll interval (seconds)</span>
            <input
              class={formControl}
              inputMode="numeric"
              value={props.state.form().pollIntervalSeconds}
              onInput={(event) =>
                props.state.updateForm({ pollIntervalSeconds: event.currentTarget.value })
              }
              placeholder="60"
            />
            <span class={formHelpText}>
              How often Pulse should refresh this TrueNAS connection.
            </span>
          </label>
          <label class={formField}>
            <span class={formLabel}>Authentication</span>
            <select
              class={formSelect}
              value={props.state.form().authMode}
              onChange={(event) =>
                props.state.updateForm({
                  authMode: event.currentTarget.value as 'apiKey' | 'userpass',
                  apiKey: '',
                  password: '',
                })
              }
            >
              <option value="apiKey">API key</option>
              <option value="userpass">Username and password</option>
            </select>
          </label>
        </div>

        <Show when={props.state.form().authMode === 'apiKey'}>
          <label class={formField}>
            <span class={formLabel}>API key</span>
            <input
              class={formControl}
              type="password"
              value={props.state.form().apiKey}
              onInput={(event) => props.state.updateForm({ apiKey: event.currentTarget.value })}
              placeholder={
                props.state.form().hasStoredApiKey
                  ? 'Saved API key retained unless replaced'
                  : ''
              }
            />
            <Show when={props.state.form().hasStoredApiKey}>
              <span class={formHelpText}>Leave this blank to keep the saved API key.</span>
            </Show>
          </label>
        </Show>

        <Show when={props.state.form().authMode === 'userpass'}>
          <div class="grid gap-4 sm:grid-cols-2">
            <label class={formField}>
              <span class={formLabel}>Username</span>
              <input
                class={formControl}
                value={props.state.form().username}
                onInput={(event) =>
                  props.state.updateForm({ username: event.currentTarget.value })
                }
                placeholder="admin"
              />
            </label>
            <label class={formField}>
              <span class={formLabel}>Password</span>
              <input
                class={formControl}
                type="password"
                value={props.state.form().password}
                onInput={(event) =>
                  props.state.updateForm({ password: event.currentTarget.value })
                }
                placeholder={
                  props.state.form().hasStoredPassword
                    ? 'Saved password retained unless replaced'
                    : ''
                }
              />
              <Show when={props.state.form().hasStoredPassword}>
                <span class={formHelpText}>Leave this blank to keep the saved password.</span>
              </Show>
            </label>
          </div>
        </Show>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class={formField}>
            <span class={formLabel}>TLS fingerprint</span>
            <input
              class={formControl}
              value={props.state.form().fingerprint}
              onInput={(event) =>
                props.state.updateForm({ fingerprint: event.currentTarget.value })
              }
              placeholder="Optional SHA256 fingerprint"
            />
            <span class={formHelpText}>Optional certificate pin for HTTPS connections.</span>
          </label>
          <div class="space-y-3 rounded-md border border-border bg-surface-alt px-4 py-3">
            <label class="flex items-center gap-3">
              <input
                type="checkbox"
                class={formCheckbox}
                checked={props.state.form().useHttps}
                onChange={(event) =>
                  props.state.updateForm({ useHttps: event.currentTarget.checked })
                }
              />
              <span class="text-sm text-base-content">Use HTTPS</span>
            </label>
            <label class="flex items-center gap-3">
              <input
                type="checkbox"
                class={formCheckbox}
                checked={props.state.form().insecureSkipVerify}
                onChange={(event) =>
                  props.state.updateForm({ insecureSkipVerify: event.currentTarget.checked })
                }
              />
              <span class="text-sm text-base-content">Skip TLS verification</span>
            </label>
            <label class="flex items-center gap-3">
              <input
                type="checkbox"
                class={formCheckbox}
                checked={props.state.form().enabled}
                onChange={(event) =>
                  props.state.updateForm({ enabled: event.currentTarget.checked })
                }
              />
              <span class="text-sm text-base-content">Enable polling immediately</span>
            </label>
          </div>
        </div>

        <div class="space-y-3 rounded-md border border-border bg-surface p-4">
          <div>
            <div class="text-sm font-semibold text-base-content">Collection scope</div>
            <p class="mt-0.5 text-xs text-muted">
              Pulse only collects the surfaces you keep enabled. Disable anything you don't need.
            </p>
          </div>
          <div class="grid gap-2 sm:grid-cols-3">
            <label class="flex items-start gap-2 text-sm text-base-content">
              <input
                type="checkbox"
                class={formCheckbox + ' mt-0.5'}
                checked={props.state.form().monitorDatasets}
                onChange={(event) =>
                  props.state.updateForm({ monitorDatasets: event.currentTarget.checked })
                }
              />
              <span>
                <span class="font-medium">Datasets</span>
                <span class="block text-xs text-muted">Dataset sizes, quotas, snapshots.</span>
              </span>
            </label>
            <label class="flex items-start gap-2 text-sm text-base-content">
              <input
                type="checkbox"
                class={formCheckbox + ' mt-0.5'}
                checked={props.state.form().monitorPools}
                onChange={(event) =>
                  props.state.updateForm({ monitorPools: event.currentTarget.checked })
                }
              />
              <span>
                <span class="font-medium">Pools</span>
                <span class="block text-xs text-muted">ZFS pool health, capacity, scrubs.</span>
              </span>
            </label>
            <label class="flex items-start gap-2 text-sm text-base-content">
              <input
                type="checkbox"
                class={formCheckbox + ' mt-0.5'}
                checked={props.state.form().monitorReplication}
                onChange={(event) =>
                  props.state.updateForm({ monitorReplication: event.currentTarget.checked })
                }
              />
              <span>
                <span class="font-medium">Replication</span>
                <span class="block text-xs text-muted">Replication tasks and their state.</span>
              </span>
            </label>
          </div>
        </div>

        <MonitoredSystemAdmissionPreview
          preview={props.state.monitoredSystemPreview()}
          loading={props.state.previewing()}
          error={props.state.monitoredSystemPreviewError()}
          errorTitle={props.state.monitoredSystemPreviewErrorTitle()}
        />

        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            class={buttonClass}
            onClick={handleCancel}
            disabled={props.state.saving() || props.state.testing()}
          >
            Cancel
          </button>
          <button
            type="button"
            class={buttonClass}
            onClick={() => void props.state.testCurrentForm()}
            disabled={props.state.saving() || props.state.testing()}
          >
            {props.state.testing() ? 'Testing…' : 'Test connection'}
          </button>
          <button
            type="button"
            class={buttonClass}
            onClick={() => void props.state.previewCurrentForm()}
            disabled={
              props.state.saving() || props.state.testing() || props.state.previewing()
            }
          >
            {props.state.previewing() ? 'Previewing…' : 'Preview impact'}
          </button>
          <button
            type="button"
            class={primaryButtonClass}
            onClick={() => void props.state.saveCurrentForm()}
            disabled={
              props.state.saving() ||
              props.state.testing() ||
              props.state.previewing() ||
              props.state.monitoredSystemAdmissionSaveBlocked()
            }
          >
            {props.state.saving() ? 'Adding…' : 'Add connection'}
          </button>
        </div>
      </Show>
    </div>
  );
};
