import { Component, Show, For, createMemo, createSignal } from 'solid-js';
import { apiFetch } from '@/utils/apiClient';
import { showError, showSuccess } from '@/utils/toast';
import XIcon from 'lucide-solid/icons/x';

interface ReportMergeModalProps {
  isOpen: boolean;
  resourceId: string;
  resourceName: string;
  sources: string[];
  onClose: () => void;
  onReported?: () => void;
}

const formatSourceLabel = (source: string) => {
  const normalized = source.toLowerCase();
  switch (normalized) {
    case 'proxmox':
      return 'Proxmox';
    case 'agent':
      return 'Agent';
    case 'docker':
      return 'Docker';
    case 'pbs':
      return 'PBS';
    case 'pmg':
      return 'PMG';
    case 'kubernetes':
      return 'Kubernetes';
    default:
      return source;
  }
};

export const ReportMergeModal: Component<ReportMergeModalProps> = (props) => {
  const [notes, setNotes] = createSignal('');
  const [isSubmitting, setIsSubmitting] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);

  const sourceLabels = createMemo(() =>
    props.sources.map((source) => formatSourceLabel(source)),
  );

  const handleSubmit = async () => {
    if (isSubmitting()) return;
    setIsSubmitting(true);
    setError(null);
    try {
      const response = await apiFetch(
        `/api/v2/resources/${encodeURIComponent(props.resourceId)}/report-merge`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            sources: props.sources,
            notes: notes().trim() || undefined,
          }),
        },
      );

      if (!response.ok) {
        const message = await response.text().catch(() => '');
        throw new Error(message || 'Failed to report merge');
      }

      showSuccess('Thanks! This merge will be reviewed');
      props.onReported?.();
      props.onClose();
      setNotes('');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to report merge';
      setError(message);
      showError('Unable to report merge', message);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Show when={props.isOpen}>
      <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
        <div class="w-full max-w-lg overflow-hidden rounded-xl bg-white shadow-2xl dark:bg-gray-800">
          <div class="flex items-start justify-between border-b border-gray-200 px-5 py-4 dark:border-gray-700">
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                Report Incorrect Merge
              </h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                We&#39;ll split this resource into separate entries in the next refresh.
              </p>
            </div>
            <button
              type="button"
              onClick={props.onClose}
              class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
              aria-label="Close"
            >
              <XIcon class="h-5 w-5" />
            </button>
          </div>

          <div class="space-y-4 px-5 py-4 text-sm text-gray-700 dark:text-gray-200">
            <div>
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Resource
              </div>
              <div class="mt-1 font-medium">{props.resourceName}</div>
              <div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{props.resourceId}</div>
            </div>

            <div>
              <div class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Merged Sources
              </div>
              <div class="mt-2 flex flex-wrap gap-2">
                <For each={sourceLabels()}>
                  {(label) => (
                    <span class="rounded-full bg-blue-100 px-2.5 py-1 text-[11px] font-semibold text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">
                      {label}
                    </span>
                  )}
                </For>
              </div>
            </div>

            <div>
              <label class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                Notes (optional)
              </label>
              <textarea
                value={notes()}
                onInput={(event) => setNotes(event.currentTarget.value)}
                rows={3}
                class="mt-2 w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-700 dark:bg-gray-900/40 dark:text-gray-200"
                placeholder="Example: Agent running on a different host with same hostname."
              />
            </div>

            <Show when={error()}>
              <div class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700 dark:border-red-900/50 dark:bg-red-900/30 dark:text-red-200">
                {error()}
              </div>
            </Show>
          </div>

          <div class="flex items-center justify-end gap-2 border-t border-gray-200 bg-gray-50 px-5 py-3 dark:border-gray-700 dark:bg-gray-900/40">
            <button
              type="button"
              onClick={props.onClose}
              class="rounded-md px-3 py-2 text-xs font-medium text-gray-600 transition-colors hover:text-gray-800 dark:text-gray-300 dark:hover:text-gray-100"
              disabled={isSubmitting()}
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleSubmit}
              disabled={isSubmitting() || props.sources.length < 2}
              class="rounded-md bg-blue-600 px-3 py-2 text-xs font-semibold text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {isSubmitting() ? 'Submitting...' : 'Report Merge'}
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
};

export default ReportMergeModal;
