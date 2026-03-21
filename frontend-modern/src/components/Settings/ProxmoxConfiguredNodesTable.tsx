import type { Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import type { PBSInstance, PMGInstance } from '@/types/api';
import type { NodeConfigWithStatus } from '@/types/nodes';
import { PveNodesTable, PbsNodesTable, PmgNodesTable } from './ConfiguredNodeTables';
import type { NodeType } from './infrastructureSettingsModel';

interface ProxmoxConfiguredNodesTableProps {
  activeAgent: NodeType;
  pveNodes: NodeConfigWithStatus[];
  pbsNodes: NodeConfigWithStatus[];
  pmgNodes: NodeConfigWithStatus[];
  agentStateResources: Resource[];
  pbsInstances: PBSInstance[];
  pmgInstances: PMGInstance[];
  temperatureMonitoringEnabled: boolean;
  onTestConnection: (nodeId: string) => void;
  onEditNode: (type: NodeType, node: NodeConfigWithStatus) => void;
  onDeleteNode: (node: NodeConfigWithStatus) => void;
  onRefreshClusterNodes: (nodeId: string) => Promise<void>;
}

export const ProxmoxConfiguredNodesTable: Component<ProxmoxConfiguredNodesTableProps> = (
  props,
) => {
  switch (props.activeAgent) {
    case 'pve':
      return (
        <PveNodesTable
          nodes={props.pveNodes}
          stateNodes={props.agentStateResources}
          globalTemperatureMonitoringEnabled={props.temperatureMonitoringEnabled}
          onTestConnection={props.onTestConnection}
          onEdit={(node) => props.onEditNode('pve', node)}
          onDelete={props.onDeleteNode}
          onRefreshCluster={props.onRefreshClusterNodes}
        />
      );
    case 'pbs':
      return (
        <PbsNodesTable
          nodes={props.pbsNodes}
          statePbs={props.pbsInstances}
          globalTemperatureMonitoringEnabled={props.temperatureMonitoringEnabled}
          onTestConnection={props.onTestConnection}
          onEdit={(node) => props.onEditNode('pbs', node)}
          onDelete={props.onDeleteNode}
        />
      );
    case 'pmg':
      return (
        <PmgNodesTable
          nodes={props.pmgNodes}
          statePmg={props.pmgInstances}
          globalTemperatureMonitoringEnabled={props.temperatureMonitoringEnabled}
          onTestConnection={props.onTestConnection}
          onEdit={(node) => props.onEditNode('pmg', node)}
          onDelete={props.onDeleteNode}
        />
      );
  }
};

export default ProxmoxConfiguredNodesTable;
