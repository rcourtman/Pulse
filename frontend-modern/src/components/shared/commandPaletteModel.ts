import {
  PRIMARY_PLATFORM_NAV_IDS,
  primaryPlatformNavigationIsVisible,
  type PlatformNavigationVisibility,
} from '@/features/platformNavigation/platformNavigationModel';
import { INFRASTRUCTURE_MAP_PATH } from '@/routing/resourceLinks';
import { isFeatureAvailable } from '@/utils/featureReleases';
import {
  applyThemeClass,
  computeIsDark,
  persistThemePreference,
  type ThemePreference,
} from '@/utils/theme';
import { isKioskMode, setKioskMode } from '@/utils/url';

export interface CommandPaletteModalProps {
  isOpen: boolean;
  onClose: () => void;
  platformVisibility: () => PlatformNavigationVisibility;
  themePreference?: () => ThemePreference;
  onSetTheme?: (preference: ThemePreference) => void;
  kioskMode?: () => boolean;
  onToggleKioskMode?: () => void;
  hasSettingsAccess?: () => boolean;
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
  availabilityPath?: string;
  standalonePath: string;
  proxmoxPath: string;
  dockerPath: string;
  kubernetesPath: string;
  kubernetesWorkloadsPath: string;
  trueNasPath: string;
  vmwarePath: string;
  vmwareNetworksPath: string;
};

