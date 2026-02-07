import type { BackupType, UnifiedBackup } from '@/types/backups';

export type BackupSourceTone = 'slate' | 'amber' | 'orange' | 'violet' | 'blue' | 'cyan';

export interface BackupSourceOption {
  key: string;
  label: string;
  tone: BackupSourceTone;
  backupTypes: BackupType[];
  legacyBackupType?: BackupType;
}

const SOURCE_PRESETS: Record<string, Omit<BackupSourceOption, 'key'>> = {
  snapshot: {
    label: 'Snapshots',
    tone: 'amber',
    backupTypes: ['snapshot'],
    legacyBackupType: 'snapshot',
  },
  pve: {
    label: 'PVE',
    tone: 'orange',
    backupTypes: ['local'],
    legacyBackupType: 'local',
  },
  pbs: {
    label: 'PBS',
    tone: 'violet',
    backupTypes: ['remote'],
    legacyBackupType: 'remote',
  },
  pmg: {
    label: 'PMG',
    tone: 'blue',
    backupTypes: ['local'],
  },
  kubernetes: {
    label: 'K8s',
    tone: 'cyan',
    backupTypes: ['remote'],
  },
};

const BASELINE_SOURCES = ['snapshot', 'pve', 'pbs'];
const SOURCE_ORDER = ['snapshot', 'pve', 'pbs', 'pmg', 'kubernetes'];

const titleCaseLabel = (value: string): string =>
  value
    .split('-')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

const toSlug = (value: string): string =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

export const normalizeBackupSourceKey = (value: string | null | undefined): string => {
  const normalized = toSlug(value || '');
  switch (normalized) {
    case 'all':
      return 'all';
    case 'snapshot':
    case 'snapshots':
      return 'snapshot';
    case 'pve':
    case 'proxmox':
    case 'proxmox-pve':
    case 'local':
      return 'pve';
    case 'pbs':
    case 'proxmox-pbs':
    case 'remote':
      return 'pbs';
    case 'pmg':
    case 'proxmox-pmg':
      return 'pmg';
    case 'k8s':
    case 'kubernetes':
      return 'kubernetes';
    default:
      return normalized;
  }
};

export const resolveSourceFromLegacyBackupType = (backupType: string | null | undefined): string => {
  const normalized = toSlug(backupType || '');
  switch (normalized) {
    case 'snapshot':
      return 'snapshot';
    case 'local':
      return 'pve';
    case 'remote':
      return 'pbs';
    default:
      return 'all';
  }
};

export const resolveLegacyBackupTypeForSource = (
  source: string | null | undefined,
): BackupType | null => {
  const normalized = normalizeBackupSourceKey(source);
  if (normalized === 'snapshot') return 'snapshot';
  if (normalized === 'pve') return 'local';
  if (normalized === 'pbs') return 'remote';
  return null;
};

export const buildBackupSourceOptions = (backups: UnifiedBackup[]): BackupSourceOption[] => {
  const bySource = new Map<string, Set<BackupType>>();

  BASELINE_SOURCES.forEach((source) => {
    const preset = SOURCE_PRESETS[source];
    bySource.set(source, new Set(preset?.backupTypes || []));
  });

  backups.forEach((backup) => {
    const key = normalizeBackupSourceKey(backup.source);
    if (!key || key === 'all') return;
    if (!bySource.has(key)) {
      bySource.set(key, new Set<BackupType>());
    }
    bySource.get(key)!.add(backup.backupType);
  });

  const orderedKeys = Array.from(bySource.keys()).sort((a, b) => {
    const indexA = SOURCE_ORDER.indexOf(a);
    const indexB = SOURCE_ORDER.indexOf(b);
    if (indexA !== -1 || indexB !== -1) {
      if (indexA === -1) return 1;
      if (indexB === -1) return -1;
      return indexA - indexB;
    }
    return a.localeCompare(b);
  });

  const allBackupTypes = new Set<BackupType>();
  backups.forEach((backup) => allBackupTypes.add(backup.backupType));
  if (allBackupTypes.size === 0) {
    allBackupTypes.add('snapshot');
    allBackupTypes.add('local');
    allBackupTypes.add('remote');
  }

  const options = orderedKeys.map<BackupSourceOption>((key) => {
    const preset = SOURCE_PRESETS[key];
    const discoveredTypes = Array.from(bySource.get(key) || []);
    const backupTypes: BackupType[] =
      discoveredTypes.length > 0
        ? discoveredTypes
        : preset?.backupTypes && preset.backupTypes.length > 0
          ? preset.backupTypes
          : ['local'];

    if (preset) {
      return {
        key,
        label: preset.label,
        tone: preset.tone,
        backupTypes,
        legacyBackupType: preset.legacyBackupType,
      };
    }

    return {
      key,
      label: titleCaseLabel(key) || 'Other',
      tone: 'slate',
      backupTypes,
    };
  });

  return [
    {
      key: 'all',
      label: 'All Sources',
      tone: 'slate',
      backupTypes: Array.from(allBackupTypes),
    },
    ...options,
  ];
};
