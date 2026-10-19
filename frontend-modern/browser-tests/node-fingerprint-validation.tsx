import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { NodesAPI } from '../src/api/nodes';
import {
  NodeCredentialSlot,
  type NodeSlotType,
} from '../src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot';
import type { InfrastructurePlatformSettingsProps } from '../src/components/Settings/proxmoxSettingsModel';
import type { NodeConfig, NodeConfigWithStatus } from '../src/types/nodes';
import '../src/index.css';

// Only HTTP fixtures supplied by the browser protocol; no real credential,
// platform, guest, certificate capture or host operation. Use the production
// editor, hook, payload builder and API transport, including save/readback.
const type = new URLSearchParams(location.search).get('type') as NodeSlotType;
if (!['pve', 'pbs', 'pmg'].includes(type)) throw new Error('Expected fixture type');
const [node, setNode] = createSignal<NodeConfigWithStatus>({
  id: `${type}-fixture`,
  type,
  name: `${type.toUpperCase()} fixture`,
  host: 'https://fixture.example.invalid:8006',
  user: 'fixture@pam',
  tokenName: 'fixture@pam!read',
  hasToken: type !== 'pmg',
  hasPassword: type === 'pmg',
  verifySSL: true,
  fingerprint: 'ab'.repeat(32),
  status: 'connected',
} as NodeConfigWithStatus);
const [saved, setSaved] = createSignal(false);
const settings = {
  securityStatus: () => null,
  resolveTemperatureMonitoringEnabled: () => true,
  temperatureMonitoringLocked: () => false,
  savingTemperatureSetting: () => false,
  handleNodeTemperatureMonitoringChange: async () => {
    throw new Error('Unexpected mutation');
  },
  handleTemperatureMonitoringChange: async () => {
    throw new Error('Unexpected mutation');
  },
  saveNode: async (draft: Partial<NodeConfig>, editing: NodeConfig | null) => {
    if (editing?.id !== node().id) throw new Error('Wrong save target');
    await NodesAPI.updateNode(editing.id, draft as NodeConfig);
    const readback = (await NodesAPI.getNodes()).find((item) => item.id === editing.id);
    if (!readback) throw new Error('Missing saved readback');
    setNode(readback as NodeConfigWithStatus);
    return true;
  },
} as unknown as InfrastructurePlatformSettingsProps;

render(
  () => (
    <main class="mx-auto max-w-5xl bg-background p-4 text-base-content">
      <h1 class="mb-4 text-lg font-semibold">Offline {type.toUpperCase()} connection editor</h1>
      <p class="mb-4 text-sm text-muted">
        Synthetic endpoint only; valid pin format is not verified trust.
      </p>
      <output class="block text-sm" aria-label="Fixture save readback">
        {saved() ? 'Saved pin read back from the synthetic API.' : 'No fixture save.'}
      </output>
      <NodeCredentialSlot
        nodeType={type}
        settings={settings}
        editingNode={node()}
        onCancel={() => {}}
        onSaved={() => setSaved(true)}
      />
    </main>
  ),
  document.getElementById('root')!,
);
