import { For, Show, createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import ActivityIcon from 'lucide-solid/icons/activity';
import ClockIcon from 'lucide-solid/icons/clock';
import CopyIcon from 'lucide-solid/icons/copy';
import DownloadIcon from 'lucide-solid/icons/download';
import FlaskConicalIcon from 'lucide-solid/icons/flask-conical';
import GitForkIcon from 'lucide-solid/icons/git-fork';
import KeyRoundIcon from 'lucide-solid/icons/key-round';
import Minimize2Icon from 'lucide-solid/icons/minimize-2';
import PlusIcon from 'lucide-solid/icons/plus';
import Redo2Icon from 'lucide-solid/icons/redo-2';
import SettingsIcon from 'lucide-solid/icons/settings';
import Undo2Icon from 'lucide-solid/icons/undo-2';
import CircleHelpIcon from 'lucide-solid/icons/circle-help';
import type { AssistantSlashCommand, AssistantSlashCommandAction } from './assistantSlashCommands';
import {
  type AssistantSlashCommandAvailability,
  filterAssistantSlashCommands,
  getAssistantSlashCommandTokens,
  groupAssistantSlashCommands,
} from './assistantSlashCommands';

interface SlashCommandAutocompleteProps {
  availability?: AssistantSlashCommandAvailability;
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
    case 'queue':
      return <ClockIcon class="h-4 w-4" aria-hidden="true" />;
    case 'compact':
      return <Minimize2Icon class="h-4 w-4" aria-hidden="true" />;
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
    case 'fixture':
      return <FlaskConicalIcon class="h-4 w-4" aria-hidden="true" />;
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
  const commands = createMemo(() =>
    filterAssistantSlashCommands(props.query, undefined, {
      availability: props.availability,
      includeDisabled: true,
    }),
  );
  const groupedCommands = createMemo(() => groupAssistantSlashCommands(commands()));
  const shouldGroupCommands = createMemo(() => !props.query.trim());
  const emptyMessage = createMemo(() => {
    const query = props.query.trim();
    return query ? `No Assistant commands match /${query}` : 'No Assistant commands available';
  });

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

  const moveSelection = (direction: -1 | 1, total: number) => {
    if (total <= 0) return;
    setSelectedIndex((index) => {
      const next = index + direction;
      if (next < 0) return total - 1;
      if (next >= total) return 0;
      return next;
    });
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    if (!props.visible) return;

    const options = commands();
    switch (event.key) {
      case 'ArrowDown':
        consumeCommandKey(event);
        moveSelection(1, options.length);
        break;
      case 'ArrowUp':
        consumeCommandKey(event);
        moveSelection(-1, options.length);
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
    <Show when={props.visible}>
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
          <Show
            when={commands().length > 0}
            fallback={
              <div class="px-3 py-3 text-xs text-muted" role="status">
                {emptyMessage()}
              </div>
            }
          >
            <For each={groupedCommands()}>
              {(group) => (
                <>
                  <Show when={shouldGroupCommands()}>
                    <div
                      role="presentation"
                      class="px-3 pb-1 pt-2 text-[10px] font-semibold uppercase text-muted"
                    >
                      {group.category}
                    </div>
                  </Show>
                  <For each={group.items}>
                    {(item) => {
                      const command = item.command;
                      return (
                        <button
                          type="button"
                          role="option"
                          aria-selected={item.index === selectedIndex()}
                          aria-disabled={command.disabled ? 'true' : undefined}
                          aria-label={`${
                            command.disabled ? 'Unavailable' : command.insertText ? 'Insert' : 'Run'
                          } /${command.name}: ${command.description}${
                            command.disabledReason ? `. ${command.disabledReason}` : ''
                          }`}
                          class={`flex w-full items-start gap-3 px-3 py-2.5 text-left transition-colors ${
                            command.disabled
                              ? 'cursor-not-allowed opacity-60'
                              : 'hover:bg-surface-hover'
                          } ${item.index === selectedIndex() ? 'bg-surface-hover' : ''}`}
                          onClick={(event) => {
                            event.stopPropagation();
                            selectCommand(command);
                          }}
                          onMouseEnter={() => setSelectedIndex(item.index)}
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
                            <Show when={command.disabledReason}>
                              <span class="mt-1 block truncate text-[11px] text-warning">
                                {command.disabledReason}
                              </span>
                            </Show>
                          </span>
                        </button>
                      );
                    }}
                  </For>
                </>
              )}
            </For>
          </Show>
        </div>
      </div>
    </Show>
  );
}
