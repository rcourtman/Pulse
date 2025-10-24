import type { Component } from 'solid-js';
import Monitor from 'lucide-solid/icons/monitor';

interface HostsIconProps {
  class?: string;
}

export const HostsIcon: Component<HostsIconProps> = (props) => (
  <Monitor class={props.class} />
);
