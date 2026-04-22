import { Component } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import Network from 'lucide-solid/icons/network';
import { NetworkBoundarySettingsSection } from './NetworkBoundarySettingsSection';
import type { NetworkSettingsPanelProps } from './networkSettingsModel';

export const NetworkSettingsPanel: Component<NetworkSettingsPanelProps> = (props) => {
  return (
    <SettingsPanel
      title="Network"
      description="Configure the public URL, CORS, embedding, and webhook network boundaries."
      icon={<Network class="w-5 h-5" strokeWidth={2} />}
      noPadding
    >
      <NetworkBoundarySettingsSection {...props} />
    </SettingsPanel>
  );
};
