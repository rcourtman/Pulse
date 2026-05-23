import {
  primaryInfrastructureNavigationIsVisible,
  type InfrastructureNavigationVisibility,
} from '@/features/infrastructureNavigation/infrastructureNavigationModel';

export interface CommandPaletteModalProps {
  isOpen: boolean;
  onClose: () => void;
  infrastructureVisibility: () => InfrastructureNavigationVisibility;
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
  agentsPath: string;
  proxmoxPath: string;
  dockerPath: string;
  kubernetesPath: string;
  kubernetesPodsPath: string;
  trueNasPath: string;
  vmwarePath: string;
  vmwareNetworksPath: string;
};

export function buildCommandPaletteCommands(options: {
  paths: CommandPaletteCommandPaths;
  infrastructureVisibility: InfrastructureNavigationVisibility;
  navigate: (path: string) => void;
}): CommandPaletteModalCommand[] {
  const commands: CommandPaletteModalCommand[] = [];

  if (primaryInfrastructureNavigationIsVisible(options.infrastructureVisibility, 'agents')) {
    commands.push({
      id: 'nav-agents',
      label: 'Go to Agents',
      description: options.paths.agentsPath,
      shortcut: 'g e',
      keywords: ['agents', 'hosts', 'machines', 'linux', 'macos', 'windows', 'unraid'],
      action: () => options.navigate(options.paths.agentsPath),
    });
  }

  if (primaryInfrastructureNavigationIsVisible(options.infrastructureVisibility, 'proxmox')) {
    commands.push({
      id: 'nav-proxmox',
      label: 'Go to Proxmox',
      description: options.paths.proxmoxPath,
      shortcut: 'g p',
      keywords: ['proxmox', 'pve', 'pbs', 'pmg', 'mail', 'backups', 'ceph', 'vm', 'lxc'],
      action: () => options.navigate(options.paths.proxmoxPath),
    });
  }

  if (primaryInfrastructureNavigationIsVisible(options.infrastructureVisibility, 'docker')) {
    commands.push({
      id: 'nav-docker',
      label: 'Go to Containers',
      description: options.paths.dockerPath,
      shortcut: 'g d',
      keywords: ['docker', 'podman', 'containers', 'compose', 'swarm', 'services'],
      action: () => options.navigate(options.paths.dockerPath),
    });
  }

  if (primaryInfrastructureNavigationIsVisible(options.infrastructureVisibility, 'kubernetes')) {
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

  if (primaryInfrastructureNavigationIsVisible(options.infrastructureVisibility, 'truenas')) {
    commands.push({
      id: 'nav-truenas',
      label: 'Go to TrueNAS',
      description: options.paths.trueNasPath,
      shortcut: 'g n',
      keywords: ['truenas', 'storage', 'disks', 'apps'],
      action: () => options.navigate(options.paths.trueNasPath),
    });
  }

  if (primaryInfrastructureNavigationIsVisible(options.infrastructureVisibility, 'vmware')) {
    commands.push(
      {
        id: 'nav-vmware',
        label: 'Go to vSphere',
        description: options.paths.vmwarePath,
        shortcut: 'g v',
        keywords: ['vmware', 'vsphere', 'esxi', 'vms', 'datastores', 'networks'],
        action: () => options.navigate(options.paths.vmwarePath),
      },
      {
        id: 'nav-vmware-networks',
        label: 'Go to vSphere Networks',
        description: options.paths.vmwareNetworksPath,
        keywords: ['vmware', 'vsphere', 'esxi', 'networks', 'portgroups'],
        action: () => options.navigate(options.paths.vmwareNetworksPath),
      },
    );
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
