import { Component, Show, createEffect, onMount } from 'solid-js';
import ShieldAlert from 'lucide-solid/icons/shield-alert';
import { CalloutCard } from '@/components/shared/CalloutCard';
import {
  formCheckbox,
  formControl,
  formField,
  formHelpText,
  formLabel,
} from '@/components/shared/Form';
import { TlsVerificationWarningBanner } from '@/components/shared/TlsVerificationWarningBanner';
import { MonitoredSystemAdmissionPreview } from '../../MonitoredSystemAdmissionPreview';
import type { VMwareConnection } from '@/api/vmware';
import type { VMwareSettingsPanelState } from '../../useVMwareSettingsPanelState';

const buttonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const primaryButtonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60';

export interface VMwareCredentialSlotProps {
  state: VMwareSettingsPanelState;
  editingConnection?: VMwareConnection | null;
  onCancel: () => void;
  onSaved: () => void;
  onToggleEnabled?: () => void;
  togglePending?: boolean;
  connectionEnabled?: boolean;
  onDelete?: () => void;
  deletePending?: boolean;
  deleteConfirming?: boolean;
  deleteError?: string | null;
}

export const VMwareCredentialSlot: Component<VMwareCredentialSlotProps> = (props) => {
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

  const isEditing = () => Boolean(props.editingConnection);

  return (
    <div class="space-y-6">
      <Show when={props.state.featureDisabled()}>
        <div class="rounded-md border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-200">
          {props.state.featureDisabledMessage() ||
            'VMware connections are disabled for this Pulse tier.'}
        </div>
      </Show>

      <Show when={!props.state.featureDisabled()}>
        <Show when={props.state.connectionFailure()}>
          {(failure) => (
            <CalloutCard
              data-testid="vmware-connection-test-feedback"
              tone={failure().tone}
              title={failure().title}
              description={
                <>
                  <p>{failure().message}</p>
                  <Show when={failure().guidance}>
                    <p class="mt-2">{failure().guidance}</p>
                  </Show>
                </>
              }
              icon={<ShieldAlert class="h-5 w-5" />}
            />
          )}
        </Show>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class={formField}>
            <span class={formLabel}>Name</span>
            <input
              class={formControl}
              value={props.state.form().name}
              onInput={(event) => props.state.updateForm({ name: event.currentTarget.value })}
              placeholder="lab-vcenter"
            />
          </label>
          <label class={formField}>
            <span class={formLabel}>Host</span>
            <input
              class={formControl}
              value={props.state.form().host}
              onInput={(event) => props.state.updateForm({ host: event.currentTarget.value })}
              placeholder="vcsa.lab.local"
            />
          </label>
          <label class={formField}>
            <span class={formLabel}>Port</span>
            <input
              class={formControl}
              inputMode="numeric"
              value={props.state.form().port}
              onInput={(event) => props.state.updateForm({ port: event.currentTarget.value })}
              placeholder="443"
            />
          </label>
          <label class={formField}>
            <span class={formLabel}>Username</span>
            <input
              class={formControl}
              value={props.state.form().username}
              onInput={(event) =>
                props.state.updateForm({ username: event.currentTarget.value })
              }
              placeholder="administrator@vsphere.local"
            />
          </label>
          <label class={`${formField} sm:col-span-2`}>
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

        <div class="space-y-3 rounded-md border border-border bg-surface-alt px-4 py-3">
          <Show when={props.state.form().insecureSkipVerify}>
            <TlsVerificationWarningBanner
              subject="this vCenter connection"
              remediation="Install a trusted certificate for vCenter before using this in production."
            />
          </Show>
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
            <span class="text-sm text-base-content">Enable this vCenter connection</span>
          </label>
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
                checked={props.state.form().monitorVms}
                onChange={(event) =>
                  props.state.updateForm({ monitorVms: event.currentTarget.checked })
                }
              />
              <span>
                <span class="font-medium">VMs</span>
                <span class="block text-xs text-muted">Power state, resources, snapshots.</span>
              </span>
            </label>
            <label class="flex items-start gap-2 text-sm text-base-content">
              <input
                type="checkbox"
                class={formCheckbox + ' mt-0.5'}
                checked={props.state.form().monitorHosts}
                onChange={(event) =>
                  props.state.updateForm({ monitorHosts: event.currentTarget.checked })
                }
              />
              <span>
                <span class="font-medium">Hosts</span>
                <span class="block text-xs text-muted">ESXi host health and capacity.</span>
              </span>
            </label>
            <label class="flex items-start gap-2 text-sm text-base-content">
              <input
                type="checkbox"
                class={formCheckbox + ' mt-0.5'}
                checked={props.state.form().monitorDatastores}
                onChange={(event) =>
                  props.state.updateForm({ monitorDatastores: event.currentTarget.checked })
                }
              />
              <span>
                <span class="font-medium">Datastores</span>
                <span class="block text-xs text-muted">Capacity and status per datastore.</span>
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

        <Show when={props.deleteConfirming}>
          <div class="rounded-md border border-border bg-surface-alt px-4 py-3 text-xs text-muted">
            Removing forgets this connection from Pulse; credentials on the platform itself are
            untouched.
          </div>
        </Show>

        <Show when={props.deleteError}>
          {(message) => (
            <div
              role="alert"
              class="rounded-md border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
            >
              {message()}
            </div>
          )}
        </Show>

        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Show when={isEditing() && props.onToggleEnabled}>
            <button
              type="button"
              class={buttonClass}
              onClick={() => props.onToggleEnabled?.()}
              disabled={
                props.state.saving() ||
                props.state.testing() ||
                props.togglePending ||
                props.deletePending
              }
            >
              {props.togglePending
                ? props.connectionEnabled
                  ? 'Pausing…'
                  : 'Resuming…'
                : props.connectionEnabled
                  ? 'Pause connection'
                  : 'Resume connection'}
            </button>
          </Show>
          <Show when={isEditing() && props.onDelete}>
            <button
              type="button"
              class={
                props.deleteConfirming
                  ? 'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-rose-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-60'
                  : 'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-rose-300 px-3 py-2 text-sm font-medium text-rose-700 transition-colors hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-rose-900 dark:text-rose-300 dark:hover:bg-rose-950'
              }
              onClick={() => props.onDelete?.()}
              disabled={
                props.state.saving() ||
                props.state.testing() ||
                props.togglePending ||
                props.deletePending
              }
            >
              {props.deletePending
                ? 'Deleting…'
                : props.deleteConfirming
                  ? 'Click again to confirm'
                  : 'Delete connection'}
            </button>
          </Show>
          <button
            type="button"
            class={buttonClass}
            onClick={handleCancel}
            disabled={
              props.state.saving() ||
              props.state.testing() ||
              props.togglePending ||
              props.deletePending
            }
          >
            Cancel
          </button>
          <button
            type="button"
            class={buttonClass}
            onClick={() => void props.state.testCurrentForm()}
            disabled={
              props.state.saving() ||
              props.state.testing() ||
              props.togglePending ||
              props.deletePending
            }
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
              props.togglePending ||
              props.deletePending ||
              props.state.monitoredSystemAdmissionSaveBlocked()
            }
          >
            {props.state.saving()
              ? isEditing()
                ? 'Saving…'
                : 'Adding…'
              : isEditing()
                ? 'Update connection'
                : 'Add connection'}
          </button>
        </div>
      </Show>
    </div>
  );
};
