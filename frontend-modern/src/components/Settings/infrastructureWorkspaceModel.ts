export type InfrastructureWorkspaceView = 'inventory' | 'install' | 'platforms' | 'operations';
export type InfrastructureWorkspaceTabView = Exclude<InfrastructureWorkspaceView, 'operations'>;

export interface InfrastructureWorkspaceTabDefinition {
  id: InfrastructureWorkspaceTabView;
  label: string;
  path: string;
}

const INFRASTRUCTURE_WORKSPACE_PATHS: Record<InfrastructureWorkspaceView, string> = {
  inventory: '/settings/infrastructure',
  install: '/settings/infrastructure/install',
  platforms: '/settings/infrastructure/platforms',
  operations: '/settings/infrastructure/operations',
};

export const INFRASTRUCTURE_WORKSPACE_TABS: readonly InfrastructureWorkspaceTabDefinition[] = [
  {
    id: 'inventory',
    label: 'Systems',
    path: INFRASTRUCTURE_WORKSPACE_PATHS.inventory,
  },
  {
    id: 'platforms',
    label: 'Connections',
    path: INFRASTRUCTURE_WORKSPACE_PATHS.platforms,
  },
  {
    id: 'install',
    label: 'Install',
    path: INFRASTRUCTURE_WORKSPACE_PATHS.install,
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
  if (pathname.startsWith('/settings/infrastructure/install')) {
    return 'install';
  }
  if (pathname.startsWith('/settings/infrastructure/operations')) {
    return 'operations';
  }
  return 'inventory';
}

export function buildInfrastructureWorkspacePath(view: InfrastructureWorkspaceView): string {
  return INFRASTRUCTURE_WORKSPACE_PATHS[view];
}
