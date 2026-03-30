import type { Component } from 'solid-js';
import { For, Show } from 'solid-js';
import Database from 'lucide-solid/icons/database';
import ShieldAlert from 'lucide-solid/icons/shield-alert';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { Card } from '@/components/shared/Card';
import { Dialog } from '@/components/shared/Dialog';
import type { TrueNASConnection } from '@/api/truenas';
import {
  buildInfrastructurePath,
  buildRecoveryPath,
  buildStoragePath,
  buildWorkloadsPath,
} from '@/routing/resourceLinks';
import { formatNumber, formatRelativeTime } from '@/utils/format';
import {
  formCheckbox,
  formControl,
  formField,
  formHelpText,
  formLabel,
  formSelect,
} from '@/components/shared/Form';
import { getSettingsConfigurationLoadingState } from '@/utils/settingsShellPresentation';
import type { TrueNASSettingsPanelState } from './useTrueNASSettingsPanelState';

const buttonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const primaryButtonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60';
const dangerButtonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-red-300 px-3 py-2 text-sm font-medium text-red-700 transition-colors hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-red-800 dark:text-red-300 dark:hover:bg-red-950/30';

const connectionMetaBadgeClass =
  'inline-flex items-center rounded-full border border-border bg-surface px-2 py-0.5 text-xs font-medium text-muted';

const getPollIntervalLabel = (seconds: number | undefined): string => {
  const value = seconds && Number.isFinite(seconds) && seconds > 0 ? seconds : 60;
  if (value % 60 === 0) {
    const minutes = value / 60;
    return minutes === 1 ? 'Poll every 1 minute' : `Poll every ${minutes} minutes`;
  }
  return value === 1 ? 'Poll every 1 second' : `Poll every ${value} seconds`;
};

const getConnectionHealthPresentation = (connection: TrueNASConnection) => {
  if (!connection.enabled) {
    return {
      label: 'Paused',
      className: 'bg-surface text-muted',
      detail: 'Polling paused',
      error: null,
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
      label: 'Sync failing',
      className: 'bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300',
      detail: lastError.at
        ? `Last error ${formatRelativeTime(lastError.at, { compact: true })}`
        : 'Last poll failed',
      error: lastError.message || null,
    };
  }

  if (lastSuccessAt) {
    return {
      label: 'Healthy',
      className: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300',
      detail: `Last sync ${formatRelativeTime(lastSuccessAt, { compact: true })}`,
      error: null,
    };
  }

  return {
    label: 'Awaiting first sync',
    className: 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300',
    detail: 'Pulse has not completed the first poll yet',
    error: null,
  };
};

const getConnectionObservedMetrics = (connection: TrueNASConnection) => {
  const observed = connection.observed;
  if (!observed) return [];
  return [
    { label: 'system', value: observed.systems },
    { label: 'pool', value: observed.storagePools },
    { label: 'dataset', value: observed.datasets },
    { label: 'app', value: observed.apps },
    { label: 'disk', value: observed.disks },
    { label: 'recovery artifact', value: observed.recoveryArtifacts },
  ].filter((item) => item.value > 0);
};

const pluralize = (count: number, singular: string): string =>
  count === 1 ? singular : `${singular}s`;

const getConnectionHandoffs = (connection: TrueNASConnection) => {
  const host = (connection.observed?.host || '').trim();
  const resourceId = (connection.observed?.resourceId || host).trim();
  if (!host && !resourceId) {
    return null;
  }
  return {
    infrastructure: buildInfrastructurePath({
      source: 'truenas',
      ...(resourceId ? { resource: resourceId } : {}),
    }),
    workloads: buildWorkloadsPath({
      type: 'app-container',
      platform: 'truenas',
      ...(host ? { agent: host } : {}),
    }),
    storage: buildStoragePath({
      source: 'truenas',
      ...(resourceId ? { node: resourceId } : {}),
    }),
    recovery: buildRecoveryPath({
      platform: 'truenas',
      ...(host ? { node: host } : {}),
    }),
  };
};

interface TrueNASSettingsPanelProps {
  state: TrueNASSettingsPanelState;
}

