import { Component } from 'solid-js';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import { NodeModalAuthenticationSection } from '@/components/Settings/NodeModalAuthenticationSection';
import { NodeModalBasicInfoSection } from '@/components/Settings/NodeModalBasicInfoSection';
import { NodeModalMonitoringSection } from '@/components/Settings/NodeModalMonitoringSection';
import { NodeModalStatusFooter } from '@/components/Settings/NodeModalStatusFooter';
import { useNodeModalState } from '@/components/Settings/useNodeModalState';
import type { NodeModalProps } from '@/components/Settings/nodeModalModel';
import type { InfrastructurePlatformSettingsProps } from '@/components/Settings/proxmoxSettingsModel';

export type NodeSlotType = 'pve' | 'pbs' | 'pmg';

export interface NodeCredentialSlotProps {
  nodeType: NodeSlotType;
  settings: InfrastructurePlatformSettingsProps;
  editingNode?: NodeConfigWithStatus | null;
  prefillNode?: Partial<NodeConfig>;
  initialAddress?: string;
  onCancel: () => void;
  onSaved: () => void;
  onDelete?: () => void;
  deletePending?: boolean;
  deleteConfirming?: boolean;
  deleteError?: string | null;
}

export const NodeCredentialSlot: Component<NodeCredentialSlotProps> = (props) => {
  const prefill: Partial<NodeConfig> | undefined = props.prefillNode ?? (
    props.initialAddress ? { host: props.initialAddress } : undefined
  );

  const handleSave = async (nodeData: Partial<NodeConfig>) => {
    await props.settings.saveNode(nodeData);
    props.onSaved();
  };

  const modalProps: NodeModalProps = {
    isOpen: true,
    nodeType: props.nodeType,
    editingNode: props.editingNode ?? undefined,
    prefillNode: prefill,
    onClose: props.onCancel,
    onSave: handleSave,
    securityStatus: props.settings.securityStatus() ?? undefined,
    temperatureMonitoringEnabled: props.settings.resolveTemperatureMonitoringEnabled(
      props.editingNode ?? null,
    ),
    temperatureMonitoringLocked: props.settings.temperatureMonitoringLocked(),
    savingTemperatureSetting: props.settings.savingTemperatureSetting(),
    onToggleTemperatureMonitoring: async (enabled) => {
      const target = props.editingNode;
      if (target?.id) {
        await props.settings.handleNodeTemperatureMonitoringChange(target.id, enabled);
      } else {
        await props.settings.handleTemperatureMonitoringChange(enabled);
      }
    },
  };

  const state = useNodeModalState(modalProps);

  return (
    <form onSubmit={state.handleSubmit} class="space-y-6">
      <NodeModalBasicInfoSection modalProps={modalProps} state={state} />
      <NodeModalAuthenticationSection modalProps={modalProps} state={state} />
      <NodeModalMonitoringSection modalProps={modalProps} state={state} />
      <NodeModalStatusFooter
        modalProps={modalProps}
        state={state}
        onDelete={props.onDelete}
        deletePending={props.deletePending}
        deleteConfirming={props.deleteConfirming}
        deleteError={props.deleteError}
      />
    </form>
  );
};
