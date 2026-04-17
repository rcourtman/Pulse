import { Component } from 'solid-js';
import { ProxmoxDirectWorkspace } from './ProxmoxDirectWorkspace';
import { SettingsSectionNav } from './SettingsSectionNav';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';

export const ProxmoxSettingsPanel: Component<InfrastructurePlatformSettingsProps> = (props) => {
  return (
    <>
      <SettingsSectionNav
        current={props.selectedAgent()}
        onSelect={props.onSelectAgent}
        class="mb-6"
      />

      <ProxmoxDirectWorkspace {...props} />
    </>
  );
};
