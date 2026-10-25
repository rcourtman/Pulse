import { createEffect, createSignal, on, onCleanup, Show } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import CircleAlertIcon from 'lucide-solid/icons/circle-alert';
import CircleHelpIcon from 'lucide-solid/icons/circle-help';
import HistoryIcon from 'lucide-solid/icons/history';
import { Dialog } from '@/components/shared/Dialog';

// retained: a last-known value; uncertain: a figure that may include cache or
// has unknown freshness; error: a read the user can fix.
export type WorkloadReadingStatusKind = 'retained' | 'uncertain' | 'error';

const READING_STATUS_ICONS = {
  retained: HistoryIcon,
  uncertain: CircleHelpIcon,
  error: CircleAlertIcon,
} as const;

interface WorkloadReadingStatusProps {
  metric: 'Memory' | 'Disk';
  guestId: string;
  guestName: string;
  label: string;
  message: string;
  kind: WorkloadReadingStatusKind;
  // Compact (phone and narrow) metric cells are about 45px wide: the icon
  // shrinks and overhangs the cell's right padding so the bar keeps room for
  // its percentage.
  compact?: boolean;
  needsAction?: boolean;
}

// This is disclosure of already supplied evidence, not a refresh, diagnostic
// or recovery action. The cue is an icon on the bar's line, not a word under
// the bar: a second line made the row read as two. Its label stays in the
// accessible name and the dialog, its explanation in the title and for screen
// readers, and the 24px target overhangs the 12px icon instead of adding
// height or taking the bar's width.
export function WorkloadReadingStatus(props: WorkloadReadingStatusProps) {
  const [open, setOpen] = createSignal(false);
  let trigger: HTMLButtonElement | undefined;
  createEffect(
    on(
      () => props.guestId,
      () => setOpen(false),
      { defer: true },
    ),
  );
  onCleanup(() => {
    if (!open()) return;
    const rowAction = trigger?.closest('tr')?.querySelector<HTMLButtonElement>('[data-row-action]');
    // A new current observation withdraws this cue. After its button is
    // removed, return keyboard users to the same row's existing disclosure.
    queueMicrotask(() => {
      if (!trigger?.isConnected && rowAction?.isConnected) rowAction.focus({ preventScroll: true });
    });
  });

  return (
    <>
      <span
        data-workload-memory-read-status={props.metric === 'Memory' ? '' : undefined}
        data-workload-disk-read-status={props.metric === 'Disk' ? '' : undefined}
        class={`inline-flex shrink-0 items-center leading-none ${
          props.needsAction ? 'text-amber-700 dark:text-amber-300' : 'text-muted'
        }`}
        title={props.message}
      >
        <button
          ref={trigger}
          type="button"
          class={`${props.compact ? '-my-1.5 -ml-[7px] -mr-[11px]' : '-m-1.5'} inline-flex h-6 w-6 items-center justify-center rounded-sm focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 focus-visible:ring-offset-surface`}
          aria-label={`${props.metric} reading for ${props.guestName}: ${props.label}`}
          data-workload-reading-label={props.label}
          data-workload-reading-kind={props.kind}
          aria-haspopup="dialog"
          aria-expanded={open()}
          onClick={(event) => {
            event.stopPropagation();
            setOpen(true);
          }}
        >
          <Dynamic
            component={READING_STATUS_ICONS[props.kind]}
            class={props.compact ? 'h-2.5 w-2.5' : 'h-3 w-3'}
            aria-hidden="true"
          />
        </button>
        <span class="sr-only">{props.message}</span>
      </span>
      <Show when={open()}>
        <Dialog
          isOpen
          ariaLabel={`${props.metric} reading for ${props.guestName}`}
          onClose={() => setOpen(false)}
          returnFocus={() => trigger}
          panelClass="max-w-sm"
        >
          <div class="flex items-center justify-between gap-3 border-b border-border px-4 py-2">
            <h2 class="text-sm font-semibold text-base-content">{props.metric} reading</h2>
            <button
              type="button"
              aria-label="Close reading details"
              class="min-h-11 rounded-sm px-3 text-xs text-muted hover:bg-surface-hover focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500"
              onClick={() => setOpen(false)}
            >
              Close
            </button>
          </div>
          <div class="space-y-2 overflow-y-auto p-4 text-sm text-base-content">
            <p class="font-semibold wrap-break-word">{props.guestName}</p>
            <p class="whitespace-normal wrap-break-word leading-relaxed">{props.message}</p>
          </div>
        </Dialog>
      </Show>
    </>
  );
}
