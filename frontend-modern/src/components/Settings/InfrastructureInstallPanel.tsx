import type { Component } from 'solid-js';
import { InfrastructureOperationsController } from './InfrastructureOperationsController';

export const InfrastructureInstallPanel: Component = () => (
  <InfrastructureOperationsController embedded showInventory={false} />
);

export default InfrastructureInstallPanel;
