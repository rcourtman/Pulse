import { Component, For, Show, createMemo, createSignal, onMount } from 'solid-js';
import { Button } from '@/components/shared/Button';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { FeatureGateSection } from '@/components/shared/FeatureGateSection';
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
  type AvailabilityHTTPAuthType,
  type AvailabilityHTTPMethod,
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
import {
  EXTERNAL_PROBE_FEATURE,
  LOCAL_PROBE_AGENT_LABEL,
  buildProbeAgentOptions,
  getExternalProbeGateBody,
  getExternalProbeGateTitle,
  getExternalProbeLockedHelpText,
  isExternalProbeLicenseError,
  isProbeAgentMissing,
} from '@/utils/availabilityProbeAgents';
import { hasFeature, loadRuntimeCapabilities, runtimeCapabilitiesLoaded } from '@/stores/license';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
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
  probeAgentId: string;
  enabled: boolean;
  pollIntervalSeconds: string;
  timeoutMillis: string;
  failureThreshold: string;
  monitorCertificate: boolean;
  certificateExpiryWarningDays: string;
  httpMethod: AvailabilityHTTPMethod;
  httpStatusMin: string;
  httpStatusMax: string;
  httpTextContains: string;
  httpJSONPath: string;
  httpJSONEquals: string;
  httpAuthType: AvailabilityHTTPAuthType;
  httpUsername: string;
  httpPassword: string;
  httpPasswordConfigured: boolean;
  httpBearerToken: string;
  httpBearerTokenConfigured: boolean;
  httpBody: string;
  httpBodyConfigured: boolean;
  httpBodyTouched: boolean;
  httpHeaders: AvailabilityHTTPHeaderForm[];
}

interface AvailabilityHTTPHeaderForm {
  id: string;
  name: string;
  value: string;
  valueConfigured: boolean;
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
  probeAgentId: '',
  enabled: true,
  pollIntervalSeconds: '60',
  timeoutMillis: '2000',
  failureThreshold: '2',
  monitorCertificate: true,
  certificateExpiryWarningDays: '30',
  httpMethod: 'GET',
  httpStatusMin: '200',
  httpStatusMax: '399',
  httpTextContains: '',
  httpJSONPath: '',
  httpJSONEquals: '',
  httpAuthType: 'none',
  httpUsername: '',
  httpPassword: '',
  httpPasswordConfigured: false,
  httpBearerToken: '',
  httpBearerTokenConfigured: false,
  httpBody: '',
  httpBodyConfigured: false,
  httpBodyTouched: false,
  httpHeaders: [],
});

const formFromTarget = (target: AvailabilityTarget): AvailabilityForm => {
  const headerSecrets = new Map(
    (target.httpSecrets?.headers ?? []).map((header) => [header.id, header.valueConfigured]),
  );
  return {
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
    probeAgentId: target.probeAgentId ?? '',
    enabled: target.enabled ?? true,
    pollIntervalSeconds: String(target.pollIntervalSeconds ?? 60),
    timeoutMillis: String(target.timeoutMillis ?? 2000),
    failureThreshold: String(target.failureThreshold ?? 2),
    monitorCertificate: target.certificateMonitoringDisabled !== true,
    certificateExpiryWarningDays: String(target.certificateExpiryWarningDays ?? 30),
    httpMethod: target.http?.method ?? 'GET',
    httpStatusMin: String(target.http?.expectedStatusMin ?? 200),
    httpStatusMax: String(target.http?.expectedStatusMax ?? 399),
    httpTextContains: target.http?.textContains ?? '',
    httpJSONPath: target.http?.jsonPath ?? '',
    httpJSONEquals: target.http?.jsonEquals ?? '',
    httpAuthType: target.http?.authentication.type ?? 'none',
    httpUsername: target.http?.authentication.username ?? '',
    httpPassword: '',
    httpPasswordConfigured: target.httpSecrets?.passwordConfigured ?? false,
    httpBearerToken: '',
    httpBearerTokenConfigured: target.httpSecrets?.bearerTokenConfigured ?? false,
    httpBody: '',
    httpBodyConfigured: target.httpSecrets?.bodyConfigured ?? false,
    httpBodyTouched: false,
    httpHeaders: (target.http?.headers ?? []).map((header) => ({
      id: header.id,
      name: header.name,
      value: '',
      valueConfigured: headerSecrets.get(header.id) ?? false,
    })),
  };
};

