import { Component, Show } from 'solid-js';
import { InfrastructureSummaryTable } from './InfrastructureSummaryTable';
import {
  type InfrastructureSelectorProps,
  useInfrastructureSelectorState,
} from './useInfrastructureSelectorState';

export type { InfrastructureSelectorProps } from './useInfrastructureSelectorState';

export const InfrastructureSelector: Component<InfrastructureSelectorProps> = (props) => {
  const infrastructureSelector = useInfrastructureSelectorState(props);

  return (
    <Show when={infrastructureSelector.showNodeSummary()}>
      <div class="mb-4 space-y-2">
        <InfrastructureSummaryTable
          nodes={infrastructureSelector.nodes()}
          pbsInstances={
            props.currentTab === 'recovery' ? infrastructureSelector.pbsInstances() : undefined
          }
          vmCounts={infrastructureSelector.vmCounts()}
          containerCounts={infrastructureSelector.containerCounts()}
          storageCounts={infrastructureSelector.storageCounts()}
          diskCounts={infrastructureSelector.diskCounts()}
          agents={infrastructureSelector.agentsForNodeSummary()}
          backupCounts={infrastructureSelector.backupCounts()}
          currentTab={props.currentTab}
          selectedNode={infrastructureSelector.selectedNode()}
          globalTemperatureMonitoringEnabled={props.globalTemperatureMonitoringEnabled}
          onNodeClick={infrastructureSelector.handleNodeClick}
        />
      </div>
    </Show>
  );
};

export default InfrastructureSelector;
