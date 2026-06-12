import type { WorkloadGuest, WorkloadType } from '@/types/workloads';
import { buildDockerPath, buildDockerRouteSearch } from '@/routing/resourceLinks';
import {
  getCanonicalWorkloadId,
  resolveWorkloadType,
  workloadMatchesPlatformScope,
} from '@/utils/workloads';

const LXC_DOCKER_HOST_PREFIX = 'proxmox-lxc-docker:';

export interface NestedWorkloadContextItem {
  id: string;
  name: string;
  status: string;
  runtimeLabel: string;
}

export interface NestedWorkloadContext {
  type: 'app-container';
  label: string;
  title: string;
  count: number;
  href: string;
  items: NestedWorkloadContextItem[];
}

export type NestedWorkloadContextByGuestId = Record<string, NestedWorkloadContext>;

interface BuildNestedWorkloadContextParams {
  guests: readonly WorkloadGuest[];
  visibleGuests: readonly WorkloadGuest[];
  excludedWorkloadTypes?: readonly WorkloadType[];
  platformScope?: string | null;
}

type NestedWorkloadContextBuildItem = NestedWorkloadContextItem & {
  hostLabel: string;
};

const cleanText = (value?: string | number | null): string => String(value ?? '').trim();

const normalizeCandidate = (value?: string | number | null): string =>
  cleanText(value).toLowerCase();

const addCandidate = (target: Set<string>, value?: string | number | null): void => {
  const normalized = normalizeCandidate(value);
  if (normalized) target.add(normalized);
};

const addHostnameCandidates = (target: Set<string>, value?: string | null): void => {
  const normalized = normalizeCandidate(value);
  if (!normalized) return;
  target.add(normalized);
  const shortName = normalized.split('.')[0];
  if (shortName) target.add(shortName);
};

const buildNodeScopedIdentityCandidates = (guest: WorkloadGuest): Set<string> => {
  const candidates = new Set<string>();
  const instance = cleanText(guest.instance);
  const node = cleanText(guest.node);
  const vmid = Number.isFinite(guest.vmid) ? String(Number(guest.vmid)) : '';

  addCandidate(candidates, getCanonicalWorkloadId(guest));
  addCandidate(candidates, guest.id);
  addCandidate(candidates, guest.displayId);
  addHostnameCandidates(candidates, guest.name);

  if (instance && node && vmid) addCandidate(candidates, `${instance}:${node}:${vmid}`);
  if (instance && vmid) addCandidate(candidates, `${instance}:${vmid}`);
  if (node && vmid) addCandidate(candidates, `${node}:${vmid}`);
  if (vmid) addCandidate(candidates, vmid);

  return candidates;
};

const addDockerHostIdentitySegments = (target: Set<string>, value?: string | null): void => {
  const normalized = normalizeCandidate(value);
  if (!normalized) return;

  addCandidate(target, normalized);
  const withoutPrefix = normalized.startsWith(LXC_DOCKER_HOST_PREFIX)
    ? normalized.slice(LXC_DOCKER_HOST_PREFIX.length)
    : normalized;
  addCandidate(target, withoutPrefix);

  const segments = withoutPrefix.split(':').filter(Boolean);
  if (segments.length >= 3) addCandidate(target, segments.slice(-3).join(':'));
  if (segments.length >= 2) addCandidate(target, segments.slice(-2).join(':'));
  if (segments.length >= 1) addCandidate(target, segments[segments.length - 1]);
};

const buildAppContainerParentCandidates = (guest: WorkloadGuest): Set<string> => {
  const candidates = new Set<string>();
  addDockerHostIdentitySegments(candidates, guest.dockerHostId);
  addHostnameCandidates(candidates, guest.contextLabel);
  addHostnameCandidates(candidates, guest.node);
  addHostnameCandidates(candidates, guest.instance);
  return candidates;
};

