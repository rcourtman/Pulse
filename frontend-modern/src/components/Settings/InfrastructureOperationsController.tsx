import type { Component } from 'solid-js';
import { Show } from 'solid-js';
import { InfrastructureInstallerSection } from './InfrastructureInstallerSection';
import { InfrastructureInventorySection } from './InfrastructureInventorySection';
import { InfrastructureStopMonitoringDialog } from './InfrastructureStopMonitoringDialog';
import {
  InfrastructureOperationsStateProvider,
  type InfrastructureOperationsStateOptions,
} from './useInfrastructureOperationsState';

export interface InfrastructureOperationsControllerProps extends InfrastructureOperationsStateOptions {
  showInstaller?: boolean;
  showInventory?: boolean;
}

export const InfrastructureOperationsController: Component<
  InfrastructureOperationsControllerProps
> = (props) => {
  return (
    <InfrastructureOperationsStateProvider embedded={props.embedded}>
      <div class="space-y-6">
        <Show when={props.showInventory ?? true}>
          <InfrastructureStopMonitoringDialog />
        </Show>
        <Show when={props.showInstaller ?? true}>
          <InfrastructureInstallerSection />
        </Show>
        <Show when={props.showInventory ?? true}>
          <InfrastructureInventorySection />
        </Show>
      </div>
    </InfrastructureOperationsStateProvider>
  );
};

export default InfrastructureOperationsController;
