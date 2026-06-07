import { For, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import ClockIcon from 'lucide-solid/icons/clock';
import CopyIcon from 'lucide-solid/icons/copy';
import DownloadIcon from 'lucide-solid/icons/download';
import GitForkIcon from 'lucide-solid/icons/git-fork';
import KeyRoundIcon from 'lucide-solid/icons/key-round';
import PlusIcon from 'lucide-solid/icons/plus';
import Redo2Icon from 'lucide-solid/icons/redo-2';
import SettingsIcon from 'lucide-solid/icons/settings';
import Undo2Icon from 'lucide-solid/icons/undo-2';
import CircleHelpIcon from 'lucide-solid/icons/circle-help';
import type { AssistantSlashCommand, AssistantSlashCommandAction } from './assistantSlashCommands';
import {
  filterAssistantSlashCommands,
  getAssistantSlashCommandTokens,
} from './assistantSlashCommands';

interface SlashCommandAutocompleteProps {
  query: string;
  visible: boolean;
  position: { top: number; left: number };
  onClose: () => void;
  onSelect: (command: AssistantSlashCommand) => void;
}

export const AssistantSlashCommandIcon = (props: { action: AssistantSlashCommandAction }) => {
  switch (props.action) {
    case 'help':
      return <CircleHelpIcon class="h-4 w-4" aria-hidden="true" />;
    case 'new':
      return <PlusIcon class="h-4 w-4" aria-hidden="true" />;
    case 'sessions':
      return <ClockIcon class="h-4 w-4" aria-hidden="true" />;
    case 'models':
      return <SettingsIcon class="h-4 w-4" aria-hidden="true" />;
    case 'providers':
      return <KeyRoundIcon class="h-4 w-4" aria-hidden="true" />;
    case 'status':
      return <ActivityIcon class="h-4 w-4" aria-hidden="true" />;
    case 'copy':
      return <CopyIcon class="h-4 w-4" aria-hidden="true" />;
    case 'export':
      return <DownloadIcon class="h-4 w-4" aria-hidden="true" />;
    case 'fork':
      return <GitForkIcon class="h-4 w-4" aria-hidden="true" />;
    case 'undo':
      return <Undo2Icon class="h-4 w-4" aria-hidden="true" />;
    case 'redo':
      return <Redo2Icon class="h-4 w-4" aria-hidden="true" />;
  }
};

export function SlashCommandAutocomplete(props: SlashCommandAutocompleteProps) {
  const [selectedIndex, setSelectedIndex] = createSignal(0);
  const commands = createMemo(() => filterAssistantSlashCommands(props.query, 8));

  createEffect(() => {
    props.query;
    setSelectedIndex(0);
  });

  const selectCommand = (command?: AssistantSlashCommand) => {
    if (!command) return;
    props.onSelect(command);
  };

  const consumeCommandKey = (event: KeyboardEvent) => {
    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation();
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    if (!props.visible) return;

    const options = commands();
    switch (event.key) {
      case 'ArrowDown':
        consumeCommandKey(event);
        setSelectedIndex((index) => Math.min(index + 1, Math.max(0, options.length - 1)));
        break;
      case 'ArrowUp':
        consumeCommandKey(event);
        setSelectedIndex((index) => Math.max(index - 1, 0));
        break;
      case 'Enter':
        consumeCommandKey(event);
        selectCommand(options[selectedIndex()]);
        break;
      case 'Tab':
        consumeCommandKey(event);
        selectCommand(options[selectedIndex()]);
        break;
      case 'Escape':
        consumeCommandKey(event);
        props.onClose();
        break;
    }
  };

  createEffect(() => {
    if (!props.visible) return;
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
  });

  return (
    <Show when={props.visible && commands().length > 0}>
      <div
        class="absolute z-50 min-w-[280px] max-w-[420px] overflow-hidden rounded-md border border-border bg-surface shadow-sm"
        style={{
          bottom: `${props.position.top}px`,
          left: `${props.position.left}px`,
        }}
        data-slash-command-autocomplete
        onClick={(event) => event.stopPropagation()}
      >
        <div class="border-b border-border px-3 py-2 text-xs font-medium text-muted">Commands</div>
        <div class="max-h-[260px] overflow-y-auto" role="listbox" aria-label="Assistant commands">
          <For each={commands()}>
            {(command, index) => (
              <button
                type="button"
                role="option"
                aria-selected={index() === selectedIndex()}
                aria-label={`Run /${command.name}: ${command.description}`}
                class={`flex w-full items-start gap-3 px-3 py-2.5 text-left transition-colors hover:bg-surface-hover ${
                  index() === selectedIndex() ? 'bg-surface-hover' : ''
                }`}
                onClick={(event) => {
                  event.stopPropagation();
                  selectCommand(command);
                }}
                onMouseEnter={() => setSelectedIndex(index())}
              >
                <span class="mt-0.5 text-muted">
                  <AssistantSlashCommandIcon action={command.action} />
                </span>
                <span class="min-w-0 flex-1">
                  <span class="flex min-w-0 items-center gap-2">
                    <span class="font-mono text-xs font-semibold text-base-content">
                      /{command.name}
                    </span>
                    <Show when={command.aliases?.length}>
                      <span class="truncate text-[10px] text-muted">
                        {getAssistantSlashCommandTokens(command)
                          .slice(1)
                          .map((alias) => `/${alias}`)
                          .join(', ')}
                      </span>
                    </Show>
                  </span>
                  <span class="mt-0.5 block truncate text-xs text-muted">
                    {command.description}
                  </span>
                </span>
              </button>
            )}
          </For>
        </div>
        <div class="flex items-center gap-2 border-t border-border px-3 py-1.5 text-xs text-muted">
          <span class="rounded bg-surface-hover px-1.5 py-0.5 text-[10px]">up/down</span>
          navigate
          <span class="rounded bg-surface-hover px-1.5 py-0.5 text-[10px]">enter</span>
          run
          <span class="rounded bg-surface-hover px-1.5 py-0.5 text-[10px]">esc</span>
          close
        </div>
      </div>
    </Show>
  );
}
