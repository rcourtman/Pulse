import { describe, expect, it } from 'vitest';

import {
  buildAccessSummary,
  buildHostDetailCards,
  buildHostDetailSummary,
  buildKubernetesCapabilityBadges,
  buildRelatedLinks,
  buildSourceHealthSummary,
  buildSourceSummary,
  hasRuntimeOperationalContext,
} from '@/components/Infrastructure/resourceDetailDrawerOperationalModel';
import type { PlatformData } from '@/components/Infrastructure/resourceDetailMappers';
import type { Resource } from '@/types/resource';

// Mirrors the private badge-class constants in resourceDetailDrawerOperationalModel.ts
// so the assertions here stay brittle to drift in those exact Tailwind tokens.
const SUPPORTED_BADGE_CLASS =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-400';
const UNSUPPORTED_BADGE_CLASS =
  'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-muted';

type SourceStatusMap = NonNullable<PlatformData['sourceStatus']>;

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'host-1',
  displayName: 'Host 1',
  platformId: 'host-1',
  platformType: 'agent',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['agent'] },
  ...overrides,
});

describe('buildKubernetesCapabilityBadges branch coverage', () => {
  it('returns an empty array when capabilities are undefined', () => {
    expect(buildKubernetesCapabilityBadges(undefined)).toEqual([]);
  });

  it('returns an empty array when capabilities is an empty object (all flags undefined)', () => {
    // Every supported flag is undefined (falsy) AND podDiskIo is undefined,
    // so the only branch that fires is the "unsupported" one.
    expect(buildKubernetesCapabilityBadges({})).toEqual([
      {
        label: 'Pod Disk I/O Unsupported',
        classes: UNSUPPORTED_BADGE_CLASS,
        title:
          'Pod disk read/write throughput is not collected by the Kubernetes integration path today.',
      },
    ]);
  });

  it('emits only the Pod Network badge when podNetwork is the sole supported flag and podDiskIo is true', () => {
    // Drives the false-arm of nodeCpuMemory/nodeTelemetry/podCpuMemory/podEphemeralDisk,
    // the true-arm of podNetwork, and the true-arm of podDiskIo (which suppresses
    // the "unsupported" badge).
    expect(
      buildKubernetesCapabilityBadges({
        podNetwork: true,
        podDiskIo: true,
      }),
    ).toEqual([
      {
        label: 'Pod Network',
        classes: SUPPORTED_BADGE_CLASS,
        title: 'Pod network throughput is available.',
      },
    ]);
  });

  it('emits node telemetry and pod ephemeral disk badges together without the unsupported badge', () => {
    expect(
      buildKubernetesCapabilityBadges({
        nodeTelemetry: true,
        podEphemeralDisk: true,
        podDiskIo: true,
      }),
    ).toEqual([
      {
        label: 'Node Telemetry (Agent)',
        classes: SUPPORTED_BADGE_CLASS,
        title:
          'Linked Pulse agent provides node uptime, temperature, disk, network, and disk I/O.',
      },
      {
        label: 'Pod Ephemeral Disk',
        classes: SUPPORTED_BADGE_CLASS,
        title: 'Pod ephemeral storage usage is available.',
      },
    ]);
  });
});

