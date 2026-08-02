import {
  Component,
  For,
  Show,
  createEffect,
  createSignal,
  createUniqueId,
  onCleanup,
} from 'solid-js';
import BookmarkIcon from 'lucide-solid/icons/bookmark';
import PlusIcon from 'lucide-solid/icons/plus';
import StarIcon from 'lucide-solid/icons/star';
import XIcon from 'lucide-solid/icons/x';
import { FilterPopoverTrigger, FilterToolbarPanel } from '@/components/shared/FilterToolbar';
import { useSavedViews, type SavedView } from './useSavedViews';

interface SavedViewsMenuProps {
  storageKey: string;
}

export const SavedViewsMenu: Component<SavedViewsMenuProps> = (props) => {
  const { views, saveCurrent, removeView, applyView, setDefault, clearDefault } = useSavedViews(
    props.storageKey,
  );
  const [open, setOpen] = createSignal(false);
  const [savePromptOpen, setSavePromptOpen] = createSignal(false);
  const [name, setName] = createSignal('');
  const panelId = createUniqueId();
  const nameInputId = createUniqueId();
  let containerRef: HTMLDivElement | undefined;
  let triggerRef: HTMLButtonElement | undefined;
  let saveActionRef: HTMLButtonElement | undefined;
  let nameInputRef: HTMLInputElement | undefined;

  const close = (restoreFocus = false) => {
    setOpen(false);
    setSavePromptOpen(false);
    setName('');
    if (restoreFocus) queueMicrotask(() => triggerRef?.focus());
  };

  const closeSavePrompt = () => {
    setSavePromptOpen(false);
    setName('');
    queueMicrotask(() => saveActionRef?.focus());
  };

  const restoreViewActionFocus = (viewId: string, action: string) => {
    queueMicrotask(() => {
      const actions = containerRef?.querySelectorAll<HTMLElement>(`[data-view-action="${action}"]`);
      Array.from(actions ?? [])
        .find((element) => element.dataset.viewId === viewId)
        ?.focus();
    });
  };

  const handleClickOutside = (event: MouseEvent) => {
    if (containerRef && !containerRef.contains(event.target as Node)) {
      close();
    }
  };

  const handleEscape = (event: KeyboardEvent) => {
    if (event.key !== 'Escape') return;
    event.preventDefault();
    if (savePromptOpen()) {
      closeSavePrompt();
      return;
    }
    close(true);
  };

  createEffect(() => {
    if (!open()) return;
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    onCleanup(() => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    });
  });

  // Auto-focus the name input when entering save mode so the user can type
  // immediately after clicking "Save current view as...".
  createEffect(() => {
    if (savePromptOpen()) {
      queueMicrotask(() => nameInputRef?.focus());
    }
  });

  const submitSave = () => {
    const view = saveCurrent(name());
    if (view) close();
  };

  const handleApply = (view: SavedView) => {
    applyView(view);
    close(true);
  };

  return (
    <div ref={containerRef} class="relative shrink-0">
      <FilterPopoverTrigger
        ref={triggerRef}
        open={open()}
        onClick={() => (open() ? close() : setOpen(true))}
        aria-haspopup="dialog"
        aria-expanded={open()}
        aria-controls={open() ? panelId : undefined}
        aria-label="Saved views"
      >
        <BookmarkIcon class="h-3.5 w-3.5" aria-hidden="true" />
        Saved
        <Show when={views().length > 0}>
          <span class="ml-0.5 rounded-full bg-surface-hover px-1.5 py-px text-[10px] font-medium text-base-content">
            {views().length}
          </span>
        </Show>
      </FilterPopoverTrigger>

      <Show when={open()}>
        <FilterToolbarPanel
          id={panelId}
          role="dialog"
          aria-label="Saved views"
          widthClass="w-64 max-w-[calc(100vw-2rem)]"
          class="left-auto right-0 top-[calc(100%+0.25rem)] z-50 p-0"
        >
          <Show
            when={savePromptOpen()}
            fallback={
              <>
                <button
                  ref={saveActionRef}
                  type="button"
                  onClick={() => setSavePromptOpen(true)}
                  class="flex w-full items-center gap-1.5 border-b border-border-subtle px-3 py-1.5 text-left text-xs text-base-content hover:bg-surface-hover"
                >
                  <PlusIcon class="h-3 w-3" />
                  Save current view as...
                </button>
                <Show when={views().length === 0}>
                  <div class="px-3 py-3 text-xs text-muted">
                    No saved views yet. Set the filters you want, then click "Save current view
                    as..." to name and recall this slice later.
                  </div>
                </Show>
                <Show when={views().length > 0}>
                  <div class="max-h-64 overflow-y-auto py-1">
                    <For each={views()}>
                      {(view) => (
                        <div class="group flex items-center justify-between gap-1 hover:bg-surface-hover">
                          <button
                            type="button"
                            onClick={() => handleApply(view)}
                            data-view-id={view.id}
                            data-view-action="apply"
                            class="min-w-0 flex-1 truncate px-3 py-1.5 text-left text-xs text-base-content"
                            title={view.name}
                          >
                            {view.name}
                          </button>
                          <button
                            type="button"
                            onClick={(event) => {
                              event.stopPropagation();
                              if (view.isDefault === true) {
                                clearDefault();
                              } else {
                                setDefault(view.id);
                              }
                              restoreViewActionFocus(view.id, 'default');
                            }}
                            data-view-id={view.id}
                            data-view-action="default"
                            aria-label={
                              view.isDefault === true
                                ? `Unset "${view.name}" as default`
                                : `Set "${view.name}" as default`
                            }
                            title={
                              view.isDefault === true
                                ? 'Default view — applied when you land on this page'
                                : 'Set as default for this page'
                            }
                            class={
                              view.isDefault === true
                                ? 'rounded-full p-0.5 text-amber-500'
                                : 'rounded-full p-0.5 text-muted transition-colors hover:text-amber-500'
                            }
                          >
                            <StarIcon
                              class={view.isDefault === true ? 'h-3 w-3 fill-current' : 'h-3 w-3'}
                            />
                          </button>
                          <button
                            type="button"
                            onClick={(event) => {
                              event.stopPropagation();
                              removeView(view.id);
                              queueMicrotask(() => saveActionRef?.focus());
                            }}
                            data-view-id={view.id}
                            data-view-action="remove"
                            aria-label={`Remove view "${view.name}"`}
                            class="mr-2 rounded-full p-0.5 text-muted opacity-100 transition-opacity hover:bg-surface hover:text-base-content sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100"
                          >
                            <XIcon class="h-3 w-3" />
                          </button>
                        </div>
                      )}
                    </For>
                  </div>
                </Show>
              </>
            }
          >
            <div class="p-3">
              <label
                for={nameInputId}
                class="block text-[10px] font-semibold uppercase tracking-wide text-muted"
              >
                Name this view
              </label>
              <input
                ref={nameInputRef}
                id={nameInputId}
                type="text"
                value={name()}
                onInput={(event) => setName(event.currentTarget.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    submitSave();
                  }
                }}
                placeholder="Stopped containers, Stale backups..."
                class="mt-1 w-full rounded-md border border-border bg-surface px-2 py-1 text-xs text-base-content outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500"
              />
              <div class="mt-2 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={closeSavePrompt}
                  class="rounded-md px-2 py-1 text-xs text-muted hover:text-base-content"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={submitSave}
                  disabled={name().trim() === ''}
                  class="rounded-md bg-blue-500 px-2 py-1 text-xs font-medium text-white hover:bg-blue-600 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  Save
                </button>
              </div>
            </div>
          </Show>
        </FilterToolbarPanel>
      </Show>
    </div>
  );
};
