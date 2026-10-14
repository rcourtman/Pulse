import { render } from 'solid-js/web';
import {
  NodeCredentialSlot,
  type NodeSlotType,
} from '../src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot';
import type { InfrastructurePlatformSettingsProps } from '../src/components/Settings/proxmoxSettingsModel';
import type { NodeConfigWithStatus } from '../src/types/nodes';
import '../src/index.css';

// Real editor, state hook, API client and CSS. Every remote response is a
// synthetic local HTTP fixture in the browser driver; no real credentials or
// platform/guest calls. Saving is deliberately not part of this journey.
const type = new URLSearchParams(location.search).get('fixtureType') as NodeSlotType;
if (!['pve', 'pbs', 'pmg'].includes(type)) throw new Error('Expected fixtureType');
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
  saveNode: async () => {
    throw new Error('Unexpected save');
  },
} as unknown as InfrastructurePlatformSettingsProps;
const node = {
  id: `${type}-fixture`,
  type,
  name: `${type.toUpperCase()} fixture`,
  host: 'https://saved.example.invalid:8006',
  user: 'fixture@pam',
  tokenName: 'fixture@pam!read',
  hasToken: type !== 'pmg',
  hasPassword: type === 'pmg',
  verifySSL: true,
  fingerprint: 'ab'.repeat(32),
  status: 'connected',
} as unknown as NodeConfigWithStatus;

render(
  () => (
    <main class="mx-auto max-w-5xl bg-background p-4 text-base-content">
      <h1 class="mb-4 text-lg font-semibold">Offline {type.toUpperCase()} connection editor</h1>
      <p class="mb-4 text-sm text-muted">
        Synthetic endpoint and credentials. No live platform or guest.
      </p>
      <NodeCredentialSlot
        nodeType={type}
        settings={settings}
        editingNode={node}
        onCancel={() => {}}
        onSaved={() => {}}
        connectionEnabled={true}
        onToggleEnabled={() => {
          throw new Error('Unexpected pause');
        }}
        onDelete={() => {
          throw new Error('Unexpected delete');
        }}
      />
    </main>
  ),
  document.getElementById('root')!,
);
