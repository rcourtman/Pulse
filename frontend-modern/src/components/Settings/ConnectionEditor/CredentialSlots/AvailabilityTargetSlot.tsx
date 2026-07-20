import { Component, For, Show, createMemo, createSignal, onMount } from 'solid-js';
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
import {
  AvailabilityTargetsAPI,
  type AvailabilityProbeProtocol,
  type AvailabilityTarget,
  type AvailabilityTargetKind,
  type AvailabilityTestResponse,
  type AvailabilityUDPMode,
} from '@/api/availabilityTargets';
import {
  AVAILABILITY_TARGET_PRESETS,
  CUSTOM_AVAILABILITY_PRESET_ID,
  applyAvailabilityTargetPreset,
  availabilityPresetById,
  type AvailabilityTargetPresetID,
} from '../availabilityTargetPresets';
import { useResources } from '@/hooks/useResources';
import { getSourcePlatformLabel } from '@/utils/sourcePlatforms';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';
import type { Resource } from '@/types/resource';

interface AvailabilityForm {
  id: string;
  name: string;
  targetKind: AvailabilityTargetKind;
  address: string;
  protocol: AvailabilityProbeProtocol;
  port: string;
  path: string;
  udpMode: AvailabilityUDPMode;
  udpRequest: string;
  udpExpectedResponse: string;
  linkedResourceId: string;
  enabled: boolean;
  pollIntervalSeconds: string;
  timeoutMillis: string;
  failureThreshold: string;
}

export interface AvailabilityTargetSlotProps {
  editingTargetId?: string | null;
  onCancel: () => void;
  onSaved: () => void;
  initialTargetKind?: AvailabilityTargetKind;
  onToggleEnabled?: () => void;
  togglePending?: boolean;
  connectionEnabled?: boolean;
  onDelete?: () => void;
  deletePending?: boolean;
  deleteConfirming?: boolean;
  deleteError?: string | null;
}

const newAvailabilityForm = (
  initialTargetKind: AvailabilityTargetKind = 'service',
): AvailabilityForm => ({
  id: '',
  name: '',
  targetKind: initialTargetKind,
  address: '',
  protocol: 'icmp',
  port: '',
  path: '',
  udpMode: 'response_required',
  udpRequest: '',
  udpExpectedResponse: '',
  linkedResourceId: '',
  enabled: true,
  pollIntervalSeconds: '60',
  timeoutMillis: '2000',
  failureThreshold: '2',
});

const formFromTarget = (target: AvailabilityTarget): AvailabilityForm => ({
  id: target.id,
  name: target.name ?? '',
  targetKind: target.targetKind ?? 'service',
  address: target.address ?? '',
  protocol: target.protocol ?? 'icmp',
  port: target.port ? String(target.port) : '',
  path: target.path ?? '',
  udpMode: target.udpMode ?? 'response_required',
  udpRequest: target.udpRequest ?? '',
  udpExpectedResponse: target.udpExpectedResponse ?? '',
  linkedResourceId: target.linkedResourceId ?? '',
  enabled: target.enabled ?? true,
  pollIntervalSeconds: String(target.pollIntervalSeconds ?? 60),
  timeoutMillis: String(target.timeoutMillis ?? 2000),
  failureThreshold: String(target.failureThreshold ?? 2),
});

const parsePositiveInt = (value: string): number | undefined => {
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
};

const payloadFromForm = (form: AvailabilityForm): AvailabilityTarget => {
  const port = parsePositiveInt(form.port);
  return {
    id: form.id,
    name: form.name.trim(),
    targetKind: form.targetKind,
    address: form.address.trim(),
    protocol: form.protocol,
    port: form.protocol === 'icmp' ? undefined : port,
    path: form.protocol === 'http' || form.protocol === 'https' ? form.path.trim() : undefined,
    udpMode: form.protocol === 'udp' ? form.udpMode : undefined,
    udpRequest: form.protocol === 'udp' ? form.udpRequest : undefined,
    udpExpectedResponse: form.protocol === 'udp' ? form.udpExpectedResponse : undefined,
    linkedResourceId: form.linkedResourceId.trim() || undefined,
    enabled: form.enabled,
    pollIntervalSeconds: parsePositiveInt(form.pollIntervalSeconds),
    timeoutMillis: parsePositiveInt(form.timeoutMillis),
    failureThreshold: parsePositiveInt(form.failureThreshold),
  };
};

