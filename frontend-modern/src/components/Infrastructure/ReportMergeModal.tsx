import { Component, Show, For, createMemo, createSignal } from 'solid-js';
import { apiFetch } from '@/utils/apiClient';
import { Dialog } from '@/components/shared/Dialog';
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
      return 'Containers';
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
  const dialogTitleId = createMemo(
    () => `report-merge-title-${props.resourceId.replace(/[^a-zA-Z0-9_-]/g, '-')}`,
  );
  const dialogDescriptionId = createMemo(
    () => `report-merge-description-${props.resourceId.replace(/[^a-zA-Z0-9_-]/g, '-')}`,
  );

  const handleSubmit = async () => {
    if (isSubmitting()) return;
    setIsSubmitting(true);
    setError(null);
    try {
      const response = await apiFetch(
        `/api/resources/${encodeURIComponent(props.resourceId)}/report-merge`,
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

      showSuccess('Resource split applied');
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
    <Dialog
      isOpen={props.isOpen}
      onClose={props.onClose}
      panelClass="max-w-lg"
      ariaLabelledBy={dialogTitleId()}
      ariaDescribedBy={dialogDescriptionId()}
    >
      <div class="flex items-start justify-between border-b border-slate-200 px-5 py-4 dark:border-slate-700">
        <div>
          <h3
            id={dialogTitleId()}
            class="text-lg font-semibold text-base-content"
          >
            Split Merged Resource
          </h3>
          <p
            id={dialogDescriptionId()}
            class="mt-1 text-xs text-muted"
          >
            Use this when two systems were combined incorrectly. Pulse will keep them separate on future refreshes.
          </p>
        </div>
        <button
          type="button"
          onClick={props.onClose}
          class="rounded-md p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-700 dark:hover:text-slate-300"
          aria-label="Close"
        >
          <XIcon class="h-5 w-5" />
        </button>
      </div>

      <div class="flex-1 overflow-y-auto space-y-4 px-5 py-4 text-sm text-base-content">
        <div>
          <div class="text-xs font-medium uppercase tracking-wide text-muted">
            Resource
          </div>
          <div class="mt-1 font-medium">{props.resourceName}</div>
          <div class="mt-0.5 text-xs text-muted">{props.resourceId}</div>
        </div>

        <div>
          <div class="text-xs font-medium uppercase tracking-wide text-muted">
            Merged Sources
          </div>
          <div class="mt-2 flex flex-wrap gap-2">
            <For each={sourceLabels()}>
              {(label) => (
                <span class="rounded-full bg-blue-100 px-2.5 py-1 text-[11px] font-semibold text-blue-700 dark:bg-blue-900 dark:text-blue-200">
                  {label}
                </span>
              )}
            </For>
          </div>
        </div>

        <div>
          <label class="text-xs font-medium uppercase tracking-wide text-muted">
            Notes (optional)
          </label>
          <textarea
            value={notes()}
            onInput={(event) => setNotes(event.currentTarget.value)}
            rows={3}
            class="mt-2 w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
            placeholder="Example: Agent running on a different host with same hostname."
          />
        </div>

        <Show when={error()}>
          <div class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700 dark:border-red-900 dark:bg-red-900 dark:text-red-200">
            {error()}
          </div>
        </Show>
      </div>

      <div class="flex items-center justify-end gap-2 border-t border-slate-200 bg-slate-50 px-5 py-3 dark:border-slate-700 dark:bg-slate-800">
        <button
          type="button"
          onClick={props.onClose}
          class="rounded-md px-3 py-2 text-xs font-medium text-slate-600 transition-colors hover:text-slate-800 dark:text-slate-300 dark:hover:text-slate-100"
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
          {isSubmitting() ? 'Submitting...' : 'Split Resource'}
        </button>
      </div>
    </Dialog>
  );
};

export default ReportMergeModal;
