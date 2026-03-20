import type { Component } from 'solid-js';
import { Show } from 'solid-js';
import {
  type InfrastructureOperationsStateOptions,
  useInfrastructureOperationsState,
} from './useInfrastructureOperationsState';

export interface InfrastructureOperationsControllerProps extends InfrastructureOperationsStateOptions {
  showInstaller?: boolean;
  showInventory?: boolean;
}

export const InfrastructureOperationsController: Component<
  InfrastructureOperationsControllerProps
> = (props) => {
  const state = useInfrastructureOperationsState({ embedded: props.embedded });

  return (
    <div class="space-y-6">
      <Show when={props.showInventory ?? true}>{state.renderStopMonitoringDialog()}</Show>
      <Show when={props.showInstaller ?? true}>{state.renderInstallerSection()}</Show>
      <Show when={props.showInventory ?? true}>{state.renderInventorySection()}</Show>
    </div>
  );
};

export default InfrastructureOperationsController;
