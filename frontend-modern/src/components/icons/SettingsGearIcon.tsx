import type { Component } from 'solid-js';
import Settings from 'lucide-solid/icons/settings';

interface SettingsGearIconProps {
  class?: string;
  title?: string;
}

export const SettingsGearIcon: Component<SettingsGearIconProps> = (props) => (
  <Settings {...props} strokeWidth={2} />
);
