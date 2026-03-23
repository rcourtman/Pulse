import { For, Show } from 'solid-js';
import { Dialog } from '@/components/shared/Dialog';
import { SearchField } from '@/components/shared/SearchField';
import {
  type CommandPaletteModalCommand,
  type CommandPaletteModalProps,
  useCommandPaletteState,
} from './useCommandPaletteState';

export type { CommandPaletteModalProps } from './useCommandPaletteState';

export function CommandPaletteModal(props: CommandPaletteModalProps) {
  const commandPalette = useCommandPaletteState(props);

  const handleSelect = (command: CommandPaletteModalCommand) => {
    commandPalette.handleSelect(command);
  };

  return (
    <Dialog
      isOpen={props.isOpen}
      onClose={props.onClose}
      panelClass="max-w-xl"
      ariaLabel="Command palette"
    >
      <div class="border-b border-border px-5 py-4">
        <SearchField
          value={commandPalette.query()}
          onChange={commandPalette.setQuery}
          inputRef={commandPalette.setInputRef}
          onKeyDown={commandPalette.handleInputKeyDown}
          placeholder="Type a command or search..."
          class="w-full"
          inputClass="bg-base"
          clearOnFocusedEscape={false}
          shortcutHint="Cmd+K"
        />
      </div>

      <div class="max-h-[320px] overflow-y-auto px-3 py-3">
        <Show
          when={commandPalette.filteredCommands().length > 0}
          fallback={<div class="px-3 py-8 text-center text-sm text-muted">No matches found.</div>}
        >
          <For each={commandPalette.filteredCommands()}>
            {(command) => (
              <button
                type="button"
                class="flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm text-base-content hover:bg-surface-hover"
                onClick={() => handleSelect(command)}
              >
                <div>
                  <div class="font-medium">{command.label}</div>
                  <Show when={command.description}>
                    <div class="text-xs text-muted">{command.description}</div>
                  </Show>
                </div>
                <Show when={command.shortcut}>
                  <span class="rounded border border-border-subtle bg-base px-2 py-1 text-[10px] font-medium text-base-content">
                    {command.shortcut}
                  </span>
                </Show>
              </button>
            )}
          </For>
        </Show>
      </div>
    </Dialog>
  );
}

export default CommandPaletteModal;
