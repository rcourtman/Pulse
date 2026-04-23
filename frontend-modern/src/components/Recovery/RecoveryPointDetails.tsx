import type { Component } from 'solid-js';
import { For, Show, createMemo, createSignal } from 'solid-js';
import { useWebSocket } from '@/contexts/appRuntime';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import type { PBSDatastore } from '@/types/api';
import type { RecoveryExternalRef, RecoveryPoint } from '@/types/recovery';
import { formatAbsoluteTime, formatBytes, formatUptime } from '@/utils/format';
import { getRecoveryItemTypeLabel, getRecoveryPointItemTypeKey } from '@/utils/recoveryItemTypePresentation';
import { getRecoveryPointLocationEntries } from '@/utils/recoveryLocationPresentation';
import {
  getRecoveryOutcomeTextClass,
  normalizeRecoveryOutcome,
} from '@/utils/recoveryOutcomePresentation';
import {
  getRecoveryPointKindLabel,
  getRecoveryPointModeLabel,
  getRecoveryPointOutcomeLabel,
  getRecoveryPointRepositoryLabel,
} from '@/utils/recoveryRecordPresentation';
import { pbsInstanceFromResource } from '@/utils/resourceStateAdapters';
import {
  getSourcePlatformLabel,
  normalizeSourcePlatformQueryValue,
} from '@/utils/sourcePlatforms';
import { getRecoveryPointPlatform } from '@/utils/recoveryPlatformModel';

interface RecoveryPointDetailsProps {
  point: RecoveryPoint;
}

const detailNumber = (p: RecoveryPoint, key: string): number | null => {
  const value = p.details?.[key];
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
};

const detailString = (p: RecoveryPoint, key: string): string => {
  const value = p.details?.[key];
  return typeof value === 'string' ? value.trim() : '';
};

const COMMON_DETAIL_LABELS: Record<string, string> = {
  instance: 'Instance',
  vmid: 'VMID',
  node: 'Node',
  snapshotName: 'Snapshot',
  volid: 'Volume',
  datastore: 'Datastore',
  namespace: 'Namespace',
  storage: 'Storage',
  taskName: 'Task',
  phase: 'Phase',
  hostname: 'Hostname',
  dataset: 'Dataset',
  k8sClusterName: 'Kubernetes Cluster',
  storageLocation: 'Storage Location',
  veleroName: 'Velero Backup',
};

const formatDurationFromISO = (
  startedAt: string | null | undefined,
  completedAt: string | null | undefined,
): string => {
  const s = startedAt ? Date.parse(startedAt) : 0;
  const c = completedAt ? Date.parse(completedAt) : 0;
  if (!s || !c || c < s) return 'n/a';
  const seconds = Math.floor((c - s) / 1000);
  if (seconds < 60) return '<1m';
  return formatUptime(seconds, true);
};

const labelForRef = (ref: RecoveryExternalRef | null | undefined): string => {
  if (!ref) return 'n/a';
  const parts: string[] = [];
  if (ref.type) parts.push(ref.type);
  if (ref.namespace && ref.name) parts.push(`${ref.namespace}/${ref.name}`);
  else if (ref.name) parts.push(ref.name);
  if (ref.class) parts.push(`class=${ref.class}`);
  if (ref.uid) parts.push(`uid=${ref.uid}`);
  if (ref.id) parts.push(`id=${ref.id}`);
  return parts.length > 0 ? parts.join(' ') : 'n/a';
};

const computeDatastoreUsagePercent = (datastore: PBSDatastore): number => {
  if (Number.isFinite(datastore.total) && datastore.total > 0 && Number.isFinite(datastore.used)) {
    const calculated = (datastore.used / datastore.total) * 100;
    return Math.min(100, Math.max(0, calculated));
  }
  if (Number.isFinite(datastore.usage)) {
    return Math.min(100, Math.max(0, datastore.usage));
  }
  return 0;
};

const usageBarColorClass = (usagePercent: number): string => {
  if (usagePercent > 85) return 'bg-red-500';
  if (usagePercent >= 70) return 'bg-amber-500';
  return 'bg-emerald-500';
};

