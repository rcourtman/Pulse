import { createEffect, createSignal, on, onCleanup, Show } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';

interface WorkloadReadingStatusProps {
  metric: 'Memory' | 'Disk' | 'Network';
  guestId: string;
  guestName: string;
  label: string;
  message: string;
  needsAction?: boolean;
}

// This is disclosure of already supplied evidence, not a refresh, diagnostic
// or recovery action. Keep the existing compact cue and its screen-reader
// explanation; the same explanation must also be readable without hovering.
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
      <p
        data-workload-memory-read-status={props.metric === 'Memory' ? '' : undefined}
        data-workload-disk-read-status={props.metric === 'Disk' ? '' : undefined}
        data-workload-network-read-status={props.metric === 'Network' ? '' : undefined}
        class={`mt-0.5 text-center text-[10px] leading-none ${
          props.needsAction ? 'text-amber-700 dark:text-amber-300' : 'text-muted'
        }`}
        title={props.message}
      >
        <button
          ref={trigger}
          type="button"
          class="inline-flex min-h-6 min-w-6 -my-1.5 items-center justify-center rounded-sm px-1 underline decoration-dotted underline-offset-2 sm:my-0 sm:min-h-0 focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 focus-visible:ring-offset-surface"
          aria-label={`${props.metric} reading for ${props.guestName}: ${props.label}`}
          aria-haspopup="dialog"
          aria-expanded={open()}
          onClick={(event) => {
            event.stopPropagation();
            setOpen(true);
          }}
        >
          <span aria-hidden="true">{props.label}</span>
        </button>
        <span class="sr-only">{props.message}</span>
      </p>
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