export function buildCommandPaletteCommands(options: {
  paths: CommandPaletteCommandPaths;
  platformVisibility: PlatformNavigationVisibility;
  // Omitted means visible, for callers that do not gate Patrol.
  patrolVisible?: boolean;
  // Omitted checks feature release availability so held features stay absent in release builds.
  mapAvailable?: boolean;
  // Omitted means visible, for callers that do not restrict Settings access.
  settingsVisible?: boolean;
  themePreference?: ThemePreference;
  onSetTheme?: (preference: ThemePreference) => void;
  kioskMode?: boolean;
  onToggleKioskMode?: () => void;
  navigate: (path: string) => void;
}): CommandPaletteModalCommand[] {
  const commands: CommandPaletteModalCommand[] = [];

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'proxmox')) {
    commands.push({
      id: 'nav-proxmox',
      label: 'Go to Proxmox',
      description: 'Nodes, VMs, containers, storage, and backups',
      shortcut: 'g p',
      keywords: [
        'proxmox',
        'pve',
        'pbs',
        'pmg',
        'mail',
        'backups',
        'ceph',
        'vm',
        'lxc',
        options.paths.proxmoxPath,
      ],
      action: () => options.navigate(options.paths.proxmoxPath),
    });
  }

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'docker')) {
    commands.push({
      id: 'nav-docker',
      label: 'Go to Docker',
      description: 'Hosts, containers, applications, and Swarm services',
      shortcut: 'g d',
      keywords: [
        'docker',
        'podman',
        'containers',
        'compose',
        'swarm',
        'services',
        options.paths.dockerPath,
      ],
      action: () => options.navigate(options.paths.dockerPath),
    });
  }

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'kubernetes')) {
    commands.push(
      {
        id: 'nav-kubernetes',
        label: 'Go to Kubernetes',
        description: 'Clusters, nodes, services, and events',
        shortcut: 'g k',
        keywords: [
          'k8s',
          'kubernetes',
          'clusters',
          'nodes',
          'deployments',
          options.paths.kubernetesPath,
        ],
        action: () => options.navigate(options.paths.kubernetesPath),
      },
      {
        id: 'nav-kubernetes-workloads',
        label: 'Go to Kubernetes Workloads',
        description: 'Deployments, pods, and controllers',
        keywords: [
          'k8s',
          'kubernetes',
          'pods',
          'deployments',
          'controllers',
          'workloads',
          options.paths.kubernetesWorkloadsPath,
        ],
        action: () => options.navigate(options.paths.kubernetesWorkloadsPath),
      },
    );
  }

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'truenas')) {
    commands.push({
      id: 'nav-truenas',
      label: 'Go to TrueNAS',
      description: 'Pools, datasets, apps, shares, and protection',
      shortcut: 'g n',
      keywords: ['truenas', 'storage', 'disks', 'apps', options.paths.trueNasPath],
      action: () => options.navigate(options.paths.trueNasPath),
    });
  }

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'vmware')) {
    commands.push(
      {
        id: 'nav-vmware',
        label: 'Go to vSphere',
        description: 'Hosts, VMs, datastores, and health',
        shortcut: 'g v',
        keywords: [
          'vmware',
          'vsphere',
          'esxi',
          'vms',
          'datastores',
          'networks',
          options.paths.vmwarePath,
        ],
        action: () => options.navigate(options.paths.vmwarePath),
      },
      {
        id: 'nav-vmware-networks',
        label: 'Go to vSphere Networks',
        description: 'Switches and port groups',
        keywords: [
          'vmware',
          'vsphere',
          'esxi',
          'networks',
          'portgroups',
          options.paths.vmwareNetworksPath,
        ],
        action: () => options.navigate(options.paths.vmwareNetworksPath),
      },
    );
  }

  if (primaryPlatformNavigationIsVisible(options.platformVisibility, 'standalone')) {
    commands.push({
      id: 'nav-standalone',
      label: 'Go to Machines',
      description: 'Pulse Agent hosts and standalone virtual machines',
      shortcut: 'g s',
      keywords: [
        'standalone',
        'agents',
        'hosts',
        'machines',
        'linux',
        'macos',
        'windows',
        'unraid',
        options.paths.standalonePath,
      ],
      action: () => options.navigate(options.paths.standalonePath),
    });
  }

  const hasAnyPrimaryInfra = PRIMARY_PLATFORM_NAV_IDS.some((id) =>
    primaryPlatformNavigationIsVisible(options.platformVisibility, id),
  );
  if (
    primaryPlatformNavigationIsVisible(options.platformVisibility, 'availability') ||
    (options.settingsVisible !== false && hasAnyPrimaryInfra)
  ) {
    const availabilityPath = options.paths.availabilityPath ?? '/availability';
    commands.push({
      id: 'nav-availability',
      label: 'Go to Availability',
      description: 'Website, service, device, and TLS certificate checks',
      shortcut: 'g c',
      keywords: [
        'availability',
        'checks',
        'uptime',
        'endpoints',
        'ping',
        'icmp',
        'tcp',
        'udp',
        'http',
        'https',
        'tls',
        'certificate',
        'mqtt',
        availabilityPath,
      ],
      action: () => options.navigate(availabilityPath),
    });
  }

  commands.push({
    id: 'nav-alerts',
    label: 'Go to Alerts',
    description: 'Active alerts, thresholds, and notification rules',
    shortcut: 'g a',
    keywords: ['alarms', 'notifications', '/alerts'],
    action: () => options.navigate('/alerts'),
  });

  if (options.patrolVisible !== false) {
    commands.push({
      id: 'nav-patrol',
      label: 'Go to Patrol',
      description: 'Operational attention and recent checks',
      shortcut: 'g r',
      keywords: ['needs attention', 'patrol', 'findings', 'ai', 'verification', '/patrol'],
      action: () => options.navigate('/patrol'),
    });
  }

  commands.push({
    id: 'nav-actions',
    label: 'Go to Actions',
    description: 'Approvals, ready tasks, and recorded outcomes',
    keywords: ['actions', 'approvals', 'tasks', 'runs', 'outcomes', '/actions'],
    action: () => options.navigate('/actions'),
  });

  const mapAvailable = options.mapAvailable ?? isFeatureAvailable('infrastructureMap');
  if (mapAvailable) {
    commands.push({
      id: 'nav-map',
      label: 'Go to Map',
      description: 'Interactive map of connected infrastructure',
      keywords: ['map', 'topology', 'infrastructure', 'graph', INFRASTRUCTURE_MAP_PATH],
      action: () => options.navigate(INFRASTRUCTURE_MAP_PATH),
    });
  }

  if (options.settingsVisible !== false) {
    commands.push({
      id: 'nav-settings',
      label: 'Go to Settings',
      description: 'Connections, alerts, appearance, and system settings',
      shortcut: 'g t',
      keywords: ['preferences', 'config', '/settings'],
      action: () => options.navigate('/settings'),
    });
  }

  commands.push({
    id: 'nav-docs',
    label: 'Go to Docs',
    description: 'Guides, setup instructions, and troubleshooting',
    keywords: ['docs', 'documentation', 'guides', 'help', 'manual', '/docs'],
    action: () => options.navigate('/docs'),
  });

  const applyTheme = (preference: ThemePreference) => {
    if (options.onSetTheme) {
      options.onSetTheme(preference);
      return;
    }
    persistThemePreference(preference);
    applyThemeClass(computeIsDark(preference));
  };

  commands.push(
    {
      id: 'theme-light',
      label: 'Switch Theme: Light',
      description:
        options.themePreference === 'light' ? 'Currently active theme' : 'Use light appearance',
      keywords: ['theme', 'light', 'appearance', 'color', 'bright'],
      action: () => applyTheme('light'),
    },
    {
      id: 'theme-dark',
      label: 'Switch Theme: Dark',
      description:
        options.themePreference === 'dark' ? 'Currently active theme' : 'Use dark appearance',
      keywords: ['theme', 'dark', 'appearance', 'color', 'night'],
      action: () => applyTheme('dark'),
    },
    {
      id: 'theme-system',
      label: 'Switch Theme: System',
      description:
        options.themePreference === 'system'
          ? 'Currently active theme'
          : 'Match your system appearance',
      keywords: ['theme', 'system', 'auto', 'appearance', 'color', 'os'],
      action: () => applyTheme('system'),
    },
  );

  if (options.settingsVisible !== false) {
    const kioskActive = options.kioskMode ?? isKioskMode();
    const toggleKiosk = () => {
      if (options.onToggleKioskMode) {
        options.onToggleKioskMode();
        return;
      }
      setKioskMode(!isKioskMode());
    };

    commands.push({
      id: 'toggle-kiosk',
      label: kioskActive ? 'Exit Kiosk Mode' : 'Enter Kiosk Mode',
      description: kioskActive
        ? 'Restore navigation bar and footer'
        : 'Hide navigation chrome for wallboard displays',
      keywords: ['kiosk', 'kiosk mode', 'fullscreen', 'wallboard', 'display', 'hide navigation'],
      action: toggleKiosk,
    });
  }

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
