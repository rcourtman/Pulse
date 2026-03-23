export interface CommandPaletteModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export type CommandPaletteModalCommand = {
  id: string;
  label: string;
  description?: string;
  shortcut?: string;
  keywords?: string[];
  action: () => void;
};

export type CommandPaletteCommandPaths = {
  infrastructurePath: string;
  workloadsPath: string;
  podWorkloadsPath: string;
  storagePath: string;
  recoveryPath: string;
};

export function buildCommandPaletteCommands(options: {
  paths: CommandPaletteCommandPaths;
  navigate: (path: string) => void;
}): CommandPaletteModalCommand[] {
  return [
    {
      id: 'nav-infrastructure',
      label: 'Go to Infrastructure',
      description: options.paths.infrastructurePath,
      shortcut: 'g i',
      keywords: ['infra', 'agents', 'nodes', 'resources'],
      action: () => options.navigate(options.paths.infrastructurePath),
    },
    {
      id: 'nav-workloads',
      label: 'Go to Workloads',
      description: options.paths.workloadsPath,
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
      action: () => options.navigate(options.paths.workloadsPath),
    },
    {
      id: 'nav-workloads-pods',
      label: 'Go to Kubernetes Pods',
      description: options.paths.podWorkloadsPath,
      keywords: ['k8s', 'kubernetes', 'pods', 'deployments', 'clusters'],
      action: () => options.navigate(options.paths.podWorkloadsPath),
    },
    {
      id: 'nav-storage',
      label: 'Go to Storage',
      description: options.paths.storagePath,
      shortcut: 'g s',
      keywords: ['ceph', 'pbs'],
      action: () => options.navigate(options.paths.storagePath),
    },
    {
      id: 'nav-recovery',
      label: 'Go to Recovery',
      description: options.paths.recoveryPath,
      shortcut: 'g b',
      keywords: ['recovery', 'backups', 'snapshots', 'replication', 'restore'],
      action: () => options.navigate(options.paths.recoveryPath),
    },
    {
      id: 'nav-alerts',
      label: 'Go to Alerts',
      description: '/alerts',
      shortcut: 'g a',
      keywords: ['alarms', 'notifications'],
      action: () => options.navigate('/alerts'),
    },
    {
      id: 'nav-settings',
      label: 'Go to Settings',
      description: '/settings',
      shortcut: 'g t',
      keywords: ['preferences', 'config'],
      action: () => options.navigate('/settings'),
    },
  ];
}

export function normalizeCommandPaletteQuery(query: string): string {
  return query.toLowerCase().trim().replace(/\s+/g, '');
}

export function filterCommandPaletteCommands(
  commands: CommandPaletteModalCommand[],
  query: string,
): CommandPaletteModalCommand[] {
  const normalizedQuery = normalizeCommandPaletteQuery(query);
  if (!normalizedQuery) return commands;

  return commands.filter((command) => {
    const haystack = [
      command.label,
      command.description ?? '',
      command.shortcut ?? '',
      ...(command.keywords ?? []),
    ]
      .join(' ')
      .toLowerCase()
      .replace(/\s+/g, '');

    return haystack.includes(normalizedQuery);
  });
}
