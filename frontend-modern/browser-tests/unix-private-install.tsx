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
const Fixture = () => (
  <InfrastructureOperationsStateProvider embedded>
    <main class="mx-auto max-w-5xl p-4 bg-base text-base-content">
      <h1 class="text-xl mb-4">Private Unix agent installation</h1>
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
  return <InfrastructureAgentDoctorPage targets={[target, removedTarget]} />;
};
render(
  () => (
    <Router>
      <Route path="*" component={Fixture} />
    </Router>
  ),
  document.getElementById('root')!,
);
