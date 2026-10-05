import { For, Show, type Component } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import { ActionIconButton, Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { FilterSegmentedControl } from '@/components/shared/FilterToolbar';
import { formControlDense } from '@/components/shared/Form';
import { StatusDot } from '@/components/shared/StatusDot';
import { getSimpleStatusIndicator } from '@/utils/status';
import {
  useWorkloadWebLinksState,
  type WorkloadWebLinksStateProps,
} from './useWorkloadWebLinksState';
import {
  getWorkloadWebLinkPlaceholder,
  getWorkloadWebLinkValue,
  type WorkloadWebLinkFilter,
} from './workloadWebLinksModel';

export interface WorkloadWebLinksDialogProps extends WorkloadWebLinksStateProps {
  returnFocus: () => HTMLElement | undefined;
}

/** Bulk web-link panel, loaded on first open by WorkloadWebLinksAction. */
export const WorkloadWebLinksDialog: Component<WorkloadWebLinksDialogProps> = (props) => {
  const state = useWorkloadWebLinksState(props);

  return (
    <Dialog
      isOpen
      onClose={state.close}
      layout="drawer-right"
      panelClass="max-w-full sm:max-w-2xl"
      ariaLabelledBy={`${state.formId}-title`}
      returnFocus={props.returnFocus}
    >
      <div class="flex h-full min-h-0 flex-col" data-testid="workload-web-links">
        <header class="flex shrink-0 items-start justify-between gap-3 border-b border-border bg-surface px-4 py-3">
          <div class="min-w-0">
            <h2 id={`${state.formId}-title`} class="text-base font-semibold text-base-content">
              Web links
            </h2>
            <p class="text-xs text-muted">
              Add the address you open in your browser for each guest. Pulse shows it as a link
              beside the guest.
            </p>
          </div>
          <ActionIconButton
            type="button"
            onClick={state.close}
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
            {state.linkedCount()} of {state.rows().length} in this view have a link
          </p>
          <FilterSegmentedControl
            aria-label="Show guests"
            value={state.filter()}
            onChange={(value) => state.setFilter(value as WorkloadWebLinkFilter)}
            options={[
              { value: 'all', label: 'All' },
              { value: 'missing', label: 'Without a link' },
            ]}
          />
        </div>

        <form
          id={state.formId}
          noValidate
          class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4"
          onSubmit={(event) => {
            event.preventDefault();
            void state.save();
          }}
        >
          <Show
            when={state.visibleRows().length > 0}
            fallback={
              <p class="py-6 text-center text-sm text-muted">
                {state.rows().length === 0
                  ? 'No guests in this view.'
                  : 'Every guest in this view has a link.'}
              </p>
            }
          >
            <ul class="divide-y divide-border">
              <For each={state.visibleRows()}>
                {(row) => {
                  const inputId = `${state.formId}-${row.metadataId.replace(/[^a-zA-Z0-9_-]/g, '_')}`;
                  const errorId = `${inputId}-error`;
                  const status = () => getSimpleStatusIndicator(row.status);
                  const error = () => state.errors()[row.metadataId];
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
                          value={getWorkloadWebLinkValue(row, state.drafts())}
                          onInput={(event) => state.setDraft(row, event.currentTarget.value)}
                          disabled={state.saving()}
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
            {state.changes().length === 0
              ? 'No unsaved changes'
              : `${state.changes().length} unsaved ${state.changes().length === 1 ? 'change' : 'changes'}`}
          </p>
          <div class="flex items-center gap-2">
            <Show when={state.changes().length > 0}>
              <Button
                variant="ghost"
                size="sm"
                onClick={state.discardChanges}
                disabled={state.saving()}
              >
                Discard
              </Button>
            </Show>
            <Button
              type="submit"
              form={state.formId}
              variant="primary"
              size="sm"
              disabled={state.changes().length === 0}
              isLoading={state.saving()}
            >
              {state.changes().length > 1 ? `Save ${state.changes().length} links` : 'Save'}
            </Button>
          </div>
        </footer>
      </div>
    </Dialog>
  );
};

export default WorkloadWebLinksDialog;
