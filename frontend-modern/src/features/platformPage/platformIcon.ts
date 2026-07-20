import type { JSX } from 'solid-js';
import CpuIcon from 'lucide-solid/icons/cpu';
import ServerIcon from 'lucide-solid/icons/server';
import UsersIcon from 'lucide-solid/icons/users';
import { DockerIcon } from '@/components/icons/DockerIcon';
import { KubernetesIcon } from '@/components/icons/KubernetesIcon';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';
import { TrueNASIcon } from '@/components/icons/TrueNASIcon';

export type PlatformIconKey =
  'proxmox' | 'docker' | 'kubernetes' | 'truenas' | 'vmware' | 'standalone' | 'systems';

export type PlatformIcon = (props: { class?: string }) => JSX.Element;

// Single source of truth for the icon shown per platform. Brand marks (Proxmox,
// Docker, Kubernetes, TrueNAS) come from inlined simple-icons SVGs; vSphere has
// no legible square brand glyph so it keeps a generic CPU mark. Systems/Standalone
// are not third-party brands and use semantic generic icons.
const PLATFORM_ICONS: Record<PlatformIconKey, PlatformIcon> = {
  proxmox: ProxmoxIcon,
  docker: DockerIcon,
  kubernetes: KubernetesIcon,
  truenas: TrueNASIcon,
  vmware: CpuIcon,
  standalone: ServerIcon,
  systems: UsersIcon,
};

export function getPlatformIcon(key: PlatformIconKey): PlatformIcon {
  return PLATFORM_ICONS[key];
}
