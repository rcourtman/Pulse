const BASE_BADGE =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap';

type KnownSourcePlatform =
  | 'proxmox-pve'
  | 'proxmox-pbs'
  | 'proxmox-pmg'
  | 'docker'
  | 'kubernetes'
  | 'truenas'
  | 'host-agent'
  | 'unraid'
  | 'synology-dsm'
  | 'vmware-vsphere'
  | 'microsoft-hyperv'
  | 'aws'
  | 'azure'
  | 'gcp'
  | 'generic';

interface SourcePlatformTone {
  label: string;
  tone: string;
}

export interface SourcePlatformBadge {
  label: string;
  title: string;
  classes: string;
}

const PLATFORM_TONES: Record<KnownSourcePlatform, SourcePlatformTone> = {
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
  'host-agent': {
    label: 'Host',
    tone: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200',
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
    tone: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300',
  },
};

const DEFAULT_TONE = 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300';

const titleize = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const normalizeKey = (value: string | null | undefined): KnownSourcePlatform | null => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return null;
  if (normalized in PLATFORM_TONES) return normalized as KnownSourcePlatform;
  return null;
};

export const getSourcePlatformBadge = (value: string | null | undefined): SourcePlatformBadge | null => {
  const normalized = normalizeKey(value);
  if (!normalized) {
    const label = titleize((value || '').toString());
    if (!label) return null;
    return {
      label,
      title: label,
      classes: `${BASE_BADGE} ${DEFAULT_TONE}`,
    };
  }

  const tone = PLATFORM_TONES[normalized];
  return {
    label: tone.label,
    title: tone.label,
    classes: `${BASE_BADGE} ${tone.tone}`,
  };
};

export const getSourcePlatformLabel = (value: string | null | undefined): string =>
  getSourcePlatformBadge(value)?.label || titleize((value || '').toString()) || 'Unknown';
