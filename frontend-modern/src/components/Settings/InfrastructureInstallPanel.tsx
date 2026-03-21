import type { Component } from 'solid-js';
import { InfrastructureInstallerSection } from './InfrastructureInstallerSection';
import { InfrastructureOperationsStateProvider } from './useInfrastructureOperationsState';

export const InfrastructureInstallPanel: Component = () => {
  return (
    <InfrastructureOperationsStateProvider embedded>
      <InfrastructureInstallerSection />
    </InfrastructureOperationsStateProvider>
  );
};

export default InfrastructureInstallPanel;
