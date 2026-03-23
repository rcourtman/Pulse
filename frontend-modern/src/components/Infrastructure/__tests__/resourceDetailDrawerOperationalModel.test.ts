import { describe, expect, it } from 'vitest';

import {
  buildHostDetailCards,
  buildHostDetailSummary,
  buildKubernetesCapabilityBadges,
  buildRelatedLinks,
  buildSourceSummary,
  hasRuntimeOperationalContext,
} from '@/components/Infrastructure/resourceDetailDrawerOperationalModel';
import type { Resource } from '@/types/resource';

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

describe('resourceDetailDrawerOperationalModel', () => {
  it('builds kubernetes capability badges from canonical metric capabilities', () => {
    expect(
      buildKubernetesCapabilityBadges({
        nodeCpuMemory: true,
        nodeTelemetry: true,
        podCpuMemory: true,
        podNetwork: false,
        podEphemeralDisk: true,
        podDiskIo: false,
      }),
    ).toEqual([
      {
        label: 'K8s Node CPU/Memory',
        classes:
          'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-400',
        title: 'Node CPU and memory metrics are available.',
      },
      {
        label: 'Node Telemetry (Agent)',
        classes:
          'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-400',
        title:
          'Linked Pulse agent provides node uptime, temperature, disk, network, and disk I/O.',
      },
      {
        label: 'Pod CPU/Memory',
        classes:
          'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-400',
        title: 'Pod CPU and memory metrics are available.',
      },
      {
        label: 'Pod Ephemeral Disk',
        classes:
          'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-cyan-100 text-cyan-700 dark:bg-cyan-900 dark:text-cyan-400',
        title: 'Pod ephemeral storage usage is available.',
      },
      {
        label: 'Pod Disk I/O Unsupported',
        classes:
          'inline-flex items-center rounded px-2 py-0.5 text-[10px] font-medium whitespace-nowrap bg-surface-alt text-muted',
        title:
          'Pod disk read/write throughput is not collected by the Kubernetes integration path today.',
      },
    ]);
  });

  it('prefers source health summaries and falls back to merged source counts', () => {
    expect(
      buildSourceSummary(
        ['agent', 'docker'],
        {
          agent: { status: 'healthy' },
          docker: { status: 'degraded' },
        },
      ),
    ).toEqual({
      label: '1/2 degraded',
      className: 'text-amber-600 dark:text-amber-400',
      title: 'agent:healthy • docker:degraded',
    });

    expect(buildSourceSummary(['agent', 'docker'], {})).toEqual({
      label: '2 sources',
      className: 'text-base-content',
      title: 'agent • docker',
    });
  });

  it('keeps host detail coverage and runtime context on canonical operational inputs', () => {
    const cards = buildHostDetailCards({
      hasProxmoxNode: true,
      hasAgentDetails: true,
      networkInterfaceCount: 2,
      diskCount: 1,
      raidCount: 0,
      temperatureRowCount: 3,
    });

    expect(cards).toEqual([
      'system',
      'hardware',
      'storage',
      'system',
      'hardware',
      'network',
      'disks',
      'temperatures',
    ]);
    expect(buildHostDetailSummary(cards)).toBe(
      '8 detail cards covering system, hardware, storage, network, disks, and temperatures.',
    );
    expect(hasRuntimeOperationalContext([], [])).toBe(false);
  });

  it('builds canonical related links from workloads and service detail surfaces', () => {
    expect(
      buildRelatedLinks(
        baseResource({
          type: 'docker-host',
          platformType: 'docker',
          platformData: { sources: ['docker'], docker: { hostSourceId: 'agent-1' } },
        }),
        'Host 1',
      ),
    ).toEqual([
      {
        href: '/workloads?type=app-container&agent=agent-1',
        label: 'Open in Workloads',
        compactLabel: 'Workloads',
        ariaLabel: 'Open related workloads for Host 1',
      },
    ]);
  });

  it('omits generic host-wide workloads links from drawer quick links', () => {
    expect(
      buildRelatedLinks(
        baseResource({
          type: 'agent',
          platformType: 'agent',
          platformData: { sources: ['agent'], agent: { hostname: 'host-1' } },
        }),
        'Host 1',
      ),
    ).toEqual([]);
  });
});
