import type { PlatformType, SourceType, ResourceType } from '@/types/resource';

export interface ResourceBadge {
  label: string;
  classes: string;
  title?: string;
}

const baseBadge = 'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap';

export type UnifiedSource = 'proxmox' | 'agent' | 'docker' | 'pbs' | 'pmg' | 'kubernetes';

const platformLabels: Record<PlatformType, string> = {
  'proxmox-pve': 'PVE',
  'proxmox-pbs': 'PBS',
  'proxmox-pmg': 'PMG',
  docker: 'Docker',
  kubernetes: 'K8s',
  truenas: 'TrueNAS',
  'host-agent': 'Host',
};

const platformClasses: Record<PlatformType, string> = {
  'proxmox-pve': 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  'proxmox-pbs': 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400',
  'proxmox-pmg': 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
  docker: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-400',
  kubernetes: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',
  truenas: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  'host-agent': 'bg-slate-100 text-slate-700 dark:bg-slate-800/60 dark:text-slate-200',
};

const sourceLabels: Record<SourceType, string> = {
  agent: 'Agent',
  api: 'API',
  hybrid: 'Hybrid',
};

const sourceClasses: Record<SourceType, string> = {
  agent: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  api: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  hybrid: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400',
};

const unifiedSourceLabels: Record<UnifiedSource, string> = {
  proxmox: 'PVE',
  agent: 'Agent',
  docker: 'Docker',
  pbs: 'PBS',
  pmg: 'PMG',
  kubernetes: 'K8s',
};

const unifiedSourceClasses: Record<UnifiedSource, string> = {
  proxmox: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  agent: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  docker: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-400',
  pbs: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400',
  pmg: 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
  kubernetes: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',
};

const typeLabels: Partial<Record<ResourceType, string>> = {
  host: 'Host',
  node: 'Node',
  'docker-host': 'Docker Host',
  'k8s-node': 'K8s Node',
  'k8s-cluster': 'K8s Cluster',
  truenas: 'TrueNAS',
};

const typeClasses = 'bg-gray-100 text-gray-700 dark:bg-gray-700/60 dark:text-gray-200';

export function getPlatformBadge(platformType?: PlatformType): ResourceBadge | null {
  if (!platformType) return null;
  return {
    label: platformLabels[platformType] ?? platformType,
    classes: `${baseBadge} ${platformClasses[platformType] ?? typeClasses}`,
    title: platformType,
  };
}

export function getSourceBadge(sourceType?: SourceType): ResourceBadge | null {
  if (!sourceType) return null;
  return {
    label: sourceLabels[sourceType] ?? sourceType,
    classes: `${baseBadge} ${sourceClasses[sourceType] ?? typeClasses}`,
    title: sourceType,
  };
}

export function getTypeBadge(resourceType?: ResourceType): ResourceBadge | null {
  if (!resourceType) return null;
  return {
    label: typeLabels[resourceType] ?? resourceType,
    classes: `${baseBadge} ${typeClasses}`,
    title: resourceType,
  };
}

export function getUnifiedSourceBadges(sources?: string[] | null): ResourceBadge[] {
  if (!sources || sources.length === 0) return [];
  const normalized = sources
    .map((source) => source.toLowerCase())
    .filter((source): source is UnifiedSource =>
      ['proxmox', 'agent', 'docker', 'pbs', 'pmg', 'kubernetes'].includes(source),
    );
  const unique = Array.from(new Set(normalized));
  return unique.map((source) => ({
    label: unifiedSourceLabels[source] ?? source,
    classes: `${baseBadge} ${unifiedSourceClasses[source] ?? typeClasses}`,
    title: source,
  }));
}
