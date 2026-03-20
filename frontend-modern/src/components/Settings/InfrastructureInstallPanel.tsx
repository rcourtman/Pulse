import type { Component } from 'solid-js';
import { useInfrastructureOperationsState } from './useInfrastructureOperationsState';

export const InfrastructureInstallPanel: Component = () => {
  const state = useInfrastructureOperationsState({ embedded: true });

  return state.renderInstallerSection();
};

export default InfrastructureInstallPanel;