const presetSensitiveFormKeys: ReadonlySet<keyof AvailabilityForm> = new Set([
  'path',
  'port',
  'protocol',
  'targetKind',
  'udpMode',
  'udpRequest',
  'udpExpectedResponse',
]);

const initialPresetForTargetKind = (
  targetKind: AvailabilityTargetKind | undefined,
): AvailabilityTargetPresetID =>
  targetKind === 'machine' ? 'ping-machine' : CUSTOM_AVAILABILITY_PRESET_ID;

export const AvailabilityTargetSlot: Component<AvailabilityTargetSlotProps> = (props) => {
  const { resources } = useResources();

  const linkableResources = createMemo<Resource[]>(() =>
    resources().filter((r) => r.type !== 'network-endpoint'),
  );

  const groupedLinkableResources = createMemo(() => {
    const groups = new Map<string, Resource[]>();
    for (const r of linkableResources()) {
      const platform = r.platformType || 'generic';
      const list = groups.get(platform);
      if (list) {
        list.push(r);
      } else {
        groups.set(platform, [r]);
      }
    }
    for (const list of groups.values()) {
      list.sort((a, b) =>
        getPreferredInfrastructureDisplayName(a).localeCompare(
          getPreferredInfrastructureDisplayName(b),
        ),
      );
    }
    return [...groups.entries()].sort((a, b) =>
      getSourcePlatformLabel(a[0]).localeCompare(getSourcePlatformLabel(b[0])),
    );
  });

  const [form, setForm] = createSignal<AvailabilityForm>(
    newAvailabilityForm(props.initialTargetKind),
  );
  const [selectedPreset, setSelectedPreset] = createSignal<AvailabilityTargetPresetID>(
    initialPresetForTargetKind(props.initialTargetKind),
  );
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [testResult, setTestResult] = createSignal<AvailabilityTestResponse | null>(null);

  const linkedResourceMissing = createMemo(() => {
    const id = form().linkedResourceId.trim();
    if (!id) return false;
    return !linkableResources().some((r) => r.id === id);
  });

  const updateForm = (patch: Partial<AvailabilityForm>, preservePreset = false) => {
    setForm((current) => ({ ...current, ...patch }));
    if (
      !preservePreset &&
      Object.keys(patch).some((key) => presetSensitiveFormKeys.has(key as keyof AvailabilityForm))
    ) {
      setSelectedPreset(CUSTOM_AVAILABILITY_PRESET_ID);
    }
    setError(null);
    setTestResult(null);
  };

  const selectedPresetConfig = () => availabilityPresetById(selectedPreset());

  const addressPlaceholder = () =>
    selectedPresetConfig()?.addressPlaceholder ??
    (form().protocol === 'http' || form().protocol === 'https'
      ? `${form().protocol}://service.local/status`
      : form().targetKind === 'machine'
        ? 'server.local'
        : form().targetKind === 'service'
          ? 'service.local'
          : 'device.local');

  const portPlaceholder = () =>
    selectedPresetConfig()?.portPlaceholder ??
    (form().protocol === 'http' || form().protocol === 'https' ? 'Optional' : '1883');

  const namePlaceholder = () =>
    form().targetKind === 'machine'
      ? 'mac-mini'
      : form().targetKind === 'service'
        ? 'mqtt-broker'
        : 'energy-monitor';

  const addButtonLabel = () =>
    form().targetKind === 'machine'
      ? 'Add machine check'
      : form().targetKind === 'service' || form().targetKind === 'device'
        ? 'Add service/device check'
        : 'Add target';

  const handlePresetChange = (presetId: AvailabilityTargetPresetID) => {
    setSelectedPreset(presetId);
    setError(null);
    setTestResult(null);
    if (presetId === CUSTOM_AVAILABILITY_PRESET_ID) return;
    setForm((current) => applyAvailabilityTargetPreset(current, presetId));
  };

  onMount(async () => {
    const targetId = props.editingTargetId?.trim();
    if (!targetId) return;
    setLoading(true);
    setError(null);
    try {
      const targets = await AvailabilityTargetsAPI.list();
      const target = targets.find((item) => item.id === targetId);
      if (!target) {
        setError('The saved availability target could not be found.');
        return;
      }
      setForm(formFromTarget(target));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load availability target.');
    } finally {
      setLoading(false);
    }
  });

  const handleTest = async () => {
    setTesting(true);
    setError(null);
    setTestResult(null);
    try {
      const result = await AvailabilityTargetsAPI.test(payloadFromForm(form()));
      setTestResult(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Availability test failed.');
    } finally {
      setTesting(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setTestResult(null);
    const payload = payloadFromForm(form());
    try {
      const targetId = props.editingTargetId?.trim();
      if (targetId) {
        await AvailabilityTargetsAPI.update(targetId, payload);
      } else {
        await AvailabilityTargetsAPI.create(payload);
      }
      props.onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save availability target.');
    } finally {
      setSaving(false);
    }
  };

  const isBusy = () => loading() || saving() || testing() || Boolean(props.deletePending);
  const isEditing = () => Boolean(props.editingTargetId);

  return (
    <div class="flex min-h-full flex-col gap-6">
      <Show when={loading()}>
        <div class="rounded-md border border-border bg-surface-alt px-4 py-3 text-sm text-muted">
          Loading target…
        </div>
      </Show>

      <div class="grid gap-4 sm:grid-cols-2">
        <FormSelect
          label="Preset"
          value={selectedPreset()}
          onChange={(event) =>
            handlePresetChange(event.currentTarget.value as AvailabilityTargetPresetID)
          }
          fieldClass="sm:col-span-2"
        >
          <option value={CUSTOM_AVAILABILITY_PRESET_ID}>Custom endpoint</option>
          {AVAILABILITY_TARGET_PRESETS.map((preset) => (
            <option value={preset.id}>{preset.label}</option>
          ))}
        </FormSelect>
        <FormSelect
          label="Target type"
          value={form().targetKind}
          onChange={(event) =>
            updateForm({ targetKind: event.currentTarget.value as AvailabilityTargetKind })
          }
        >
          <option value="machine">Machine or server</option>
          <option value="service">Service endpoint</option>
          <option value="device">Device or controller</option>
        </FormSelect>
        <label class={formField}>
          <span class={formLabel}>Name</span>
          <input
            class={formControl}
            value={form().name}
            onInput={(event) => updateForm({ name: event.currentTarget.value })}
            placeholder={namePlaceholder()}
          />
        </label>
        <FormSelect
          label="Probe"
          value={form().protocol}
          onChange={(event) =>
            updateForm({ protocol: event.currentTarget.value as AvailabilityProbeProtocol })
          }
        >
          <option value="icmp">ICMP ping</option>
          <option value="tcp">TCP port</option>
          <option value="udp">UDP datagram</option>
          <option value="http">HTTP</option>
          <option value="https">HTTPS</option>
        </FormSelect>
        <label class={`${formField} sm:col-span-2`}>
          <span class={formLabel}>
            {form().protocol === 'http' || form().protocol === 'https' ? 'URL or host' : 'Address'}
          </span>
          <input
            class={formControl}
            value={form().address}
            onInput={(event) => updateForm({ address: event.currentTarget.value })}
            placeholder={addressPlaceholder()}
          />
          <span class={formHelpText}>
            {form().protocol === 'icmp'
              ? 'Use a hostname or IP address. Pulse will run one ping per poll.'
              : form().protocol === 'tcp'
                ? 'Use a hostname or IP address and the port to open.'
                : form().protocol === 'udp'
                  ? 'Use a hostname or unicast IP. UDP checks send one small datagram per poll.'
                  : 'Use a full URL or a hostname. HTTP statuses below 500 count as reachable.'}
          </span>
        </label>
        <FormSelect
          label="Link to resource (optional)"
          value={form().linkedResourceId}
          onChange={(event) => updateForm({ linkedResourceId: event.currentTarget.value })}
          fieldClass="sm:col-span-2"
          help="Link this check to a known resource so its status appears on that resource's row. Leave empty to attach only when its IP address or full hostname has one exact match."
        >
          <option value="">Attach on one exact address match (recommended)</option>
          <Show when={linkedResourceMissing()}>
            <option value={form().linkedResourceId}>
              {form().linkedResourceId} (not currently discovered)
            </option>
          </Show>
          <For each={groupedLinkableResources()}>
            {([platform, items]) => (
              <optgroup label={getSourcePlatformLabel(platform)}>
                <For each={items}>
                  {(resource) => {
                    const typeLabel = getResourceTypeLabel(resource.type);
                    return (
                      <option value={resource.id}>
                        {getPreferredInfrastructureDisplayName(resource)}
                        {typeLabel ? ` (${typeLabel})` : ''}
                      </option>
                    );
                  }}
                </For>
              </optgroup>
            )}
          </For>
        </FormSelect>
        <Show when={form().protocol !== 'icmp'}>
          <label class={formField}>
            <span class={formLabel}>Port</span>
            <input
              class={formControl}
              inputMode="numeric"
              value={form().port}
              onInput={(event) => updateForm({ port: event.currentTarget.value })}
              placeholder={portPlaceholder()}
            />
          </label>
        </Show>
        <Show when={form().protocol === 'http' || form().protocol === 'https'}>
          <label class={formField}>
            <span class={formLabel}>Path override</span>
            <input
              class={formControl}
              value={form().path}
              onInput={(event) => updateForm({ path: event.currentTarget.value })}
              placeholder="/health"
            />
          </label>
        </Show>
        <Show when={form().protocol === 'udp'}>
          <FormSelect
            label="UDP result policy"
            value={form().udpMode}
            onChange={(event) =>
              updateForm({ udpMode: event.currentTarget.value as AvailabilityUDPMode })
            }
            help="Response required is alert-safe. Open or filtered reports silence as indeterminate and only fails on an explicit port-unreachable response."
          >
            <option value="response_required">Require a response</option>
            <option value="open_or_filtered">Accept open or filtered</option>
          </FormSelect>
          <label class={formField}>
            <span class={formLabel}>Request payload</span>
            <input
              class={formControl}
              value={form().udpRequest}
              onInput={(event) => updateForm({ udpRequest: event.currentTarget.value })}
              placeholder={form().udpMode === 'response_required' ? 'PING' : 'Optional'}
            />
            <span class={formHelpText}>UTF-8 bytes, up to 512 bytes.</span>
          </label>
          <label class={`${formField} sm:col-span-2`}>
            <span class={formLabel}>Expected response (optional)</span>
            <input
              class={formControl}
              value={form().udpExpectedResponse}
              onInput={(event) => updateForm({ udpExpectedResponse: event.currentTarget.value })}
              placeholder="PONG"
            />
            <span class={formHelpText}>
              When set, the response must match these UTF-8 bytes exactly.
            </span>
          </label>
        </Show>
        <label class={formField}>
          <span class={formLabel}>Poll interval (seconds)</span>
          <input
            class={formControl}
            inputMode="numeric"
            value={form().pollIntervalSeconds}
            onInput={(event) => updateForm({ pollIntervalSeconds: event.currentTarget.value })}
            placeholder="60"
          />
        </label>
        <label class={formField}>
          <span class={formLabel}>Timeout (milliseconds)</span>
          <input
            class={formControl}
            inputMode="numeric"
            value={form().timeoutMillis}
            onInput={(event) => updateForm({ timeoutMillis: event.currentTarget.value })}
            placeholder="2000"
          />
        </label>
        <label class={formField}>
          <span class={formLabel}>Failure threshold</span>
          <input
            class={formControl}
            inputMode="numeric"
            value={form().failureThreshold}
            onInput={(event) => updateForm({ failureThreshold: event.currentTarget.value })}
            placeholder="2"
          />
          <span class={formHelpText}>
            Consecutive failures before the target is treated as down.
          </span>
        </label>
        <div class="flex items-center rounded-md border border-border bg-surface-alt px-4 py-3">
          <label class="flex items-center gap-3">
            <input
              type="checkbox"
              class={formCheckbox}
              checked={form().enabled}
              onChange={(event) => updateForm({ enabled: event.currentTarget.checked })}
            />
            <span class="text-sm text-base-content">Enable this availability target</span>
          </label>
        </div>
      </div>

      <Show when={testResult()}>
        {(result) => (
          <CalloutCard
            role={result().success ? 'status' : 'alert'}
            tone={
              result().outcome === 'indeterminate'
                ? 'warning'
                : result().success
                  ? 'success'
                  : 'danger'
            }
            scale="compact"
            padding="sm"
            description={
              result().outcome === 'indeterminate'
                ? `No UDP rejection was received in ${result().latencyMillis} ms; the port is open or filtered, not proven reachable.`
                : result().success
                  ? `Probe reached the target in ${result().latencyMillis} ms.`
                  : result().error || 'Probe failed.'
            }
          />
        )}
      </Show>

      <Show when={error()}>
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

      <Show when={props.deleteConfirming}>
        <div class="rounded-md border border-border bg-surface-alt px-4 py-3 text-xs text-muted">
          Click remove again to confirm. Historical resource data and alerts remain available.
        </div>
      </Show>

      <div class="sticky bottom-0 -mx-4 mt-auto border-t border-border bg-surface px-4 py-3 shadow-[0_-8px_16px_rgba(15,23,42,0.04)]">
        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex flex-col gap-2 sm:flex-row">
            <Button
              variant="outline"
              size="settingsAction"
              onClick={props.onCancel}
              disabled={isBusy()}
            >
              Cancel
            </Button>
            <Button
              variant="outline"
              size="settingsAction"
              onClick={handleTest}
              disabled={isBusy()}
            >
              {testing() ? 'Testing…' : 'Test probe'}
            </Button>
          </div>
          <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Show when={isEditing() && props.onToggleEnabled}>
              <Button
                variant="outline"
                size="settingsAction"
                onClick={props.onToggleEnabled}
                disabled={isBusy() || props.togglePending}
              >
                {props.togglePending
                  ? props.connectionEnabled
                    ? 'Pausing…'
                    : 'Resuming…'
                  : props.connectionEnabled
                    ? 'Pause target'
                    : 'Resume target'}
              </Button>
            </Show>
            <Show when={isEditing() && props.onDelete}>
              <Button
                variant={props.deleteConfirming ? 'danger' : 'dangerOutline'}
                size="settingsAction"
                onClick={props.onDelete}
                disabled={isBusy()}
              >
                {props.deletePending
                  ? 'Removing…'
                  : props.deleteConfirming
                    ? 'Click again to confirm'
                    : 'Remove target'}
              </Button>
            </Show>
            <Button
              variant="primary"
              size="settingsAction"
              onClick={handleSave}
              disabled={isBusy()}
            >
              {saving() ? 'Saving…' : isEditing() ? 'Save target' : addButtonLabel()}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
};
