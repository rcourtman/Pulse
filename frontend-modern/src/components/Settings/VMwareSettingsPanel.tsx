import type { Component } from 'solid-js';
import { For, Show } from 'solid-js';
import Boxes from 'lucide-solid/icons/boxes';
import ShieldAlert from 'lucide-solid/icons/shield-alert';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { Card } from '@/components/shared/Card';
import { Dialog } from '@/components/shared/Dialog';
import type { VMwareConnection } from '@/api/vmware';
import { formatNumber, formatRelativeTime } from '@/utils/format';
import {
  formCheckbox,
  formControl,
  formField,
  formHelpText,
  formLabel,
} from '@/components/shared/Form';
import { getSettingsConfigurationLoadingState } from '@/utils/settingsShellPresentation';
import type { VMwareSettingsPanelState } from './useVMwareSettingsPanelState';

const buttonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const primaryButtonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60';
const dangerButtonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-red-300 px-3 py-2 text-sm font-medium text-red-700 transition-colors hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-red-800 dark:text-red-300 dark:hover:bg-red-950/30';

const connectionMetaBadgeClass =
  'inline-flex items-center rounded-full border border-border bg-surface px-2 py-0.5 text-xs font-medium text-muted';

const getObservedIssueSummary = (connection: VMwareConnection): string | null => {
  const observed = connection.observed;
  const firstIssue = observed?.issues?.find((issue) => !!issue.message);
  if (!firstIssue?.message) return null;
  const issueCount = observed?.issueCount ?? observed?.issues?.length ?? 0;
  if (issueCount > 1) {
    return `${firstIssue.message} (+${issueCount - 1} more degraded reads)`;
  }
  return firstIssue.message;
};

const getConnectionHealthPresentation = (connection: VMwareConnection) => {
  if (!connection.enabled) {
    return {
      label: 'Disabled',
      className: 'bg-surface text-muted',
      detail: 'Manual validation paused',
      note: null,
      noteClassName: '',
    };
  }

  const poll = connection.poll;
  const lastSuccessAt = poll?.lastSuccessAt;
  const lastError = poll?.lastError;
  const lastErrorAfterSuccess =
    !!lastError?.at &&
    (!lastSuccessAt || new Date(lastError.at).getTime() >= new Date(lastSuccessAt).getTime());

  if (lastError && lastErrorAfterSuccess) {
    return {
      label: 'Runtime failing',
      className: 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300',
      detail: lastError.at
        ? `Last check ${formatRelativeTime(lastError.at, { compact: true })}`
        : 'Last check failed',
      note: lastError.message || null,
      noteClassName: 'text-red-700 dark:text-red-300',
    };
  }

  if (lastSuccessAt && connection.observed?.degraded) {
    return {
      label: 'Degraded',
      className: 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300',
      detail: `Last check ${formatRelativeTime(lastSuccessAt, { compact: true })} with partial enrichment`,
      note: getObservedIssueSummary(connection),
      noteClassName: 'text-amber-700 dark:text-amber-300',
    };
  }

  if (lastSuccessAt) {
    return {
      label: 'Healthy',
      className: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300',
      detail: `Last check ${formatRelativeTime(lastSuccessAt, { compact: true })}`,
      note: null,
      noteClassName: '',
    };
  }

  return {
    label: 'Awaiting first poll',
    className: 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300',
    detail: 'Pulse has not completed the first vCenter poll yet',
    note: null,
    noteClassName: '',
  };
};

const getConnectionObservedMetrics = (connection: VMwareConnection) => {
  const observed = connection.observed;
  if (!observed) return [];
  return [
    { label: 'host', value: observed.hosts },
    { label: 'vm', value: observed.vms },
    { label: 'datastore', value: observed.datastores },
  ].filter((item) => item.value > 0);
};

const pluralize = (count: number, singular: string): string =>
  count === 1 ? singular : `${singular}s`;

interface VMwareSettingsPanelProps {
  state: VMwareSettingsPanelState;
}

