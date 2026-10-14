// Exercise the actual connection editor and save payload without a live host.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { NodeCredentialSlot } from '../src/components/Settings/ConnectionEditor/CredentialSlots/NodeCredentialSlot';
import type { InfrastructurePlatformSettingsProps } from '../src/components/Settings/proxmoxSettingsModel';
import type { NodeConfigWithStatus } from '../src/types/nodes';
import '../src/index.css';

function Fixture() {
  const requestedType = new URLSearchParams(location.search).get('type');
  const type = requestedType === 'pbs' || requestedType === 'pmg' ? requestedType : 'pve';
  const [saved, setSaved] = createSignal<unknown>(null);
  const [finished, setFinished] = createSignal(false);
  const source = {
    id: `${type}-fixture`,
    type,
    name: `Fixture ${type.toUpperCase()}`,
    host: `https://${type}.example.invalid:${type === 'pbs' ? 8007 : 8006}`,
    user: 'monitoring@pam',
    tokenName: 'monitoring@pam!fixture',
    hasToken: true,
    verifySSL: true,
    monitorVMs: true,
    monitorContainers: true,
    monitorStorage: true,
    monitorBackups: true,
    monitorPhysicalDisks: false,
    monitorDatastores: true,
    monitorSyncJobs: true,
    monitorVerifyJobs: true,
    monitorPruneJobs: true,
    monitorGarbageJobs: true,
    excludeDatastores: [],
    monitorMailStats: true,
    monitorQueues: true,
    monitorQuarantine: true,
    monitorDomainStats: true,
    status: 'connected',
  } as NodeConfigWithStatus;
  const settings = {
    securityStatus: () => null,
    resolveTemperatureMonitoringEnabled: () => false,
    temperatureMonitoringLocked: () => false,
    savingTemperatureSetting: () => false,
    handleNodeTemperatureMonitoringChange: async () => {},
    handleTemperatureMonitoringChange: async () => {},
    saveNode: async (data, target) => {
      setSaved({ data, targetId: target?.id });
      return true;
    },
  } as InfrastructurePlatformSettingsProps;
  return (
    <main class="mx-auto max-w-3xl bg-base p-4 text-base-content">
      <h1 class="mb-4 text-lg font-semibold">Edit {type.toUpperCase()} connection</h1>
      <NodeCredentialSlot
        nodeType={type}
        editingNode={source}
        settings={settings}
        onCancel={() => {}}
        onSaved={() => setFinished(true)}
      />
      <output data-testid="save-result">{JSON.stringify(saved())}</output>
      <output data-testid="save-finished">{String(finished())}</output>
    </main>
  );
}
render(() => <Fixture />, document.getElementById('root')!);
