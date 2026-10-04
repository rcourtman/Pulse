import { For, Show, createMemo, createSignal, createUniqueId, type Component } from 'solid-js';
import Link2Icon from 'lucide-solid/icons/link-2';
import XIcon from 'lucide-solid/icons/x';
import { GuestMetadataAPI } from '@/api/guestMetadata';
import { ActionIconButton, Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { FilterSegmentedControl } from '@/components/shared/FilterToolbar';
import { formControlDense } from '@/components/shared/Form';
import { StatusDot } from '@/components/shared/StatusDot';
import { TABLE_CARD_HEADER_CLEAR_BUTTON_CLASS } from '@/components/shared/TableCardHeader';
import { notificationStore } from '@/stores/notifications';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import type { WorkloadGuest } from '@/types/workloads';
import { dispatchResourceMetadataChanged } from '@/utils/resourceMetadataEvents';
import { getSimpleStatusIndicator } from '@/utils/status';
import {
  buildWorkloadWebLinkRows,
  filterWorkloadWebLinkRows,
  formatWorkloadWebLinkCount,
  getWorkloadWebLinkChanges,
  getWorkloadWebLinkPlaceholder,
  getWorkloadWebLinkValue,
  keepStableWorkloadWebLinkRows,
  validateWorkloadWebLinkChanges,
  type WorkloadGuestMetadataMap,
  type WorkloadWebLinkDrafts,
  type WorkloadWebLinkFilter,
  type WorkloadWebLinkRow,
} from './workloadWebLinksModel';

export interface WorkloadWebLinksActionProps {
  guests: () => WorkloadGuest[];
  guestMetadata: () => WorkloadGuestMetadataMap;
}

/**
 * Table-header entry point for setting many web links at once. Drafts live
 * here rather than in the dialog so closing it by accident keeps typed links.
 */
export const WorkloadWebLinksAction: Component<WorkloadWebLinksActionProps> = (props) => {
  const formId = `workload-web-links-${createUniqueId()}`;
  const [open, setOpen] = createSignal(false);
  const [filter, setFilter] = createSignal<WorkloadWebLinkFilter>('all');
  const [drafts, setDrafts] = createSignal<WorkloadWebLinkDrafts>({});
  const [errors, setErrors] = createSignal<Record<string, string>>({});
  const [saving, setSaving] = createSignal(false);
  let triggerRef: HTMLButtonElement | undefined;

  // Rows only track live inventory while the dialog is open, so a closed
  // editor adds no per-tick work to the workloads table.
  const rows = createMemo<WorkloadWebLinkRow[]>(
    (previous) =>
      open()
        ? keepStableWorkloadWebLinkRows(
            previous,
            buildWorkloadWebLinkRows(props.guests(), props.guestMetadata()),
          )
        : previous,
    [],
  );
  const visibleRows = createMemo(() => filterWorkloadWebLinkRows(rows(), filter()));
  const linkedCount = createMemo(() => rows().filter((row) => row.savedUrl).length);
  const changes = createMemo(() => getWorkloadWebLinkChanges(rows(), drafts()));

  const setDraft = (metadataId: string, value: string) => {
    setDrafts((current) => ({ ...current, [metadataId]: value }));
    if (errors()[metadataId]) {
      setErrors(({ [metadataId]: _cleared, ...rest }) => rest);
    }
  };

  const discardChanges = () => {
    setDrafts({});
    setErrors({});
  };

  const focusFirstError = (failed: Record<string, string>) => {
    const firstId = rows().find((row) => failed[row.metadataId])?.metadataId;
    if (!firstId) return;
    queueMicrotask(() =>
      Array.from(document.querySelectorAll<HTMLInputElement>(`#${formId} input`))
        .find((input) => input.dataset.metadataId === firstId)
        ?.focus(),
    );
  };

  const save = async () => {
    const pending = changes();
    if (pending.length === 0 || saving()) return;

    const invalid = validateWorkloadWebLinkChanges(pending);
    if (Object.keys(invalid).length > 0) {
      setErrors(invalid);
      setFilter('all');
      focusFirstError(invalid);
      return;
    }

    setSaving(true);
    const failed: Record<string, string> = {};
    let savedCount = 0;
    for (const change of pending) {
      try {
        await GuestMetadataAPI.updateMetadata(change.metadataId, { customUrl: change.url });
        dispatchResourceMetadataChanged({
          metadataKind: 'guest',
          metadataId: change.metadataId,
          customUrl: change.url,
        });
        setDrafts(({ [change.metadataId]: _saved, ...rest }) => rest);
        savedCount += 1;
      } catch (error) {
        failed[change.metadataId] =
          error instanceof Error && error.message ? error.message : 'Could not save this link.';
      }
    }
    setSaving(false);
    setErrors(failed);

    if (Object.keys(failed).length === 0) {
      notificationStore.success(`Saved ${formatWorkloadWebLinkCount(savedCount)}.`);
      setOpen(false);
      return;
    }
    focusFirstError(failed);
  };

  return (
    <Show when={!presentationPolicyIsReadOnly()}>
      <button
        ref={triggerRef}
        type="button"
        class={`${TABLE_CARD_HEADER_CLEAR_BUTTON_CLASS} min-h-6 gap-1`}
        aria-haspopup="dialog"
        onClick={() => setOpen(true)}
      >
        <Link2Icon class="h-3.5 w-3.5" aria-hidden="true" />
        Edit links
        <Show when={changes().length > 0}>
          <span class="text-amber-600 dark:text-amber-400">(unsaved)</span>
        </Show>
      </button>
      <Dialog
        isOpen={open()}
        onClose={() => setOpen(false)}
        layout="drawer-right"
        panelClass="max-w-full sm:max-w-2xl"
        ariaLabelledBy={`${formId}-title`}
        returnFocus={() => triggerRef}
      >
        <div class="flex h-full min-h-0 flex-col" data-testid="workload-web-links">
          <header class="flex shrink-0 items-start justify-between gap-3 border-b border-border bg-surface px-4 py-3">
            <div class="min-w-0">
              <h2 id={`${formId}-title`} class="text-base font-semibold text-base-content">
                Web links
              </h2>
              <p class="text-xs text-muted">
                Add the address you open in your browser for each guest. Pulse shows it as a link
                beside the guest.
              </p>
            </div>
            <ActionIconButton
              type="button"
              onClick={() => setOpen(false)}
              label="Close web links"
              title="Close"
              tone="muted"
              size="md"
            >
              <XIcon class="h-5 w-5" aria-hidden="true" />
            </ActionIconButton>
          </header>

          <div class="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-border px-4 py-2">
            <p class="text-xs text-muted" data-testid="workload-web-links-summary">
              {linkedCount()} of {rows().length} in this view have a link
            </p>
            <FilterSegmentedControl
              aria-label="Show guests"
              value={filter()}
              onChange={(value) => setFilter(value as WorkloadWebLinkFilter)}
              options={[
                { value: 'all', label: 'All' },
                { value: 'missing', label: 'Without a link' },
              ]}
            />
          </div>

          <form
            id={formId}
            noValidate
            class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4"
            onSubmit={(event) => {
              event.preventDefault();
              void save();
            }}
          >
            <Show
              when={visibleRows().length > 0}
              fallback={
                <p class="py-6 text-center text-sm text-muted">
                  {rows().length === 0
                    ? 'No guests in this view.'
                    : 'Every guest in this view has a link.'}
                </p>
              }
            >
              <ul class="divide-y divide-border">
                <For each={visibleRows()}>
                  {(row) => {
                    const inputId = `${formId}-${row.metadataId.replace(/[^a-zA-Z0-9_-]/g, '_')}`;
                    const errorId = `${inputId}-error`;
                    const status = () => getSimpleStatusIndicator(row.status);
                    const error = () => errors()[row.metadataId];
                    return (
                      <li class="flex flex-col gap-1.5 py-2.5 sm:flex-row sm:items-start sm:gap-3">
                        <div class="min-w-0 sm:w-52 sm:shrink-0 sm:pt-1.5">
                          <label
                            for={inputId}
                            class="flex items-center gap-1.5 text-sm font-medium text-base-content"
                          >
                            <StatusDot
                              variant={status().variant}
                              ariaLabel={status().label}
                              title={status().label}
                              size="xs"
                            />
                            <span class="break-all">{row.name}</span>
                          </label>
                          <Show when={row.detail}>
                            <p class="pl-3 text-[11px] text-muted">{row.detail}</p>
                          </Show>
                        </div>
                        <div class="min-w-0 flex-1">
                          <input
                            id={inputId}
                            data-metadata-id={row.metadataId}
                            type="url"
                            inputMode="url"
                            autocomplete="off"
                            spellcheck={false}
                            class={formControlDense}
                            placeholder={getWorkloadWebLinkPlaceholder(row)}
                            value={getWorkloadWebLinkValue(row, drafts())}
                            onInput={(event) => setDraft(row.metadataId, event.currentTarget.value)}
                            disabled={saving()}
                            aria-invalid={error() ? 'true' : undefined}
                            aria-describedby={error() ? errorId : undefined}
                          />
                          <Show when={error()}>
                            <p id={errorId} class="mt-1 text-xs text-red-600 dark:text-red-400">
                              {error()}
                            </p>
                          </Show>
                        </div>
                      </li>
                    );
                  }}
                </For>
              </ul>
            </Show>
          </form>

          <footer class="flex shrink-0 flex-wrap items-center justify-between gap-2 border-t border-border bg-surface px-4 py-3">
            <p class="text-xs text-muted" role="status" aria-live="polite">
              {changes().length === 0
                ? 'No unsaved changes'
                : `${changes().length} unsaved ${changes().length === 1 ? 'change' : 'changes'}`}
            </p>
            <div class="flex items-center gap-2">
              <Show when={changes().length > 0}>
                <Button variant="ghost" size="sm" onClick={discardChanges} disabled={saving()}>
                  Discard
                </Button>
              </Show>
              <Button
                type="submit"
                form={formId}
                variant="primary"
                size="sm"
                disabled={changes().length === 0}
                isLoading={saving()}
              >
                {changes().length > 1 ? `Save ${changes().length} links` : 'Save'}
              </Button>
            </div>
          </footer>
        </div>
      </Dialog>
    </Show>
  );
};

export default WorkloadWebLinksAction;
