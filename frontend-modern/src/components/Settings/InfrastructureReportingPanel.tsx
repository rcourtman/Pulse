import type { Component } from 'solid-js';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { InfrastructurePlatformConnectionsSummaryCard } from './InfrastructurePlatformConnectionsSummaryCard';
import { InfrastructureInventorySection } from './InfrastructureInventorySection';
import { InfrastructureStopMonitoringDialog } from './InfrastructureStopMonitoringDialog';
import type { ProxmoxSettingsPanelProps } from './proxmoxSettingsModel';
import { InfrastructureOperationsStateProvider } from './useInfrastructureOperationsState';

interface InfrastructureReportingPanelProps extends ProxmoxSettingsPanelProps {
  onManagePlatformConnections: () => void;
}

export const InfrastructureReportingPanel: Component<InfrastructureReportingPanelProps> = (
  props,
) => {
  return (
    <div class="space-y-6">
      <InfrastructureOperationsStateProvider>
        <InfrastructureStopMonitoringDialog />
        <InfrastructureInventorySection />
      </InfrastructureOperationsStateProvider>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]">
        <InfrastructurePlatformConnectionsSummaryCard
          pveCount={props.pveNodes().length}
          pbsCount={props.pbsNodes().length}
          pmgCount={props.pmgNodes().length}
          onManagePlatformConnections={props.onManagePlatformConnections}
        />
      </div>

      <AgentProfilesPanel />
    </div>
  );
};

export default InfrastructureReportingPanel;
