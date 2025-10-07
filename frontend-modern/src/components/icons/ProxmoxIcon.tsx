import type { Component } from 'solid-js';
import { siProxmox } from 'simple-icons';

interface ProxmoxIconProps {
  class?: string;
  title?: string;
}

export const ProxmoxIcon: Component<ProxmoxIconProps> = (props) => (
  <svg
    class={props.class}
    role="img"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    aria-label={props.title ?? 'Proxmox'}
  >
    <title>{props.title ?? 'Proxmox'}</title>
    <path d={siProxmox.path} fill="currentColor" />
  </svg>
);
