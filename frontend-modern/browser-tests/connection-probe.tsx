import { render } from 'solid-js/web';
import { ConnectionEditor } from '../src/components/Settings/ConnectionEditor/ConnectionEditor';
import { NodeCredentialSlot } from '../src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot';
import type { InfrastructurePlatformSettingsProps } from '../src/components/Settings/proxmoxSettingsModel';
import '../src/index.css';

// Exercise the real probe/editor/state/API client and the downstream Proxmox
// credential slot. The driver supplies only synthetic local HTTP responses.
const unexpectedMutation = async () => {
  throw new Error('Unexpected mutation');
};
const settings = {
  securityStatus: () => null,
  resolveTemperatureMonitoringEnabled: () => true,
  temperatureMonitoringLocked: () => false,
  savingTemperatureSetting: () => false,
  handleNodeTemperatureMonitoringChange: unexpectedMutation,
  handleTemperatureMonitoringChange: unexpectedMutation,
  saveNode: unexpectedMutation,
} as unknown as InfrastructurePlatformSettingsProps;

render(
  () => (
    <main class="mx-auto max-w-5xl bg-background p-4 text-base-content">
      <h1 class="mb-4 text-lg font-semibold">Offline API connection setup</h1>
      <p class="mb-4 text-sm text-muted">
        Synthetic endpoints. No live platform, credentials or guest.
      </p>
      <ConnectionEditor
        onClose={() => {}}
        renderCredentialSlot={({ type, candidate, onCancel, onSaved }) => {
          if (type !== 'pve' && type !== 'pbs' && type !== 'pmg')
            throw new Error('Unexpected platform');
          return (
            <NodeCredentialSlot
              nodeType={type}
              settings={settings}
              prefillNode={
                candidate
                  ? { type, host: candidate.host, name: new URL(candidate.host).hostname }
                  : undefined
              }
              importCandidate={candidate ? { kind: 'probe', candidate } : null}
              onCancel={onCancel}
              onSaved={onSaved}
            />
          );
        }}
      />
    </main>
  ),
  document.getElementById('root')!,
);
