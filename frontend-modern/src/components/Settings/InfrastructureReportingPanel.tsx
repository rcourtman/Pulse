import type { Component } from 'solid-js';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { InfrastructureDirectConnectionsSummaryCard } from './InfrastructureDirectConnectionsSummaryCard';
import { InfrastructureInventorySection } from './InfrastructureInventorySection';
import { InfrastructureStopMonitoringDialog } from './InfrastructureStopMonitoringDialog';
import type { ProxmoxSettingsPanelProps } from './proxmoxSettingsModel';
import { InfrastructureOperationsStateProvider } from './useInfrastructureOperationsState';

interface InfrastructureReportingPanelProps extends ProxmoxSettingsPanelProps {
  onManageDirectConnections: () => void;
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
        <InfrastructureDirectConnectionsSummaryCard
          pveCount={props.pveNodes().length}
          pbsCount={props.pbsNodes().length}
          pmgCount={props.pmgNodes().length}
          onManageDirectConnections={props.onManageDirectConnections}
        />
      </div>

      <AgentProfilesPanel />
    </div>
  );
};

export default InfrastructureReportingPanel;