const parsePositiveInt = (value: string): number | undefined => {
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
};

const availabilityTestSuccessDescription = (result: AvailabilityTestResponse): string => {
  const application = result.application;
  const base =
    application?.outcome === 'passed'
      ? `Endpoint answered in ${result.latencyMillis} ms and the application contract passed${application.statusCode ? ` (HTTP ${application.statusCode})` : ''}.`
      : `Probe reached the target in ${result.latencyMillis} ms.`;
  const certificate = result.certificate;
  if (!certificate) return base;
  const trust = certificate.trustStatus.replace('-', ' ');
  const expiry = new Date(certificate.notAfter);
  if (!Number.isFinite(expiry.getTime())) {
    return `${base} Certificate: ${trust}.`;
  }
  return `${base} Certificate: ${trust}, expires ${expiry.toLocaleDateString(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })}.`;
};

const availabilityTestFailureDescription = (result: AvailabilityTestResponse): string => {
  if (result.transportOutcome === 'reachable' && result.application?.outcome === 'failed') {
    const status = result.application.statusCode ? ` HTTP ${result.application.statusCode}.` : '';
    return `Endpoint answered in ${result.latencyMillis} ms, but the application contract failed.${status} ${result.error || 'The response did not satisfy the configured assertion.'}`;
  }
  return result.error || 'Probe failed.';
};

