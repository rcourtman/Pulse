import { Show, For, createMemo, createSignal, createEffect } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { Dialog } from '@/components/shared/Dialog';
import { SearchField } from '@/components/shared/SearchField';
import {
  buildRecoveryPath,
  buildInfrastructurePath,
  buildStoragePath,
  buildWorkloadsPath,
} from '@/routing/resourceLinks';

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
  const infrastructurePath = buildInfrastructurePath();
  const workloadsPath = buildWorkloadsPath();
  const podWorkloadsPath = buildWorkloadsPath({ type: 'pod' });
  const storagePath = buildStoragePath();
  const recoveryPath = buildRecoveryPath();

  let inputRef: HTMLInputElement | undefined;

  const commands = createMemo<Command[]>(() => {
    const base: Command[] = [
      {
        id: 'nav-infrastructure',
        label: 'Go to Infrastructure',
        description: infrastructurePath,
        shortcut: 'g i',
        keywords: ['infra', 'agents', 'nodes', 'resources'],
        action: () => navigate(infrastructurePath),
      },
      {
        id: 'nav-workloads',
        label: 'Go to Workloads',
        description: workloadsPath,
        shortcut: 'g w',
        keywords: [
          'vm',
          'system-container',
          'app-container',
          'docker',
          'k8s',
          'kubernetes',
          'pods',
        ],
        action: () => navigate(workloadsPath),
      },
      {
        id: 'nav-workloads-pods',
        label: 'Go to Kubernetes Pods',
        description: podWorkloadsPath,
        keywords: ['k8s', 'kubernetes', 'pods', 'deployments', 'clusters'],
        action: () => navigate(podWorkloadsPath),
      },
      {
        id: 'nav-storage',
        label: 'Go to Storage',
        description: storagePath,
        shortcut: 'g s',
        keywords: ['ceph', 'pbs'],
        action: () => navigate(storagePath),
      },
      {
        id: 'nav-recovery',
        label: 'Go to Recovery',
        description: recoveryPath,
        shortcut: 'g b',
        keywords: ['recovery', 'backups', 'snapshots', 'replication', 'restore'],
        action: () => navigate(recoveryPath),
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
    ];
    return base;
  });

  const normalizedQuery = createMemo(() => query().toLowerCase().trim().replace(/\s+/g, ''));

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
    <Dialog
      isOpen={props.isOpen}
      onClose={props.onClose}
      panelClass="max-w-xl"
      ariaLabel="Command palette"
    >
      <div class="border-b border-border px-5 py-4">
        <SearchField
          value={query()}
          onChange={setQuery}
          inputRef={(el) => (inputRef = el)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              const first = filteredCommands()[0];
              if (first) {
                e.preventDefault();
                handleSelect(first);
              }
            }
          }}
          placeholder="Type a command or search..."
          class="w-full"
          inputClass="bg-base"
          clearOnFocusedEscape={false}
          shortcutHint="Cmd+K"
        />
      </div>

      <div class="max-h-[320px] overflow-y-auto px-3 py-3">
        <Show
          when={filteredCommands().length > 0}
          fallback={<div class="px-3 py-8 text-center text-sm text-muted">No matches found.</div>}
        >
          <For each={filteredCommands()}>
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
                  <span class="rounded bg-base px-2 py-1 text-[10px] font-medium text-base-content border border-border-subtle">
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
