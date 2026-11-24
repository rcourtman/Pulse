import { Component } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { NetworkBoundarySettingsSection } from './NetworkBoundarySettingsSection';
import type { NetworkSettingsPanelProps } from './networkSettingsModel';

export const NetworkSettingsPanel: Component<NetworkSettingsPanelProps> = (props) => {
  return (
    <SettingsPanel noPadding bodyClass="divide-y divide-border">
      <NetworkBoundarySettingsSection {...props} />
    </SettingsPanel>
  );
};