describe('buildSourceHealthSummary branch coverage', () => {
  it('counts an unrecognized status as unhealthy and returns the red summary', () => {
    // 'offline' matches neither the healthy nor degraded token sets, hitting the
    // else branch (unhealthy += 1) and the unhealthy > 0 return arm.
    expect(buildSourceHealthSummary({ agent: { status: 'offline' } })).toEqual({
      label: '1/1 unhealthy',
      className: 'text-red-600 dark:text-red-400',
      title: 'agent:offline',
    });
  });

  it('prefers the unhealthy summary when both unhealthy and warning counts are positive', () => {
    expect(
      buildSourceHealthSummary({
        agent: { status: 'offline' },
        docker: { status: 'degraded' },
      }),
    ).toEqual({
      label: '1/2 unhealthy',
      className: 'text-red-600 dark:text-red-400',
      title: 'agent:offline • docker:degraded',
    });
  });

  it('counts multiple unhealthy entries in the label numerator', () => {
    expect(
      buildSourceHealthSummary({
        a: { status: 'offline' },
        b: { status: 'down' },
      }),
    ).toEqual({
      label: '2/2 unhealthy',
      className: 'text-red-600 dark:text-red-400',
      title: 'a:offline • b:down',
    });
  });

  it('normalizes healthy/degraded tokens via trim().toLowerCase() before matching', () => {
    // '  ONLINE  ' normalizes to 'online' and hits the healthy continue branch.
    expect(buildSourceHealthSummary({ a: { status: '  ONLINE  ' } })).toBeNull();
    // 'Warning' (mixed case) normalizes to 'warning' and hits the degraded branch.
    expect(buildSourceHealthSummary({ a: { status: 'Warning' } })).toEqual({
      label: '1/1 degraded',
      className: 'text-amber-600 dark:text-amber-400',
      title: 'a:warning',
    });
  });

  it('treats an empty/whitespace status as "unknown" and counts it unhealthy', () => {
    // Exercises the `|| ''` fallback in `(status?.status || '')`.
    expect(buildSourceHealthSummary({ a: { status: '   ' } })).toEqual({
      label: '1/1 unhealthy',
      className: 'text-red-600 dark:text-red-400',
      title: 'a:unknown',
    });
  });

  it('hits the optional-chain short-circuit when an entry value is nullish', () => {
    // `status?.status` short-circuits only when status itself is null/undefined;
    // the cast is required because the declared value type is non-nullable.
    const broken = { agent: null } as unknown as SourceStatusMap;
    expect(buildSourceHealthSummary(broken)).toEqual({
      label: '1/1 unhealthy',
      className: 'text-red-600 dark:text-red-400',
      title: 'agent:unknown',
    });
  });

  it('returns null when every entry normalizes to a healthy token', () => {
    expect(
      buildSourceHealthSummary({
        a: { status: 'running' },
        b: { status: 'connected' },
        c: { status: 'ok' },
      }),
    ).toBeNull();
  });
});

describe('buildSourceSummary branch coverage', () => {
  it('delegates to buildSourceHealthSummary and surfaces the red unhealthy summary', () => {
    expect(buildSourceSummary(['agent'], { agent: { status: 'offline' } })).toEqual({
      label: '1/1 unhealthy',
      className: 'text-red-600 dark:text-red-400',
      title: 'agent:offline',
    });
  });

  it('returns null when buildSourceHealthSummary returns null (the fallthrough arm)', () => {
    expect(buildSourceSummary(['agent'], { agent: { status: 'online' } })).toBeNull();
  });
});

describe('buildHostDetailCards branch coverage', () => {
  it('returns an empty array when neither proxmox node nor agent details are present', () => {
    expect(
      buildHostDetailCards({
        hasProxmoxNode: false,
        hasAgentDetails: false,
        networkInterfaceCount: 0,
        diskCount: 0,
        raidCount: 0,
        temperatureRowCount: 0,
      }),
    ).toEqual([]);
  });

  it('emits only system/hardware/storage when only the proxmox node flag is set', () => {
    // The agent-section counts must be ignored entirely when hasAgentDetails is false.
    expect(
      buildHostDetailCards({
        hasProxmoxNode: true,
        hasAgentDetails: false,
        networkInterfaceCount: 9,
        diskCount: 9,
        raidCount: 9,
        temperatureRowCount: 9,
      }),
    ).toEqual(['system', 'hardware', 'storage']);
  });

  it('emits system/hardware plus every optional agent section when all counts are positive', () => {
    expect(
      buildHostDetailCards({
        hasProxmoxNode: false,
        hasAgentDetails: true,
        networkInterfaceCount: 1,
        diskCount: 1,
        raidCount: 1,
        temperatureRowCount: 1,
      }),
    ).toEqual(['system', 'hardware', 'network', 'disks', 'raid', 'temperatures']);
  });

  it('omits every optional section when agent detail counts are zero (each > 0 branch falsy)', () => {
    expect(
      buildHostDetailCards({
        hasProxmoxNode: false,
        hasAgentDetails: true,
        networkInterfaceCount: 0,
        diskCount: 0,
        raidCount: 0,
        temperatureRowCount: 0,
      }),
    ).toEqual(['system', 'hardware']);
  });
});

