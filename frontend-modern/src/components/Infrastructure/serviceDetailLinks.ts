import type { Resource } from '@/types/resource';
import { buildRecoveryPath, PMG_THRESHOLDS_PATH } from '@/routing/resourceLinks';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';

export type ServiceDetailLink = {
  href: string;
  label: string;
  compactLabel: string;
  ariaLabel: string;
};

export const buildServiceDetailLinks = (resource: Resource): ServiceDetailLink[] => {
  const label = getPreferredInfrastructureDisplayName(resource);

  if (resource.type === 'pbs') {
    return [
      {
        href: buildRecoveryPath({ provider: 'proxmox-pbs', mode: 'remote' }),
        label: 'Open in Recovery',
        compactLabel: 'Recovery',
        ariaLabel: `Open PBS backups in Recovery for ${label}`,
      },
    ];
  }

  if (resource.type === 'pmg') {
    return [
      {
        href: PMG_THRESHOLDS_PATH,
        label: 'Open PMG thresholds',
        compactLabel: 'Thresholds',
        ariaLabel: `Open PMG thresholds for ${label}`,
      },
    ];
  }

  return [];
};
