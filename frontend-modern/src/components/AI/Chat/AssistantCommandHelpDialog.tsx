import { For, Show, createEffect, onCleanup } from 'solid-js';
import XIcon from 'lucide-solid/icons/x';
import {
  ASSISTANT_SLASH_COMMANDS,
  getAssistantSlashCommandTokens,
  type AssistantSlashCommand,
} from './assistantSlashCommands';
import { AssistantSlashCommandIcon } from './SlashCommandAutocomplete';
import {
  AI_CHAT_COMMAND_HELP_CLOSE_LABEL,
  AI_CHAT_COMMAND_HELP_TITLE,
} from '@/utils/aiChatPresentation';

interface AssistantCommandHelpDialogProps {
  onClose: () => void;
  onRunCommand: (command: AssistantSlashCommand) => void;
}

export function AssistantCommandHelpDialog(props: AssistantCommandHelpDialogProps) {
  const consumeDialogCloseKey = (event: KeyboardEvent) => {
    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation();
  };

  const handleKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'Escape') return;
    consumeDialogCloseKey(event);
    props.onClose();
  };

  createEffect(() => {
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
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
        <div class="max-h-[28rem] overflow-y-auto p-2" role="list">
          <For each={ASSISTANT_SLASH_COMMANDS}>
            {(command) => {
              const aliases = () => getAssistantSlashCommandTokens(command).slice(1);
              return (
                <button
                  type="button"
                  class="flex w-full items-start gap-3 rounded-md px-3 py-2.5 text-left transition-colors hover:bg-surface-hover focus:bg-surface-hover focus:outline-none"
                  onClick={() => props.onRunCommand(command)}
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
                  </span>
                </button>
              );
            }}
          </For>
        </div>
      </section>
    </div>
  );
}
