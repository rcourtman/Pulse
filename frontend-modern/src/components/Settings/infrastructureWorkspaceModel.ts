export type InfrastructureAddStep = 'agent' | 'pve' | 'pbs' | 'pmg' | 'truenas' | 'vmware';
export type InfrastructurePanelStep = 'pick' | InfrastructureAddStep;

const INFRASTRUCTURE_BASE_PATH = '/settings/infrastructure';

export function buildInfrastructureWorkspacePath(_view?: string): string {
  return INFRASTRUCTURE_BASE_PATH;
}

export function deriveAddStepFromLegacyPath(
  pathname: string,
): InfrastructurePanelStep | null {
  if (pathname.startsWith('/settings/infrastructure/install')) return 'agent';
  if (pathname.startsWith('/settings/infrastructure/platforms/proxmox/pbs')) return 'pbs';
  if (pathname.startsWith('/settings/infrastructure/platforms/proxmox/pmg')) return 'pmg';
  if (pathname.startsWith('/settings/infrastructure/platforms/proxmox')) return 'pve';
  if (pathname.startsWith('/settings/infrastructure/platforms/truenas')) return 'truenas';
  if (pathname.startsWith('/settings/infrastructure/platforms/vmware')) return 'vmware';
  if (pathname.startsWith('/settings/infrastructure/platforms')) return 'pick';
  if (pathname.startsWith('/settings/infrastructure/proxmox')) return 'pve';
  if (pathname.startsWith('/settings/infrastructure/truenas')) return 'truenas';
  if (pathname.startsWith('/settings/infrastructure/vmware')) return 'vmware';
  return null;
}
