const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const asPositiveInteger = (value: unknown): number | undefined => {
  if (typeof value === 'number' && Number.isInteger(value) && value > 0) {
    return value;
  }
  if (typeof value === 'string' && /^\d+$/.test(value.trim())) {
    const parsed = Number.parseInt(value.trim(), 10);
    if (parsed > 0) {
      return parsed;
    }
  }
  return undefined;
};

const asGuestKeyPart = (value: unknown): string | undefined => {
  const normalized = asString(value);
  return normalized && !normalized.includes('/') ? normalized : undefined;
};

const uniqueIds = (...values: unknown[]): string[] => {
  const ids: string[] = [];
  const seen = new Set<string>();

  values.forEach((value) => {
    const normalized = asString(value);
    if (!normalized || seen.has(normalized)) return;
    seen.add(normalized);
    ids.push(normalized);
  });

  return ids;
};

interface GuestOverrideResourceLike {
  id?: string;
  instance?: string;
  node?: string;
  vmid?: number | string;
  proxmox?: unknown;
  platformData?: unknown;
}

interface GuestOverrideIdentity {
  id?: string;
  instance: string;
  node: string;
  vmid: number;
}

const stableGuestOverrideKey = (instance: string, vmid: number): string =>
  `guest:${instance}:${vmid}`;

const legacyGuestStableOverrideKey = (instance: string, vmid: number): string =>
  `${instance}-${vmid}`;

const legacyClusterGuestOverrideKey = (instance: string, node: string, vmid: number): string =>
  `${instance}-${node}-${vmid}`;

const parseCanonicalGuestOverrideKey = (key: string): GuestOverrideIdentity | undefined => {
  const trimmed = key.trim();
  const parts = trimmed.split(':');
  if (parts.length !== 3 || parts[0]?.toLowerCase() === 'guest') {
    return undefined;
  }

  const instance = asGuestKeyPart(parts[0]);
  const node = asGuestKeyPart(parts[1]);
  const vmid = asPositiveInteger(parts[2]);
  if (!instance || !node || !vmid) {
    return undefined;
  }

  return {
    id: trimmed,
    instance,
    node,
    vmid,
  };
};

export const getGuestOverrideIdentity = (
  resource: GuestOverrideResourceLike,
): GuestOverrideIdentity | undefined => {
  const platformData = asRecord(resource.platformData);
  const directProxmox = asRecord(resource.proxmox);
  const platformProxmox = asRecord(platformData?.proxmox);

  const node =
    asGuestKeyPart(resource.node) ||
    asGuestKeyPart(directProxmox?.node) ||
    asGuestKeyPart(directProxmox?.nodeName) ||
    asGuestKeyPart(platformProxmox?.node) ||
    asGuestKeyPart(platformProxmox?.nodeName) ||
    asGuestKeyPart(platformData?.node) ||
    asGuestKeyPart(platformData?.nodeName);

  const instance =
    asGuestKeyPart(resource.instance) ||
    asGuestKeyPart(directProxmox?.instance) ||
    asGuestKeyPart(platformProxmox?.instance) ||
    asGuestKeyPart(platformData?.instance) ||
    node;

  const vmid =
    asPositiveInteger(resource.vmid) ||
    asPositiveInteger(directProxmox?.vmid) ||
    asPositiveInteger(platformProxmox?.vmid) ||
    asPositiveInteger(platformData?.vmid);

  if (!instance || !node || !vmid) {
    return undefined;
  }

  return {
    id: asString(resource.id),
    instance,
    node,
    vmid,
  };
};

export const canonicalGuestOverrideResourceId = (
  resource: GuestOverrideResourceLike,
): string | undefined => {
  const identity = getGuestOverrideIdentity(resource);
  if (!identity) {
    return undefined;
  }
  return `${identity.instance}:${identity.node}:${identity.vmid}`;
};

export const guestOverrideStorageId = (resource: GuestOverrideResourceLike): string => {
  const identity = getGuestOverrideIdentity(resource);
  if (!identity) {
    return asString(resource.id) || '';
  }
  if (identity.instance !== identity.node) {
    return stableGuestOverrideKey(identity.instance, identity.vmid);
  }
  return `${identity.instance}:${identity.node}:${identity.vmid}`;
};

export const guestOverrideIdCandidates = (resource: GuestOverrideResourceLike): string[] => {
  const identity = getGuestOverrideIdentity(resource);
  if (!identity) {
    return uniqueIds(resource.id);
  }

  const canonicalId = `${identity.instance}:${identity.node}:${identity.vmid}`;
  const stableId =
    identity.instance !== identity.node
      ? stableGuestOverrideKey(identity.instance, identity.vmid)
      : canonicalId;

  return uniqueIds(
    stableId,
    canonicalId,
    identity.id,
    legacyGuestStableOverrideKey(identity.instance, identity.vmid),
    identity.instance !== identity.node
      ? legacyClusterGuestOverrideKey(identity.instance, identity.node, identity.vmid)
      : undefined,
  );
};

export const normalizeGuestOverrideKey = (key: string): string => {
  const parsed = parseCanonicalGuestOverrideKey(key);
  if (!parsed || parsed.instance === parsed.node) {
    return key.trim();
  }
  return stableGuestOverrideKey(parsed.instance, parsed.vmid);
};
