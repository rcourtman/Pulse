import { Show, type JSX } from 'solid-js';
import Boxes from 'lucide-solid/icons/boxes';
import Network from 'lucide-solid/icons/network';
import Server from 'lucide-solid/icons/server';
import SquareStack from 'lucide-solid/icons/square-stack';
import Tags from 'lucide-solid/icons/tags';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';
import type { Resource } from '@/features/alerts/thresholds/tableTypes';

const KUBERNETES_WORKLOAD_COLUMNS = [
  'CPU %',
  'Memory %',
  'Disk %',
  'Disk R MB/s',
  'Disk W MB/s',
  'Net In MB/s',
  'Net Out MB/s',
];
const KUBERNETES_NODE_COLUMNS = ['CPU %', 'Memory %', 'Disk %'];

function KubernetesResourceSection(
  props: ThresholdsTableSectionProps & {
    id: string;
    title: string;
    resources: () => Resource[];
    columns: string[];
    icon: JSX.Element;
    typeKey: 'k8s-cluster' | 'k8s-node' | 'k8s-namespace' | 'k8s-deployment' | 'pod';
    defaults?: Record<string, number | undefined>;
  },
) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection(props.id)}>
      <CollapsibleSection
        id={props.id}
        title={props.title}
        resourceCount={props.resources().length}
        collapsed={state.isCollapsed(props.id)}
        onToggle={() => state.toggleSection(props.id)}
        icon={props.icon}
        isGloballyDisabled={tableProps.disableAllKubernetes()}
        emptyMessage="No Kubernetes alert targets match the current filters."
      >
        <div ref={state.registerSection(props.id)} class="scroll-mt-24">
          <ResourceTable
            title=""
            resources={props.resources()}
            columns={props.columns}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage="No Kubernetes alert targets match the current filters."
            onEdit={state.startEditing}
            onSaveEdit={state.saveEdit}
            onCancelEdit={state.cancelEdit}
            onRemoveOverride={state.removeOverride}
            onToggleDisabled={state.toggleDisabled}
            showOfflineAlertsColumn={false}
            editingId={state.editingId}
            editingThresholds={state.editingThresholds}
            setEditingThresholds={state.setEditingThresholds}
            editingNote={state.editingNote}
            setEditingNote={state.setEditingNote}
            onBulkEdit={(ids) => state.handleBulkEdit(ids, props.columns)}
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            globalDefaults={props.defaults}
            setGlobalDefaults={props.defaults ? tableProps.setKubernetesDefaults : undefined}
            setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
            globalDisableFlag={tableProps.disableAllKubernetes}
            onToggleGlobalDisable={() =>
              tableProps.setDisableAllKubernetes(!tableProps.disableAllKubernetes())
            }
            showDelayColumn={props.columns.length > 0}
            globalDelaySeconds={tableProps.timeThresholds()[props.typeKey]}
            metricDelaySeconds={tableProps.metricTimeThresholds()[props.typeKey] ?? {}}
            onMetricDelayChange={(metric, value) =>
              state.updateMetricDelay(props.typeKey, metric, value)
            }
            factoryDefaults={props.defaults ? tableProps.factoryKubernetesDefaults : undefined}
            onResetDefaults={props.defaults ? tableProps.resetKubernetesDefaults : undefined}
          />
        </div>
      </CollapsibleSection>
    </Show>
  );
}

export function ThresholdsTableKubernetesTab(props: ThresholdsTableSectionProps) {
  const defaults = () => props.tableProps.kubernetesDefaults ?? {};

  return (
    <>
      <KubernetesResourceSection
        {...props}
        id="kubernetesClusters"
        title="Clusters"
        resources={props.state.kubernetesClustersWithOverrides}
        columns={KUBERNETES_WORKLOAD_COLUMNS}
        icon={<Network class="w-5 h-5" />}
        typeKey="k8s-cluster"
        defaults={defaults()}
      />
      <KubernetesResourceSection
        {...props}
        id="kubernetesNodes"
        title="Nodes"
        resources={props.state.kubernetesNodesWithOverrides}
        columns={KUBERNETES_NODE_COLUMNS}
        icon={<Server class="w-5 h-5" />}
        typeKey="k8s-node"
        defaults={defaults()}
      />
      <KubernetesResourceSection
        {...props}
        id="kubernetesNamespaces"
        title="Namespaces"
        resources={props.state.kubernetesNamespacesWithOverrides}
        columns={[]}
        icon={<Tags class="w-5 h-5" />}
        typeKey="k8s-namespace"
      />
      <KubernetesResourceSection
        {...props}
        id="kubernetesDeployments"
        title="Deployments"
        resources={props.state.kubernetesDeploymentsWithOverrides}
        columns={KUBERNETES_WORKLOAD_COLUMNS}
        icon={<SquareStack class="w-5 h-5" />}
        typeKey="k8s-deployment"
        defaults={defaults()}
      />
      <KubernetesResourceSection
        {...props}
        id="kubernetesPods"
        title="Pods"
        resources={props.state.kubernetesPodsWithOverrides}
        columns={KUBERNETES_WORKLOAD_COLUMNS}
        icon={<Boxes class="w-5 h-5" />}
        typeKey="pod"
        defaults={defaults()}
      />
    </>
  );
}
