import type { Component } from 'solid-js';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { InfrastructurePlatformConnectionsSummaryCard } from './InfrastructurePlatformConnectionsSummaryCard';
import { InfrastructureInventorySection } from './InfrastructureInventorySection';
import { InfrastructureStopMonitoringDialog } from './InfrastructureStopMonitoringDialog';
import { InfrastructureOperationsStateProvider } from './useInfrastructureOperationsState';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';

interface InfrastructureReportingPanelProps extends InfrastructurePlatformSettingsProps {
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
          pveCount={props.platformConnectionsSummary().pveCount}
          pbsCount={props.platformConnectionsSummary().pbsCount}
          pmgCount={props.platformConnectionsSummary().pmgCount}
          truenasCount={props.platformConnectionsSummary().truenasCount}
          truenasAvailable={props.platformConnectionsSummary().truenasAvailable}
          vmwareCount={props.platformConnectionsSummary().vmwareCount}
          vmwareAvailable={props.platformConnectionsSummary().vmwareAvailable}
          onManagePlatformConnections={props.onManagePlatformConnections}
        />
      </div>

      <AgentProfilesPanel />
    </div>
  );
};

export default InfrastructureReportingPanel;
