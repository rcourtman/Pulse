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
import { MonitoredSystemAdmissionPreview } from '../../MonitoredSystemAdmissionPreview';
import type { VMwareSettingsPanelState } from '../../useVMwareSettingsPanelState';

const buttonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const primaryButtonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60';

export interface VMwareCredentialSlotProps {
  state: VMwareSettingsPanelState;
  onCancel: () => void;
  onSaved: () => void;
}

export const VMwareCredentialSlot: Component<VMwareCredentialSlotProps> = (props) => {
  let primed = false;

  onMount(() => {
    if (!props.state.dialogOpen()) {
      props.state.openCreateDialog();
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
