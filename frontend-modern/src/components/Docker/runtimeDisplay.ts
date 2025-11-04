import type { DockerContainer, DockerHost } from '@/types/api';

export type RuntimeKind = 'docker' | 'podman' | 'containerd' | 'cri-o' | 'nerdctl' | 'unknown';

export interface RuntimeDisplayInfo {
  id: RuntimeKind;
  label: string;
  badgeClass: string;
  raw: string;
}

export interface RuntimeDisplayOptions {
  hint?: string | null;
  defaultId?: Exclude<RuntimeKind, 'unknown'> | 'unknown';
}

const BADGE_CLASSES: Record<Exclude<RuntimeKind, 'unknown'>, string> = {
  docker: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
  podman: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
  containerd: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
  'cri-o': 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
  nerdctl: 'bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-200',
};

const UNKNOWN_BADGE_CLASS = 'bg-gray-200 text-gray-700 dark:bg-gray-700 dark:text-gray-300';

const LABEL_BY_KIND: Record<RuntimeKind, string> = {
  docker: 'Docker',
  podman: 'Podman',
  containerd: 'containerd',
  'cri-o': 'CRI-O',
  nerdctl: 'nerdctl',
  unknown: 'Container runtime',
};

const normalize = (value?: string | null) => value?.trim() ?? '';

const MATCHERS: Array<{ id: Exclude<RuntimeKind, 'unknown'>; keywords: string[] }> = [
  { id: 'podman', keywords: ['podman', 'libpod'] },
  { id: 'nerdctl', keywords: ['nerdctl'] },
  { id: 'containerd', keywords: ['containerd'] },
  { id: 'cri-o', keywords: ['cri-o', 'crio'] },
  { id: 'docker', keywords: ['docker', 'moby engine', 'moby'] },
];

const detectRuntime = (value?: string | null) => {
  const raw = normalize(value);
  if (!raw) {
    return null;
  }

  const normalized = raw.toLowerCase();
  for (const matcher of MATCHERS) {
    if (matcher.keywords.some((keyword) => normalized.includes(keyword))) {
      return { id: matcher.id, raw };
    }
  }

  return null;
};

const formatRuntimeLabel = (value: string): string => {
  if (!value) return LABEL_BY_KIND.unknown;

  const cleaned = value.replace(/\s+/g, ' ').trim();

  if (/^cri-?o$/i.test(cleaned)) {
    return LABEL_BY_KIND['cri-o'];
  }

  return cleaned
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((segment) => {
      if (segment.toLowerCase() === 'cri' || segment.toLowerCase() === 'crio') {
        return 'CRI';
      }
      if (segment.toLowerCase() === 'o') {
        return 'O';
      }
      if (segment === segment.toUpperCase()) {
        return segment;
      }
      return segment.charAt(0).toUpperCase() + segment.slice(1);
    })
    .join(' ');
};

export const getRuntimeDisplay = (
  runtime?: string | null,
  options: RuntimeDisplayOptions = {},
): RuntimeDisplayInfo => {
  const candidates = [
    { value: runtime, raw: normalize(runtime) },
    { value: options.hint, raw: normalize(options.hint) },
  ];

  for (const candidate of candidates) {
    const detected = detectRuntime(candidate.value);
    if (detected) {
      const label = LABEL_BY_KIND[detected.id];
      const badgeClass = BADGE_CLASSES[detected.id];
      return {
        id: detected.id,
        label,
        badgeClass,
        raw: detected.raw,
      };
    }
  }

  const firstRaw = candidates.find((candidate) => candidate.raw !== '')?.raw ?? '';
  const defaultKind: RuntimeKind = options.defaultId ?? 'unknown';

  if (defaultKind !== 'unknown') {
    return {
      id: defaultKind,
      label: LABEL_BY_KIND[defaultKind],
      badgeClass: BADGE_CLASSES[defaultKind],
      raw: firstRaw,
    };
  }

  const label = firstRaw ? formatRuntimeLabel(firstRaw) : LABEL_BY_KIND.unknown;

  return {
    id: 'unknown',
    label,
    badgeClass: UNKNOWN_BADGE_CLASS,
    raw: firstRaw,
  };
};

const PODMAN_LABEL_PREFIXES = ['io.podman.', 'io.containers.', 'net.containers.podman'];

const hasPodmanSignature = (container?: DockerContainer | null) => {
  if (!container?.labels) return false;
  const labels = container.labels;
  return Object.keys(labels).some((key) =>
    PODMAN_LABEL_PREFIXES.some((prefix) => key.startsWith(prefix)),
  ) || Object.values(labels).some((value) => value?.toLowerCase().includes('podman'));
};

const hostHasPodmanSignature = (host?: Pick<DockerHost, 'containers'> | null) => {
  if (!host?.containers || host.containers.length === 0) return false;
  return host.containers.some((container) => hasPodmanSignature(container));
};

export interface ResolveRuntimeOptions extends RuntimeDisplayOptions {
  requireSignatureForGuess?: boolean;
}

const stripVersion = (value?: string | null) => {
  if (!value) return '';
  const match = value.match(/\d+(?:\.\d+){0,2}/);
  return match ? match[0] : value.trim();
};

const isLikelyPodmanVersion = (value?: string | null) => {
  const clean = stripVersion(value);
  if (!clean) return false;
  const parts = clean.split('.');
  const major = Number.parseInt(parts[0] || '', 10);
  if (!Number.isFinite(major)) return false;
  return major >= 2 && major <= 6;
};

export const resolveHostRuntime = (
  host: Pick<DockerHost, 'runtime' | 'runtimeVersion' | 'dockerVersion' | 'containers'>,
  options: ResolveRuntimeOptions = {},
): RuntimeDisplayInfo => {
  const hint = options.hint ?? host.runtimeVersion ?? host.dockerVersion ?? null;
  const base = getRuntimeDisplay(host.runtime, {
    ...options,
    hint,
    defaultId: options.defaultId ?? 'docker',
  });
  if (base.id !== 'docker' && base.id !== 'unknown') {
    return base;
  }

  const hasSignature = hostHasPodmanSignature(host);
  if (hasSignature) {
    return getRuntimeDisplay('podman', { hint });
  }

  if (!options.requireSignatureForGuess && isLikelyPodmanVersion(hint)) {
    return getRuntimeDisplay('podman', { hint });
  }

  if (base.id === 'unknown') {
    return getRuntimeDisplay('docker', { hint });
  }

  return base;
};
