import {
  primaryPlatformNavigationIsVisible,
  type PlatformNavigationVisibility,
} from '@/features/platformNavigation/platformNavigationModel';

export interface CommandPaletteModalProps {
  isOpen: boolean;
  onClose: () => void;
  platformVisibility: () => PlatformNavigationVisibility;
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
  proxmoxPath: string;
  dockerPath: string;
  kubernetesPath: string;
  kubernetesPodsPath: string;
  trueNasPath: string;
  vmwarePath: string;
};

export function buildCommandPaletteCommands(options: {
  paths: CommandPaletteCommandPaths;
  platformVisibility: PlatformNavigationVisibility;
  navigate: (path: string) => void;
}): CommandPaletteModalCommand[] {
  const commands: CommandPaletteModalCommand[] = [];

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'proxmox')) {
    commands.push({
      id: 'nav-proxmox',
      label: 'Go to Proxmox',
      description: options.paths.proxmoxPath,
      shortcut: 'g p',
      keywords: ['proxmox', 'pve', 'pbs', 'pmg', 'mail', 'backups', 'ceph', 'vm', 'lxc'],
      action: () => options.navigate(options.paths.proxmoxPath),
    });
  }

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'docker')) {
    commands.push({
      id: 'nav-docker',
      label: 'Go to Containers',
      description: options.paths.dockerPath,
      shortcut: 'g d',
      keywords: ['docker', 'podman', 'containers', 'compose', 'swarm', 'services'],
      action: () => options.navigate(options.paths.dockerPath),
    });
  }

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'kubernetes')) {
    commands.push(
      {
        id: 'nav-kubernetes',
        label: 'Go to Kubernetes',
        description: options.paths.kubernetesPath,
        shortcut: 'g k',
        keywords: ['k8s', 'kubernetes', 'clusters', 'nodes', 'deployments'],
        action: () => options.navigate(options.paths.kubernetesPath),
      },
      {
        id: 'nav-kubernetes-pods',
        label: 'Go to Kubernetes Pods',
        description: options.paths.kubernetesPodsPath,
        keywords: ['k8s', 'kubernetes', 'pods', 'workloads'],
        action: () => options.navigate(options.paths.kubernetesPodsPath),
      },
    );
  }

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'truenas')) {
    commands.push({
      id: 'nav-truenas',
      label: 'Go to TrueNAS',
      description: options.paths.trueNasPath,
      shortcut: 'g n',
      keywords: ['truenas', 'storage', 'disks', 'apps'],
      action: () => options.navigate(options.paths.trueNasPath),
    });
  }

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'vmware')) {
    commands.push({
      id: 'nav-vmware',
      label: 'Go to vSphere',
      description: options.paths.vmwarePath,
      shortcut: 'g v',
      keywords: ['vmware', 'vsphere', 'esxi', 'vms', 'datastores'],
      action: () => options.navigate(options.paths.vmwarePath),
    });
  }

  commands.push(
    {
      id: 'nav-alerts',
      label: 'Go to Alerts',
      description: '/alerts',
      shortcut: 'g a',
      keywords: ['alarms', 'notifications'],
      action: () => options.navigate('/alerts'),
    },
    {
      id: 'nav-patrol',
      label: 'Go to Patrol',
      description: '/patrol',
      shortcut: 'g r',
      keywords: ['patrol', 'findings', 'ai', 'verification'],
      action: () => options.navigate('/patrol'),
    },
    {
      id: 'nav-settings',
      label: 'Go to Settings',
      description: '/settings',
      shortcut: 'g t',
      keywords: ['preferences', 'config'],
      action: () => options.navigate('/settings'),
    },
  );

  return commands;
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
