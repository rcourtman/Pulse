import { Component } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { NetworkBoundarySettingsSection } from './NetworkBoundarySettingsSection';
import type { NetworkSettingsPanelProps } from './networkSettingsModel';

export const NetworkSettingsPanel: Component<NetworkSettingsPanelProps> = (props) => {
  return (
    <Card
      data-settings-panel
      padding="none"
      border={false}
      class="overflow-hidden border border-border divide-y divide-border"
    >
      <NetworkBoundarySettingsSection {...props} />
    </Card>
  );
};
