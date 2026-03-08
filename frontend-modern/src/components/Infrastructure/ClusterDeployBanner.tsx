import { Component, Show, createMemo } from 'solid-js';
import type { ResourceGroup } from '@/components/Infrastructure/infrastructureSelectors';
import InfoIcon from 'lucide-solid/icons/info';
import RocketIcon from 'lucide-solid/icons/rocket';

interface ClusterDeployBannerProps {
  group: ResourceGroup;
  onDeploy: (clusterId: string, clusterName: string) => void;
}

/**
 * Compact inline banner shown in cluster group headers when unmonitored
 * PVE nodes exist and a source agent is available to deploy from.
 */
export const ClusterDeployBanner: Component<ClusterDeployBannerProps> = (props) => {
  // Visibility conditions:
  // 1. Non-empty cluster name (not Standalone)
  // 2. At least one resource has platformType === 'proxmox-pve'
  // 3. At least one resource has agent?.agentId (source agent exists)
  // 4. At least one PVE node does NOT have an agent (unmonitored)
  const deployInfo = createMemo(() => {
    const resources = props.group.resources;
    const cluster = props.group.cluster;

    if (!cluster) return null;

    // Only consider actual PVE node resources (not VMs/containers).
    const pveNodes = resources.filter(
      (r) => r.type === 'agent' && r.platformType === 'proxmox-pve',
    );
    if (pveNodes.length === 0) return null;

    const hasSourceAgent = pveNodes.some((r) => r.agent?.agentId);
    if (!hasSourceAgent) return null;

    const unmonitoredCount = pveNodes.filter((r) => !r.agent?.agentId).length;
    if (unmonitoredCount === 0) return null;

    return { cluster, unmonitoredCount };
  });

  return (
    <Show when={deployInfo()}>
      {(info) => (
        <div class="flex items-center gap-2 mt-1">
          <div class="flex items-center gap-1.5 text-[11px] text-blue-600 dark:text-blue-400">
            <InfoIcon class="w-3 h-3" />
            <span>
              {info().unmonitoredCount} node{info().unmonitoredCount !== 1 ? 's' : ''} unmonitored
            </span>
          </div>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              props.onDeploy(info().cluster, info().cluster);
            }}
            class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium text-blue-700 bg-blue-100 hover:bg-blue-200 dark:text-blue-300 dark:bg-blue-900 dark:hover:bg-blue-800 transition-colors"
          >
            <RocketIcon class="w-2.5 h-2.5" />
            Review & Deploy
          </button>
        </div>
      )}
    </Show>
  );
};
