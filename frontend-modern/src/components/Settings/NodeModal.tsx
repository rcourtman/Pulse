import { Component } from 'solid-js';
import { NodeModalAuthenticationSection } from '@/components/Settings/NodeModalAuthenticationSection';
import { NodeModalBasicInfoSection } from '@/components/Settings/NodeModalBasicInfoSection';
import { NodeModalMonitoringSection } from '@/components/Settings/NodeModalMonitoringSection';
import { NodeModalStatusFooter } from '@/components/Settings/NodeModalStatusFooter';
import type { NodeModalProps } from '@/components/Settings/nodeModalModel';
import { useNodeModalState } from '@/components/Settings/useNodeModalState';
import { Dialog } from '@/components/shared/Dialog';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { getNodeProductName } from '@/utils/nodeModalPresentation';

export const NodeModal: Component<NodeModalProps> = (props) => {
  const state = useNodeModalState(props);

  return (
    <Dialog
      isOpen={props.isOpen}
      onClose={props.onClose}
      panelClass="max-w-2xl"
      ariaLabel={`${state.isEditingExistingNode() ? 'Edit' : 'Add'} ${getNodeProductName(props.nodeType)} node`}
    >
      <div class="relative w-full">
        <form onSubmit={state.handleSubmit}>
          <div class="flex items-center justify-between p-4 border-b border-border">
            <SectionHeader
              title={`${state.isEditingExistingNode() ? 'Edit' : 'Add'} ${getNodeProductName(props.nodeType)} node`}
              size="md"
              class="flex-1"
            />
            <button type="button" onClick={props.onClose} class="text-slate-400 hover:text-muted">
              <svg
                width="20"
                height="20"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>

          <div class="p-6 space-y-6">
            <NodeModalBasicInfoSection modalProps={props} state={state} />
            <NodeModalAuthenticationSection modalProps={props} state={state} />
            <NodeModalMonitoringSection modalProps={props} state={state} />
          </div>

          <NodeModalStatusFooter modalProps={props} state={state} />
        </form>
      </div>
    </Dialog>
  );
};
