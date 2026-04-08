/**
 * PlatformBadge: Shows a small colored badge identifying the hypervisor/cloud platform
 * for a resource (node, VM, container).
 *
 * Used in unified views where resources from multiple providers are mixed.
 */
import { Component, Show } from 'solid-js';

interface PlatformBadgeProps {
  platform?: string;
  providerName?: string;
  size?: 'sm' | 'md';
}

const platformConfig: Record<string, { label: string; color: string; textColor: string }> = {
  proxmox:  { label: 'PVE',    color: 'bg-orange-900/50 border-orange-700', textColor: 'text-orange-400' },
  vmware:   { label: 'VMware', color: 'bg-blue-900/50 border-blue-700',     textColor: 'text-blue-400' },
  libvirt:  { label: 'KVM',    color: 'bg-amber-900/50 border-amber-700',   textColor: 'text-amber-400' },
  nutanix:  { label: 'Nutanix',color: 'bg-green-900/50 border-green-700',   textColor: 'text-green-400' },
  aws:      { label: 'AWS',    color: 'bg-yellow-900/50 border-yellow-700', textColor: 'text-yellow-400' },
  azure:    { label: 'Azure',  color: 'bg-cyan-900/50 border-cyan-700',     textColor: 'text-cyan-400' },
  gcp:      { label: 'GCP',    color: 'bg-red-900/50 border-red-700',       textColor: 'text-red-400' },
  hyperv:   { label: 'Hyper-V',color: 'bg-purple-900/50 border-purple-700', textColor: 'text-purple-400' },
};

export const PlatformBadge: Component<PlatformBadgeProps> = (props) => {
  const config = () => platformConfig[props.platform || ''] || null;
  const isSmall = () => props.size !== 'md';

  return (
    <Show when={config()}>
      <span
        class={`inline-flex items-center border rounded font-medium ${config()!.color} ${config()!.textColor}`}
        classList={{
          'px-1 py-0 text-[9px]': isSmall(),
          'px-1.5 py-0.5 text-[10px]': !isSmall(),
        }}
        title={props.providerName || props.platform}
      >
        {config()!.label}
      </span>
    </Show>
  );
};
