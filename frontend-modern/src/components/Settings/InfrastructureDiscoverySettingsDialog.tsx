import { Component, Show } from 'solid-js';
import X from 'lucide-solid/icons/x';
import { Dialog } from '@/components/shared/Dialog';
import { DiscoverySettingsForm } from './DiscoverySettingsForm';
import type { DiscoverySettingsFormProps } from './discoverySettingsModel';

interface InfrastructureDiscoverySettingsDialogProps extends DiscoverySettingsFormProps {
  isOpen: boolean;
  onClose: () => void;
}

const closeButtonClass =
  'inline-flex h-9 w-9 items-center justify-center rounded-md border border-border text-base-content transition-colors hover:bg-surface-hover';

export const InfrastructureDiscoverySettingsDialog: Component<
  InfrastructureDiscoverySettingsDialogProps
> = (props) => {
  return (
    <Show when={props.isOpen}>
      <Dialog
        isOpen={true}
        onClose={props.onClose}
        ariaLabel="Discovery settings"
        panelClass="max-w-3xl"
      >
        <div class="flex h-full min-h-0 flex-col">
          <div class="flex items-start justify-between gap-4 border-b border-border bg-surface-alt px-4 py-4 sm:px-6">
            <div class="space-y-1">
              <h2 class="text-base font-semibold text-base-content">Discovery settings</h2>
              <p class="text-sm text-muted">
                Configure the saved network scope and background scan behavior for infrastructure
                source discovery.
              </p>
            </div>
            <button
              type="button"
              onClick={props.onClose}
              class={closeButtonClass}
              aria-label="Close discovery settings dialog"
            >
              <X class="h-4 w-4" />
            </button>
          </div>

          <div class="min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-6">
            <DiscoverySettingsForm {...props} />
          </div>
        </div>
      </Dialog>
    </Show>
  );
};
