export type InfrastructureWorkspaceView = 'install' | 'platforms' | 'inventory';

export interface InfrastructureWorkspaceTabDefinition {
  id: InfrastructureWorkspaceView;
  label: string;
  path: string;
}

export const INFRASTRUCTURE_WORKSPACE_TABS: readonly InfrastructureWorkspaceTabDefinition[] = [
  {
    id: 'install',
    label: 'Install on a host',
    path: '/settings/infrastructure/install',
  },
  {
    id: 'platforms',
    label: 'Platform connections',
    path: '/settings/infrastructure/platforms',
  },
  {
    id: 'inventory',
    label: 'Reporting & control',
    path: '/settings/infrastructure/operations',
  },
];

export function getInfrastructureWorkspaceViewFromPath(
  pathname: string,
): InfrastructureWorkspaceView {
  if (
    pathname.startsWith('/settings/infrastructure/platforms') ||
    pathname.startsWith('/settings/infrastructure/proxmox') ||
    pathname.startsWith('/settings/infrastructure/api') ||
    pathname.startsWith('/settings/infrastructure/truenas')
  ) {
    return 'platforms';
  }
  if (pathname.startsWith('/settings/infrastructure/operations')) {
    return 'inventory';
  }
  if (pathname.startsWith('/settings/infrastructure/install')) {
    return 'install';
  }
  return 'install';
}

export function buildInfrastructureWorkspacePath(
  view: InfrastructureWorkspaceView,
): string {
  return (
    INFRASTRUCTURE_WORKSPACE_TABS.find((tab) => tab.id === view)?.path ??
    '/settings/infrastructure/install'
  );
}
