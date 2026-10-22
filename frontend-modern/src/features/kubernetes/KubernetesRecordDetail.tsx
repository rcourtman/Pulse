import { type Component, type JSX } from 'solid-js';
import {
  InlineDetailPanel,
  compactDetailRows,
  compactDetailSections,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import { RecordOwnerLink } from '@/features/platformPage/RecordOwnerLink';
import type { ResourcePageSubject } from '@/features/platformNavigation/resourcePagePath';
import type { Resource } from '@/types/resource';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import type { StatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { kubernetesClusterLabel, mapKubernetesEventSeverity } from './kubernetesPageModel';
import { makeDetailTimestampRow } from '@/features/platformPage/platformDetailTimestampRow';

// Events, ConfigMaps, Secrets, RBAC objects and namespace policies are
// records about a cluster, not objects Pulse monitors: they carry no alerts,
// history or maintenance of their own. Their drawer is a record panel headed
// by the object's name and its row's own status dot.
export const KubernetesRecordDetailPanel: Component<{
  testId: string;
  resource: Resource;
  title: string;
  status: StatusIndicator;
  summary?: JSX.Element;
  actions?: JSX.Element;
  /** Built against the shared relative-time clock so ages keep moving while open. */
  sections: (now: number) => Array<DetailSection | null>;
  onClose: () => void;
}> = (props) => {
  const now = useRelativeTimeNow();
  return (
    <InlineDetailPanel
      testId={props.testId}
      detailFor={props.resource.id}
      title={props.title}
      status={props.status}
      summary={props.summary}
      actions={props.actions}
      sections={compactDetailSections(props.sections(now()))}
      onClose={props.onClose}
    />
  );
};

export const kubernetesTimestampRow = makeDetailTimestampRow;

/** Where an object lives: its kind, namespace and cluster, and when it was made. */
export const kubernetesObjectSection = (
  resource: Resource,
  kind: string,
  now: number,
): DetailSection => ({
  label: 'Object',
  rows: compactDetailRows([
    makeDetailRow('Kind', kind),
    makeDetailRow('Namespace', asTrimmedString(resource.kubernetes?.namespace), { wrap: true }),
    makeDetailRow('Cluster', kubernetesClusterLabel(resource), { wrap: true }),
    kubernetesTimestampRow('Created', resource.kubernetes?.createdAt, now),
  ]),
});

// Every label, key above value, because keys such as
// app.kubernetes.io/managed-by are too long for the label column. Resources
// that only carry the registry's tags list them on one wrapped row.
export const kubernetesLabelsSection = (resource: Resource): DetailSection | null => {
  const labels = Object.entries(resource.kubernetes?.labels ?? {}).sort(([a], [b]) =>
    a.localeCompare(b),
  );
  if (labels.length > 0) {
    return {
      label: `Labels (${labels.length})`,
      rows: compactDetailRows(
        labels.map(([key, value]) =>
          makeDetailRow(key, asTrimmedString(value) || '(empty)', {
            layout: 'stacked',
            wrap: true,
          }),
        ),
      ),
    };
  }
  const tags = (resource.tags ?? []).map((tag) => asTrimmedString(tag)).filter(Boolean);
  if (tags.length === 0) return null;
  return {
    label: 'Labels',
    rows: compactDetailRows([makeDetailRow('Tags', tags.join(', '), { wrap: true })]),
  };
};

// The Kubernetes kinds whose objects a Kubernetes page lists and filters by
// name (KUBERNETES_TAB_BY_TYPE in resourcePagePath). An event about any other
// kind gets no link: there is no row to bring into view.
const KUBERNETES_KIND_RESOURCE_TYPES: Record<string, string> = {
  Node: 'k8s-node',
  Pod: 'pod',
  Deployment: 'k8s-deployment',
  StatefulSet: 'k8s-statefulset',
  DaemonSet: 'k8s-daemonset',
  ReplicaSet: 'k8s-replicaset',
  Job: 'k8s-job',
  CronJob: 'k8s-cronjob',
  Service: 'k8s-service',
  Ingress: 'k8s-ingress',
  EndpointSlice: 'k8s-endpoint-slice',
  PersistentVolume: 'k8s-persistent-volume',
  PersistentVolumeClaim: 'k8s-persistent-volume-claim',
  StorageClass: 'k8s-storage-class',
  Namespace: 'k8s-namespace',
  ConfigMap: 'k8s-configmap',
  Secret: 'k8s-secret',
  ServiceAccount: 'k8s-serviceaccount',
  Role: 'k8s-role',
  ClusterRole: 'k8s-cluster-role',
  RoleBinding: 'k8s-role-binding',
  ClusterRoleBinding: 'k8s-cluster-role-binding',
  ResourceQuota: 'k8s-resource-quota',
  NetworkPolicy: 'k8s-network-policy',
  LimitRange: 'k8s-limit-range',
  PodDisruptionBudget: 'k8s-pod-disruption-budget',
};

export const kubernetesEventInvolvedSubject = (resource: Resource): ResourcePageSubject | null => {
  const kind = asTrimmedString(resource.kubernetes?.involvedKind);
  const name = asTrimmedString(resource.kubernetes?.involvedName);
  const type = kind ? KUBERNETES_KIND_RESOURCE_TYPES[kind] : undefined;
  if (!name || !type) return null;
  const namespace = asTrimmedString(resource.kubernetes?.namespace);
  return { id: namespace ? `${namespace}/${name}` : name, name, type };
};

export const kubernetesEventInvolvedObject = (resource: Resource): string =>
  [resource.kubernetes?.involvedKind, resource.kubernetes?.involvedName]
    .map((part) => asTrimmedString(part))
    .filter(Boolean)
    .join('/');

export const kubernetesEventName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const buildEventDetailSections = (resource: Resource, now: number): DetailSection[] => {
  const k = resource.kubernetes;
  // Kubernetes keeps the first and last time it saw a repeating event;
  // Pulse stores the last one as the event's createdAt. The newer events API
  // reports one eventTime instead.
  const firstSeen = asTrimmedString(k?.firstSeen) || asTrimmedString(k?.eventTime);
  const lastSeen = asTrimmedString(k?.createdAt);
  const count = typeof k?.count === 'number' && k.count > 0 ? k.count : undefined;
  const severity = mapKubernetesEventSeverity(k?.eventType);
  return [
    {
      label: 'Event',
      rows: compactDetailRows([
        makeDetailRow('Reason', asTrimmedString(k?.reason), { wrap: true }),
        makeDetailRow('Type', severity.label, {
          tone: severity.variant === 'warning' ? 'warning' : undefined,
        }),
        count ? makeDetailRow('Count', count === 1 ? 'Once' : `${count} times`) : null,
        kubernetesTimestampRow(
          lastSeen && lastSeen !== firstSeen ? 'First seen' : 'Seen',
          firstSeen,
          now,
        ),
        lastSeen && lastSeen !== firstSeen
          ? kubernetesTimestampRow('Last seen', lastSeen, now)
          : null,
        makeDetailRow('Event name', kubernetesEventName(resource), { wrap: true }),
      ]),
    },
    {
      label: 'Object',
      rows: compactDetailRows([
        makeDetailRow('Kind', asTrimmedString(k?.involvedKind)),
        makeDetailRow('Name', asTrimmedString(k?.involvedName), { wrap: true }),
        makeDetailRow('Namespace', asTrimmedString(k?.namespace), { wrap: true }),
        makeDetailRow('Cluster', kubernetesClusterLabel(resource), { wrap: true }),
      ]),
    },
  ];
};

// An event is about another object: the drawer is headed by that object, says
// what happened in Kubernetes' own words, and links to the object where a
// Kubernetes page can filter to it.
export const KubernetesEventDetailPanel: Component<{
  resource: Resource;
  status: StatusIndicator;
  onClose: () => void;
}> = (props) => (
  <KubernetesRecordDetailPanel
    testId="kubernetes-event-detail"
    resource={props.resource}
    title={kubernetesEventInvolvedObject(props.resource) || kubernetesEventName(props.resource)}
    status={props.status}
    summary={
      asTrimmedString(props.resource.kubernetes?.message) ||
      asTrimmedString(props.resource.kubernetes?.reason) ||
      undefined
    }
    actions={
      <RecordOwnerLink
        subject={kubernetesEventInvolvedSubject(props.resource)}
        name={asTrimmedString(props.resource.kubernetes?.involvedName)}
      />
    }
    sections={(now) => buildEventDetailSections(props.resource, now)}
    onClose={props.onClose}
  />
);