const payloadFromForm = (form: AvailabilityForm): AvailabilityTarget => {
  const port = parsePositiveInt(form.port);
  const isHTTP = form.protocol === 'http' || form.protocol === 'https';
  const httpPassword = form.httpPassword ? form.httpPassword : undefined;
  const httpBearerToken = form.httpBearerToken ? form.httpBearerToken : undefined;
  const httpBody =
    form.httpMethod === 'POST' && (form.httpBodyTouched || form.httpBody)
      ? form.httpBody
      : undefined;
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
    // Always serialized, never `undefined`: the server decodes updates onto the
    // existing record, so an explicit empty string is what clears a probe
    // assignment and moves the check back to the local Pulse server.
    probeAgentId: form.probeAgentId.trim(),
    enabled: form.enabled,
    pollIntervalSeconds: parsePositiveInt(form.pollIntervalSeconds),
    timeoutMillis: parsePositiveInt(form.timeoutMillis),
    failureThreshold: parsePositiveInt(form.failureThreshold),
    certificateMonitoringDisabled: form.protocol === 'https' ? !form.monitorCertificate : false,
    certificateExpiryWarningDays:
      form.protocol === 'https' ? parsePositiveInt(form.certificateExpiryWarningDays) : 0,
    http: isHTTP
      ? {
          method: form.httpMethod,
          headers: form.httpHeaders.map((header) => ({
            id: header.id,
            name: header.name.trim(),
            value: header.value || undefined,
          })),
          authentication: {
            type: form.httpAuthType,
            username: form.httpAuthType === 'basic' ? form.httpUsername.trim() : undefined,
            password: form.httpAuthType === 'basic' ? httpPassword : undefined,
            bearerToken: form.httpAuthType === 'bearer' ? httpBearerToken : undefined,
          },
          body: httpBody,
          expectedStatusMin: parsePositiveInt(form.httpStatusMin) ?? 200,
          expectedStatusMax: parsePositiveInt(form.httpStatusMax) ?? 399,
          textContains:
            form.httpMethod !== 'HEAD' ? form.httpTextContains.trim() || undefined : undefined,
          jsonPath: form.httpMethod !== 'HEAD' ? form.httpJSONPath.trim() || undefined : undefined,
          jsonEquals:
            form.httpMethod !== 'HEAD' ? form.httpJSONEquals.trim() || undefined : undefined,
        }
      : undefined,
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
  let feedbackElement: HTMLDivElement | undefined;
  const revealFeedback = () => {
    if (typeof feedbackElement?.scrollIntoView === 'function') {
      feedbackElement.scrollIntoView({ block: 'nearest' });
    }
  };
  // Set when the server rejects a probe assignment with the canonical 402, so a
  // stale cached capability set still lands on the upgrade gate.
  const [probeLicenseRejected, setProbeLicenseRejected] = createSignal(false);

  const probeAgentOptions = createMemo(() => buildProbeAgentOptions(resources()));
  const probeAgentMissing = createMemo(() =>
    isProbeAgentMissing(probeAgentOptions(), form().probeAgentId),
  );
  const externalProbeLicensed = createMemo(
    () =>
      !probeLicenseRejected() && runtimeCapabilitiesLoaded() && hasFeature(EXTERNAL_PROBE_FEATURE),
  );
  const externalProbeLocked = createMemo(
    () => probeLicenseRejected() || (runtimeCapabilitiesLoaded() && !externalProbeLicensed()),
  );

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

  const updateHTTPHeader = (index: number, patch: Partial<AvailabilityHTTPHeaderForm>) => {
    updateForm({
      httpHeaders: form().httpHeaders.map((header, headerIndex) =>
        headerIndex === index ? { ...header, ...patch } : header,
      ),
    });
  };

  const addHTTPHeader = () => {
    const id =
      globalThis.crypto?.randomUUID?.() ??
      `http-header-${Date.now()}-${form().httpHeaders.length + 1}`;
    updateForm({
      httpHeaders: [...form().httpHeaders, { id, name: '', value: '', valueConfigured: false }],
    });
  };

  const removeHTTPHeader = (index: number) => {
    updateForm({
      httpHeaders: form().httpHeaders.filter((_, headerIndex) => headerIndex !== index),
    });
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
    void loadRuntimeCapabilities();
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
      queueMicrotask(revealFeedback);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Availability test failed.');
      queueMicrotask(revealFeedback);
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
      if (payload.probeAgentId && isExternalProbeLicenseError(err)) {
        setProbeLicenseRejected(true);
        setError(getExternalProbeGateBody());
        return;
      }
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
                  : 'Use a full URL or hostname, then define the accepted application response below.'}
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
        <div class="space-y-3 sm:col-span-2">
          <FormSelect
            label="Run from"
            value={form().probeAgentId}
            disabled={externalProbeLocked()}
            onChange={(event) => updateForm({ probeAgentId: event.currentTarget.value })}
            help={
              externalProbeLocked()
                ? getExternalProbeLockedHelpText()
                : 'Run this check from the Pulse server, or hand it to a connected Pulse Agent host so it is probed from that network.'
            }
          >
            <option value="">{LOCAL_PROBE_AGENT_LABEL}</option>
            <Show when={probeAgentMissing()}>
              <option value={form().probeAgentId}>
                {form().probeAgentId} (not currently connected)
              </option>
            </Show>
            <For each={probeAgentOptions()}>
              {(option) => <option value={option.id}>{option.label}</option>}
            </For>
          </FormSelect>
          <Show when={externalProbeLocked()}>
            <div class="rounded-md border border-border bg-surface-alt p-4">
              <FeatureGateSection
                title={getExternalProbeGateTitle()}
                body={getExternalProbeGateBody()}
                upgradeDestination={getUpgradeActionDestination(EXTERNAL_PROBE_FEATURE)}
                showUpgradePrompts={!presentationPolicyHidesUpgradePrompts()}
              />
            </div>
          </Show>
        </div>
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
        <Show when={form().protocol === 'http' || form().protocol === 'https'}>
          <section class="space-y-4 rounded-lg border border-border bg-surface-alt/40 p-4 sm:col-span-2">
            <div>
              <h3 class="text-sm font-semibold text-base-content">
                What proves this service is working?
              </h3>
              <p class="mt-1 text-xs text-muted">
                Pulse records whether the endpoint answered separately from whether this response
                contract passed. Response bodies and credentials are never stored as evidence.
              </p>
            </div>

            <div class="grid gap-4 sm:grid-cols-3">
              <FormSelect
                label="Request method"
                value={form().httpMethod}
                onChange={(event) =>
                  updateForm({ httpMethod: event.currentTarget.value as AvailabilityHTTPMethod })
                }
              >
                <option value="HEAD">HEAD</option>
                <option value="GET">GET</option>
                <option value="POST">POST</option>
              </FormSelect>
              <label class={formField}>
                <span class={formLabel}>Accepted status from</span>
                <input
                  class={formControl}
                  inputMode="numeric"
                  value={form().httpStatusMin}
                  onInput={(event) => updateForm({ httpStatusMin: event.currentTarget.value })}
                  placeholder="200"
                />
              </label>
              <label class={formField}>
                <span class={formLabel}>Accepted status to</span>
                <input
                  class={formControl}
                  inputMode="numeric"
                  value={form().httpStatusMax}
                  onInput={(event) => updateForm({ httpStatusMax: event.currentTarget.value })}
                  placeholder="399"
                />
              </label>
            </div>

            <Show when={form().httpMethod === 'POST'}>
              <label class={formField}>
                <span class={formLabel}>Request body (optional)</span>
                <textarea
                  class={`${formControl} min-h-24 font-mono text-xs`}
                  value={form().httpBody}
                  onInput={(event) =>
                    updateForm({
                      httpBody: event.currentTarget.value,
                      httpBodyConfigured: false,
                      httpBodyTouched: true,
                    })
                  }
                  placeholder={
                    form().httpBodyConfigured
                      ? 'Stored securely — leave blank to keep it'
                      : '{"operation":"health"}'
                  }
                />
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span class={formHelpText}>
                    Up to 8 KiB. Kept out of history and test output.
                  </span>
                  <Show when={form().httpBodyConfigured && !form().httpBodyTouched}>
                    <button
                      type="button"
                      class="text-xs font-medium text-error hover:underline"
                      onClick={() =>
                        updateForm({
                          httpBody: '',
                          httpBodyConfigured: false,
                          httpBodyTouched: true,
                        })
                      }
                    >
                      Remove stored body
                    </button>
                  </Show>
                </div>
              </label>
            </Show>

            <div class="grid gap-4 sm:grid-cols-2">
              <FormSelect
                label="Authentication"
                value={form().httpAuthType}
                onChange={(event) =>
                  updateForm({
                    httpAuthType: event.currentTarget.value as AvailabilityHTTPAuthType,
                  })
                }
              >
                <option value="none">None</option>
                <option value="basic">Basic authentication</option>
                <option value="bearer">Bearer token</option>
              </FormSelect>
              <Show when={form().httpAuthType === 'basic'}>
                <label class={formField}>
                  <span class={formLabel}>Username</span>
                  <input
                    class={formControl}
                    value={form().httpUsername}
                    autocomplete="username"
                    onInput={(event) => updateForm({ httpUsername: event.currentTarget.value })}
                  />
                </label>
                <label class={formField}>
                  <span class={formLabel}>Password</span>
                  <input
                    class={formControl}
                    type="password"
                    autocomplete="new-password"
                    value={form().httpPassword}
                    onInput={(event) =>
                      updateForm({
                        httpPassword: event.currentTarget.value,
                        httpPasswordConfigured: false,
                      })
                    }
                    placeholder={
                      form().httpPasswordConfigured
                        ? 'Stored securely — leave blank to keep it'
                        : 'Required'
                    }
                  />
                </label>
              </Show>
              <Show when={form().httpAuthType === 'bearer'}>
                <label class={formField}>
                  <span class={formLabel}>Bearer token</span>
                  <input
                    class={formControl}
                    type="password"
                    autocomplete="new-password"
                    value={form().httpBearerToken}
                    onInput={(event) =>
                      updateForm({
                        httpBearerToken: event.currentTarget.value,
                        httpBearerTokenConfigured: false,
                      })
                    }
                    placeholder={
                      form().httpBearerTokenConfigured
                        ? 'Stored securely — leave blank to keep it'
                        : 'Required'
                    }
                  />
                </label>
              </Show>
            </div>

            <div class="space-y-3">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <div class={formLabel}>Request headers</div>
                  <div class={formHelpText}>
                    Optional operator-defined headers. Values are write-only.
                  </div>
                </div>
                <Button variant="outline" size="settingsAction" onClick={addHTTPHeader}>
                  Add header
                </Button>
              </div>
              <For each={form().httpHeaders}>
                {(header, index) => (
                  <div class="grid gap-2 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
                    <label class={formField}>
                      <span class={formLabel}>Header name</span>
                      <input
                        class={formControl}
                        value={header.name}
                        onInput={(event) =>
                          updateHTTPHeader(index(), { name: event.currentTarget.value })
                        }
                        placeholder="X-Health-Check"
                      />
                    </label>
                    <label class={formField}>
                      <span class={formLabel}>Header value</span>
                      <input
                        class={formControl}
                        type="password"
                        value={header.value}
                        onInput={(event) =>
                          updateHTTPHeader(index(), {
                            value: event.currentTarget.value,
                            valueConfigured: false,
                          })
                        }
                        placeholder={
                          header.valueConfigured
                            ? 'Stored securely — leave blank to keep it'
                            : 'Value'
                        }
                      />
                    </label>
                    <Button
                      variant="dangerOutline"
                      size="settingsAction"
                      onClick={() => removeHTTPHeader(index())}
                    >
                      Remove
                    </Button>
                  </div>
                )}
              </For>
            </div>

            <Show when={form().httpMethod !== 'HEAD'}>
              <div class="grid gap-4 sm:grid-cols-2">
                <label class={formField}>
                  <span class={formLabel}>Response contains text (optional)</span>
                  <input
                    class={formControl}
                    value={form().httpTextContains}
                    onInput={(event) => updateForm({ httpTextContains: event.currentTarget.value })}
                    placeholder="healthy"
                  />
                </label>
                <div />
                <label class={formField}>
                  <span class={formLabel}>JSON path (optional)</span>
                  <input
                    class={formControl}
                    value={form().httpJSONPath}
                    onInput={(event) => updateForm({ httpJSONPath: event.currentTarget.value })}
                    placeholder="data.status"
                  />
                  <span class={formHelpText}>
                    Dot fields and bounded array indexes are supported.
                  </span>
                </label>
                <label class={formField}>
                  <span class={formLabel}>JSON value equals (optional)</span>
                  <input
                    class={formControl}
                    value={form().httpJSONEquals}
                    onInput={(event) => updateForm({ httpJSONEquals: event.currentTarget.value })}
                    placeholder="ok"
                  />
                </label>
              </div>
            </Show>
          </section>
        </Show>
        <Show when={form().protocol === 'https'}>
          <div class="flex items-center rounded-md border border-border bg-surface-alt px-4 py-3">
            <label class="flex items-center gap-3">
              <input
                type="checkbox"
                class={formCheckbox}
                checked={form().monitorCertificate}
                onChange={(event) =>
                  updateForm({ monitorCertificate: event.currentTarget.checked })
                }
              />
              <span class="text-sm text-base-content">Monitor TLS certificate validity</span>
            </label>
          </div>
          <label class={formField}>
            <span class={formLabel}>Expiry warning (days)</span>
            <input
              class={formControl}
              aria-label="Expiry warning (days)"
              inputMode="numeric"
              value={form().certificateExpiryWarningDays}
              disabled={!form().monitorCertificate}
              onInput={(event) =>
                updateForm({ certificateExpiryWarningDays: event.currentTarget.value })
              }
              placeholder="30"
            />
            <span class={formHelpText}>
              Raise an alert when the certificate enters this expiry window.
            </span>
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

      <div ref={feedbackElement} class="scroll-mb-56 space-y-2 sm:scroll-mb-24">
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
                  ? `No UDP rejection was received in ${result().latencyMillis} ms. The port is open or filtered, not proven reachable.`
                  : result().success
                    ? availabilityTestSuccessDescription(result())
                    : availabilityTestFailureDescription(result())
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
      </div>

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
