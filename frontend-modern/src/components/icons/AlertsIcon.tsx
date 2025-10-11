import type { Component } from 'solid-js';
import Bell from 'lucide-solid/icons/bell';

interface AlertsIconProps {
  class?: string;
  title?: string;
}

export const AlertsIcon: Component<AlertsIconProps> = (props) => (
  <Bell {...props} strokeWidth={2} />
);
