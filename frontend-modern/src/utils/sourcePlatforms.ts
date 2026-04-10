import type { PlatformType, SourceType } from '@/types/resource';
import {
  KNOWN_SOURCE_PLATFORM_KEYS as GENERATED_KNOWN_SOURCE_PLATFORM_KEYS,
  PRESENTATION_ONLY_PLATFORM_IDS,
  SOURCE_PLATFORM_PRESENTATION as GENERATED_SOURCE_PLATFORM_PRESENTATION,
  type GeneratedKnownSourcePlatform,
  getSourcePlatformManifestEntry,
  SOURCE_PLATFORM_ALIAS_MAP,
} from '@/utils/platformSupportManifest';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';

export type PresentationOnlySourcePlatform = (typeof PRESENTATION_ONLY_PLATFORM_IDS)[number];
export type KnownSourcePlatform = GeneratedKnownSourcePlatform;

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
  hasTrueNAS: boolean;
  hasVMware: boolean;
}

export const SOURCE_PLATFORM_PRESENTATION: Record<KnownSourcePlatform, SourcePlatformPresentation> =
  GENERATED_SOURCE_PLATFORM_PRESENTATION as Record<KnownSourcePlatform, SourcePlatformPresentation>;

export const KNOWN_SOURCE_PLATFORM_KEYS =
  GENERATED_KNOWN_SOURCE_PLATFORM_KEYS as readonly KnownSourcePlatform[];

const PLATFORM_ALIASES = SOURCE_PLATFORM_ALIAS_MAP as Record<
  string,
  Exclude<KnownSourcePlatform, 'generic'>
>;

export const normalizeSourcePlatformKey = (
  value: string | null | undefined,
): KnownSourcePlatform | null => {
  const normalized = (value || '').trim().toLowerCase();
  if (!normalized) return null;
  if (Object.prototype.hasOwnProperty.call(SOURCE_PLATFORM_PRESENTATION, normalized)) {
    return normalized as KnownSourcePlatform;
  }
  if (Object.prototype.hasOwnProperty.call(PLATFORM_ALIASES, normalized)) {
    return PLATFORM_ALIASES[normalized];
  }
  return null;
};

export const getSourcePlatformPresentation = (
  value: string | null | undefined,
): SourcePlatformPresentation | null => {
  const manifestPlatform = getSourcePlatformManifestEntry(value);
  if (manifestPlatform) {
    return SOURCE_PLATFORM_PRESENTATION[
      manifestPlatform.id as Exclude<KnownSourcePlatform, 'generic'>
    ];
  }

  const normalized = normalizeSourcePlatformKey(value);
  return normalized ? SOURCE_PLATFORM_PRESENTATION[normalized] : null;
};

export const getSourcePlatformLabel = (value: string | null | undefined): string =>
  getSourcePlatformPresentation(value)?.label ||
  titleCaseDelimitedLabel((value || '').toString()) ||
  'Unknown';

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
    hasTrueNAS: false,
    hasVMware: false,
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
      case 'truenas':
        flags.hasTrueNAS = true;
        break;
      case 'vmware-vsphere':
        flags.hasVMware = true;
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
  if (flags.hasVMware) return 'vmware-vsphere';
  if (flags.hasTrueNAS) return 'truenas';
  if (flags.hasKubernetes) return 'kubernetes';
  if (flags.hasDocker) return 'docker';
  if (flags.hasAgent) return 'agent';
  return undefined;
};

export const resolveSourceTypeFromSources = (sources?: string[]): SourceType => {
  const flags = readSourcePlatformFlags(sources);
  const hasOther =
    flags.hasProxmox ||
    flags.hasDocker ||
    flags.hasKubernetes ||
    flags.hasPbs ||
    flags.hasPmg ||
    flags.hasTrueNAS ||
    flags.hasVMware;
  if (flags.hasAgent && hasOther) return 'hybrid';
  if (flags.hasAgent) return 'agent';
  return 'api';
};
