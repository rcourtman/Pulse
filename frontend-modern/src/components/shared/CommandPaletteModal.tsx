import { Show, For, createMemo, createSignal, createEffect } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { Dialog } from '@/components/shared/Dialog';
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
  const kubernetesWorkloadsPath = buildWorkloadsPath({ type: 'k8s' });
  const lxcWorkloadsPath = buildWorkloadsPath({ type: 'lxc' });
  const storagePath = buildStoragePath();
  const recoveryPath = buildRecoveryPath();
  const classicProxmoxPath = buildInfrastructurePath({ source: 'proxmox' });
  const classicHostsPath = buildInfrastructurePath({ source: 'agent' });
  const classicDockerHostsPath = buildInfrastructurePath({ source: 'docker' });
  const classicServicesPath = buildInfrastructurePath({ source: 'pmg' });
  const classicContainersPath = buildWorkloadsPath({ type: 'docker' });
  const classicLxcPath = lxcWorkloadsPath;

  let inputRef: HTMLInputElement | undefined;

  const commands = createMemo<Command[]>(() => {
    const base: Command[] = [
      {
        id: 'nav-infrastructure',
        label: 'Go to Infrastructure',
        description: infrastructurePath,
        shortcut: 'g i',
        keywords: ['infra', 'hosts', 'nodes'],
        action: () => navigate(infrastructurePath),
      },
      {
        id: 'nav-workloads',
        label: 'Go to Workloads',
        description: workloadsPath,
        shortcut: 'g w',
        keywords: ['vm', 'lxc', 'docker', 'k8s', 'kubernetes', 'pods'],
        action: () => navigate(workloadsPath),
      },
      {
        id: 'nav-workloads-k8s',
        label: 'Go to Kubernetes Workloads',
        description: kubernetesWorkloadsPath,
        keywords: ['k8s', 'kubernetes', 'pods', 'deployments', 'clusters'],
        action: () => navigate(kubernetesWorkloadsPath),
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
      {
        id: 'nav-migration-guide',
        label: 'Open Migration Guide',
        description: '/migration-guide',
        keywords: ['migration', 'legacy', 'routes', 'navigation', 'moved'],
        action: () => navigate('/migration-guide'),
      },
    ];

    return [
      {
        id: 'nav-classic-proxmox',
        label: 'Go to Proxmox (v5 entry point)',
        description: classicProxmoxPath,
        shortcut: 'g p',
        keywords: ['proxmox', 'pve', 'legacy', 'v5', 'migration', 'entry'],
        action: () => navigate(classicProxmoxPath),
      },
      {
        id: 'nav-classic-hosts',
        label: 'Go to Hosts (v5 entry point)',
        description: classicHostsPath,
        shortcut: 'g h',
        keywords: ['hosts', 'agent', 'legacy', 'v5', 'migration', 'entry'],
        action: () => navigate(classicHostsPath),
      },
      {
        id: 'nav-classic-docker-hosts',
        label: 'Go to Container Hosts (v5 entry point)',
        description: classicDockerHostsPath,
        shortcut: 'g d',
        keywords: ['containers', 'docker', 'podman', 'hosts', 'legacy', 'v5', 'migration', 'entry'],
        action: () => navigate(classicDockerHostsPath),
      },
      {
        id: 'nav-classic-services',
        label: 'Go to Services (v5 entry point)',
        description: classicServicesPath,
        shortcut: 'g v',
        keywords: ['services', 'pmg', 'mail', 'legacy', 'v5', 'migration', 'entry'],
        action: () => navigate(classicServicesPath),
      },
      {
        id: 'nav-classic-containers',
        label: 'Go to Containers (v5 entry point)',
        description: classicContainersPath,
        shortcut: 'g c',
        keywords: ['containers', 'docker', 'podman', 'workloads', 'legacy', 'v5', 'migration', 'entry'],
        action: () => navigate(classicContainersPath),
      },
      {
        id: 'nav-classic-lxc',
        label: 'Go to LXC Containers (v5 entry point)',
        description: classicLxcPath,
        shortcut: 'g l',
        keywords: ['lxc', 'containers', 'proxmox', 'workloads', 'legacy', 'v5', 'migration', 'entry'],
        action: () => navigate(classicLxcPath),
      },
      {
        id: 'nav-classic-kubernetes',
        label: 'Go to Kubernetes (v5 entry point)',
        description: kubernetesWorkloadsPath,
        shortcut: 'g k',
        keywords: ['k8s', 'kubernetes', 'pods', 'legacy', 'v5', 'migration', 'entry'],
        action: () => navigate(kubernetesWorkloadsPath),
      },
      ...base,
    ];
  });

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
    <Dialog
      isOpen={props.isOpen}
      onClose={props.onClose}
      panelClass="max-w-xl"
      ariaLabel="Command palette"
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
    </Dialog>
  );
}

export default CommandPaletteModal;
