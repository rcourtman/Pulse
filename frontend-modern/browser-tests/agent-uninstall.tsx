// Production settings state, installer and token dialog; synthetic API data only.
import { Route, Router } from '@solidjs/router';
import { render } from 'solid-js/web';
import {
  InfrastructureOperationsStateProvider,
  useInfrastructureOperationsContext,
} from '../src/components/Settings/useInfrastructureOperationsState';
import { InfrastructureInstallerSection } from '../src/components/Settings/InfrastructureInstallerSection';
import { InfrastructureAgentDoctorPage } from '../src/components/Settings/InfrastructureAgentDoctorPage';
import type { InfrastructureAgentDoctorTarget } from '../src/components/Settings/infrastructureAgentUpdateCommandsModel';
import { InfrastructureWorkspace } from '../src/components/Settings/InfrastructureWorkspace';
import { TokenRevealDialog } from '../src/components/TokenRevealDialog';
import '../src/index.css';

const Operations = () => {
  const state = useInfrastructureOperationsContext();
  const identity = {
    agentActionId: 'agent-fixture-42',
    agentId: 'unused-alias',
    hostname: 'node.example',
  };
  const connection = {
    id: 'agent:agent-fixture-42',
    type: 'agent',
    name: 'node.example',
    address: 'node.example',
    state: 'unauthorized',
    agentIdentity: { hostname: 'node.example', commandsEnabled: false },
  } as Parameters<typeof state.getAgentConnectionUpgradeCommand>[0];
  return (
    <section class="space-y-2 p-4 border border-border" aria-label="Lifecycle command fixture">
      <p>Actual operations state: identity and trust options must survive.</p>
      <button onClick={() => navigator.clipboard.writeText(state.getUninstallCommand(identity))}>
        Copy Unix uninstall
      </button>
      <button
        onClick={() =>
          navigator.clipboard.writeText(
            state.getAgentConnectionUpgradeCommand(connection, ['--enable-docker'], 'linux', true),
          )
        }
      >
        Copy Unix credential repair
      </button>
      <button
        onClick={() =>
          navigator.clipboard.writeText(
            state.getAgentConnectionUpgradeCommand(connection, ['--enable-docker'], 'linux', false),
          )
        }
      >
        Copy Unix saved-state update
      </button>
    </section>
  );
};
const Fixture = () =>
  new URLSearchParams(location.search).has('workspace') ? (
    <Workspace />
  ) : (
    <InfrastructureOperationsStateProvider embedded>
      <main class="mx-auto max-w-5xl p-4 bg-base text-base-content">
        <h1 class="text-xl mb-4">Saved-credential agent removal</h1>
        <InfrastructureInstallerSection />
        <Operations />
        <Doctor />
        <TokenRevealDialog />
      </main>
    </InfrastructureOperationsStateProvider>
  );
const Doctor = () => {
  const target = {
    key: 'agent:doctor-fixture',
    connectionId: 'agent:doctor-fixture',
    displayName: 'doctor-fixture',
    contextLabel: 'Machine',
    currentVersion: '6.4.5',
    installFlags: ['--enable-docker'],
    status: 'critical',
    reasons: [],
    evidence: [],
    needsUpdate: false,
    needsCredentialRepair: true,
    commandPlatform: 'linux',
    safeCollector: false,
    actionRunnerPosture: [],
    actionRunnerCredentialEligible: false,
    source: 'ledger',
    connection: {
      id: 'agent:doctor-fixture',
      type: 'agent',
      name: 'doctor-fixture',
      address: 'doctor.example',
      state: 'unauthorized',
      agentIdentity: { hostname: 'doctor.example', commandsEnabled: false },
    },
  } as InfrastructureAgentDoctorTarget;
  const removedTarget = {
    ...target,
    key: 'agent:removed-fixture',
    connectionId: 'agent:removed-fixture',
    displayName: 'removed-fixture',
    status: 'removed',
    needsCredentialRepair: false,
    connection: undefined,
    diagnostic: { agentId: 'removed-agent-42', hostname: 'removed.example' },
  } as InfrastructureAgentDoctorTarget;
  const removedWindows = {
    ...removedTarget,
    key: 'agent:removed-windows',
    connectionId: 'agent:removed-windows',
    displayName: 'removed-windows',
    commandPlatform: 'windows',
    diagnostic: { agentId: 'removed-windows-42', hostname: 'removed-windows.example' },
  } as InfrastructureAgentDoctorTarget;
  return <InfrastructureAgentDoctorPage targets={[target, removedTarget, removedWindows]} />;
};
const baseProps = () =>
  ({
    selectedAgent: () => 'pve',
    onSelectAgent: () => {},
    initialLoadComplete: () => true,
    discoveryEnabled: () => false,
    discoveryMode: () => 'auto',
    discoverySubnetDraft: () => '',
    discoverySubnetError: () => undefined,
    discoveryScanStatus: () => ({ scanning: false }),
    discoveredNodes: () => [],
    savingDiscoverySettings: () => false,
    envOverrides: () => ({}),
    agentStateResources: () => [],
    pbsInstances: () => [],
    pmgInstances: () => [],
    pveNodes: () => [],
    pbsNodes: () => [],
    pmgNodes: () => [],
    trueNASSettings: {
      closeDialog: () => {},
      connections: () => [],
    } as any,
    vmwareSettings: {
      closeDialog: () => {},
      connections: () => [],
    } as any,
    temperatureMonitoringEnabled: () => false,
    triggerDiscoveryScan: () => {},
    loadDiscoveredNodes: () => {},
    handleDiscoveryEnabledChange: () => {},
    handleDiscoveryModeChange: () => {},
    setDiscoveryMode: () => {},
    setDiscoverySubnetDraft: () => {},
    setDiscoverySubnetError: () => {},
    setLastCustomSubnet: () => {},
    commitDiscoverySubnet: () => {},
    parseSubnetList: () => [],
    normalizeSubnetList: (value: string) => value,
    isValidCIDR: () => true,
    currentDraftSubnetValue: () => '',
    discoverySubnetInputRef: () => {},
    testNodeConnection: () => {},
    requestDeleteNode: () => {},
    refreshClusterNodes: () => {},
    setShowNodeModal: () => {},
    editingNode: () => null,
    setEditingNode: () => {},
    setCurrentNodeType: () => {},
    modalResetKey: () => 0,
    setModalResetKey: () => {},
    isNodeModalVisible: () => false,
    securityStatus: () => null,
    resolveTemperatureMonitoringEnabled: () => false,
    temperatureMonitoringLocked: () => false,
    savingTemperatureSetting: () => false,
    handleTemperatureMonitoringChange: () => {},
    disableDockerUpdateActions: () => false,
    disableDockerUpdateActionsLocked: () => false,
    savingDockerUpdateActions: () => false,
    handleDisableDockerUpdateActionsChange: () => {},
    handleNodeTemperatureMonitoringChange: () => {},
    saveNode: () => {},
    showDeleteNodeModal: () => false,
    cancelDeleteNode: () => {},
    deleteNode: () => {},
    deleteNodeLoading: () => false,
    nodePendingDeleteLabel: () => '',
    nodePendingDeleteHost: () => '',
    nodePendingDeleteType: () => '',
    nodePendingDeleteTypeLabel: () => '',
  }) as any;

const Workspace = () => (
  <main class="mx-auto max-w-5xl p-4">
    <InfrastructureWorkspace {...baseProps()} />
  </main>
);
render(
  () => (
    <Router>
      <Route path="*" component={Fixture} />
    </Router>
  ),
  document.getElementById('root')!,
);
