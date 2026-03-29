import type { Component } from 'solid-js';
import { Match, Switch, createMemo } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Subtabs } from '@/components/shared/Subtabs';
import { ProxmoxSettingsPanel } from './ProxmoxSettingsPanel';
import {
  PLATFORM_CONNECTIONS_TABS,
  buildPlatformConnectionsPath,
  getPlatformConnectionsViewFromPath,
} from './platformConnectionsModel';
import { TrueNASSettingsPanel } from './TrueNASSettingsPanel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';

export const PlatformConnectionsWorkspace: Component<InfrastructurePlatformSettingsProps> = (
  props,
) => {
  const navigate = useNavigate();
  const location = useLocation();
  const activeView = createMemo(() => getPlatformConnectionsViewFromPath(location.pathname));

  return (
    <div class="space-y-6">
      <Subtabs
        value={activeView()}
        onChange={(value) => navigate(buildPlatformConnectionsPath(value as 'proxmox' | 'truenas'))}
        ariaLabel="Platform connections"
        tabs={PLATFORM_CONNECTIONS_TABS.map((tab) => ({
          value: tab.id,
          label: tab.label,
        }))}
      />

      <Switch>
        <Match when={activeView() === 'truenas'}>
          <TrueNASSettingsPanel state={props.trueNASSettings} />
        </Match>

        <Match when={activeView() === 'proxmox'}>
          <ProxmoxSettingsPanel {...props} embedded />
        </Match>
      </Switch>
    </div>
  );
};

export default PlatformConnectionsWorkspace;
