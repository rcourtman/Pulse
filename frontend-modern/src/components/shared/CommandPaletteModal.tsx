import { Show, For, createMemo, createSignal, createEffect } from 'solid-js';
import { useNavigate } from '@solidjs/router';

interface CommandPaletteModalProps {
  isOpen: boolean;
  onClose: () => void;
}

type Command = {
  id: string;
  label: string;
  description?: string;
  shortcut?: string;
  keywords?: string[];
  action: () => void;
};

export function CommandPaletteModal(props: CommandPaletteModalProps) {
  const navigate = useNavigate();
  const [query, setQuery] = createSignal('');

  let inputRef: HTMLInputElement | undefined;

  const commands = createMemo<Command[]>(() => [
    {
      id: 'nav-infrastructure',
      label: 'Go to Infrastructure',
      description: '/infrastructure',
      shortcut: 'g i',
      keywords: ['infra', 'hosts', 'nodes'],
      action: () => navigate('/infrastructure'),
    },
    {
      id: 'nav-workloads',
      label: 'Go to Workloads',
      description: '/workloads',
      shortcut: 'g w',
      keywords: ['vm', 'lxc', 'docker'],
      action: () => navigate('/workloads'),
    },
    {
      id: 'nav-storage',
      label: 'Go to Storage',
      description: '/storage',
      shortcut: 'g s',
      keywords: ['ceph', 'pbs'],
      action: () => navigate('/storage'),
    },
    {
      id: 'nav-backups',
      label: 'Go to Backups',
      description: '/backups',
      shortcut: 'g b',
      keywords: ['replication'],
      action: () => navigate('/backups'),
    },
    {
      id: 'nav-alerts',
      label: 'Go to Alerts',
      description: '/alerts',
      shortcut: 'g a',
      keywords: ['alarms', 'notifications'],
      action: () => navigate('/alerts'),
    },
    {
      id: 'nav-settings',
      label: 'Go to Settings',
      description: '/settings',
      shortcut: 'g t',
      keywords: ['preferences', 'config'],
      action: () => navigate('/settings'),
    },
  ]);

  const normalizedQuery = createMemo(() =>
    query()
      .toLowerCase()
      .trim()
      .replace(/\s+/g, '')
  );

  const filteredCommands = createMemo(() => {
    const q = normalizedQuery();
    if (!q) return commands();
    return commands().filter((cmd) => {
      const haystack = [
        cmd.label,
        cmd.description ?? '',
        cmd.shortcut ?? '',
        ...(cmd.keywords ?? []),
      ]
        .join(' ')
        .toLowerCase()
        .replace(/\s+/g, '');
      return haystack.includes(q);
    });
  });

  const handleSelect = (command: Command) => {
    command.action();
    props.onClose();
  };

  createEffect(() => {
    if (props.isOpen) {
      setQuery('');
      queueMicrotask(() => inputRef?.focus());
    } else {
      setQuery('');
    }
  });

  return (
    <Show when={props.isOpen}>
      <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        onClick={props.onClose}
        role="dialog"
        aria-modal="true"
      >
        <div
          class="w-full max-w-xl rounded-lg bg-white shadow-xl dark:bg-gray-800"
          onClick={(e) => e.stopPropagation()}
        >
          <div class="border-b border-gray-200 px-5 py-4 dark:border-gray-700">
            <div class="flex items-center gap-2 rounded-md border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 shadow-sm focus-within:border-blue-500 focus-within:ring-2 focus-within:ring-blue-500/20 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-200 dark:focus-within:border-blue-400">
              <svg class="h-4 w-4 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <input
                ref={(el) => (inputRef = el)}
                type="text"
                value={query()}
                onInput={(e) => setQuery(e.currentTarget.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Escape') {
                    e.preventDefault();
                    props.onClose();
                    return;
                  }
                  if (e.key === 'Enter') {
                    const first = filteredCommands()[0];
                    if (first) {
                      e.preventDefault();
                      handleSelect(first);
                    }
                  }
                }}
                placeholder="Type a command or search..."
                class="w-full bg-transparent text-sm text-gray-800 placeholder-gray-400 focus:outline-none dark:text-gray-100 dark:placeholder-gray-500"
              />
              <span class="text-[11px] text-gray-400 dark:text-gray-500">Cmd+K</span>
            </div>
          </div>

          <div class="max-h-[320px] overflow-y-auto px-3 py-3">
            <Show
              when={filteredCommands().length > 0}
              fallback={
                <div class="px-3 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                  No matches found.
                </div>
              }
            >
              <For each={filteredCommands()}>
                {(command) => (
                  <button
                    type="button"
                    class="flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm text-gray-700 hover:bg-blue-50 dark:text-gray-200 dark:hover:bg-blue-900/30"
                    onClick={() => handleSelect(command)}
                  >
                    <div>
                      <div class="font-medium">{command.label}</div>
                      <Show when={command.description}>
                        <div class="text-xs text-gray-500 dark:text-gray-400">
                          {command.description}
                        </div>
                      </Show>
                    </div>
                    <Show when={command.shortcut}>
                      <span class="rounded bg-gray-100 px-2 py-1 text-[10px] font-medium text-gray-600 dark:bg-gray-700 dark:text-gray-200">
                        {command.shortcut}
                      </span>
                    </Show>
                  </button>
                )}
              </For>
            </Show>
          </div>
        </div>
      </div>
    </Show>
  );
}

export default CommandPaletteModal;
