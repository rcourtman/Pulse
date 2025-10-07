import type { Component } from 'solid-js';
import { siProxmox } from 'simple-icons';

interface ProxmoxBackupIconProps {
  class?: string;
  title?: string;
}

export const ProxmoxBackupIcon: Component<ProxmoxBackupIconProps> = (props) => (
  <svg
    class={props.class}
    role="img"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    aria-label={props.title ?? 'Proxmox Backup'}
  >
    <title>{props.title ?? 'Proxmox Backup'}</title>
    <path d={siProxmox.path} fill="currentColor" />
  </svg>
);
