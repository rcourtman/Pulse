export type InfrastructureWorkspaceView = 'inventory' | 'install' | 'platforms' | 'operations';

const INFRASTRUCTURE_WORKSPACE_PATHS: Record<InfrastructureWorkspaceView, string> = {
  inventory: '/settings/infrastructure',
  install: '/settings/infrastructure/install',
  platforms: '/settings/infrastructure/platforms',
  operations: '/settings/infrastructure/operations',
};

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
