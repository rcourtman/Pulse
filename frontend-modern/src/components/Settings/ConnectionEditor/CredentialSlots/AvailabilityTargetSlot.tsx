import { Component, Show, createSignal, onMount } from 'solid-js';
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
} from '@/api/availabilityTargets';
import {
  AVAILABILITY_TARGET_PRESETS,
  CUSTOM_AVAILABILITY_PRESET_ID,
  applyAvailabilityTargetPreset,
  availabilityPresetById,
  type AvailabilityTargetPresetID,
} from '../availabilityTargetPresets';

const buttonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const primaryButtonClass =
  'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60';

interface AvailabilityForm {
  id: string;
  name: string;
  targetKind: AvailabilityTargetKind;
  address: string;
  protocol: AvailabilityProbeProtocol;
  port: string;
  path: string;
  enabled: boolean;
  pollIntervalSeconds: string;
  timeoutMillis: string;
  failureThreshold: string;
}

export interface AvailabilityTargetSlotProps {
  editingTargetId?: string | null;
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

const newAvailabilityForm = (): AvailabilityForm => ({
  id: '',
  name: '',
  targetKind: 'service',
  address: '',
  protocol: 'icmp',
  port: '',
  path: '',
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
    path: form.protocol === 'http' ? form.path.trim() : undefined,
    enabled: form.enabled,
    pollIntervalSeconds: parsePositiveInt(form.pollIntervalSeconds),
    timeoutMillis: parsePositiveInt(form.timeoutMillis),
    failureThreshold: parsePositiveInt(form.failureThreshold),
  };
};

const testToneClass = (result: AvailabilityTestResponse) =>
  result.success
    ? 'border-green-300 bg-green-50 text-green-800 dark:border-green-900 dark:bg-green-950 dark:text-green-200'
    : 'border-rose-300 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200';

const presetSensitiveFormKeys: ReadonlySet<keyof AvailabilityForm> = new Set([
  'path',
  'port',
  'protocol',
  'targetKind',
]);

export const AvailabilityTargetSlot: Component<AvailabilityTargetSlotProps> = (props) => {
  const [form, setForm] = createSignal<AvailabilityForm>(newAvailabilityForm());
  const [selectedPreset, setSelectedPreset] = createSignal<AvailabilityTargetPresetID>(
    CUSTOM_AVAILABILITY_PRESET_ID,
  );
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [testing, setTesting] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [testResult, setTestResult] = createSignal<AvailabilityTestResponse | null>(null);

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
    (form().protocol === 'http' ? 'http://device.local/status' : 'device.local');

  const portPlaceholder = () =>
    selectedPresetConfig()?.portPlaceholder ?? (form().protocol === 'http' ? 'Optional' : '1883');

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
            placeholder="energy-monitor"
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
          <option value="http">HTTP</option>
        </FormSelect>
        <label class={`${formField} sm:col-span-2`}>
          <span class={formLabel}>{form().protocol === 'http' ? 'URL or host' : 'Address'}</span>
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
                : 'Use a full URL or a hostname. HTTP statuses below 500 count as reachable.'}
          </span>
        </label>
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
        <Show when={form().protocol === 'http'}>
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
          <div class={`rounded-md border px-4 py-3 text-sm ${testToneClass(result())}`}>
            {result().success
              ? `Probe reached the target in ${result().latencyMillis} ms.`
              : result().error || 'Probe failed.'}
          </div>
        )}
      </Show>

      <Show when={error()}>
        {(message) => (
          <div
            role="alert"
            class="rounded-md border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
          >
            {message()}
          </div>
        )}
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

      <Show when={props.deleteConfirming}>
        <div class="rounded-md border border-border bg-surface-alt px-4 py-3 text-xs text-muted">
          Click remove again to confirm. Historical resource data and alerts remain available.
        </div>
      </Show>

      <div class="sticky bottom-0 -mx-4 mt-auto border-t border-border bg-surface px-4 py-3 shadow-[0_-8px_16px_rgba(15,23,42,0.04)]">
        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex flex-col gap-2 sm:flex-row">
            <button type="button" onClick={props.onCancel} class={buttonClass} disabled={isBusy()}>
              Cancel
            </button>
            <button type="button" onClick={handleTest} class={buttonClass} disabled={isBusy()}>
              {testing() ? 'Testing…' : 'Test probe'}
            </button>
          </div>
          <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Show when={isEditing() && props.onToggleEnabled}>
              <button
                type="button"
                onClick={props.onToggleEnabled}
                disabled={isBusy() || props.togglePending}
                class={buttonClass}
              >
                {props.togglePending
                  ? props.connectionEnabled
                    ? 'Pausing…'
                    : 'Resuming…'
                  : props.connectionEnabled
                    ? 'Pause target'
                    : 'Resume target'}
              </button>
            </Show>
            <Show when={isEditing() && props.onDelete}>
              <button
                type="button"
                onClick={props.onDelete}
                disabled={isBusy()}
                class={
                  props.deleteConfirming
                    ? 'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md bg-rose-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-60'
                    : 'inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-rose-300 px-3 py-2 text-sm font-medium text-rose-700 transition-colors hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-rose-900 dark:text-rose-300 dark:hover:bg-rose-950'
                }
              >
                {props.deletePending
                  ? 'Removing…'
                  : props.deleteConfirming
                    ? 'Click again to confirm'
                    : 'Remove target'}
              </button>
            </Show>
            <button
              type="button"
              onClick={handleSave}
              class={primaryButtonClass}
              disabled={isBusy()}
            >
              {saving() ? 'Saving…' : isEditing() ? 'Save target' : 'Add target'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
