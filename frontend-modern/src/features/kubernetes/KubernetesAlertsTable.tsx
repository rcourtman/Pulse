import { For, Show, type Component, type JSX } from 'solid-js';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
  type DetailValueTone,
} from '@/components/shared/DetailSectionTable';
import { AlertSeverityBadge, AlertSeverityDot } from '@/components/shared/AlertSeverityBadge';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { filterChipStatusDot } from '@/components/shared/FilterBar';
import {
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformTableFilterOption,
  PlatformTableShell,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { ResourceType } from '@/types/resource';
import { getAlertFilteredEmptyState } from '@/utils/alertOverviewPresentation';
import { formatAlertSeverityLabel } from '@/utils/alertSeverityPresentation';
import {
  filterKubernetesIncidents,
  type KubernetesIncidentRow,
  type KubernetesIncidentSeverityFilter,
} from './kubernetesPageModel';

const KUBERNETES_INCIDENT_STATUS_OPTIONS: PlatformTableFilterOption<KubernetesIncidentSeverityFilter>[] =
  [
    { value: 'all', label: 'All' },
    {
      value: 'critical',
      label: 'Critical',
      tone: 'danger',
      leading: filterChipStatusDot('bg-red-500'),
    },
    {
      value: 'warning',
      label: 'Warning',
      tone: 'warning',
      leading: filterChipStatusDot('bg-amber-500'),
    },
    {
      value: 'info',
      label: 'Info',
      tone: 'success',
      leading: filterChipStatusDot('bg-emerald-500'),
    },
  ];

const formatResourceType = (type: ResourceType): string => {
  switch (type) {
    case 'agent':
      return 'Node';
    case 'k8s-cluster':
      return 'Cluster';
    case 'k8s-node':
      return 'Node';
    case 'pod':
      return 'Pod';
    case 'k8s-deployment':
      return 'Deployment';
    case 'k8s-replicaset':
      return 'ReplicaSet';
    case 'k8s-statefulset':
      return 'StatefulSet';
    case 'k8s-daemonset':
      return 'DaemonSet';
    case 'k8s-job':
      return 'Job';
    case 'k8s-cronjob':
      return 'CronJob';
    case 'k8s-service':
      return 'Service';
    case 'k8s-ingress':
      return 'Ingress';
    case 'k8s-namespace':
      return 'Namespace';
    case 'k8s-event':
      return 'Event';
    case 'k8s-persistent-volume':
      return 'PersistentVolume';
    case 'k8s-persistent-volume-claim':
      return 'PVC';
    default:
      return type;
  }
};

const formatCode = (code: string): string => {
  const normalized = code.trim().replace(/^k8s_/, '');
  if (!normalized) return '-';
  return normalized
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');
};

const formatStartedAt = (value: string | undefined): string => {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return '-';
  if (parsed.getUTCFullYear() < 2000) return '-';
  return parsed.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const detailDateTime = (value?: string): string => {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  if (parsed.getUTCFullYear() < 2000) return '-';
  return parsed.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

type AlertDetailTone = DetailValueTone;
type AlertDetailSection = DetailSection;

const detailRow = makeDetailRow;

const alertTone = (severity: KubernetesIncidentRow['severityBucket']): AlertDetailTone => {
  if (severity === 'critical') return 'danger';
  if (severity === 'warning') return 'warning';
  return 'muted';
};

const buildAlertDetailSections = (incident: KubernetesIncidentRow): AlertDetailSection[] => {
  const k = incident.resource.kubernetes;
  const owner = k?.ownerKind && k?.ownerName ? `${k.ownerKind}/${k.ownerName}` : undefined;
  return compactDetailSections([
    {
      label: 'Alert',
      rows: compactDetailRows([
        detailRow('Severity', formatAlertSeverityLabel(incident.severity), {
          tone: alertTone(incident.severityBucket),
        }),
        detailRow('Summary', incident.summary),
        detailRow('Signal', incident.label),
        detailRow('Code', formatCode(incident.code), { title: incident.code }),
      ]),
    },
    {
      label: 'Affected resource',
      rows: compactDetailRows([
        detailRow('Name', incident.resourceName),
        detailRow('Type', formatResourceType(incident.resourceType)),
        detailRow('Cluster', k?.clusterName || k?.clusterId),
        detailRow('Namespace', k?.namespace),
        detailRow('Node', k?.nodeName),
        detailRow('Owner', owner),
        detailRow('Resource ID', incident.resourceId),
      ]),
    },
    {
      label: 'Source',
      rows: compactDetailRows([
        detailRow('Started', detailDateTime(incident.startedAt)),
        detailRow('Provider', incident.source),
      ]),
    },
    {
      label: 'Action',
      rows: compactDetailRows([detailRow('Recommended', incident.action)]),
    },
  ]);
};

const AlertDetail: Component<{ incident: KubernetesIncidentRow; onClose: () => void }> = (
  props,
) => (
  <InlineDetailPanel
    testId="kubernetes-alert-detail"
    detailFor={props.incident.id}
    title="Kubernetes alert detail"
    summary={`${formatAlertSeverityLabel(props.incident.severity)} · ${formatCode(
      props.incident.code,
    )}`}
    sections={buildAlertDetailSections(props.incident)}
    detailAttributes={{ 'data-kubernetes-alert-detail-for': props.incident.id }}
    onClose={props.onClose}
  />
);

export const KubernetesAlertsTable: Component<{
  incidents: KubernetesIncidentRow[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.incidents,
    initialStatus: 'all' as KubernetesIncidentSeverityFilter,
    filter: filterKubernetesIncidents,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-alert-drawer' });
  const filteredEmptyState = () => getAlertFilteredEmptyState('Kubernetes alerts', 'severity');

  return (
    <Show
      when={props.incidents.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search Kubernetes alerts"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={KUBERNETES_INCIDENT_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="alerts"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title={filteredEmptyState().title}
              description={filteredEmptyState().description}
            />
          }
        >
          <PlatformTableShell
            title="Active Alerts"
            tableClass="min-w-full table-fixed text-xs md:min-w-[960px]"
            header={
              <>
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[22%]`}>
                  Resource
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('badge')} md:w-[10%]`}>
                  Severity
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[34%]`}>
                  Alert
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                >
                  Scope
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[10%]`}
                >
                  Started
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden xl:table-cell md:w-[10%]`}
                >
                  Action
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
                  {(incident) => {
                    const k = () => incident.resource.kubernetes;
                    const detailRowId = () => drawer.detailRowId(incident);
                    const isExpanded = () => drawer.isExpanded(incident);
                    const scopeText = () => {
                      const cluster = k()?.clusterName || k()?.clusterId;
                      const namespace = k()?.namespace;
                      if (cluster && namespace) return `${cluster}/${namespace}`;
                      return cluster || namespace || '-';
                    };
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-alert-row={incident.id}
                          onClick={() => drawer.toggle(incident)}
                          onKeyDown={drawer.handleActivationKey(incident)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={incident.resourceName}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(incident)}
                              />
                              <AlertSeverityDot
                                size="sm"
                                severity={incident.severity}
                                bucket={incident.severityBucket}
                              />
                              <div class="min-w-0">
                                <div
                                  class="truncate font-medium text-base-content"
                                  title={incident.resourceName}
                                >
                                  {incident.resourceName}
                                </div>
                                <div class="truncate text-[10px] text-muted">
                                  {formatResourceType(incident.resourceType)}
                                  <Show when={k()?.ownerKind && k()?.ownerName}>
                                    · {k()?.ownerKind}/{k()?.ownerName}
                                  </Show>
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('badge')}>
                            <AlertSeverityBadge
                              severity={incident.severity}
                              bucket={incident.severityBucket}
                            />
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span class="block truncate text-base-content" title={incident.summary}>
                              {incident.summary}
                            </span>
                            <span class="block truncate text-[10px] text-muted">
                              {incident.label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden md:table-cell`}
                          >
                            <span class="block truncate text-base-content" title={scopeText()}>
                              {scopeText()}
                            </span>
                            <span class="block truncate text-[10px] text-muted">
                              {k()?.nodeName || formatCode(incident.code)}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                          >
                            {formatStartedAt(incident.startedAt)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content xl:table-cell`}
                          >
                            <span class="block truncate" title={incident.action}>
                              {incident.action}
                            </span>
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={6}
                            data-inline-detail-for={incident.id}
                            data-kubernetes-alert-detail-row={incident.id}
                          >
                            <AlertDetail
                              incident={incident}
                              onClose={() => drawer.close(incident)}
                            />
                          </InlineDetailTableRow>
                        </Show>
                      </>
                    );
                  }}
                </For>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default KubernetesAlertsTable;
