import type { AvailabilityTargetKind } from '@/api/availabilityTargets';

/**
 * What an availability check is on. The kind is the owner's statement of what
 * the thing is, stored with the check, and it is the one place every surface
 * reads: the form that sets it, the lists that name it and the map, which
 * draws network gear as gear. No surface works a kind out for itself. A
 * `machine` check is still a check: only a Pulse agent makes a Machines row.
 */
export interface AvailabilityTargetKindInfo {
  value: AvailabilityTargetKind;
  /** The choice in the form. */
  label: string;
  /** For a list or a badge. */
  short: string;
  /** In a sentence, a button or under a map tile. */
  noun: string;
  namePlaceholder: string;
  addressPlaceholder: string;
}

export const AVAILABILITY_TARGET_KINDS: readonly AvailabilityTargetKindInfo[] = [
  {
    value: 'service',
    label: 'Website or service',
    short: 'Service',
    noun: 'service',
    namePlaceholder: 'mqtt-broker',
    addressPlaceholder: 'service.local',
  },
  {
    value: 'machine',
    label: 'Machine or server',
    short: 'Machine',
    noun: 'machine',
    namePlaceholder: 'mac-mini',
    addressPlaceholder: 'server.local',
  },
  {
    value: 'router',
    label: 'Router or firewall',
    short: 'Router',
    noun: 'router',
    namePlaceholder: 'router',
    addressPlaceholder: '192.168.1.1',
  },
  {
    value: 'switch',
    label: 'Switch',
    short: 'Switch',
    noun: 'switch',
    namePlaceholder: 'core-switch',
    addressPlaceholder: 'switch.local',
  },
  {
    value: 'access_point',
    label: 'Access point',
    short: 'Access point',
    noun: 'access point',
    namePlaceholder: 'ap-living-room',
    addressPlaceholder: 'ap.local',
  },
  {
    value: 'device',
    label: 'Other device',
    short: 'Device',
    noun: 'device',
    namePlaceholder: 'energy-monitor',
    addressPlaceholder: 'device.local',
  },
];

const KIND_BY_VALUE = new Map(AVAILABILITY_TARGET_KINDS.map((kind) => [kind.value, kind]));

export const AVAILABILITY_TARGET_KIND_VALUES: readonly AvailabilityTargetKind[] =
  AVAILABILITY_TARGET_KINDS.map((kind) => kind.value);

/** The kind a stored or typed value names, or nothing when it names none. */
export function normalizeAvailabilityTargetKind(
  value: string | null | undefined,
): AvailabilityTargetKind | undefined {
  const normalized = value?.trim().toLowerCase();
  return AVAILABILITY_TARGET_KIND_VALUES.find((kind) => kind === normalized);
}

export const getAvailabilityTargetKindInfo = (
  value: string | null | undefined,
): AvailabilityTargetKindInfo | undefined => {
  const kind = normalizeAvailabilityTargetKind(value);
  return kind ? KIND_BY_VALUE.get(kind) : undefined;
};

export type AvailabilityNetworkGearKind = 'router' | 'switch' | 'access_point';

/** A router, a switch or an access point: what a network is made of. */
export const isNetworkGearKind = (
  value: string | null | undefined,
): value is AvailabilityNetworkGearKind => {
  const kind = normalizeAvailabilityTargetKind(value);
  return kind === 'router' || kind === 'switch' || kind === 'access_point';
};

// Words an owner gives the thing, and the systems a router runs. `gateway`
// is left out: an API gateway and a mail gateway are checked too, and a
// check on a real default gateway is known by its address.
const ROUTER_WORDS = new Set([
  'router',
  'firewall',
  'modem',
  'opnsense',
  'pfsense',
  'mikrotik',
  'routeros',
  'edgerouter',
  'openwrt',
  'fritzbox',
  'udm',
  'usg',
]);
const SWITCH_WORDS = new Set(['switch', 'switches']);
const ACCESS_POINT_WORDS = new Set(['ap', 'aps', 'wifi', 'wlan', 'accesspoint']);
// A console is not network gear, whatever it is called.
const NOT_GEAR_WORDS = new Set(['nintendo', 'kvm']);

export interface AvailabilityTargetKindSubject {
  name?: string;
  address?: string;
  /** Addresses that monitored machines use as their default gateway. */
  gatewayAddresses?: ReadonlySet<string>;
}

/**
 * The network gear a check looks like it is on, to offer in the form. It is
 * a suggestion and nothing reads it but the form: the owner takes it or
 * leaves it, and what they save is the kind. Whole words only, so
 * `apt-cache` is no access point.
 */
export const suggestAvailabilityTargetKind = (
  subject: AvailabilityTargetKindSubject,
): AvailabilityNetworkGearKind | undefined => {
  const words = (subject.name ?? '')
    .toLowerCase()
    .replace(/wi-fi/g, 'wifi')
    // FRITZ!Box is one word to its owner.
    .replace(/!/g, '')
    .replace(/access[\s_-]*points?/g, 'accesspoint')
    .split(/[^a-z0-9]+/)
    .filter(Boolean);
  if (!words.some((word) => NOT_GEAR_WORDS.has(word))) {
    if (words.some((word) => ROUTER_WORDS.has(word))) return 'router';
    if (words.some((word) => SWITCH_WORDS.has(word))) return 'switch';
    if (words.some((word) => ACCESS_POINT_WORDS.has(word))) return 'access_point';
  }
  const address = subject.address?.trim();
  if (address && subject.gatewayAddresses?.has(address)) return 'router';
  return undefined;
};

/** The words on the button that saves a new check of this kind. */
export const getAvailabilityTargetAddLabel = (value: string | null | undefined): string => {
  const kind = normalizeAvailabilityTargetKind(value);
  if (kind === 'service' || kind === 'device') return 'Add service/device check';
  const info = kind ? KIND_BY_VALUE.get(kind) : undefined;
  return info ? `Add ${info.noun} check` : 'Add availability check';
};