export const TrueNASSettingsPanel: Component<TrueNASSettingsPanelProps> = (props) => {
  const state = props.state;

  return (
    <div class="space-y-6">
      <CalloutCard
        tone="info"
        title="TrueNAS platform integration"
        description={
          <>
            <p>
              Connect TrueNAS through its API so Pulse can ingest pools, datasets, disks, disk
              temperatures, apps, alerts, and recovery artifacts as a first-class platform instead
              of treating the NAS like a generic host.
            </p>
            <p class="mt-2">
              Use the unified agent on TrueNAS only as optional augmentation later if there is
              extra host telemetry the API cannot provide.
            </p>
          </>
        }
        icon={<Database class="h-5 w-5" />}
      />

      <Show when={state.featureDisabled()}>
        <CalloutCard
          tone="warning"
          title="TrueNAS integration is disabled"
          description={
            <>
              <p>
                {state.featureDisabledMessage() ||
                  'TrueNAS integration has been explicitly disabled on this Pulse server.'}
              </p>
              <p class="mt-2">
                Remove <code>PULSE_ENABLE_TRUENAS=false</code> or set it back to{' '}
                <code>true</code> on the Pulse server, then restart the service before managing
                TrueNAS connections.
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
                <h3 class="text-base font-semibold text-base-content">TrueNAS connections</h3>
                <p class="text-sm text-muted">
                  Manage the API-backed TrueNAS systems Pulse should poll and normalize into the
                  unified resource model for infrastructure, workloads, alerts, storage, and
                  recovery.
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
                <button
                  type="button"
                  class={primaryButtonClass}
                  onClick={state.openCreateDialog}
                >
                  Add TrueNAS connection
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
                title="Failed to load TrueNAS connections"
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

            <Show when={!state.loading() && !state.loadingError() && state.connections().length === 0}>
              <div class="rounded-lg border border-dashed border-border bg-surface-alt px-6 py-12 text-center">
                <p class="text-base font-medium text-base-content">No TrueNAS connections yet</p>
                <p class="mt-1 text-sm text-muted">
                  Add the first TrueNAS API endpoint Pulse should poll.
                </p>
              </div>
            </Show>

            <Show when={!state.loading() && !state.loadingError() && state.connections().length > 0}>
              <div class="space-y-3">
                <For each={state.connections()}>
                  {(connection) => {
                    const health = () => getConnectionHealthPresentation(connection);
                    const observedMetrics = () => getConnectionObservedMetrics(connection);
                    const handoffs = () => getConnectionHandoffs(connection);

                    return (
                      <div
                        class="rounded-lg border border-border bg-surface-alt px-4 py-4"
                        data-testid={`truenas-connection-${connection.id}`}
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
                                <span>
                                  {connection.apiKey ? 'API key auth' : 'Username/password auth'}
                                </span>
                                <span aria-hidden="true">•</span>
                                <span>{connection.useHttps ? 'HTTPS' : 'HTTP'}</span>
                                <Show when={connection.insecureSkipVerify}>
                                  <>
                                    <span aria-hidden="true">•</span>
                                    <span>Skip TLS verification</span>
                                  </>
                                </Show>
                                <span aria-hidden="true">•</span>
                                <span>{getPollIntervalLabel(connection.pollIntervalSeconds)}</span>
                                <Show when={connection.poll?.lastAttemptAt || health().detail}>
                                  <>
                                    <span aria-hidden="true">•</span>
                                    <span>{health().detail}</span>
                                  </>
                                </Show>
                              </div>

                              <Show when={health().error}>
                                <p class="text-xs font-medium text-red-700 dark:text-red-300">
                                  {health().error}
                                </p>
                              </Show>
                            </div>

                            <Show when={connection.observed}>
                              <div class="space-y-2">
                                <div class="flex flex-wrap gap-2">
                                  <Show when={connection.observed?.collectedAt}>
                                    <span class={connectionMetaBadgeClass}>
                                      Synced{' '}
                                      {formatRelativeTime(connection.observed?.collectedAt, {
                                        compact: true,
                                      })}
                                    </span>
                                  </Show>
                                  <For each={observedMetrics()}>
                                    {(item) => (
                                      <span class={connectionMetaBadgeClass}>
                                        {formatNumber(item.value)} {pluralize(item.value, item.label)}
                                      </span>
                                    )}
                                  </For>
                                </div>

                                <Show when={handoffs()}>
                                  {(links) => (
                                    <div class="flex flex-wrap items-center gap-2">
                                      <a
                                        href={links().infrastructure}
                                        class={buttonClass}
                                        data-testid={`truenas-connection-${connection.id}-infrastructure`}
                                      >
                                        Infrastructure
                                      </a>
                                      <a
                                        href={links().workloads}
                                        class={buttonClass}
                                        data-testid={`truenas-connection-${connection.id}-workloads`}
                                      >
                                        Workloads
                                      </a>
                                      <a
                                        href={links().storage}
                                        class={buttonClass}
                                        data-testid={`truenas-connection-${connection.id}-storage`}
                                      >
                                        Storage
                                      </a>
                                      <a
                                        href={links().recovery}
                                        class={buttonClass}
                                        data-testid={`truenas-connection-${connection.id}-recovery`}
                                      >
                                        Recovery
                                      </a>
                                    </div>
                                  )}
                                </Show>
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
        ariaLabel={state.editingConnection() ? 'Edit TrueNAS connection' : 'Add TrueNAS connection'}
        panelClass="w-full max-w-2xl"
      >
        <div class="space-y-6 p-6">
          <div class="space-y-1">
            <h3 class="text-lg font-semibold text-base-content">
              {state.editingConnection() ? 'Edit TrueNAS connection' : 'Add TrueNAS connection'}
            </h3>
            <p class="text-sm text-muted">
              Configure the API endpoint Pulse should poll for this TrueNAS system.
            </p>
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <label class={formField}>
              <span class={formLabel}>Name</span>
              <input
                class={formControl}
                value={state.form().name}
                onInput={(event) => state.updateForm({ name: event.currentTarget.value })}
                placeholder="tower"
              />
            </label>
            <label class={formField}>
              <span class={formLabel}>Host</span>
              <input
                class={formControl}
                value={state.form().host}
                onInput={(event) => state.updateForm({ host: event.currentTarget.value })}
                placeholder="truenas.local"
              />
            </label>
            <label class={formField}>
              <span class={formLabel}>Port</span>
              <input
                class={formControl}
                inputMode="numeric"
                value={state.form().port}
                onInput={(event) => state.updateForm({ port: event.currentTarget.value })}
                placeholder={state.form().useHttps ? '443' : '80'}
              />
            </label>
            <label class={formField}>
              <span class={formLabel}>Poll interval (seconds)</span>
              <input
                class={formControl}
                inputMode="numeric"
                value={state.form().pollIntervalSeconds}
                onInput={(event) =>
                  state.updateForm({ pollIntervalSeconds: event.currentTarget.value })
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
                value={state.form().authMode}
                onChange={(event) =>
                  state.updateForm({
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

          <Show when={state.form().authMode === 'apiKey'}>
            <label class={formField}>
              <span class={formLabel}>API key</span>
              <input
                class={formControl}
                type="password"
                value={state.form().apiKey}
                onInput={(event) => state.updateForm({ apiKey: event.currentTarget.value })}
                placeholder={
                  state.form().hasStoredApiKey ? 'Saved API key retained unless replaced' : ''
                }
              />
              <Show when={state.form().hasStoredApiKey}>
                <span class={formHelpText}>
                  Leave this blank to keep the saved API key.
                </span>
              </Show>
            </label>
          </Show>

          <Show when={state.form().authMode === 'userpass'}>
            <div class="grid gap-4 sm:grid-cols-2">
              <label class={formField}>
                <span class={formLabel}>Username</span>
                <input
                  class={formControl}
                  value={state.form().username}
                  onInput={(event) => state.updateForm({ username: event.currentTarget.value })}
                  placeholder="admin"
                />
              </label>
              <label class={formField}>
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
                  <span class={formHelpText}>
                    Leave this blank to keep the saved password.
                  </span>
                </Show>
              </label>
            </div>
          </Show>

          <div class="grid gap-4 sm:grid-cols-2">
            <label class={formField}>
              <span class={formLabel}>TLS fingerprint</span>
              <input
                class={formControl}
                value={state.form().fingerprint}
                onInput={(event) => state.updateForm({ fingerprint: event.currentTarget.value })}
                placeholder="Optional SHA256 fingerprint"
              />
              <span class={formHelpText}>
                Optional certificate pin for HTTPS connections.
              </span>
            </label>
            <div class="space-y-3 rounded-md border border-border bg-surface-alt px-4 py-3">
              <label class="flex items-center gap-3">
                <input
                  type="checkbox"
                  class={formCheckbox}
                  checked={state.form().useHttps}
                  onChange={(event) => state.updateForm({ useHttps: event.currentTarget.checked })}
                />
                <span class="text-sm text-base-content">Use HTTPS</span>
              </label>
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
                <span class="text-sm text-base-content">Enable polling immediately</span>
              </label>
            </div>
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
        ariaLabel="Delete TrueNAS connection"
        panelClass="w-full max-w-lg"
      >
        <div class="space-y-5 p-6">
          <div class="space-y-1">
            <h3 class="text-lg font-semibold text-base-content">Delete TrueNAS connection</h3>
            <p class="text-sm text-muted">
              Remove{' '}
              <span class="font-medium text-base-content">
                {state.pendingDeleteConnection()?.name || state.pendingDeleteConnection()?.host}
              </span>{' '}
              from the configured TrueNAS API integrations.
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

export default TrueNASSettingsPanel;
