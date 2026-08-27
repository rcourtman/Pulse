import { createMemo, createSignal, For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { ThresholdsTable } from '@/components/Alerts/ThresholdsTable';
import {
  AlertIntentPolicyPanel,
  type AlertIntentPolicySelectionTarget,
} from '../AlertIntentPolicyPanel';
import type { AlertIntentSignal } from '@/api/alertIntentPolicies';

import type { ThresholdsTabProps } from '../thresholds/thresholdsTabModel';

export function ThresholdsTab(props: ThresholdsTabProps) {
  const [intentSelectionTarget, setIntentSelectionTarget] =
    createSignal<AlertIntentPolicySelectionTarget | null>(null);
  let intentSelectionRequest = 0;

  const configureResourceIntent = (resourceId: string, signal: AlertIntentSignal) => {
    intentSelectionRequest += 1;
    setIntentSelectionTarget({
      resourceId,
      signal,
      requestId: intentSelectionRequest,
    });
  };

  const evaluationProfiles = [
    { key: 'all', label: 'All resources' },
    { key: 'vm', label: 'Virtual machines' },
    { key: 'app-container', label: 'Application containers' },
    { key: 'node', label: 'Infrastructure nodes' },
    { key: 'agent', label: 'Pulse agents' },
    { key: 'k8s-node', label: 'Kubernetes nodes' },
    { key: 'truenas-system', label: 'TrueNAS systems' },
    { key: 'vmware-host', label: 'VMware hosts' },
    { key: 'pbs', label: 'Proxmox Backup Servers' },
  ] as const;
  const globalCPUWindow = createMemo(() => props.metricEvaluationWindows?.().all?.cpu ?? 300);
  const evaluationLabel = (seconds: number) => {
    if (seconds === 0) return 'Current value';
    if (seconds < 60) return `${seconds} seconds`;
    return `${seconds / 60} minute${seconds === 60 ? '' : 's'}`;
  };
  const updateCPUWindow = (profile: string, rawValue: string) => {
    if (!props.setMetricEvaluationWindows) return;
    props.setMetricEvaluationWindows((previous) => {
      const next = structuredClone(previous);
      if (rawValue === 'inherit') {
        if (next[profile]) {
          delete next[profile].cpu;
          if (Object.keys(next[profile]).length === 0) delete next[profile];
        }
        return next;
      }
      next[profile] = { ...(next[profile] ?? {}), cpu: Number(rawValue) };
      return next;
    });
    props.setHasUnsavedChanges(true);
  };
  const evaluationProfileControl = (profile: (typeof evaluationProfiles)[number]) => {
    const configured = () => props.metricEvaluationWindows?.()[profile.key]?.cpu;
    return (
      <label class="block rounded-lg border border-gray-200 p-3 dark:border-gray-700">
        <span class="block text-xs font-medium text-gray-700 dark:text-gray-300">
          {profile.label}
        </span>
        <select
          aria-label={`${profile.label} CPU evaluation window`}
          class="mt-2 w-full rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-900 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
          value={
            profile.key === 'all'
              ? String(configured() ?? 300)
              : configured() === undefined
                ? 'inherit'
                : String(configured())
          }
          onInput={(event) => updateCPUWindow(profile.key, event.currentTarget.value)}
        >
          <Show when={profile.key !== 'all'}>
            <option value="inherit">Inherit ({evaluationLabel(globalCPUWindow())})</option>
          </Show>
          <option value="0">Current value</option>
          <option value="60">1 minute average</option>
          <option value="300">5 minute average</option>
          <option value="900">15 minute average</option>
        </select>
      </label>
    );
  };

  return (
    <div class="space-y-4">
      <AlertIntentPolicyPanel
        resources={props.allResources}
        selectionTarget={intentSelectionTarget()}
      />
      <Show when={props.metricEvaluationWindows && props.setMetricEvaluationWindows}>
        <Card padding="md">
          <div class="space-y-4">
            <div>
              <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
                CPU evaluation window
              </h3>
              <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
                Pulse averages CPU over time before applying trigger and recovery thresholds. This
                filters harmless bursts without delaying a sustained incident or splitting its
                timeline.
              </p>
            </div>
            <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
              <For each={evaluationProfiles}>
                {(profile) => (
                  <div class={profile.key === 'all' ? 'block' : 'hidden sm:block'}>
                    {evaluationProfileControl(profile)}
                  </div>
                )}
              </For>
            </div>
            <details class="rounded-lg border border-gray-200 p-3 sm:hidden dark:border-gray-700">
              <summary class="cursor-pointer text-sm font-medium text-gray-800 dark:text-gray-200">
                Platform overrides
              </summary>
              <div class="mt-3 space-y-3">
                <For each={evaluationProfiles.slice(1)}>
                  {(profile) => evaluationProfileControl(profile)}
                </For>
              </div>
            </details>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              A rolling rule waits for enough recent, gap-free samples. If history is incomplete,
              Pulse holds the existing incident state instead of firing or resolving on weak data.
            </p>
          </div>
        </Card>
      </Show>
      <ThresholdsTable
        onConfigureResourceIntent={configureResourceIntent}
        overrides={props.overrides}
        setOverrides={props.setOverrides}
        rawOverridesConfig={props.rawOverridesConfig}
        setRawOverridesConfig={props.setRawOverridesConfig}
        allGuests={props.allGuests}
        nodes={props.nodes}
        agents={props.agents}
        storage={props.storage}
        containerRuntimes={props.containerRuntimes}
        dockerHosts={props.dockerHosts}
        allResources={props.allResources}
        pbsInstances={props.pbsInstances}
        pmgInstances={props.pmgInstances}
        pmgThresholds={props.pmgThresholds}
        setPMGThresholds={props.setPMGThresholds}
        guestDefaults={props.guestDefaults()}
        setGuestDefaults={props.setGuestDefaults}
        guestDisableConnectivity={props.guestDisableConnectivity}
        setGuestDisableConnectivity={props.setGuestDisableConnectivity}
        guestPoweredOffSeverity={props.guestPoweredOffSeverity}
        setGuestPoweredOffSeverity={props.setGuestPoweredOffSeverity}
        nodeDefaults={props.nodeDefaults()}
        pbsDefaults={props.pbsDefaults()}
        kubernetesDefaults={props.kubernetesDefaults()}
        trueNASDefaults={props.trueNASDefaults()}
        trueNASDiskDefaults={props.trueNASDiskDefaults()}
        vmwareDefaults={props.vmwareDefaults()}
        agentDefaults={props.agentDefaults()}
        diskTempByType={props.diskTempByType()}
        setDiskTempByType={props.setDiskTempByType}
        setNodeDefaults={props.setNodeDefaults}
        setPBSDefaults={props.setPBSDefaults}
        setKubernetesDefaults={props.setKubernetesDefaults}
        setTrueNASDefaults={props.setTrueNASDefaults}
        setTrueNASDiskDefaults={props.setTrueNASDiskDefaults}
        setVMwareDefaults={props.setVMwareDefaults}
        setAgentDefaults={props.setAgentDefaults}
        dockerDefaults={props.dockerDefaults()}
        dockerDisableConnectivity={props.dockerDisableConnectivity}
        setDockerDisableConnectivity={props.setDockerDisableConnectivity}
        dockerPoweredOffSeverity={props.dockerPoweredOffSeverity}
        setDockerPoweredOffSeverity={props.setDockerPoweredOffSeverity}
        setDockerDefaults={props.setDockerDefaults}
        dockerIgnoredPrefixes={props.dockerIgnoredPrefixes}
        setDockerIgnoredPrefixes={props.setDockerIgnoredPrefixes}
        ignoredGuestPrefixes={props.ignoredGuestPrefixes}
        setIgnoredGuestPrefixes={props.setIgnoredGuestPrefixes}
        guestTagWhitelist={props.guestTagWhitelist}
        setGuestTagWhitelist={props.setGuestTagWhitelist}
        guestTagBlacklist={props.guestTagBlacklist}
        setGuestTagBlacklist={props.setGuestTagBlacklist}
        storageDefault={props.storageDefault}
        setStorageDefault={props.setStorageDefault}
        resetGuestDefaults={props.resetGuestDefaults}
        resetNodeDefaults={props.resetNodeDefaults}
        resetPBSDefaults={props.resetPBSDefaults}
        resetKubernetesDefaults={props.resetKubernetesDefaults}
        resetTrueNASDefaults={props.resetTrueNASDefaults}
        resetTrueNASDiskDefaults={props.resetTrueNASDiskDefaults}
        resetVMwareDefaults={props.resetVMwareDefaults}
        resetAgentDefaults={props.resetAgentDefaults}
        resetDockerDefaults={props.resetDockerDefaults}
        resetDockerIgnoredPrefixes={props.resetDockerIgnoredPrefixes}
        resetStorageDefault={props.resetStorageDefault}
        factoryGuestDefaults={props.factoryGuestDefaults}
        factoryNodeDefaults={props.factoryNodeDefaults}
        factoryPBSDefaults={props.factoryPBSDefaults}
        factoryKubernetesDefaults={props.factoryKubernetesDefaults}
        factoryTrueNASDefaults={props.factoryTrueNASDefaults}
        factoryTrueNASDiskDefaults={props.factoryTrueNASDiskDefaults}
        factoryVMwareDefaults={props.factoryVMwareDefaults}
        factoryAgentDefaults={props.factoryAgentDefaults}
        factoryDockerDefaults={props.factoryDockerDefaults}
        factoryStorageDefault={props.factoryStorageDefault}
        timeThresholds={props.timeThresholds}
        metricTimeThresholds={props.metricTimeThresholds}
        setMetricTimeThresholds={props.setMetricTimeThresholds}
        snapshotDefaults={props.snapshotDefaults}
        setSnapshotDefaults={props.setSnapshotDefaults}
        snapshotFactoryDefaults={props.snapshotFactoryDefaults}
        resetSnapshotDefaults={props.resetSnapshotDefaults}
        backupDefaults={props.backupDefaults}
        setBackupDefaults={props.setBackupDefaults}
        backupFactoryDefaults={props.backupFactoryDefaults}
        resetBackupDefaults={props.resetBackupDefaults}
        setHasUnsavedChanges={props.setHasUnsavedChanges}
        activeAlerts={props.activeAlerts}
        removeAlerts={props.removeAlerts}
        disableAllNodes={props.disableAllNodes}
        setDisableAllNodes={props.setDisableAllNodes}
        disableAllGuests={props.disableAllGuests}
        setDisableAllGuests={props.setDisableAllGuests}
        disableAllAgents={props.disableAllAgents}
        setDisableAllAgents={props.setDisableAllAgents}
        disableAllStorage={props.disableAllStorage}
        setDisableAllStorage={props.setDisableAllStorage}
        disableAllPBS={props.disableAllPBS}
        setDisableAllPBS={props.setDisableAllPBS}
        disableAllPMG={props.disableAllPMG}
        setDisableAllPMG={props.setDisableAllPMG}
        disableAllDockerHosts={props.disableAllDockerHosts}
        setDisableAllDockerHosts={props.setDisableAllDockerHosts}
        disableAllDockerServices={props.disableAllDockerServices}
        setDisableAllDockerServices={props.setDisableAllDockerServices}
        disableAllDockerContainers={props.disableAllDockerContainers}
        setDisableAllDockerContainers={props.setDisableAllDockerContainers}
        disableAllKubernetes={props.disableAllKubernetes}
        setDisableAllKubernetes={props.setDisableAllKubernetes}
        disableAllTrueNAS={props.disableAllTrueNAS}
        setDisableAllTrueNAS={props.setDisableAllTrueNAS}
        disableAllVMware={props.disableAllVMware}
        setDisableAllVMware={props.setDisableAllVMware}
        disableAllNodesOffline={props.disableAllNodesOffline}
        setDisableAllNodesOffline={props.setDisableAllNodesOffline}
        disableAllGuestsOffline={props.disableAllGuestsOffline}
        setDisableAllGuestsOffline={props.setDisableAllGuestsOffline}
        disableAllAgentsOffline={props.disableAllAgentsOffline}
        setDisableAllAgentsOffline={props.setDisableAllAgentsOffline}
        disableAllPBSOffline={props.disableAllPBSOffline}
        setDisableAllPBSOffline={props.setDisableAllPBSOffline}
        disableAllPMGOffline={props.disableAllPMGOffline}
        setDisableAllPMGOffline={props.setDisableAllPMGOffline}
        disableAllDockerHostsOffline={props.disableAllDockerHostsOffline}
        setDisableAllDockerHostsOffline={props.setDisableAllDockerHostsOffline}
      />
    </div>
  );
}
