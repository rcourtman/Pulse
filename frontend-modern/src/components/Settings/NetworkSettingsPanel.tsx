import { Component } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import Network from 'lucide-solid/icons/network';
import { NetworkBoundarySettingsSection } from './NetworkBoundarySettingsSection';
import { NetworkDiscoverySection } from './NetworkDiscoverySection';
import type { NetworkSettingsPanelProps } from './networkSettingsModel';

export const NetworkSettingsPanel: Component<NetworkSettingsPanelProps> = (props) => {
  return (
    <SettingsPanel
      title="Network"
      description="Configure discovery, CORS, embedding, and webhook network boundaries."
      icon={<Network class="w-5 h-5" strokeWidth={2} />}
      noPadding
      bodyClass="divide-y divide-border"
    >
      <NetworkDiscoverySection {...props} />
      <NetworkBoundarySettingsSection {...props} />
    </SettingsPanel>
  );
};
