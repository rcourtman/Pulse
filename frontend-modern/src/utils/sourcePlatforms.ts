import type { PlatformType, SourceType } from '@/types/resource';

export type KnownSourcePlatform =
  | 'proxmox-pve'
  | 'proxmox-pbs'
  | 'proxmox-pmg'
  | 'docker'
  | 'kubernetes'
  | 'truenas'
  | 'agent'
  | 'unraid'
  | 'synology-dsm'
  | 'vmware-vsphere'
  | 'microsoft-hyperv'
  | 'aws'
  | 'azure'
  | 'gcp'
  | 'generic';

export interface SourcePlatformPresentation {
  label: string;
  tone: string;
}

export interface SourcePlatformFlags {
  hasAgent: boolean;
  hasProxmox: boolean;
  hasDocker: boolean;
  hasKubernetes: boolean;
  hasPbs: boolean;
  hasPmg: boolean;
}

export const SOURCE_PLATFORM_PRESENTATION: Record<KnownSourcePlatform, SourcePlatformPresentation> =
  {
    'proxmox-pve': {
      label: 'PVE',
      tone: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-400',
    },
    'proxmox-pbs': {
      label: 'PBS',
      tone: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-400',
    },
    'proxmox-pmg': {
      label: 'PMG',
      tone: 'bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-400',
    },
    docker: {
      label: 'Containers',
      tone: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-400',
    },
    kubernetes: {
      label: 'K8s',
      tone: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-400',
    },
    truenas: {
      label: 'TrueNAS',
      tone: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-400',
    },
    agent: {
      label: 'Agent',
      tone: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-400',
    },
    unraid: {
      label: 'Unraid',
      tone: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
    },
    'synology-dsm': {
      label: 'Synology',
      tone: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
    },
    'vmware-vsphere': {
      label: 'vSphere',
      tone: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
    },
    'microsoft-hyperv': {
      label: 'Hyper-V',
      tone: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-300',
    },
    aws: {
      label: 'AWS',
      tone: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
    },
    azure: {
      label: 'Azure',
      tone: 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-300',
    },
    gcp: {
      label: 'GCP',
      tone: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
    },
    generic: {
      label: 'Generic',
      tone: 'bg-surface-alt text-base-content',
    },
  };

const PLATFORM_ALIASES: Record<string, KnownSourcePlatform> = {
  pve: 'proxmox-pve',
  proxmox: 'proxmox-pve',
  pbs: 'proxmox-pbs',
  pmg: 'proxmox-pmg',
  k8s: 'kubernetes',
};

const titleize = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

export const normalizeSourcePlatformKey = (
  value: string | null | undefined,
): KnownSourcePlatform | null => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return null;
  if (normalized in SOURCE_PLATFORM_PRESENTATION) return normalized as KnownSourcePlatform;
  if (normalized in PLATFORM_ALIASES) return PLATFORM_ALIASES[normalized];
  return null;
};

export const getSourcePlatformPresentation = (
  value: string | null | undefined,
): SourcePlatformPresentation | null => {
  const normalized = normalizeSourcePlatformKey(value);
  return normalized ? SOURCE_PLATFORM_PRESENTATION[normalized] : null;
};

export const getSourcePlatformLabel = (value: string | null | undefined): string =>
  getSourcePlatformPresentation(value)?.label || titleize((value || '').toString()) || 'Unknown';

export const normalizeSourcePlatformQueryValue = (value: string | null | undefined): string => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return '';
  if (normalized === 'all') return 'all';
  return normalizeSourcePlatformKey(normalized) || normalized;
};

export const readSourcePlatformFlags = (sources?: string[]): SourcePlatformFlags => {
  const flags: SourcePlatformFlags = {
    hasAgent: false,
    hasProxmox: false,
    hasDocker: false,
    hasKubernetes: false,
    hasPbs: false,
    hasPmg: false,
  };

  if (!sources || sources.length === 0) {
    return flags;
  }

  for (const source of sources) {
    switch (normalizeSourcePlatformKey(source) || source.toLowerCase()) {
      case 'agent':
        flags.hasAgent = true;
        break;
      case 'proxmox-pve':
        flags.hasProxmox = true;
        break;
      case 'docker':
        flags.hasDocker = true;
        break;
      case 'kubernetes':
        flags.hasKubernetes = true;
        break;
      case 'proxmox-pbs':
        flags.hasPbs = true;
        break;
      case 'proxmox-pmg':
        flags.hasPmg = true;
        break;
      default:
        break;
    }
  }

  return flags;
};

export const resolvePlatformTypeFromSources = (sources?: string[]): PlatformType | undefined => {
  const flags = readSourcePlatformFlags(sources);
  if (flags.hasProxmox) return 'proxmox-pve';
  if (flags.hasPbs) return 'proxmox-pbs';
  if (flags.hasPmg) return 'proxmox-pmg';
  if (flags.hasDocker) return 'docker';
  if (flags.hasKubernetes) return 'kubernetes';
  if (flags.hasAgent) return 'agent';
  return undefined;
};

export const resolveSourceTypeFromSources = (sources?: string[]): SourceType => {
  const flags = readSourcePlatformFlags(sources);
  const hasOther =
    flags.hasProxmox || flags.hasDocker || flags.hasKubernetes || flags.hasPbs || flags.hasPmg;
  if (flags.hasAgent && hasOther) return 'hybrid';
  if (flags.hasAgent) return 'agent';
  return 'api';
};
