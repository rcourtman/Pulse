export type InfrastructureWorkspaceView = 'install' | 'direct' | 'inventory';

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
    id: 'direct',
    label: 'Direct Proxmox',
    path: '/settings/infrastructure/proxmox',
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
  if (pathname.startsWith('/settings/infrastructure/proxmox')) {
    return 'direct';
  }
  if (pathname.startsWith('/settings/infrastructure/install')) {
    return 'install';
  }
  return 'inventory';
}

export function buildInfrastructureWorkspacePath(
  view: InfrastructureWorkspaceView,
): string {
  return (
    INFRASTRUCTURE_WORKSPACE_TABS.find((tab) => tab.id === view)?.path ??
    '/settings/infrastructure/operations'
  );
}