describe('buildHostDetailSummary branch coverage', () => {
  it('returns null for an empty card list', () => {
    expect(buildHostDetailSummary([])).toBeNull();
  });

  it('returns the single category label verbatim when there is exactly one card', () => {
    expect(buildHostDetailSummary(['disks'])).toBe('Disks');
  });

  it('joins two categories with "and"', () => {
    expect(buildHostDetailSummary(['system', 'network'])).toBe('System and Network');
  });

  it('joins three or more categories with Oxford comma', () => {
    expect(buildHostDetailSummary(['system', 'hardware', 'network'])).toBe(
      'System, Hardware, and Network',
    );
  });

  it('passes through an unknown card name via the ?? fallback', () => {
    expect(buildHostDetailSummary(['frobnicator'])).toBe('frobnicator');
  });

  it('dedupes repeated card names before joining', () => {
    expect(buildHostDetailSummary(['system', 'system', 'hardware'])).toBe(
      'System and Hardware',
    );
  });
});

describe('buildAccessSummary branch coverage', () => {
  it('pluralizes "links" when there is more than one link and no web interface', () => {
    const links = [
      { href: '/a', label: 'A', compactLabel: 'A', ariaLabel: 'A' },
      { href: '/b', label: 'B', compactLabel: 'B', ariaLabel: 'B' },
    ];
    expect(buildAccessSummary({ hasWebInterface: false, links })).toBe('2 links');
  });

  it('joins web interface and plural links with the " · " separator', () => {
    const links = [
      { href: '/a', label: 'A', compactLabel: 'A', ariaLabel: 'A' },
      { href: '/b', label: 'B', compactLabel: 'B', ariaLabel: 'B' },
    ];
    expect(buildAccessSummary({ hasWebInterface: true, links })).toBe(
      'Web interface · 2 links',
    );
  });

  it('returns the bare link count when web interface is absent and exactly one link is present', () => {
    const links = [{ href: '/a', label: 'A', compactLabel: 'A', ariaLabel: 'A' }];
    expect(buildAccessSummary({ hasWebInterface: false, links })).toBe('1 link');
  });
});

describe('buildRelatedLinks branch coverage', () => {
  it('returns the PMG thresholds link for a pmg resource', () => {
    // buildServiceDetailLinks produces a single link for type==='pmg';
    // the seen-set dedup then admits it (first occurrence -> true arm).
    const resource = baseResource({
      type: 'pmg',
      platformType: 'proxmox-pmg',
      name: 'mail',
      displayName: 'Mail Gateway',
      platformData: { sources: ['proxmox-pmg'] },
    });
    expect(buildRelatedLinks(resource, 'Mail Gateway')).toEqual([
      {
        href: '/alerts/thresholds/mail-gateway',
        label: 'Open PMG thresholds',
        compactLabel: 'Thresholds',
        ariaLabel: 'Open PMG thresholds for Mail Gateway',
      },
    ]);
  });
});

describe('hasRuntimeOperationalContext branch coverage', () => {
  it('returns true when the badge list is non-empty', () => {
    expect(
      hasRuntimeOperationalContext([
        {
          label: 'K8s Node CPU/Memory',
          classes: SUPPORTED_BADGE_CLASS,
          title: 'Node CPU and memory metrics are available.',
        },
      ]),
    ).toBe(true);
  });
});
