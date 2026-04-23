import { Component } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { NetworkBoundarySettingsSection } from './NetworkBoundarySettingsSection';
import type { NetworkSettingsPanelProps } from './networkSettingsModel';

export const NetworkSettingsPanel: Component<NetworkSettingsPanelProps> = (props) => {
  return (
    <SettingsPanel
      title="Network"
      description="Configure the public URL, CORS, embedding, and webhook network boundaries."
      noPadding
    >
      <NetworkBoundarySettingsSection {...props} />
    </SettingsPanel>
  );
};
