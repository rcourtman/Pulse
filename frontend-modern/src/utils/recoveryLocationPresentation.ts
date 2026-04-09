import type { RecoveryPoint } from '@/types/recovery';

export type RecoveryLocationFacetKind = 'cluster' | 'node' | 'namespace';

interface RecoveryLocationFacetPresentation {
  allLabel: string;
  label: string;
}

const RECOVERY_LOCATION_FACET_PRESENTATION: Record<
  RecoveryLocationFacetKind,
  RecoveryLocationFacetPresentation
> = {
  cluster: {
    allLabel: 'Any cluster or site',
    label: 'Cluster / Site',
  },
  node: {
    allLabel: 'Any host or agent',
    label: 'Host / Agent',
  },
  namespace: {
    allLabel: 'Any namespace or group',
    label: 'Namespace / Group',
  },
};

export function getRecoveryLocationFacetLabel(kind: RecoveryLocationFacetKind): string {
  return RECOVERY_LOCATION_FACET_PRESENTATION[kind].label;
}

export function getRecoveryLocationFacetAllLabel(kind: RecoveryLocationFacetKind): string {
  return RECOVERY_LOCATION_FACET_PRESENTATION[kind].allLabel;
}

export function getRecoveryPointLocationEntries(
  point: RecoveryPoint,
): Array<{ key: RecoveryLocationFacetKind; label: string; value: string }> {
  const cluster = String(point.display?.clusterLabel || point.cluster || '').trim();
  const node = String(
    point.display?.nodeHostLabel || point.display?.nodeAgentLabel || point.node || '',
  ).trim();
  const namespace = String(point.display?.namespaceLabel || point.namespace || '').trim();

  const entries: Array<{ key: RecoveryLocationFacetKind; label: string; value: string }> = [
    { key: 'cluster', label: getRecoveryLocationFacetLabel('cluster'), value: cluster },
    { key: 'node', label: getRecoveryLocationFacetLabel('node'), value: node },
    { key: 'namespace', label: getRecoveryLocationFacetLabel('namespace'), value: namespace },
  ];
  return entries.filter((entry) => entry.value !== '');
}
