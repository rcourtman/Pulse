import { For, Show, createEffect, createMemo, createSignal, onCleanup, onMount } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import {
  type AssistantSlashCommandAvailability,
  filterAssistantSlashCommands,
  getAssistantSlashCommandTokens,
  groupAssistantSlashCommands,
  type AssistantSlashCommand,
} from './assistantSlashCommands';
import { AssistantSlashCommandIcon } from './SlashCommandAutocomplete';
import {
  AI_CHAT_COMMAND_HELP_CLOSE_LABEL,
  AI_CHAT_COMMAND_HELP_EMPTY_STATE,
  AI_CHAT_COMMAND_HELP_SEARCH_LABEL,
  AI_CHAT_COMMAND_HELP_SEARCH_PLACEHOLDER,
  AI_CHAT_COMMAND_HELP_TITLE,
} from '@/utils/aiChatPresentation';

interface AssistantCommandHelpDialogProps {
  availability?: AssistantSlashCommandAvailability;
  onClose: () => void;
  onRunCommand: (command: AssistantSlashCommand) => void;
}

export function AssistantCommandHelpDialog(props: AssistantCommandHelpDialogProps) {
  const [commandSearchQuery, setCommandSearchQuery] = createSignal('');
  const [selectedCommandIndex, setSelectedCommandIndex] = createSignal(0);
  let searchInputRef: HTMLInputElement | undefined;

  const commands = createMemo(() =>
    filterAssistantSlashCommands(commandSearchQuery(), undefined, {
      availability: props.availability,
      includeDisabled: true,
    }),
  );
  const groupedCommands = createMemo(() => groupAssistantSlashCommands(commands()));
  const shouldGroupCommands = createMemo(() => !commandSearchQuery().trim());
  const selectedCommand = () => commands()[selectedCommandIndex()];

  const consumeDialogCloseKey = (event: KeyboardEvent) => {
    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation();
  };

  const moveSelectedCommand = (direction: -1 | 1) => {
    const count = commands().length;
    if (count === 0) {
      setSelectedCommandIndex(0);
      return;
    }

    setSelectedCommandIndex((index) => Math.max(0, Math.min(count - 1, index + direction)));
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    const controlOnly = event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey;

    switch (event.key) {
      case 'Escape':
        consumeDialogCloseKey(event);
        props.onClose();
        break;
      case 'ArrowDown':
        consumeDialogCloseKey(event);
        moveSelectedCommand(1);
        break;
      case 'ArrowUp':
        consumeDialogCloseKey(event);
        moveSelectedCommand(-1);
        break;
      case 'Home':
        consumeDialogCloseKey(event);
        setSelectedCommandIndex(0);
        break;
      case 'End':
        consumeDialogCloseKey(event);
        setSelectedCommandIndex(Math.max(0, commands().length - 1));
        break;
      case 'Enter': {
        consumeDialogCloseKey(event);
        const command = selectedCommand();
        if (!command || command.disabled) return;
        props.onRunCommand(command);
        break;
      }
      default:
        if (controlOnly && event.key.toLowerCase() === 'u') {
          consumeDialogCloseKey(event);
          setCommandSearchQuery('');
        }
        break;
    }
  };

  createEffect(() => {
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
  });

  createEffect(() => {
    commandSearchQuery();
    setSelectedCommandIndex(0);
  });

  createEffect(() => {
    const count = commands().length;
    if (selectedCommandIndex() >= count) {
      setSelectedCommandIndex(Math.max(0, count - 1));
    }
  });

  onMount(() => {
    queueMicrotask(() => searchInputRef?.focus());
  });

  return (
    <div
      class="absolute inset-0 z-50 flex items-end bg-slate-950/20 p-3 sm:items-center sm:justify-center"
      onClick={props.onClose}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-label={AI_CHAT_COMMAND_HELP_TITLE}
        class="max-h-[min(34rem,calc(100%-1.5rem))] w-full max-w-[30rem] overflow-hidden rounded-md border border-border bg-surface shadow-xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div class="flex min-h-12 items-center justify-between gap-3 border-b border-border px-4 py-3">
          <h3 class="text-sm font-semibold text-base-content">{AI_CHAT_COMMAND_HELP_TITLE}</h3>
          <button
            type="button"
            onClick={props.onClose}
            class="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
            title={AI_CHAT_COMMAND_HELP_CLOSE_LABEL}
            aria-label={AI_CHAT_COMMAND_HELP_CLOSE_LABEL}
          >
            <XIcon class="h-4 w-4" aria-hidden="true" />
          </button>
        </div>
        <div class="border-b border-border px-3 py-2">
          <input
            ref={searchInputRef}
            value={commandSearchQuery()}
            onInput={(event) => setCommandSearchQuery(event.currentTarget.value)}
            type="search"
            aria-label={AI_CHAT_COMMAND_HELP_SEARCH_LABEL}
            placeholder={AI_CHAT_COMMAND_HELP_SEARCH_PLACEHOLDER}
            class="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm text-base-content placeholder:text-muted focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500/20"
          />
        </div>
        <div
          class="max-h-[25rem] overflow-y-auto p-2"
          role="listbox"
          aria-label={AI_CHAT_COMMAND_HELP_TITLE}
        >
          <Show
            when={commands().length > 0}
            fallback={
              <div class="px-3 py-6 text-center text-xs text-muted">
                {AI_CHAT_COMMAND_HELP_EMPTY_STATE}
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
                      const aliases = () => getAssistantSlashCommandTokens(command).slice(1);
                      return (
                        <button
                          type="button"
                          role="option"
                          aria-selected={item.index === selectedCommandIndex()}
                          aria-disabled={command.disabled ? 'true' : undefined}
                          disabled={command.disabled}
                          class={`flex w-full items-start gap-3 rounded-md px-3 py-2.5 text-left transition-colors focus:outline-none ${
                            command.disabled
                              ? 'cursor-not-allowed opacity-55'
                              : 'hover:bg-surface-hover focus:bg-surface-hover'
                          } ${item.index === selectedCommandIndex() ? 'bg-surface-hover' : ''}`}
                          onClick={() => {
                            if (command.disabled) return;
                            props.onRunCommand(command);
                          }}
                          onMouseEnter={() => setSelectedCommandIndex(item.index)}
                        >
                          <span class="mt-0.5 text-muted">
                            <AssistantSlashCommandIcon action={command.action} />
                          </span>
                          <span class="min-w-0 flex-1">
                            <span class="flex min-w-0 flex-wrap items-center gap-2">
                              <span class="font-mono text-xs font-semibold text-base-content">
                                /{command.name}
                              </span>
                              <Show when={aliases().length > 0}>
                                <span class="min-w-0 truncate text-[10px] text-muted">
                                  {aliases()
                                    .map((alias) => `/${alias}`)
                                    .join(', ')}
                                </span>
                              </Show>
                            </span>
                            <span class="mt-0.5 block text-xs leading-5 text-muted">
                              {command.description}
                            </span>
                            <Show when={command.disabled && command.disabledReason}>
                              <span class="mt-0.5 block text-[11px] leading-4 text-muted">
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
      </section>
    </div>
  );
}