const formatRuntimeLabel = (runtime?: string | null): string => {
  const normalized = normalizeCandidate(runtime);
  if (normalized === 'podman') return 'Podman';
  return 'Docker';
};

const formatStatusLabel = (status?: string | null): string => cleanText(status) || 'unknown';

const createNestedItem = (guest: WorkloadGuest): NestedWorkloadContextBuildItem => ({
  id: getCanonicalWorkloadId(guest),
  name: cleanText(guest.name) || cleanText(guest.containerId) || getCanonicalWorkloadId(guest),
  status: formatStatusLabel(guest.status),
  runtimeLabel: formatRuntimeLabel(guest.containerRuntime),
  hostLabel: cleanText(guest.dockerHostName) || cleanText(guest.contextLabel),
});

const chooseContextLabel = (items: readonly NestedWorkloadContextItem[]): string => {
  const labels = Array.from(new Set(items.map((item) => item.runtimeLabel)));
  if (labels.length === 1) return labels[0];
  return 'Containers';
};

const chooseDockerHostFilter = (items: readonly NestedWorkloadContextBuildItem[]): string => {
  const labels = Array.from(new Set(items.map((item) => item.hostLabel).filter(Boolean)));
  return labels.length === 1 ? labels[0] : '';
};

const buildNestedWorkloadHref = (host: string): string =>
  `${buildDockerPath('overview')}${buildDockerRouteSearch({ host })}`;

const toNestedWorkloadContextItem = (
  item: NestedWorkloadContextBuildItem,
): NestedWorkloadContextItem => ({
  id: item.id,
  name: item.name,
  status: item.status,
  runtimeLabel: item.runtimeLabel,
});

export const buildNestedWorkloadContextByGuestId = ({
  guests,
  visibleGuests,
  excludedWorkloadTypes,
  platformScope,
}: BuildNestedWorkloadContextParams): NestedWorkloadContextByGuestId => {
  const excludedTypes = new Set(excludedWorkloadTypes ?? []);
  if (!excludedTypes.has('app-container')) return {};

  const parentKeyOwners = new Map<string, string | null>();
  const parentById = new Map<string, WorkloadGuest>();
  for (const guest of visibleGuests) {
    const type = resolveWorkloadType(guest);
    if (type !== 'vm' && type !== 'system-container') continue;

    const parentId = getCanonicalWorkloadId(guest);
    parentById.set(parentId, guest);
    for (const candidate of buildNodeScopedIdentityCandidates(guest)) {
      const existing = parentKeyOwners.get(candidate);
      parentKeyOwners.set(
        candidate,
        existing === undefined || existing === parentId ? parentId : null,
      );
    }
  }

  const itemsByParentId = new Map<string, NestedWorkloadContextBuildItem[]>();
  for (const guest of guests) {
    if (resolveWorkloadType(guest) !== 'app-container') continue;
    if (!workloadMatchesPlatformScope(guest, platformScope)) continue;

    let parentId: string | null = null;
    for (const candidate of buildAppContainerParentCandidates(guest)) {
      const owner = parentKeyOwners.get(candidate);
      if (owner) {
        parentId = owner;
        break;
      }
    }
    if (!parentId || !parentById.has(parentId)) continue;

    const items = itemsByParentId.get(parentId) ?? [];
    items.push(createNestedItem(guest));
    itemsByParentId.set(parentId, items);
  }

  const contexts: NestedWorkloadContextByGuestId = {};
  for (const [parentId, items] of itemsByParentId) {
    const sortedItems = [...items].sort((a, b) => a.name.localeCompare(b.name));
    const label = chooseContextLabel(sortedItems);
    const host = chooseDockerHostFilter(sortedItems);
    contexts[parentId] = {
      type: 'app-container',
      label,
      title: label === 'Containers' ? 'Nested containers' : `Nested ${label}`,
      count: sortedItems.length,
      href: buildNestedWorkloadHref(host),
      items: sortedItems.map(toNestedWorkloadContextItem),
    };
  }

  return contexts;
};