export const VMwareSettingsPanel: Component<VMwareSettingsPanelProps> = (props) => {
  const state = props.state;

  return (
    <div class="space-y-6">
      <CalloutCard
        tone="info"
        title="VMware vSphere platform integration"
        description={
          <>
            <p>
              Connect VMware through vCenter so Pulse can validate API-backed access and stage the
              canonical phase-1 floor for hosts, virtual machines, datastores, and shared alert or
              assistant read paths.
            </p>
            <p class="mt-2">
              Phase 1 is vCenter-only and API-first. Direct ESXi onboarding and write-path control
              stay out of scope here.
            </p>
          </>
        }
        icon={<Boxes class="h-5 w-5" />}
      />

      <Show when={state.featureDisabled()}>
        <CalloutCard
          tone="warning"
          title="VMware integration is disabled"
          description={
            <>
              <p>
                {state.featureDisabledMessage() ||
                  'VMware integration has been explicitly disabled on this Pulse server.'}
              </p>
              <p class="mt-2">
                Remove <code>PULSE_ENABLE_VMWARE=false</code> or set it back to <code>true</code> on
                the Pulse server, then restart the service before managing VMware connections.
              </p>
            </>
          }
          icon={<ShieldAlert class="h-5 w-5" />}
        />
      </Show>

      <Show when={!state.featureDisabled()}>
        <Card padding="lg" class="rounded-xl border border-border shadow-sm">
          <div class="space-y-4">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div class="space-y-1">
                <h3 class="text-base font-semibold text-base-content">VMware connections</h3>
                <p class="text-sm text-muted">
                  Manage the vCenter endpoints Pulse should validate and use as the shared VMware
                  platform onboarding boundary.
                </p>
              </div>
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  class={buttonClass}
                  onClick={() => void state.loadConnections()}
                  disabled={state.loading()}
                >
                  Refresh
                </button>
                <button type="button" class={primaryButtonClass} onClick={state.openCreateDialog}>
                  Add VMware connection
                </button>
              </div>
            </div>

            <Show when={state.loading()}>
              <div class="flex items-center justify-center rounded-md border border-dashed border-border bg-surface-alt py-12 text-sm text-muted">
                {getSettingsConfigurationLoadingState().text}
              </div>
            </Show>

            <Show when={!state.loading() && state.loadingError()}>
              <CalloutCard
                tone="danger"
                title="Failed to load VMware connections"
                description={
                  <>
                    <p>{state.loadingError()}</p>
                    <button
                      type="button"
                      class={`mt-3 ${buttonClass}`}
                      onClick={() => void state.loadConnections()}
                    >
                      Retry
                    </button>
                  </>
                }
              />
            </Show>

            <Show
              when={!state.loading() && !state.loadingError() && state.connections().length === 0}
            >
              <div class="rounded-lg border border-dashed border-border bg-surface-alt px-6 py-12 text-center">
                <p class="text-base font-medium text-base-content">No VMware connections yet</p>
                <p class="mt-1 text-sm text-muted">
                  Add the first vCenter endpoint Pulse should validate.
                </p>
              </div>
            </Show>

            <Show
              when={!state.loading() && !state.loadingError() && state.connections().length > 0}
            >
              <div class="space-y-3">
                <For each={state.connections()}>
                  {(connection) => {
                    const health = () => getConnectionHealthPresentation(connection);
                    const observedMetrics = () => getConnectionObservedMetrics(connection);

                    return (
                      <div
                        class="rounded-lg border border-border bg-surface-alt px-4 py-4"
                        data-testid={`vmware-connection-${connection.id}`}
                      >
                        <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                          <div class="space-y-3">
                            <div class="space-y-2">
                              <div class="flex flex-wrap items-center gap-2">
                                <h4 class="text-sm font-semibold text-base-content">
                                  {connection.name || connection.host}
                                </h4>
                                <span
                                  class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                                    connection.enabled
                                      ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
                                      : 'bg-surface text-muted'
                                  }`}
                                >
                                  {connection.enabled ? 'Enabled' : 'Disabled'}
                                </span>
                                <span
                                  class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${health().className}`}
                                >
                                  {health().label}
                                </span>
                              </div>

                              <p class="text-sm text-muted">
                                {connection.host}
                                {connection.port ? `:${connection.port}` : ''}
                              </p>

                              <div class="flex flex-wrap items-center gap-2 text-xs text-muted">
                                <span>{connection.username || 'Username not set'}</span>
                                <span aria-hidden="true">•</span>
                                <span>vCenter</span>
                                <Show when={connection.insecureSkipVerify}>
                                  <>
                                    <span aria-hidden="true">•</span>
                                    <span>Skip TLS verification</span>
                                  </>
                                </Show>
                                <Show when={connection.poll?.lastAttemptAt || health().detail}>
                                  <>
                                    <span aria-hidden="true">•</span>
                                    <span>{health().detail}</span>
                                  </>
                                </Show>
                              </div>

                              <Show when={health().note}>
                                <p class={`text-xs font-medium ${health().noteClassName}`}>
                                  {health().note}
                                </p>
                              </Show>
                            </div>

                            <Show when={connection.observed}>
                              <div class="space-y-2">
                                <div class="flex flex-wrap gap-2">
                                  <Show when={connection.observed?.collectedAt}>
                                    <span class={connectionMetaBadgeClass}>
                                      Observed{' '}
                                      {formatRelativeTime(connection.observed?.collectedAt, {
                                        compact: true,
                                      })}
                                    </span>
                                  </Show>
                                  <For each={observedMetrics()}>
                                    {(item) => (
                                      <span class={connectionMetaBadgeClass}>
                                        {formatNumber(item.value)}{' '}
                                        {pluralize(item.value, item.label)}
                                      </span>
                                    )}
                                  </For>
                                  <Show when={connection.observed?.viRelease}>
                                    <span class={connectionMetaBadgeClass}>
                                      VI JSON {connection.observed?.viRelease}
                                    </span>
                                  </Show>
                                  <Show when={connection.observed?.degraded}>
                                    <span class="inline-flex items-center rounded-full border border-amber-300 bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300">
                                      {formatNumber(connection.observed?.issueCount ?? 0)} degraded{' '}
                                      {pluralize(connection.observed?.issueCount ?? 0, 'read')}
                                    </span>
                                  </Show>
                                </div>
                              </div>
                            </Show>
                          </div>

                          <div class="flex flex-wrap items-center gap-2">
                            <button
                              type="button"
                              class={buttonClass}
                              onClick={() => void state.testSavedConnection(connection)}
                              disabled={state.testing()}
                            >
                              Test
                            </button>
                            <button
                              type="button"
                              class={buttonClass}
                              onClick={() => state.openEditDialog(connection)}
                            >
                              Edit
                            </button>
                            <button
                              type="button"
                              class={dangerButtonClass}
                              onClick={() => state.openDeleteDialog(connection)}
                            >
                              Delete
                            </button>
                          </div>
                        </div>
                      </div>
                    );
                  }}
                </For>
              </div>
            </Show>
          </div>
        </Card>
      </Show>

      <Dialog
        isOpen={state.dialogOpen()}
        onClose={state.closeDialog}
        ariaLabel={state.editingConnection() ? 'Edit VMware connection' : 'Add VMware connection'}
        panelClass="w-full max-w-2xl"
      >
        <div class="space-y-6 p-6">
          <div class="space-y-1">
            <h3 class="text-lg font-semibold text-base-content">
              {state.editingConnection() ? 'Edit VMware connection' : 'Add VMware connection'}
            </h3>
            <p class="text-sm text-muted">
              Configure the vCenter endpoint Pulse should validate for the VMware platform.
            </p>
          </div>

          <Show when={state.connectionFailure()}>
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
                value={state.form().name}
                onInput={(event) => state.updateForm({ name: event.currentTarget.value })}
                placeholder="lab-vcenter"
              />
            </label>
            <label class={formField}>
              <span class={formLabel}>Host</span>
              <input
                class={formControl}
                value={state.form().host}
                onInput={(event) => state.updateForm({ host: event.currentTarget.value })}
                placeholder="vcsa.lab.local"
              />
            </label>
            <label class={formField}>
              <span class={formLabel}>Port</span>
              <input
                class={formControl}
                inputMode="numeric"
                value={state.form().port}
                onInput={(event) => state.updateForm({ port: event.currentTarget.value })}
                placeholder="443"
              />
            </label>
            <label class={formField}>
              <span class={formLabel}>Username</span>
              <input
                class={formControl}
                value={state.form().username}
                onInput={(event) => state.updateForm({ username: event.currentTarget.value })}
                placeholder="administrator@vsphere.local"
              />
            </label>
            <label class={`${formField} sm:col-span-2`}>
              <span class={formLabel}>Password</span>
              <input
                class={formControl}
                type="password"
                value={state.form().password}
                onInput={(event) => state.updateForm({ password: event.currentTarget.value })}
                placeholder={
                  state.form().hasStoredPassword ? 'Saved password retained unless replaced' : ''
                }
              />
              <Show when={state.form().hasStoredPassword}>
                <span class={formHelpText}>Leave this blank to keep the saved password.</span>
              </Show>
            </label>
          </div>

          <div class="space-y-3 rounded-md border border-border bg-surface-alt px-4 py-3">
            <label class="flex items-center gap-3">
              <input
                type="checkbox"
                class={formCheckbox}
                checked={state.form().insecureSkipVerify}
                onChange={(event) =>
                  state.updateForm({ insecureSkipVerify: event.currentTarget.checked })
                }
              />
              <span class="text-sm text-base-content">Skip TLS verification</span>
            </label>
            <label class="flex items-center gap-3">
              <input
                type="checkbox"
                class={formCheckbox}
                checked={state.form().enabled}
                onChange={(event) => state.updateForm({ enabled: event.currentTarget.checked })}
              />
              <span class="text-sm text-base-content">Enable this vCenter connection</span>
            </label>
          </div>

          <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <button
              type="button"
              class={buttonClass}
              onClick={state.closeDialog}
              disabled={state.saving() || state.testing()}
            >
              Cancel
            </button>
            <button
              type="button"
              class={buttonClass}
              onClick={() => void state.testCurrentForm()}
              disabled={state.saving() || state.testing()}
            >
              {state.testing() ? 'Testing…' : 'Test connection'}
            </button>
            <button
              type="button"
              class={primaryButtonClass}
              onClick={() => void state.saveCurrentForm()}
              disabled={state.saving() || state.testing()}
            >
              {state.saving()
                ? state.editingConnection()
                  ? 'Saving…'
                  : 'Adding…'
                : state.editingConnection()
                  ? 'Save connection'
                  : 'Add connection'}
            </button>
          </div>
        </div>
      </Dialog>

      <Dialog
        isOpen={state.deleteDialogOpen()}
        onClose={state.closeDeleteDialog}
        ariaLabel="Delete VMware connection"
        panelClass="w-full max-w-lg"
      >
        <div class="space-y-5 p-6">
          <div class="space-y-1">
            <h3 class="text-lg font-semibold text-base-content">Delete VMware connection</h3>
            <p class="text-sm text-muted">
              Remove{' '}
              <span class="font-medium text-base-content">
                {state.pendingDeleteConnection()?.name || state.pendingDeleteConnection()?.host}
              </span>{' '}
              from the configured VMware platform connections.
            </p>
          </div>

          <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <button
              type="button"
              class={buttonClass}
              onClick={state.closeDeleteDialog}
              disabled={state.deleting()}
            >
              Cancel
            </button>
            <button
              type="button"
              class={dangerButtonClass}
              onClick={() => void state.deletePendingConnection()}
              disabled={state.deleting()}
            >
              {state.deleting() ? 'Deleting…' : 'Delete connection'}
            </button>
          </div>
        </div>
      </Dialog>
    </div>
  );
};

export default VMwareSettingsPanel;
