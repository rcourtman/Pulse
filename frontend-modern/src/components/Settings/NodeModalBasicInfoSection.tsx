import { Component, Show } from 'solid-js';
import type { NodeModalProps } from '@/components/Settings/nodeModalModel';
import type { NodeModalState } from '@/components/Settings/useNodeModalState';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { controlClass, formField, formHelpText, labelClass } from '@/components/shared/Form';
import {
  getNodeEndpointHelp,
  getNodeEndpointPlaceholder,
  getNodeGuestUrlPlaceholder,
} from '@/utils/nodeModalPresentation';

interface NodeModalBasicInfoSectionProps {
  modalProps: NodeModalProps;
  state: NodeModalState;
}

export const NodeModalBasicInfoSection: Component<NodeModalBasicInfoSectionProps> = (props) => {
  const { modalProps, state } = props;

  return (
    <div>
      <SectionHeader
        title="Basic information"
        size="sm"
        class="mb-4"
        titleClass="text-base-content"
      />
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class={formField}>
          <label class={labelClass('flex items-center gap-2')}>
            Node Name <span class="text-red-500">*</span>
          </label>
          <input
            type="text"
            value={state.formData().name}
            onInput={(event) => state.updateField('name', event.currentTarget.value)}
            placeholder="Pulse uses this label across dashboards"
            required
            class={controlClass()}
          />
          <p class={formHelpText}>
            Required and must be unique. We can auto-fill it from the Endpoint URL if you leave it
            blank.
          </p>
        </div>

        <div class={formField}>
          <label class={labelClass('flex items-center gap-1')}>
            Endpoint URL <span class="text-red-500">*</span>
          </label>
          <input
            type="text"
            value={state.formData().host}
            onInput={(event) => state.updateField('host', event.currentTarget.value)}
            placeholder={getNodeEndpointPlaceholder(modalProps.nodeType)}
            required
            class={controlClass()}
          />
          <Show when={getNodeEndpointHelp(modalProps.nodeType)}>
            {(help) => <p class={formHelpText}>{help()}</p>}
          </Show>
        </div>

        <div class={formField}>
          <label class={labelClass('flex items-center gap-1')}>
            Guest URL <span class="text-slate-500 text-xs font-normal">(Optional)</span>
          </label>
          <input
            type="text"
            value={state.formData().guestURL}
            onInput={(event) => state.updateField('guestURL', event.currentTarget.value)}
            placeholder={getNodeGuestUrlPlaceholder(modalProps.nodeType)}
            class={controlClass()}
          />
          <p class={formHelpText}>
            Optional guest-accessible URL for navigation. If specified, this URL will be used when
            opening the web UI instead of the Endpoint URL.
          </p>
        </div>
      </div>
    </div>
  );
};
