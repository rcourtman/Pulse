import { Component, Show, createEffect, createMemo, onMount } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { CalloutCard } from '@/components/shared/CalloutCard';
import {
  formCheckbox,
  formControl,
  formField,
  formHelpText,
  formLabel,
} from '@/components/shared/Form';
import { FormSelect } from '@/components/shared/FormSelect';
import { TlsVerificationWarningBanner } from '@/components/shared/TlsVerificationWarningBanner';
import { humanizeConnectionError } from '@/utils/connectionErrorPresentation';
import { MonitoredSystemImpactPreview } from '../../MonitoredSystemImpactPreview';
import type { TrueNASConnection } from '@/api/truenas';
import type { TrueNASSettingsPanelState } from '../../useTrueNASSettingsPanelState';

export interface TrueNASCredentialSlotProps {
  state: TrueNASSettingsPanelState;
  editingConnection?: TrueNASConnection | null;
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

const describeTransport = (transport: NonNullable<TrueNASConnection['transport']>): string => {
  let description: string;
  if (transport.mode === 'jsonrpc-websocket') {
    description = `Transport: JSON-RPC WebSocket${transport.tls ? ' over TLS' : ''} · authentication: ${transport.authMechanism || 'session'}.`;
  } else if (transport.mode === 'legacy-rest') {
    description =
      `Transport: legacy REST (${transport.applianceVersion || 'legacy TrueNAS'}). ${transport.legacyReason || ''}`.trim();
  } else {
    description = 'Transport negotiation has not completed yet.';
  }
  if (transport.lastError) {
    description += ` Last error: ${transport.lastError}`;
  }
  return description;
};

export const TrueNASCredentialSlot: Component<TrueNASCredentialSlotProps> = (props) => {
  let primed = false;
  let cancelled = false;
  let connectionActionArea: HTMLDivElement | undefined;

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
    if (primed && !cancelled && !open && !props.state.saving()) {
      props.onSaved();
    }
  });

  createEffect(() => {
    if (props.editingConnection && typeof props.connectionEnabled === 'boolean') {
      if (props.state.form().enabled !== props.connectionEnabled) {
        props.state.updateForm({ enabled: props.connectionEnabled });
      }
    }
  });

  const handleCancel = () => {
    cancelled = true;
    props.state.closeDialog();
    props.onCancel();
  };

  const revealConnectionFeedback = () => {
    if (connectionActionArea?.isConnected) {
      // Reflow can put the result or the actions below a scrolled dialog edge.
      // Keep them in view without taking focus from the editor.
      connectionActionArea.scrollIntoView?.({ block: 'nearest', inline: 'nearest' });
    }
  };
  const handleTest = async () => {
    await props.state.testCurrentForm();
    if (props.state.connectionFeedback()) revealConnectionFeedback();
  };
  const handleSave = async () => {
    await props.state.saveCurrentForm();
    if (props.state.connectionFeedback()?.kind === 'destination-refused')
      revealConnectionFeedback();
  };

  const isEditing = () => Boolean(props.editingConnection);
  const connectionFeedback = createMemo(() => {
    const result = props.state.connectionFeedback();
    if (!result) return null;
    switch (result.kind) {
      case 'passed':
        return {
          title: 'System information read',
          description:
            'Pools, datasets, replication, and metrics were not checked. No settings were saved.',
          accent: 'border-emerald-500',
        };
      case 'unconfirmed':
        return {
          title: 'Connection test not confirmed',
          description: 'Pulse did not get a confirmed result. No settings were saved.',
          accent: 'border-amber-500',
        };
      case 'failed': {
        const humanized = humanizeConnectionError(result.detail);
        return {
          title: 'Connection test failed',
          description: result.detail,
          hint: humanized?.hint && humanized.hint !== result.detail ? humanized.hint : undefined,
          accent: 'border-red-500',
        };
      }
      case 'destination-refused':
        return {
          title: 'TrueNAS address not allowed',
          description: 'Use an address allowed for this workspace. Nothing was saved.',
          accent: 'border-red-500',
        };
    }
  });

  return (
    <div class="space-y-3">
      <Show when={props.state.featureDisabled()}>
        <CalloutCard
          tone="warning"
          scale="compact"
          padding="sm"
          description={
            props.state.featureDisabledMessage() ||
            'TrueNAS connections are disabled for this Pulse tier.'
          }
        />
      </Show>

      <Show when={!props.state.featureDisabled()}>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class={formField}>
            <span class={formLabel}>
              Name
              <span class="ml-0.5 text-red-500" aria-hidden="true">
                *
              </span>
            </span>
            <input
              class={formControl}
              value={props.state.form().name}
              onInput={(event) => props.state.updateForm({ name: event.currentTarget.value })}
              placeholder="truenas-1"
            />
          </label>
          <label class={formField}>
            <span class={formLabel}>
              Host
              <span class="ml-0.5 text-red-500" aria-hidden="true">
                *
              </span>
            </span>
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
          <FormSelect
            label="Authentication"
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
          </FormSelect>
        </div>

        <Show when={props.state.form().authMode === 'apiKey'}>
          <div class="grid gap-4 sm:grid-cols-2">
            <label class={formField}>
              <span class={formLabel}>
                API key owner username
                <span class="ml-0.5 text-red-500" aria-hidden="true">
                  *
                </span>
              </span>
              <input
                class={formControl}
                value={props.state.form().username}
                onInput={(event) => props.state.updateForm({ username: event.currentTarget.value })}
                placeholder="pulse-monitor"
              />
              <span class={formHelpText}>
                The TrueNAS username that owns this API key (for example, truenas_admin or root).
              </span>
            </label>
            <label class={formField}>
              <span class={formLabel}>
                API key
                <Show when={!props.state.form().hasStoredApiKey}>
                  <span class="ml-0.5 text-red-500" aria-hidden="true">
                    *
                  </span>
                </Show>
              </span>
              <input
                class={formControl}
                type="password"
                value={props.state.form().apiKey}
                onInput={(event) => props.state.updateForm({ apiKey: event.currentTarget.value })}
                placeholder={
                  props.state.form().hasStoredApiKey ? 'Saved API key retained unless replaced' : ''
                }
              />
              <Show when={props.state.form().hasStoredApiKey}>
                <span class={formHelpText}>Leave this blank to keep the saved API key.</span>
              </Show>
            </label>
          </div>
        </Show>

        <Show when={props.state.form().authMode === 'userpass'}>
          <div class="grid gap-4 sm:grid-cols-2">
            <label class={formField}>
              <span class={formLabel}>
                Username
                <span class="ml-0.5 text-red-500" aria-hidden="true">
                  *
                </span>
              </span>
              <input
                class={formControl}
                value={props.state.form().username}
                onInput={(event) => props.state.updateForm({ username: event.currentTarget.value })}
                placeholder="admin"
              />
            </label>
            <label class={formField}>
              <span class={formLabel}>
                Password
                <Show when={!props.state.form().hasStoredPassword}>
                  <span class="ml-0.5 text-red-500" aria-hidden="true">
                    *
                  </span>
                </Show>
              </span>
              <input
                class={formControl}
                type="password"
                value={props.state.form().password}
                onInput={(event) => props.state.updateForm({ password: event.currentTarget.value })}
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
            <Show when={props.state.form().insecureSkipVerify}>
              <TlsVerificationWarningBanner
                subject="this TrueNAS connection"
                remediation="Install a trusted certificate or configure the TLS fingerprint before using this in production."
              />
            </Show>
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
            <Show when={!(isEditing() && props.onToggleEnabled)}>
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
            </Show>
          </div>
        </div>

        <Show when={props.editingConnection?.transport}>
          {(transport) => (
            <CalloutCard
              tone={transport().mode === 'legacy-rest' ? 'warning' : 'info'}
              scale="compact"
              padding="sm"
              description={describeTransport(transport())}
            />
          )}
        </Show>

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

        <MonitoredSystemImpactPreview
          preview={props.state.monitoredSystemPreview()}
          loading={props.state.previewing()}
          error={props.state.monitoredSystemPreviewError()}
          errorTitle={props.state.monitoredSystemPreviewErrorTitle()}
        />

        <Show when={props.deleteConfirming}>
          <div class="rounded-md border border-border bg-surface-alt px-4 py-3 text-xs text-muted">
            Removing forgets this connection from Pulse. Credentials on the platform itself are
            untouched.
          </div>
        </Show>

        <Show when={props.deleteError}>
          {(message) => (
            <CalloutCard
              role="alert"
              tone="danger"
              scale="compact"
              padding="sm"
              description={message()}
            />
          )}
        </Show>

        <div
          ref={connectionActionArea}
          class="flex flex-col xl:flex-row xl:items-end xl:justify-between"
          classList={{ 'gap-2': Boolean(connectionFeedback()) }}
        >
          <div role="status" aria-atomic="true" class="min-w-0 xl:flex-1">
            <Show when={connectionFeedback()}>
              {(feedback) => (
                <div
                  class={`wrap-anywhere border-l-2 py-0.5 pl-2 text-xs leading-snug ${feedback().accent}`}
                >
                  <p class="font-semibold text-base-content">{feedback().title}</p>
                  <p class="text-muted">{feedback().description}</p>
                  <Show when={feedback().hint}>
                    {(hint) => <p class="mt-0.5 text-base-content/90">{hint()}</p>}
                  </Show>
                </div>
              )}
            </Show>
          </div>

          <div class="flex flex-col-reverse gap-2 sm:flex-row sm:flex-wrap sm:justify-end">
            <Show when={isEditing() && props.onToggleEnabled}>
              <Button
                variant="outline"
                size="settingsAction"
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
              </Button>
            </Show>
            <Show when={isEditing() && props.onDelete}>
              <Button
                variant={props.deleteConfirming ? 'danger' : 'dangerOutline'}
                size="settingsAction"
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
              </Button>
            </Show>
            <Button
              variant="outline"
              size="settingsAction"
              onClick={handleCancel}
              disabled={
                props.state.saving() ||
                props.state.testing() ||
                props.togglePending ||
                props.deletePending
              }
            >
              Cancel
            </Button>
            <div class="grid grid-cols-2 gap-2 sm:flex">
              <Button
                variant="outline"
                size="settingsAction"
                class="order-2 whitespace-normal sm:order-none"
                onClick={() => void handleTest()}
                disabled={
                  props.state.saving() ||
                  props.state.testing() ||
                  props.togglePending ||
                  props.deletePending
                }
              >
                {props.state.testing() ? 'Testing…' : 'Test connection'}
              </Button>
              <Button
                variant="outline"
                size="settingsAction"
                class="whitespace-normal"
                onClick={() => void props.state.previewCurrentForm()}
                disabled={props.state.saving() || props.state.testing() || props.state.previewing()}
                title="Show the resources Pulse would ingest from this connection without saving or starting polling."
              >
                {props.state.previewing() ? 'Previewing…' : 'Preview impact'}
              </Button>
            </div>
            <Button
              variant="primary"
              size="settingsAction"
              onClick={() => void handleSave()}
              disabled={
                props.state.saving() ||
                props.state.testing() ||
                props.state.previewing() ||
                props.togglePending ||
                props.deletePending
              }
            >
              {props.state.saving()
                ? isEditing()
                  ? 'Saving…'
                  : 'Adding…'
                : isEditing()
                  ? 'Update connection'
                  : 'Add connection'}
            </Button>
          </div>
        </div>
      </Show>
    </div>
  );
};
