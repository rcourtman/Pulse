import { For, Show, createEffect, createSignal, onCleanup, type Accessor } from 'solid-js';
import type { DashboardWidgetDef, DashboardWidgetId } from './dashboardWidgets';

type DashboardCustomizerProps = {
  allWidgets: Accessor<DashboardWidgetDef[]>;
  isHidden: (id: DashboardWidgetId) => boolean;
  toggleWidget: (id: DashboardWidgetId) => void;
  moveUp: (id: DashboardWidgetId) => void;
  moveDown: (id: DashboardWidgetId) => void;
  resetToDefaults: () => void;
  isDefault: Accessor<boolean>;
};

export function DashboardCustomizer(props: DashboardCustomizerProps) {
  const [open, setOpen] = createSignal(false);
  let rootRef: HTMLDivElement | undefined;

  const handleOutsideClick = (event: MouseEvent) => {
    if (!rootRef) return;
    if (!rootRef.contains(event.target as Node)) {
      setOpen(false);
    }
  };

  createEffect(() => {
    if (open()) {
      document.addEventListener('mousedown', handleOutsideClick);
    } else {
      document.removeEventListener('mousedown', handleOutsideClick);
    }
  });

  onCleanup(() => {
    document.removeEventListener('mousedown', handleOutsideClick);
  });

  return (
    <div class="relative shrink-0" ref={rootRef}>
      <button
        type="button"
        class={`inline-flex items-center gap-1.5 whitespace-nowrap px-2.5 py-1.5 text-xs font-medium rounded-md transition-all ${
          open()
            ? 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300'
            : 'bg-surface-hover text-muted hover:bg-slate-200 dark:hover:bg-slate-600'
        }`}
        title="Customize dashboard widgets"
        onClick={() => setOpen(!open())}
      >
        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M11.983 5.25a2.25 2.25 0 100-4.5 2.25 2.25 0 000 4.5zM11.983 23.25a2.25 2.25 0 100-4.5 2.25 2.25 0 000 4.5zM3.862 8.113a2.25 2.25 0 10-3.182-3.182 2.25 2.25 0 003.182 3.182zM23.286 19.355a2.25 2.25 0 10-3.182-3.182 2.25 2.25 0 003.182 3.182zM5.25 11.983a2.25 2.25 0 10-4.5 0 2.25 2.25 0 004.5 0zM23.25 11.983a2.25 2.25 0 10-4.5 0 2.25 2.25 0 004.5 0zM3.862 15.855a2.25 2.25 0 10-3.182 3.182 2.25 2.25 0 003.182-3.182zM23.286 4.611a2.25 2.25 0 10-3.182 3.182 2.25 2.25 0 003.182-3.182z"
          />
          <path stroke-linecap="round" stroke-linejoin="round" d="M11.983 6.75v10.5M6.75 11.983h10.5M16.868 7.098l-9.77 9.77M16.868 16.868l-9.77-9.77" />
        </svg>
        <span>Customize</span>
      </button>

      <Show when={open()}>
        <div class="absolute right-0 mt-1 w-64 rounded-md border border-slate-200 bg-white shadow-sm z-50 dark:border-slate-700 dark:bg-slate-800">
          <div class="px-3 py-2 border-b border-slate-100 dark:border-slate-700">
            <div class="flex items-center justify-between">
              <span class="text-xs font-medium text-base-content">Dashboard Widgets</span>
              <Show when={!props.isDefault()}>
                <button
                  type="button"
                  class="text-[10px] text-blue-600 dark:text-blue-400 hover:underline"
                  onClick={() => props.resetToDefaults()}
                >
                  Reset
                </button>
              </Show>
            </div>
          </div>

          <div class="max-h-64 overflow-y-auto py-1">
            <For each={props.allWidgets()}>
              {(widget, index) => {
                const visible = () => !props.isHidden(widget.id);
                const isFirst = () => index() === 0;
                const isLast = () => index() === props.allWidgets().length - 1;

                return (
                  <div class="flex items-center gap-2.5 px-3 py-2 hover:bg-surface-hover transition-colors">
                    <label class="flex min-w-0 flex-1 items-center gap-2.5 cursor-pointer">
                      <input
                        type="checkbox"
                        class="w-3.5 h-3.5 rounded border-slate-300 text-blue-600 focus:ring-blue-500 focus:ring-offset-0 dark:border-slate-600 dark:bg-slate-700 dark:checked:bg-blue-600"
                        checked={visible()}
                        onChange={() => props.toggleWidget(widget.id)}
                      />
                      <span
                        class={`truncate text-sm ${
                          visible() ? 'text-base-content' : 'text-muted'
                        }`}
                      >
                        {widget.label}
                      </span>
                    </label>

                    <div class="flex items-center gap-0.5">
                      <button
                        type="button"
                        class="rounded p-1 text-slate-500 transition-colors hover:bg-slate-200 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-600 dark:hover:text-slate-200 disabled:opacity-30 disabled:cursor-not-allowed"
                        title="Move up"
                        aria-label={`Move ${widget.label} up`}
                        disabled={isFirst()}
                        onClick={() => props.moveUp(widget.id)}
                      >
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                          <path stroke-linecap="round" stroke-linejoin="round" d="m18 15-6-6-6 6" />
                        </svg>
                      </button>
                      <button
                        type="button"
                        class="rounded p-1 text-slate-500 transition-colors hover:bg-slate-200 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-600 dark:hover:text-slate-200 disabled:opacity-30 disabled:cursor-not-allowed"
                        title="Move down"
                        aria-label={`Move ${widget.label} down`}
                        disabled={isLast()}
                        onClick={() => props.moveDown(widget.id)}
                      >
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                          <path stroke-linecap="round" stroke-linejoin="round" d="m6 9 6 6 6-6" />
                        </svg>
                      </button>
                    </div>
                  </div>
                );
              }}
            </For>
          </div>
        </div>
      </Show>
    </div>
  );
}