export const RecoveryPointDetails: Component<RecoveryPointDetailsProps> = (props) => {
  const { state } = useWebSocket();
  const point = () => props.point;
  const platformKey = createMemo(() =>
    normalizeSourcePlatformQueryValue(getRecoveryPointPlatform(point())),
  );
  const platformLabel = createMemo(() =>
    getSourcePlatformLabel(platformKey() || getRecoveryPointPlatform(point())),
  );
  const platformBadge = createMemo(() =>
    getSourcePlatformBadge(platformKey() || getRecoveryPointPlatform(point())),
  );
  const isPbsPlatform = createMemo(() => platformKey() === 'proxmox-pbs');

  const pbsComment = createMemo(() => {
    const value = point().details?.comment;
    return typeof value === 'string' ? value.trim() : '';
  });

  const pbsOwner = createMemo(() => {
    const value = point().details?.owner;
    return typeof value === 'string' ? value.trim() : '';
  });

  const pbsFiles = createMemo(() => {
    const value = point().details?.files;
    if (!Array.isArray(value)) return [];
    return value
      .map((file) => (typeof file === 'string' ? file.trim() : String(file ?? '').trim()))
      .filter(Boolean);
  });

  const matchedDatastore = createMemo<{ datastore: PBSDatastore; instanceName: string } | null>(
    () => {
      if (!isPbsPlatform() || typeof window === 'undefined') return null;

      const repositoryRef = point().repositoryRef;
      const instanceName =
        typeof repositoryRef?.namespace === 'string' ? repositoryRef.namespace.trim() : '';
      const datastoreName =
        typeof repositoryRef?.name === 'string' ? repositoryRef.name.trim() : '';
      if (!instanceName || !datastoreName) return null;

      const instances = (state.resources || [])
        .filter((resource) => resource.type === 'pbs')
        .map(pbsInstanceFromResource)
        .filter((instance): instance is NonNullable<typeof instance> => Boolean(instance));
      const instance = instances.find((item) => item.name === instanceName);
      if (!instance) return null;

      const datastore = (instance.datastores || []).find((item) => item.name === datastoreName);
      if (!datastore) return null;

      return { datastore, instanceName: instance.name };
    },
  );

  const hasPlatformDetails = createMemo(
    () =>
      isPbsPlatform() &&
      (pbsComment().length > 0 ||
        pbsOwner().length > 0 ||
        point().immutable === true ||
        pbsFiles().length > 0 ||
        point().verified != null ||
        matchedDatastore() != null),
  );

  const startedMs = createMemo(() => {
    const startedAt = point().startedAt;
    return startedAt ? Date.parse(startedAt) : 0;
  });

  const completedMs = createMemo(() => {
    const completedAt = point().completedAt;
    return completedAt ? Date.parse(completedAt) : 0;
  });

  const sizeBytes = createMemo(() => {
    const top = point().sizeBytes;
    if (typeof top === 'number' && Number.isFinite(top)) return top;
    return detailNumber(point(), 'sizeBytes');
  });
  const itemLabel = createMemo(() => {
    const displayLabel = String(point().display?.itemLabel || '').trim();
    if (displayLabel) return displayLabel;
    const refLabel = labelForRef(point().itemRef || point().subjectRef);
    if (refLabel !== 'n/a') return refLabel;
    return point().itemResourceId || point().id;
  });
  const targetLabel = createMemo(() => {
    const repositoryLabel = getRecoveryPointRepositoryLabel(point());
    if (repositoryLabel) return repositoryLabel;
    const refLabel = labelForRef(point().repositoryRef);
    return refLabel === 'n/a' ? 'No target recorded' : refLabel;
  });
  const normalizedOutcome = createMemo(() => normalizeRecoveryOutcome(point().outcome));
  const outcomeLabel = createMemo(() => getRecoveryPointOutcomeLabel(point().outcome));
  const readiness = createMemo(() => {
    const outcome = normalizedOutcome();
    if (outcome === 'failed') {
      return {
        label: 'Not restorable',
        detail: 'This event did not produce a usable recovery point.',
        className: 'text-rose-600 dark:text-rose-400',
      };
    }
    if (outcome === 'running') {
      return {
        label: 'In progress',
        detail: 'Recovery metadata is still being collected.',
        className: 'text-blue-600 dark:text-blue-400',
      };
    }
    if (point().verified === true) {
      return {
        label: 'Verified candidate',
        detail: 'Verification completed for this recovery point.',
        className: 'text-emerald-600 dark:text-emerald-400',
      };
    }
    if (point().verified === false) {
      return {
        label: 'Verification failed',
        detail: 'Treat this artifact as suspect until verification is rerun.',
        className: 'text-amber-600 dark:text-amber-400',
      };
    }
    if (outcome === 'success') {
      return {
        label: 'Available candidate',
        detail: 'The event succeeded, but no verification result was recorded.',
        className: 'text-emerald-600 dark:text-emerald-400',
      };
    }
    return {
      label: 'Review required',
      detail: 'The event needs an operator review before restore use.',
      className: 'text-amber-600 dark:text-amber-400',
    };
  });
  const failureDetail = createMemo(() => {
    const error = detailString(point(), 'error');
    if (error) return error;
    const status = detailString(point(), 'status');
    if (normalizedOutcome() === 'failed' && status) return status;
    return '';
  });
  const operatorSummaryPairs = createMemo(() => [
    {
      k: 'Outcome',
      v: outcomeLabel(),
      valueClass: getRecoveryOutcomeTextClass(normalizedOutcome()),
      detail: formatDurationFromISO(point().startedAt, point().completedAt),
    },
    {
      k: 'Artifact',
      v: `${getRecoveryPointKindLabel(point().kind)} / ${getRecoveryPointModeLabel(point().mode)}`,
      valueClass: 'text-base-content',
      detail: itemLabel(),
    },
    {
      k: 'Target',
      v: targetLabel(),
      valueClass: 'text-base-content',
      detail: platformLabel() || 'n/a',
    },
    {
      k: 'Restore readiness',
      v: readiness().label,
      valueClass: readiness().className,
      detail: readiness().detail,
    },
  ]);

  const summaryPairs = createMemo(() => {
    const p = point();
    const pairs: { k: string; v: string }[] = [];
    const itemType = getRecoveryItemTypeLabel(getRecoveryPointItemTypeKey(p));

    pairs.push({ k: 'ID', v: p.id });
    if (itemType && itemType !== 'Unknown') pairs.push({ k: 'Item Type', v: itemType });
    pairs.push({ k: 'Platform', v: platformLabel() || 'n/a' });
    for (const entry of getRecoveryPointLocationEntries(p)) {
      pairs.push({ k: entry.label, v: entry.value });
    }
    pairs.push({ k: 'Point Type', v: getRecoveryPointKindLabel(p.kind) });
    pairs.push({ k: 'Method', v: getRecoveryPointModeLabel(p.mode) });
    pairs.push({ k: 'Outcome', v: getRecoveryPointOutcomeLabel(p.outcome) });
    pairs.push({ k: 'Duration', v: formatDurationFromISO(p.startedAt, p.completedAt) });
    pairs.push({
      k: 'Size',
      v: typeof sizeBytes() === 'number' ? formatBytes(sizeBytes()!) : 'n/a',
    });

    if (p.verified != null) pairs.push({ k: 'Verified', v: p.verified ? 'Verified' : 'Not Verified' });
    if (p.encrypted != null) pairs.push({ k: 'Encrypted', v: p.encrypted ? 'Encrypted' : 'Not Encrypted' });
    if (p.itemResourceId) pairs.push({ k: 'Item Resource', v: p.itemResourceId });
    if (p.repositoryResourceId) pairs.push({ k: 'Target Resource', v: p.repositoryResourceId });
    if (p.itemRef || p.subjectRef) pairs.push({ k: 'Item Ref', v: labelForRef(p.itemRef || p.subjectRef) });
    if (p.repositoryRef) pairs.push({ k: 'Target Ref', v: labelForRef(p.repositoryRef) });

    const commonDetailKeys = [
      'instance',
      'vmid',
      'node',
      'snapshotName',
      'volid',
      'datastore',
      'namespace',
      'storage',
      'taskName',
      'phase',
      'hostname',
      'dataset',
      'k8sClusterName',
      'storageLocation',
      'veleroName',
    ];
    for (const k of commonDetailKeys) {
      const v = p.details?.[k];
      if (v == null) continue;
      pairs.push({ k: COMMON_DETAIL_LABELS[k] || k, v: String(v) });
    }

    return pairs;
  });

  const prettyJSON = createMemo(() => {
    const p = point();
    const payload = {
      ...p,
      details: p.details || undefined,
    };
    try {
      return JSON.stringify(payload, null, 2);
    } catch {
      return String(payload);
    }
  });

  const [copied, setCopied] = createSignal(false);
  const [showPbsFiles, setShowPbsFiles] = createSignal(false);
  const copyJSON = async () => {
    try {
      await navigator.clipboard.writeText(prettyJSON());
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1200);
    } catch {
      // Ignore clipboard failures (e.g. insecure context).
    }
  };

  return (
    <div class="space-y-3">
      <div class="flex justify-end">
        <button
          type="button"
          onClick={() => void copyJSON()}
          class="rounded-md border border-border bg-surface px-2.5 py-1 text-xs font-medium text-base-content hover:bg-surface-hover"
        >
          <Show when={copied()} fallback="Copy JSON">
            Copied
          </Show>
        </button>
      </div>

      <div class="rounded-md border border-border bg-surface-alt/40 p-3">
        <div class="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div>
            <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">
              Recovery Point Summary
            </div>
            <div class="mt-1 text-sm font-medium text-base-content break-words">
              {itemLabel()}
            </div>
          </div>
          <Show when={platformBadge()}>
            {(badge) => (
              <span class={badge().classes} title={badge().title}>
                {badge().label}
              </span>
            )}
          </Show>
        </div>
        <div class="grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-4">
          <For each={operatorSummaryPairs()}>
            {(pair) => (
              <div class="rounded border border-border bg-surface px-3 py-2 text-xs">
                <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">
                  {pair.k}
                </div>
                <div class={`mt-0.5 text-[13px] font-semibold ${pair.valueClass}`}>
                  {pair.v}
                </div>
                <div class="mt-1 text-[11px] leading-4 text-muted break-words">{pair.detail}</div>
              </div>
            )}
          </For>
        </div>
        <Show when={failureDetail()}>
          <div class="mt-3 rounded border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-xs text-rose-700 dark:text-rose-200">
            <div class="font-semibold">Failure detail</div>
            <div class="mt-1 break-words font-mono text-[11px]">{failureDetail()}</div>
          </div>
        </Show>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
        <For each={summaryPairs()}>
          {(pair) => (
            <div class="rounded border border-border bg-surface px-3 py-2 text-xs">
              <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">
                {pair.k}
              </div>
              <div class="mt-0.5 font-mono text-[11px] text-base-content break-all">{pair.v}</div>
            </div>
          )}
        </For>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-2 text-xs text-muted">
        <div class="rounded border border-border bg-surface px-3 py-2">
          <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">Started</div>
          <div class="mt-0.5 font-mono text-[11px] text-base-content">
            {startedMs() > 0 ? formatAbsoluteTime(startedMs()) : 'n/a'}
          </div>
        </div>
        <div class="rounded border border-border bg-surface px-3 py-2">
          <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">Completed</div>
          <div class="mt-0.5 font-mono text-[11px] text-base-content">
            {completedMs() > 0 ? formatAbsoluteTime(completedMs()) : 'n/a'}
          </div>
        </div>
      </div>

      <Show when={hasPlatformDetails()}>
        <div class="rounded border border-border bg-surface p-3">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">
                Platform Details
              </div>
              <div class="mt-1 text-xs text-muted">
                Platform-specific recovery metadata, verification state, and target health.
              </div>
            </div>
            <Show when={platformBadge()}>
              {(badge) => (
                <span class={badge().classes} title={badge().title}>
                  {badge().label}
                </span>
              )}
            </Show>
          </div>
          <div class="mt-2 space-y-2">
            <Show when={pbsComment()}>
              <div class="rounded border border-emerald-100 border-l-4 border-l-emerald-400 bg-emerald-50 px-3 py-2 text-xs italic leading-relaxed text-emerald-900 dark:border-emerald-900 dark:border-l-emerald-500 dark:bg-emerald-950/30 dark:text-emerald-100">
                {pbsComment()}
              </div>
            </Show>

            <Show when={pbsOwner()}>
              <div class="rounded border border-border bg-surface px-3 py-2 text-xs">
                <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">
                  Owner
                </div>
                <div class="mt-0.5 font-mono text-[11px] text-base-content break-all">
                  {pbsOwner()}
                </div>
              </div>
            </Show>

            <Show when={point().immutable === true}>
              <div>
                <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-medium text-emerald-700 dark:bg-emerald-900/50 dark:text-emerald-300">
                  <svg
                    class="h-3 w-3"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 3l7 4v5c0 5-3.5 7.5-7 9-3.5-1.5-7-4-7-9V7l7-4z"
                    />
                  </svg>
                  Protected
                </span>
              </div>
            </Show>

            <Show when={isPbsPlatform() && point().verified != null}>
              <div class="rounded border border-border bg-surface px-3 py-2 text-xs">
                <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">
                  Verification
                </div>
                <div class="mt-0.5 flex items-center gap-2">
                  {point().verified ? (
                    <span class="inline-flex items-center gap-1 text-green-600 dark:text-green-400">
                      <svg
                        class="h-3.5 w-3.5"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2.5"
                          d="M5 13l4 4L19 7"
                        />
                      </svg>
                      Verified
                    </span>
                  ) : (
                    <span class="text-amber-600 dark:text-amber-400">Failed</span>
                  )}
                  <Show when={typeof point().details?.verificationState === 'string'}>
                    <span class="font-mono text-[11px] text-muted">
                      ({String(point().details?.verificationState)})
                    </span>
                  </Show>
                </div>
                <Show
                  when={
                    typeof point().details?.verificationUpid === 'string' &&
                    String(point().details?.verificationUpid || '').length > 0
                  }
                >
                  <div class="mt-1 font-mono text-[10px] text-slate-400 break-all">
                    UPID: {String(point().details?.verificationUpid)}
                  </div>
                </Show>
              </div>
            </Show>

            <Show when={matchedDatastore()}>
              {(match) => {
                const datastore = match().datastore;
                const usagePercent = computeDatastoreUsagePercent(datastore);
                const status = String(datastore.status || '').trim();
                const dedupFactor = datastore.deduplicationFactor;
                return (
                  <div class="rounded border border-border px-3 py-2 text-xs">
                    <div class="flex items-center justify-between gap-2">
                      <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">
                        Target Health
                      </div>
                      <Show when={status.length > 0}>
                        <span class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium">
                          {status}
                        </span>
                      </Show>
                    </div>

                    <div class="mt-0.5 font-mono text-[11px] text-base-content break-all">
                      Datastore: {datastore.name}
                    </div>

                    <div class="mt-2">
                      <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">
                        Space Usage
                      </div>
                      <div class="mt-0.5 font-mono text-[11px] text-base-content">
                        {formatBytes(datastore.used, 2)} / {formatBytes(datastore.total, 2)} (
                        {Math.round(usagePercent)}%)
                      </div>
                      <div class="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-surface-hover">
                        <div
                          class={`h-full rounded-full transition-[width] ${usageBarColorClass(usagePercent)}`}
                          style={{ width: `${usagePercent}%` }}
                        />
                      </div>
                    </div>

                    <Show
                      when={
                        typeof dedupFactor === 'number' &&
                        Number.isFinite(dedupFactor) &&
                        dedupFactor > 0
                      }
                    >
                      <div class="mt-2">
                        <div class="text-[10px] font-semibold uppercase tracking-wide text-muted">
                          Dedup Factor
                        </div>
                        <div class="mt-0.5 font-mono text-[11px] text-base-content">
                          {Number(dedupFactor).toFixed(2)}x
                        </div>
                      </div>
                    </Show>
                  </div>
                );
              }}
            </Show>

            <Show when={pbsFiles().length > 0}>
              <div class="rounded border border-border">
                <button
                  type="button"
                  onClick={() => setShowPbsFiles((v) => !v)}
                  class="flex w-full items-center justify-between px-3 py-2 text-left text-xs hover:bg-surface-hover"
                >
                  <span class="text-[10px] font-semibold uppercase tracking-wide text-muted">
                    Files ({pbsFiles().length})
                  </span>
                  <svg
                    class={`h-3 w-3 transition-transform ${showPbsFiles() ? 'rotate-180' : ''}`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </button>
                <Show when={showPbsFiles()}>
                  <div class="border-t border-border px-3 py-2">
                    <ul class="space-y-1">
                      <For each={pbsFiles()}>
                        {(file) => (
                          <li class="font-mono text-[11px] break-all text-base-content">{file}</li>
                        )}
                      </For>
                    </ul>
                  </div>
                </Show>
              </div>
            </Show>
          </div>
        </div>
      </Show>

      <details class="rounded border border-border bg-surface">
        <summary class="cursor-pointer px-3 py-2 text-[10px] font-semibold uppercase tracking-wide text-muted hover:bg-surface-hover">
          Technical details
        </summary>
        <div class="border-t border-border px-3 py-2">
          <pre class="overflow-auto text-[11px] leading-relaxed text-base-content font-mono">
            {prettyJSON()}
          </pre>
        </div>
      </details>
    </div>
  );
};

export default RecoveryPointDetails;
