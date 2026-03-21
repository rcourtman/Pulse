import { Show, type Component } from 'solid-js';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import { NodeModal } from './NodeModal';
import type { NodeType } from './infrastructureSettingsModel';

interface ProxmoxNodeModalStackProps {
  modalResetKey: number;
  prefillNode: Partial<NodeConfig> | null;
  editingNodeType: NodeType | null;
  editingNode: () => NodeConfigWithStatus | null;
  isNodeModalVisible: (type: NodeType) => boolean;
  securityStatus: SecurityStatusInfo | null;
  resolveTemperatureMonitoringEnabled: (node?: NodeConfigWithStatus | null) => boolean;
  temperatureMonitoringLocked: boolean;
  savingTemperatureSetting: boolean;
  handleNodeTemperatureMonitoringChange: (nodeId: string, enabled: boolean | null) => Promise<void>;
  handleTemperatureMonitoringChange: (enabled: boolean) => Promise<void>;
  saveNode: (nodeData: Partial<NodeConfig>) => Promise<void>;
  onClose: () => void;
}

const PROXMOX_NODE_TYPES: readonly NodeType[] = ['pve', 'pbs', 'pmg'];

export const ProxmoxNodeModalStack: Component<ProxmoxNodeModalStackProps> = (props) => {
  return (
    <>
      {PROXMOX_NODE_TYPES.map((type) => (
        <Show when={props.isNodeModalVisible(type)}>
          <NodeModal
            isOpen={true}
            resetKey={props.modalResetKey}
            onClose={props.onClose}
            nodeType={type}
            editingNode={props.editingNodeType === type ? props.editingNode() ?? undefined : undefined}
            prefillNode={props.prefillNode?.type === type ? props.prefillNode ?? undefined : undefined}
            securityStatus={props.securityStatus ?? undefined}
            temperatureMonitoringEnabled={props.resolveTemperatureMonitoringEnabled(
              props.editingNodeType === type ? props.editingNode() : null,
            )}
            temperatureMonitoringLocked={props.temperatureMonitoringLocked}
            savingTemperatureSetting={props.savingTemperatureSetting}
            onToggleTemperatureMonitoring={
              props.editingNode()?.id
                ? (enabled: boolean) =>
                    props.handleNodeTemperatureMonitoringChange(props.editingNode()!.id, enabled)
                : props.handleTemperatureMonitoringChange
            }
            onSave={props.saveNode}
          />
        </Show>
      ))}
    </>
  );
};

export default ProxmoxNodeModalStack;
