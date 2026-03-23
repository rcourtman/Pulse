import { Component, Suspense } from 'solid-js';
import { DiscoveryTab } from '../Discovery/DiscoveryTab';
import { SystemInfoCard } from '@/components/shared/cards/SystemInfoCard';
import { HardwareCard } from '@/components/shared/cards/HardwareCard';
import { RootDiskCard } from '@/components/shared/cards/RootDiskCard';
import { NetworkInterfacesCard } from '@/components/shared/cards/NetworkInterfacesCard';
import { DisksCard } from '@/components/shared/cards/DisksCard';
import { WebInterfaceUrlField } from '@/components/shared/WebInterfaceUrlField';
import { getDiscoveryLoadingState } from '@/utils/discoveryPresentation';
import {
  resolveInfrastructureDetailsDrawerDiscoveryHostname,
  resolveInfrastructureDetailsDrawerMetadataId,
  type InfrastructureDetailsDrawerProps,
} from './infrastructureDetailsDrawerModel';
import { useInfrastructureDetailsDrawerState } from './useInfrastructureDetailsDrawerState';

export const InfrastructureDetailsDrawer: Component<InfrastructureDetailsDrawerProps> = (props) => {
  const drawer = useInfrastructureDetailsDrawerState();
  const metadataId = () => resolveInfrastructureDetailsDrawerMetadataId(props.node, props.agent);
  const discoveryHostname = () =>
    resolveInfrastructureDetailsDrawerDiscoveryHostname(props.node, props.agent);

  return (
    <div class="space-y-3">
      {/* Tabs */}
      <div class="flex items-center gap-6 border-b border-border px-1 mb-1">
        <button
          onClick={() => drawer.setActiveTab('overview')}
          class={`pb-2 text-sm font-medium transition-colors relative ${
            drawer.activeTab() === 'overview'
              ? 'text-blue-600 dark:text-blue-400'
              : ' hover:text-muted'
          }`}
        >
          Overview
          {drawer.activeTab() === 'overview' && (
            <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
          )}
        </button>
        <button
          onClick={() => drawer.setActiveTab('discovery')}
          class={`pb-2 text-sm font-medium transition-colors relative ${
            drawer.activeTab() === 'discovery'
              ? 'text-blue-600 dark:text-blue-400'
              : ' hover:text-muted'
          }`}
        >
          Discovery
          {drawer.activeTab() === 'discovery' && (
            <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 dark:bg-blue-400 rounded-t-full" />
          )}
        </button>
      </div>

      {/* Overview Tab */}
      <div
        class={drawer.activeTab() === 'overview' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        <div class="flex flex-wrap gap-3 [&>*]:flex-1 [&>*]:basis-[calc(25%-0.75rem)] [&>*]:min-w-[200px] [&>*]:max-w-full [&>*]:overflow-hidden">
          <SystemInfoCard variant="node" node={props.node} />
          <HardwareCard variant="node" node={props.node} />
          <RootDiskCard node={props.node} />
          <NetworkInterfacesCard interfaces={props.agent?.networkInterfaces} />
          <DisksCard disks={props.agent?.disks} />
        </div>
        <div class="mt-3">
          <WebInterfaceUrlField
            metadataKind="agent"
            metadataId={metadataId()}
            targetLabel="agent"
            customUrl={props.customUrl}
            onCustomUrlChange={(url) => props.onCustomUrlChange?.(metadataId(), url)}
          />
        </div>
      </div>

      {/* Discovery Tab */}
      <div
        class={drawer.activeTab() === 'discovery' ? '' : 'hidden'}
        style={{ 'overflow-anchor': 'none' }}
      >
        <Suspense
          fallback={
            <div class="flex items-center justify-center py-8">
              <div class="animate-spin h-6 w-6 border-2 border-blue-500 border-t-transparent rounded-full" />
              <span class="ml-2 text-sm text-muted">{getDiscoveryLoadingState().text}</span>
            </div>
          }
        >
          <DiscoveryTab
            resourceType="agent"
            agentId={metadataId()}
            resourceId={metadataId()}
            hostname={discoveryHostname()}
          />
        </Suspense>
      </div>
    </div>
  );
};
