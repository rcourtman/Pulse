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
  onToggleEnabled?: () => void;
  togglePending?: boolean;
  connectionEnabled?: boolean;
  onDelete?: () => void;
  deletePending?: boolean;
  deleteConfirming?: boolean;
  deleteError?: string | null;
}

export const NodeCredentialSlot: Component<NodeCredentialSlotProps> = (props) => {
  const prefill: Partial<NodeConfig> | undefined =
    props.prefillNode ?? (props.initialAddress ? { host: props.initialAddress } : undefined);

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
      <section
        aria-label="Infrastructure connection sequence"
        class="rounded-md border border-border bg-surface-alt px-4 py-3"
      >
        <div class="grid gap-3 text-sm sm:grid-cols-3">
          <div class="flex items-start gap-2">
            <span class="mt-0.5 inline-flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-blue-600 text-xs font-semibold text-white">
              1
            </span>
            <div>
              <div class="font-medium text-base-content">Endpoint</div>
              <div class="text-xs text-muted">Name and API address</div>
            </div>
          </div>
          <div class="flex items-start gap-2">
            <span class="mt-0.5 inline-flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-blue-600 text-xs font-semibold text-white">
              2
            </span>
            <div>
              <div class="font-medium text-base-content">Authentication</div>
              <div class="text-xs text-muted">Assisted setup or manual token</div>
            </div>
          </div>
          <div class="flex items-start gap-2">
            <span class="mt-0.5 inline-flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-blue-600 text-xs font-semibold text-white">
              3
            </span>
            <div>
              <div class="font-medium text-base-content">Coverage</div>
              <div class="text-xs text-muted">Monitoring scope and save</div>
            </div>
          </div>
        </div>
      </section>
      <NodeModalBasicInfoSection modalProps={modalProps} state={state} />
      <NodeModalAuthenticationSection modalProps={modalProps} state={state} />
      <NodeModalMonitoringSection modalProps={modalProps} state={state} />
      <NodeModalStatusFooter
        modalProps={modalProps}
        state={state}
        onToggleEnabled={props.onToggleEnabled}
        togglePending={props.togglePending}
        connectionEnabled={props.connectionEnabled}
        onDelete={props.onDelete}
        deletePending={props.deletePending}
        deleteConfirming={props.deleteConfirming}
        deleteError={props.deleteError}
      />
    </form>
  );
};
