import { For, Show, createEffect, createMemo, createSignal, onMount } from 'solid-js';

import {
  AlertIntentPoliciesAPI,
  type AlertIntentPolicyDocument,
  type AlertIntentPolicyPreview,
  type AlertIntentRule,
  type AlertIntentSignal,
} from '@/api/alertIntentPolicies';
import { Button } from '@/components/shared/Button';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { Card } from '@/components/shared/Card';
import {
  formCheckbox,
  formControl,
  formField,
  formHelpText,
  formLabel,
} from '@/components/shared/Form';
import { FormSelect } from '@/components/shared/FormSelect';
import type { Resource } from '@/types/resource';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';

const SIGNALS: ReadonlyArray<{ value: AlertIntentSignal; label: string }> = [
  { value: 'state.offline', label: 'Offline / powered off' },
  { value: 'incident.availability', label: 'Availability incident' },
  { value: 'metric.cpu', label: 'CPU threshold' },
  { value: 'metric.memory', label: 'Memory threshold' },
  { value: 'metric.disk', label: 'Disk threshold' },
];

const nonNegativeInt = (value: string, fallback = 0): number => {
  const parsed = Number.parseInt(value.trim(), 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
};

const detachedDocument = (document: AlertIntentPolicyDocument): AlertIntentPolicyDocument =>
  structuredClone(document);

const ruleFor = (
  document: AlertIntentPolicyDocument | null,
  resourceId: string,
  signal: AlertIntentSignal,
): AlertIntentRule => document?.resources?.[resourceId]?.[signal] ?? {};

export function AlertIntentPolicyPanel(props: { resources: readonly Resource[] }) {
  const [expanded, setExpanded] = createSignal(false);
  const [document, setDocument] = createSignal<AlertIntentPolicyDocument | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [message, setMessage] = createSignal<string | null>(null);

  const [offlineGrace, setOfflineGrace] = createSignal('0');
  const [availabilityGrace, setAvailabilityGrace] = createSignal('0');
  const [honorOperatorState, setHonorOperatorState] = createSignal(false);
  const [backupAware, setBackupAware] = createSignal(false);
  const [backupPostGrace, setBackupPostGrace] = createSignal('60');
  const [backupMaxDeferral, setBackupMaxDeferral] = createSignal('3600');

  const sortedResources = createMemo(() =>
    [...props.resources]
      .filter((resource) => resource.id?.trim())
      .sort((a, b) =>
        getPreferredInfrastructureDisplayName(a).localeCompare(
          getPreferredInfrastructureDisplayName(b),
        ),
      ),
  );
  const [resourceId, setResourceId] = createSignal('');
  const [signal, setSignal] = createSignal<AlertIntentSignal>('state.offline');
  const [overrideGrace, setOverrideGrace] = createSignal('');
  const [overrideOperatorMode, setOverrideOperatorMode] = createSignal<
    'inherit' | 'honor' | 'ignore'
  >('inherit');
  const [overrideBackupMode, setOverrideBackupMode] = createSignal<
    'inherit' | 'enabled' | 'disabled'
  >('inherit');
  const [overrideBackupPostGrace, setOverrideBackupPostGrace] = createSignal('60');
  const [overrideBackupMaxDeferral, setOverrideBackupMaxDeferral] = createSignal('3600');
  const [previewBackupActive, setPreviewBackupActive] = createSignal(false);
  const [previewing, setPreviewing] = createSignal(false);
  const [preview, setPreview] = createSignal<AlertIntentPolicyPreview | null>(null);

  const selectedResource = createMemo(() =>
    sortedResources().find((resource) => resource.id === resourceId()),
  );

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const loaded = await AlertIntentPoliciesAPI.get();
      setDocument(loaded);
      const offline = loaded.defaults?.['state.offline'] ?? {};
      const availability = loaded.defaults?.['incident.availability'] ?? {};
      setOfflineGrace(String(offline.graceSeconds ?? 0));
      setAvailabilityGrace(String(availability.graceSeconds ?? 0));
      setHonorOperatorState(offline.honorOperatorState ?? false);
      setBackupAware(offline.backupOffline?.enabled ?? false);
      setBackupPostGrace(String(offline.backupOffline?.postGraceSeconds ?? 60));
      setBackupMaxDeferral(String(offline.backupOffline?.maxDeferralSeconds ?? 3600));
      if (!resourceId() && sortedResources().length > 0) {
        setResourceId(sortedResources()[0].id);
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to load alert intent policies.');
    } finally {
      setLoading(false);
    }
  };

  onMount(() => void load());

  createEffect(() => {
    const resources = sortedResources();
    const selected = resourceId();
    if (resources.length === 0) {
      if (selected) setResourceId('');
      return;
    }
    if (!resources.some((resource) => resource.id === selected)) {
      setResourceId(resources[0].id);
    }
  });

  createEffect(() => {
    const current = ruleFor(document(), resourceId(), signal());
    setOverrideGrace(current.graceSeconds === undefined ? '' : String(current.graceSeconds));
    setOverrideOperatorMode(
      current.honorOperatorState === undefined
        ? 'inherit'
        : current.honorOperatorState
          ? 'honor'
          : 'ignore',
    );
    setOverrideBackupMode(
      current.backupOffline === undefined
        ? 'inherit'
        : current.backupOffline.enabled
          ? 'enabled'
          : 'disabled',
    );
    setOverrideBackupPostGrace(String(current.backupOffline?.postGraceSeconds ?? 60));
    setOverrideBackupMaxDeferral(String(current.backupOffline?.maxDeferralSeconds ?? 3600));
    setPreview(null);
  });

  const persist = async (next: AlertIntentPolicyDocument, successMessage: string) => {
    setSaving(true);
    setError(null);
    setMessage(null);
    try {
      const updated = await AlertIntentPoliciesAPI.update(next);
      setDocument(updated);
      setMessage(successMessage);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to save alert intent policies.');
    } finally {
      setSaving(false);
    }
  };

  const saveDefaults = async () => {
    const current = document();
    if (!current) return;
    const next = detachedDocument(current);
    next.defaults ??= {};
    next.defaults['state.offline'] = {
      ...(next.defaults['state.offline'] ?? {}),
      graceSeconds: nonNegativeInt(offlineGrace()),
      honorOperatorState: honorOperatorState(),
      backupOffline: {
        enabled: backupAware(),
        postGraceSeconds: nonNegativeInt(backupPostGrace(), 60),
        maxDeferralSeconds: nonNegativeInt(backupMaxDeferral(), 3600),
      },
    };
    next.defaults['incident.availability'] = {
      ...(next.defaults['incident.availability'] ?? {}),
      graceSeconds: nonNegativeInt(availabilityGrace()),
      honorOperatorState: honorOperatorState(),
    };
    await persist(next, 'Default intent policy saved.');
  };

  const saveOverride = async () => {
    const current = document();
    const id = resourceId();
    if (!current || !id) return;
    const next = detachedDocument(current);
    next.resources ??= {};
    next.resources[id] ??= {};
    const rule: AlertIntentRule = {};
    if (overrideGrace().trim() !== '') {
      rule.graceSeconds = nonNegativeInt(overrideGrace());
    }
    if (overrideOperatorMode() !== 'inherit') {
      rule.honorOperatorState = overrideOperatorMode() === 'honor';
    }
    if (signal() === 'state.offline' && overrideBackupMode() !== 'inherit') {
      rule.backupOffline = {
        enabled: overrideBackupMode() === 'enabled',
        postGraceSeconds: nonNegativeInt(overrideBackupPostGrace(), 60),
        maxDeferralSeconds: nonNegativeInt(overrideBackupMaxDeferral(), 3600),
      };
    }
    if (Object.keys(rule).length === 0) {
      delete next.resources[id][signal()];
      if (Object.keys(next.resources[id]).length === 0) delete next.resources[id];
    } else {
      next.resources[id][signal()] = rule;
    }
    await persist(next, 'Resource intent override saved.');
  };

  const removeOverride = async () => {
    const current = document();
    const id = resourceId();
    if (!current || !id || !current.resources?.[id]?.[signal()]) return;
    const next = detachedDocument(current);
    delete next.resources?.[id]?.[signal()];
    if (next.resources?.[id] && Object.keys(next.resources[id]).length === 0) {
      delete next.resources[id];
    }
    await persist(next, 'Resource intent override removed.');
  };

  const runPreview = async () => {
    const resource = selectedResource();
    if (!resource) return;
    setPreviewing(true);
    setError(null);
    try {
      setPreview(
        await AlertIntentPoliciesAPI.preview({
          resourceId: resource.id,
          resourceType: resource.type,
          signal: signal(),
          conditionActive: true,
          firstMatchedAt: new Date().toISOString(),
          ...(signal() === 'state.offline'
            ? {
                backupActive: previewBackupActive(),
                backupObservedAt: new Date().toISOString(),
              }
            : {}),
        }),
      );
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Failed to preview alert intent policy.');
    } finally {
      setPreviewing(false);
    }
  };

  const hasSelectedOverride = createMemo(() =>
    Boolean(document()?.resources?.[resourceId()]?.[signal()]),
  );

  return (
    <Card padding="md" class="space-y-4">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 class="text-base font-semibold text-base-content">Alert intent & grace</h2>
          <p class="mt-1 text-sm text-muted">
            Delay expected transients, honor operator maintenance state, and extend guest offline
            grace while a backup is active.
          </p>
        </div>
        <Button variant="secondary" size="sm" onClick={() => setExpanded((value) => !value)}>
          {expanded() ? 'Hide policies' : 'Configure policies'}
        </Button>
      </div>

      <Show when={error()}>
        {(text) => <CalloutCard role="alert" tone="danger" scale="compact" description={text()} />}
      </Show>
      <Show when={message()}>
        {(text) => (
          <CalloutCard role="status" tone="success" scale="compact" description={text()} />
        )}
      </Show>

      <Show when={expanded()}>
        <Show
          when={!loading() && document()}
          fallback={<div class="text-sm text-muted">Loading intent policies…</div>}
        >
          <div class="grid gap-4 border-t border-border pt-4 sm:grid-cols-2 lg:grid-cols-3">
            <label class={formField}>
              <span class={formLabel}>Default offline grace (seconds)</span>
              <input
                class={formControl}
                inputMode="numeric"
                value={offlineGrace()}
                onInput={(event) => setOfflineGrace(event.currentTarget.value)}
              />
            </label>
            <label class={formField}>
              <span class={formLabel}>Default availability grace (seconds)</span>
              <input
                class={formControl}
                inputMode="numeric"
                value={availabilityGrace()}
                onInput={(event) => setAvailabilityGrace(event.currentTarget.value)}
              />
            </label>
            <label class="flex items-center gap-3 rounded-md border border-border px-3 py-2">
              <input
                type="checkbox"
                class={formCheckbox}
                checked={honorOperatorState()}
                onChange={(event) => setHonorOperatorState(event.currentTarget.checked)}
              />
              <span class="text-sm text-base-content">
                Honor maintenance and intentionally-offline state
              </span>
            </label>
            <label class="flex items-center gap-3 rounded-md border border-border px-3 py-2">
              <input
                type="checkbox"
                class={formCheckbox}
                checked={backupAware()}
                onChange={(event) => setBackupAware(event.currentTarget.checked)}
              />
              <span class="text-sm text-base-content">
                Extend offline grace during Proxmox backups
              </span>
            </label>
            <label class={formField}>
              <span class={formLabel}>Post-backup grace (seconds)</span>
              <input
                class={formControl}
                inputMode="numeric"
                disabled={!backupAware()}
                value={backupPostGrace()}
                onInput={(event) => setBackupPostGrace(event.currentTarget.value)}
              />
            </label>
            <label class={formField}>
              <span class={formLabel}>Maximum backup deferral (seconds)</span>
              <input
                class={formControl}
                inputMode="numeric"
                disabled={!backupAware()}
                value={backupMaxDeferral()}
                onInput={(event) => setBackupMaxDeferral(event.currentTarget.value)}
              />
              <span class={formHelpText}>
                Hard cap prevents a stale backup signal from hiding a real outage.
              </span>
            </label>
          </div>
          <div class="flex justify-end">
            <Button
              variant="primary"
              isLoading={saving()}
              disabled={saving()}
              onClick={() => void saveDefaults()}
            >
              Save defaults
            </Button>
          </div>

          <div class="space-y-4 border-t border-border pt-4">
            <div>
              <h3 class="text-sm font-semibold text-base-content">
                Per-resource override and preview
              </h3>
              <p class="mt-1 text-xs text-muted">
                Resource values override type and global policies field by field.
              </p>
            </div>
            <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <FormSelect
                label="Resource"
                value={resourceId()}
                onChange={(event) => setResourceId(event.currentTarget.value)}
                fieldClass="lg:col-span-2"
              >
                <For each={sortedResources()}>
                  {(resource) => (
                    <option value={resource.id}>
                      {getPreferredInfrastructureDisplayName(resource)} · {resource.type}
                    </option>
                  )}
                </For>
              </FormSelect>
              <FormSelect
                label="Signal"
                value={signal()}
                onChange={(event) => setSignal(event.currentTarget.value as AlertIntentSignal)}
              >
                <For each={SIGNALS}>
                  {(item) => <option value={item.value}>{item.label}</option>}
                </For>
              </FormSelect>
              <label class={formField}>
                <span class={formLabel}>Grace override (seconds)</span>
                <input
                  class={formControl}
                  inputMode="numeric"
                  value={overrideGrace()}
                  onInput={(event) => setOverrideGrace(event.currentTarget.value)}
                  placeholder="Inherit"
                />
                <span class={formHelpText}>Leave blank to inherit.</span>
              </label>
              <FormSelect
                label="Operator state override"
                value={overrideOperatorMode()}
                onChange={(event) =>
                  setOverrideOperatorMode(
                    event.currentTarget.value as 'inherit' | 'honor' | 'ignore',
                  )
                }
              >
                <option value="inherit">Inherit</option>
                <option value="honor">Honor operator state</option>
                <option value="ignore">Ignore operator state</option>
              </FormSelect>
              <Show when={signal() === 'state.offline'}>
                <FormSelect
                  label="Backup handling override"
                  value={overrideBackupMode()}
                  onChange={(event) =>
                    setOverrideBackupMode(
                      event.currentTarget.value as 'inherit' | 'enabled' | 'disabled',
                    )
                  }
                >
                  <option value="inherit">Inherit</option>
                  <option value="enabled">Extend during backups</option>
                  <option value="disabled">Do not extend</option>
                </FormSelect>
                <label class={formField}>
                  <span class={formLabel}>Post-backup grace</span>
                  <input
                    class={formControl}
                    inputMode="numeric"
                    disabled={overrideBackupMode() !== 'enabled'}
                    value={overrideBackupPostGrace()}
                    onInput={(event) => setOverrideBackupPostGrace(event.currentTarget.value)}
                  />
                </label>
                <label class={formField}>
                  <span class={formLabel}>Maximum deferral</span>
                  <input
                    class={formControl}
                    inputMode="numeric"
                    disabled={overrideBackupMode() !== 'enabled'}
                    value={overrideBackupMaxDeferral()}
                    onInput={(event) => setOverrideBackupMaxDeferral(event.currentTarget.value)}
                  />
                </label>
                <label class="flex items-center gap-3 rounded-md border border-border px-3 py-2">
                  <input
                    type="checkbox"
                    class={formCheckbox}
                    checked={previewBackupActive()}
                    onChange={(event) => setPreviewBackupActive(event.currentTarget.checked)}
                  />
                  <span class="text-sm text-base-content">Preview with backup active</span>
                </label>
              </Show>
            </div>
            <div class="flex flex-wrap justify-end gap-2">
              <Show when={hasSelectedOverride()}>
                <Button variant="danger" disabled={saving()} onClick={() => void removeOverride()}>
                  Remove override
                </Button>
              </Show>
              <Button
                variant="secondary"
                isLoading={previewing()}
                disabled={!resourceId() || previewing()}
                onClick={() => void runPreview()}
              >
                Preview current policy
              </Button>
              <Button
                variant="primary"
                isLoading={saving()}
                disabled={!resourceId() || saving()}
                onClick={() => void saveOverride()}
              >
                Save override
              </Button>
            </div>
            <Show when={preview()}>
              {(result) => (
                <CalloutCard
                  role="status"
                  tone={
                    result().status === 'would_activate'
                      ? 'danger'
                      : result().status === 'expected_transient'
                        ? 'info'
                        : 'warning'
                  }
                  scale="compact"
                  title={result().status.replace(/_/g, ' ')}
                  description={`${result().reason.replace(/_/g, ' ')} · ${result().effective.graceSeconds}s grace${result().remainingSeconds ? ` · ${result().remainingSeconds}s remaining` : ''}`}
                />
              )}
            </Show>
          </div>
        </Show>
      </Show>
    </Card>
  );
}
