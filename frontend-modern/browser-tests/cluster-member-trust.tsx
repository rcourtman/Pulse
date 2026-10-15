import { createSignal, onMount, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { NodesAPI } from '@/api/nodes';
import { NodeCredentialSlot } from '@/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot';
import type { InfrastructurePlatformSettingsProps } from '@/components/Settings/proxmoxSettingsModel';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import '@/index.css';

// Safe synthetic HTTP configuration passes through the production normalizer,
// mounted editor/state, refreshed props and explicit update client. No platform,
// certificate or guest is contacted. The browser driver rejects other APIs.
function Fixture() {
  const [node, setNode] = createSignal<NodeConfigWithStatus>();
  const reload = async () => {
    const nodes = await NodesAPI.getNodes();
    setNode(nodes[0] as NodeConfigWithStatus);
  };
  const settings = {
    securityStatus: () => null,
    resolveTemperatureMonitoringEnabled: () => false,
    temperatureMonitoringLocked: () => false,
    savingTemperatureSetting: () => false,
    saveNode: async (data: NodeConfig, target: NodeConfig) => {
      const result = await NodesAPI.updateNode(target.id, data);
      return result.success;
    },
  } as unknown as InfrastructurePlatformSettingsProps;
  Object.assign(window, { reloadClusterConfig: reload });
  onMount(reload);
  return (
    <main class="mx-auto max-w-4xl bg-background p-4 text-base-content">
      <h1 class="mb-4 text-lg font-semibold">Synthetic cluster settings</h1>
      <Show when={node()}>
        <NodeCredentialSlot
          nodeType="pve"
          settings={settings}
          editingNode={node()}
          onCancel={() => {}}
          onSaved={() => {}}
        />
      </Show>
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
