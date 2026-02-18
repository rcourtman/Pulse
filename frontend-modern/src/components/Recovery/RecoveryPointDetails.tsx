import type { Component } from 'solid-js';
import { For, Show, createMemo, createSignal } from 'solid-js';
import type { RecoveryExternalRef, RecoveryPoint } from '@/types/recovery';
import { formatAbsoluteTime, formatBytes, formatUptime } from '@/utils/format';

interface RecoveryPointDetailsProps {
  point: RecoveryPoint;
}

const detailNumber = (p: RecoveryPoint, key: string): number | null => {
  const value = p.details?.[key];
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
};

const formatDurationFromISO = (startedAt: string | null | undefined, completedAt: string | null | undefined): string => {
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

export const RecoveryPointDetails: Component<RecoveryPointDetailsProps> = (props) => {
  const point = () => props.point;

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

  const summaryPairs = createMemo(() => {
    const p = point();
    const pairs: { k: string; v: string }[] = [];

    pairs.push({ k: 'ID', v: p.id });
    pairs.push({ k: 'Provider', v: String(p.provider || 'n/a') });
    pairs.push({ k: 'Kind', v: String(p.kind || 'n/a') });
    pairs.push({ k: 'Mode', v: String(p.mode || 'n/a') });
    pairs.push({ k: 'Outcome', v: String(p.outcome || 'unknown') });
    pairs.push({ k: 'Duration', v: formatDurationFromISO(p.startedAt, p.completedAt) });
    pairs.push({ k: 'Size', v: typeof sizeBytes() === 'number' ? formatBytes(sizeBytes()!) : 'n/a' });

    if (p.verified != null) pairs.push({ k: 'Verified', v: p.verified ? 'true' : 'false' });
    if (p.encrypted != null) pairs.push({ k: 'Encrypted', v: p.encrypted ? 'true' : 'false' });
    if (p.immutable != null) pairs.push({ k: 'Immutable', v: p.immutable ? 'true' : 'false' });

    if (p.subjectResourceId) pairs.push({ k: 'Subject Resource', v: p.subjectResourceId });
    if (p.repositoryResourceId) pairs.push({ k: 'Repository Resource', v: p.repositoryResourceId });
    if (p.subjectRef) pairs.push({ k: 'Subject Ref', v: labelForRef(p.subjectRef) });
    if (p.repositoryRef) pairs.push({ k: 'Repository Ref', v: labelForRef(p.repositoryRef) });

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
      pairs.push({ k: `details.${k}`, v: String(v) });
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
          class="rounded-md border border-gray-200 bg-white px-2.5 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
        >
          <Show when={copied()} fallback="Copy JSON">
            Copied
          </Show>
        </button>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
        <For each={summaryPairs()}>
          {(pair) => (
            <div class="rounded border border-gray-200 bg-white/70 px-3 py-2 text-xs dark:border-gray-700 dark:bg-gray-900/30">
              <div class="text-[10px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">{pair.k}</div>
              <div class="mt-0.5 font-mono text-[11px] text-gray-800 dark:text-gray-200 break-all">{pair.v}</div>
            </div>
          )}
        </For>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-2 text-xs text-gray-500 dark:text-gray-400">
        <div class="rounded border border-gray-200 bg-white/70 px-3 py-2 dark:border-gray-700 dark:bg-gray-900/30">
          <div class="text-[10px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">Started</div>
          <div class="mt-0.5 font-mono text-[11px] text-gray-800 dark:text-gray-200">
            {startedMs() > 0 ? formatAbsoluteTime(startedMs()) : 'n/a'}
          </div>
        </div>
        <div class="rounded border border-gray-200 bg-white/70 px-3 py-2 dark:border-gray-700 dark:bg-gray-900/30">
          <div class="text-[10px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">Completed</div>
          <div class="mt-0.5 font-mono text-[11px] text-gray-800 dark:text-gray-200">
            {completedMs() > 0 ? formatAbsoluteTime(completedMs()) : 'n/a'}
          </div>
        </div>
      </div>

      <div class="rounded border border-gray-200 bg-white/70 p-3 dark:border-gray-700 dark:bg-gray-900/30">
        <div class="text-[10px] font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">Raw</div>
        <pre class="mt-2 overflow-auto text-[11px] leading-relaxed text-gray-800 dark:text-gray-200 font-mono">
{prettyJSON()}
        </pre>
      </div>
    </div>
  );
};

export default RecoveryPointDetails;

