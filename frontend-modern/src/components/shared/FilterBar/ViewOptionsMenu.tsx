import {
  type Component,
  type JSX,
  Show,
  createEffect,
  createSignal,
  createUniqueId,
  onCleanup,
} from 'solid-js';
import SlidersHorizontalIcon from 'lucide-solid/icons/sliders-horizontal';

import { FilterPopoverTrigger, FilterToolbarPanel } from '@/components/shared/FilterToolbar';

interface ViewOptionsMenuProps {
  children: JSX.Element;
  label?: string;
  description?: string;
}

export const ViewOptionsMenu: Component<ViewOptionsMenuProps> = (props) => {
  const [open, setOpen] = createSignal(false);
  const panelId = createUniqueId();
  let containerRef: HTMLDivElement | undefined;
  let triggerRef: HTMLButtonElement | undefined;

  const close = (restoreFocus = false) => {
    setOpen(false);
    if (restoreFocus) queueMicrotask(() => triggerRef?.focus());
  };

  createEffect(() => {
    if (!open()) return;

    const handlePointerDown = (event: MouseEvent) => {
      if (containerRef && !containerRef.contains(event.target as Node)) close();
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      close(true);
    };

    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    });
  });

  return (
    <div ref={containerRef} class="relative shrink-0">
      <FilterPopoverTrigger
        ref={triggerRef}
        open={open()}
        onClick={() => setOpen((value) => !value)}
        aria-haspopup="dialog"
        aria-expanded={open()}
        aria-controls={open() ? panelId : undefined}
        title="Change table presentation"
      >
        <SlidersHorizontalIcon class="h-3.5 w-3.5" aria-hidden="true" />
        View
      </FilterPopoverTrigger>

      <Show when={open()}>
        <FilterToolbarPanel
          id={panelId}
          role="dialog"
          aria-label={props.label ?? 'View preferences'}
          widthClass="w-80 max-w-[calc(100vw-2rem)]"
          class="left-0 right-auto top-[calc(100%+0.25rem)] z-50 max-h-[min(38rem,calc(100vh-8rem))] overflow-y-auto p-3 md:left-auto md:right-0"
        >
          <div class="mb-3 border-b border-border-subtle pb-2">
            <div class="text-xs font-medium text-base-content">
              {props.label ?? 'View preferences'}
            </div>
            <div class="mt-0.5 text-[11px] leading-4 text-muted">
              {props.description ?? 'These choices are remembered for future visits.'}
            </div>
          </div>
          <div class="space-y-3">{props.children}</div>
        </FilterToolbarPanel>
      </Show>
    </div>
  );
};
