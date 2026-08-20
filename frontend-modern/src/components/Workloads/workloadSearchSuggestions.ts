import type { SearchInputSuggestion } from '@/components/shared/SearchInput';
import type { WorkloadGuest } from '@/types/workloads';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';
import { getWorkloadTypePresentation } from '@/utils/workloadTypePresentation';
import { getCanonicalWorkloadId, resolveWorkloadType } from '@/utils/workloads';
import { getWorkloadSearchCandidates } from './workloadSelectors';

const clean = (value: unknown): string =>
  typeof value === 'string' || typeof value === 'number' ? String(value).trim() : '';

const workloadDisplayId = (guest: WorkloadGuest): string =>
  clean(guest.displayId) ||
  (Number.isFinite(guest.vmid) && guest.vmid > 0 ? String(guest.vmid) : '');

const workloadLabel = (guest: WorkloadGuest): string =>
  clean(guest.name) || workloadDisplayId(guest) || clean(guest.id) || 'Unnamed workload';

const workloadDescription = (guest: WorkloadGuest): string => {
  const type = getWorkloadTypePresentation(resolveWorkloadType(guest)).label;
  const displayId = workloadDisplayId(guest);
  const identity = displayId ? `${type} ${displayId}` : type;
  const node = clean(guest.node) || clean(guest.dockerHostName) || clean(guest.contextLabel);
  const status = titleCaseDelimitedLabel(clean(guest.status));
  return [identity, node, status].filter(Boolean).join(' · ');
};

export const buildWorkloadSearchSuggestions = (
  workloads: readonly WorkloadGuest[],
): SearchInputSuggestion[] => {
  const nameCounts = new Map<string, number>();
  for (const workload of workloads) {
    const name = workloadLabel(workload).toLowerCase();
    nameCounts.set(name, (nameCounts.get(name) ?? 0) + 1);
  }

  const seen = new Set<string>();
  const suggestions: SearchInputSuggestion[] = [];
  const scopeFields: Array<{
    key: 'node' | 'host' | 'context' | 'cluster' | 'namespace';
    label: string;
    value: (workload: WorkloadGuest) => string;
  }> = [
    { key: 'node', label: 'Node', value: (workload) => clean(workload.node) },
    { key: 'host', label: 'Host', value: (workload) => clean(workload.dockerHostName) },
    { key: 'context', label: 'Context', value: (workload) => clean(workload.contextLabel) },
    { key: 'cluster', label: 'Cluster', value: (workload) => clean(workload.clusterName) },
    { key: 'namespace', label: 'Namespace', value: (workload) => clean(workload.namespace) },
  ];

  for (const field of scopeFields) {
    const values = new Set(workloads.map(field.value).filter(Boolean));
    for (const value of values) {
      suggestions.push({
        id: `workload-scope:${field.key}:${value}`,
        label: value,
        value,
        description: field.label,
        group: 'Infrastructure',
        keywords: [field.label, value],
      });
    }
  }

  for (const workload of workloads) {
    const canonicalId = getCanonicalWorkloadId(workload) || workload.id;
    if (!canonicalId || seen.has(canonicalId)) continue;
    seen.add(canonicalId);

    const label = workloadLabel(workload);
    const duplicateName = (nameCounts.get(label.toLowerCase()) ?? 0) > 1;
    suggestions.push({
      id: `workload:${canonicalId}`,
      label,
      value: duplicateName ? canonicalId : label,
      description: workloadDescription(workload),
      group: 'Infrastructure',
      keywords: getWorkloadSearchCandidates(workload).map(clean).filter(Boolean),
      completions: [
        workload.name,
        workload.displayId,
        workload.vmid,
        workload.id,
        workload.node,
        workload.dockerHostName,
        workload.contextLabel,
        workload.clusterName,
      ]
        .map(clean)
        .filter(Boolean),
    });
  }

  return suggestions;
};
