import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { createRoot, createSignal } from 'solid-js';
import type { Resource } from '@/types/resource';
import {
  PLATFORM_TABLE_COMPACT_DATE_TIME_FORMAT,
  PlatformTableCountRatioValue,
  PlatformTableDateTimeValue,
  PlatformTableMetricFallback,
  PlatformTableNumberValue,
  PlatformTablePercentValue,
  PlatformTableRelativeTimeValue,
  PlatformTableTemperatureValue,
  createPlatformTableFilterState,
  formatPlatformTableBytesValue,
  formatPlatformTableCountRatioValue,
  formatPlatformTableDateTimeValue,
  formatPlatformTableIntegerValue,
  formatPlatformTablePercentValue,
  formatPlatformTableRelativeTimeValue,
  formatPlatformTableTitleCaseValue,
  formatPlatformTableUptimeValue,
  filterPlatformResources,
  formatPlatformTableTextValue,
  getPlatformTableFiniteMetric,
  summarizePlatformTableValues,
  type PlatformResourceStatusFilter,
} from '../sharedPlatformPage';

afterEach(cleanup);

const makeResource = (
  partial: Partial<Resource> & Pick<Resource, 'id' | 'type' | 'status'>,
): Resource => ({
  name: partial.id,
  displayName: partial.id,
  platformId: 'lab',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  lastSeen: 1_700_000_000_000,
  ...partial,
});

describe('filterPlatformResources', () => {
  const resources: Resource[] = [
    makeResource({ id: 'host-alpha', type: 'agent', status: 'online' }),
    makeResource({ id: 'host-bravo', type: 'agent', status: 'running' }),
    makeResource({ id: 'host-charlie', type: 'agent', status: 'degraded' }),
    makeResource({ id: 'host-delta', type: 'agent', status: 'offline' }),
    makeResource({ id: 'host-echo', type: 'agent', status: 'stopped' }),
    makeResource({ id: 'host-foxtrot', type: 'agent', status: 'paused' }),
    makeResource({
      id: 'host-golf',
      type: 'agent',
      status: 'warning' as Resource['status'],
    }),
    makeResource({
      id: 'host-with-tag',
      type: 'agent',
      status: 'online',
      tags: ['prod', 'gpu'],
    }),
  ];

  it('keeps all rows when no filters apply', () => {
    expect(filterPlatformResources(resources, '', 'all')).toHaveLength(resources.length);
  });

  it('collapses online/running into the online status chip', () => {
    const filtered = filterPlatformResources(resources, '', 'online');
    expect(filtered.map((r) => r.id).sort()).toEqual(
      ['host-alpha', 'host-bravo', 'host-with-tag'].sort(),
    );
  });

  it('collapses degraded/warning/paused into the degraded chip', () => {
    const filtered = filterPlatformResources(resources, '', 'degraded');
    expect(filtered.map((r) => r.id).sort()).toEqual(
      ['host-charlie', 'host-foxtrot', 'host-golf'].sort(),
    );
  });

  it('collapses offline/stopped into the offline chip', () => {
    const filtered = filterPlatformResources(resources, '', 'offline');
    expect(filtered.map((r) => r.id).sort()).toEqual(['host-delta', 'host-echo'].sort());
  });

  it('searches against id, display name, parent, and tags case-insensitively', () => {
    expect(filterPlatformResources(resources, 'ALPHA', 'all').map((r) => r.id)).toEqual([
      'host-alpha',
    ]);
    expect(filterPlatformResources(resources, 'gpu', 'all').map((r) => r.id)).toEqual([
      'host-with-tag',
    ]);
  });

  it('searches the platform-native metadata that bespoke tables still consume directly', () => {
    // Docker / Kubernetes lookups moved to their per-platform helpers
    // (filterDockerResources / filterKubernetesResources) so the shared
    // helper stays platform-agnostic. The two providers that still consume
    // this filter directly — Proxmox Mail Gateway and the vSphere hosts
    // table — keep their native-metadata coverage here.
    const nativeRows: Resource[] = [
      makeResource({
        id: 'pmg-host',
        type: 'agent',
        status: 'online',
        pmg: { hostname: 'pmg-primary', version: '8.2.4' },
      }),
      makeResource({
        id: 'vsphere-host',
        type: 'agent',
        status: 'online',
        vmware: { clusterName: 'prod-cluster', runtimeHostName: 'esxi-04' },
      }),
    ];

    expect(filterPlatformResources(nativeRows, 'pmg-primary', 'all').map((r) => r.id)).toEqual([
      'pmg-host',
    ]);
    expect(filterPlatformResources(nativeRows, 'prod-cluster', 'all').map((r) => r.id)).toEqual([
      'vsphere-host',
    ]);
  });

  it('no longer matches docker.* or kubernetes.* fields directly', () => {
    const dockerOnlyRow = makeResource({
      id: 'docker-host',
      type: 'agent',
      status: 'online',
      docker: { runtimeVersion: '24.0.7', swarm: { nodeRole: 'manager' } },
    });
    const k8sOnlyRow = makeResource({
      id: 'k8s-deploy',
      type: 'k8s-deployment',
      status: 'online',
      kubernetes: { clusterName: 'prod-cluster', namespace: 'payments' },
    });

    expect(filterPlatformResources([dockerOnlyRow], 'manager', 'all')).toEqual([]);
    expect(filterPlatformResources([k8sOnlyRow], 'payments', 'all')).toEqual([]);
  });

  it('combines search and status filters', () => {
    const filtered = filterPlatformResources(resources, 'host', 'degraded');
    expect(filtered.map((r) => r.id).sort()).toEqual(
      ['host-charlie', 'host-foxtrot', 'host-golf'].sort(),
    );
  });

  it('supports platform table status resolvers for source-aware display state', () => {
    const filtered = filterPlatformResources(resources, '', 'degraded', (resource) =>
      resource.id === 'host-alpha' ? 'degraded' : resource.status,
    );
    expect(filtered.map((r) => r.id).sort()).toEqual(
      ['host-alpha', 'host-charlie', 'host-foxtrot', 'host-golf'].sort(),
    );
  });

  it('centralizes provider table filter state and row counts', () => {
    createRoot((dispose) => {
      try {
        const state = createPlatformTableFilterState({
          resources: () => resources,
          initialStatus: 'all' as PlatformResourceStatusFilter,
          filter: filterPlatformResources,
        });

        expect(state.total()).toBe(resources.length);
        expect(state.visible()).toBe(resources.length);
        expect(state.hasActiveFilters()).toBe(false);

        state.setSearch('gpu');
        expect(state.filtered().map((r) => r.id)).toEqual(['host-with-tag']);
        expect(state.visible()).toBe(1);
        expect(state.hasActiveFilters()).toBe(true);

        state.setSearch('host');
        state.setStatus('offline');
        expect(
          state
            .filtered()
            .map((r) => r.id)
            .sort(),
        ).toEqual(['host-delta', 'host-echo']);

        state.resetFilters();
        expect(state.search()).toBe('');
        expect(state.status()).toBe('all');
        expect(state.visible()).toBe(resources.length);
        expect(state.hasActiveFilters()).toBe(false);
      } finally {
        dispose();
      }
    });
  });

  it('supports page-owned filter state for stacked table toolbars', () => {
    createRoot((dispose) => {
      try {
        const [search, setSearch] = createSignal('');
        const [status, setStatus] = createSignal<PlatformResourceStatusFilter>('all');
        const state = createPlatformTableFilterState({
          resources: () => resources,
          initialStatus: 'all' as PlatformResourceStatusFilter,
          filter: filterPlatformResources,
          externalSearch: search,
          externalStatus: status,
          onExternalSearchChange: setSearch,
          onExternalStatusChange: setStatus,
        });

        state.setSearch('gpu');
        expect(search()).toBe('gpu');
        expect(state.filtered().map((r) => r.id)).toEqual(['host-with-tag']);

        state.setStatus('online');
        expect(status()).toBe('online');
        expect(state.visible()).toBe(1);
        expect(state.hasActiveFilters()).toBe(true);

        state.resetFilters();
        expect(search()).toBe('');
        expect(status()).toBe('all');
        expect(state.visible()).toBe(resources.length);
        expect(state.hasActiveFilters()).toBe(false);
      } finally {
        dispose();
      }
    });
  });
});

describe('formatPlatformTableTextValue', () => {
  it('trims text values and uses the canonical platform-table empty cell marker', () => {
    expect(formatPlatformTableTextValue('  kubelet  ')).toBe('kubelet');
    expect(formatPlatformTableTextValue('')).toBe('—');
    expect(formatPlatformTableTextValue(undefined)).toBe('—');
    expect(formatPlatformTableTextValue(null)).toBe('—');
    expect(formatPlatformTableTextValue(' ', 'n/a')).toBe('n/a');
  });
});

describe('formatPlatformTableTitleCaseValue', () => {
  it('formats table status labels with the canonical title-case fallback', () => {
    expect(formatPlatformTableTitleCaseValue(' RUNNING ')).toBe('Running');
    expect(formatPlatformTableTitleCaseValue('degraded')).toBe('Degraded');
    expect(formatPlatformTableTitleCaseValue('')).toBe('Unknown');
    expect(formatPlatformTableTitleCaseValue(undefined)).toBe('Unknown');
    expect(formatPlatformTableTitleCaseValue(' ', 'Unavailable')).toBe('Unavailable');
  });
});

describe('summarizePlatformTableValues', () => {
  it('trims values and keeps empty summaries display-only', () => {
    expect(summarizePlatformTableValues([undefined, null, '', '  '])).toEqual({
      label: '—',
      title: '',
      values: [],
    });
    expect(summarizePlatformTableValues(undefined, { emptyText: '-' })).toEqual({
      label: '-',
      title: '',
      values: [],
    });
  });

  it('builds compact labels with full title text and normalized values', () => {
    expect(summarizePlatformTableValues([' alpha ', 'bravo', 'charlie'])).toEqual({
      label: 'alpha, bravo +1',
      title: 'alpha, bravo, charlie',
      values: ['alpha', 'bravo', 'charlie'],
    });
  });

  it('supports caller-owned visibility and value transforms', () => {
    expect(
      summarizePlatformTableValues(['nfs', 'smb', 'iscsi', 'webdav'], {
        maxVisible: 3,
        transform: (value) => value.toUpperCase(),
      }),
    ).toEqual({
      label: 'NFS, SMB, ISCSI +1',
      title: 'NFS, SMB, ISCSI, WEBDAV',
      values: ['NFS', 'SMB', 'ISCSI', 'WEBDAV'],
    });
  });
});

describe('formatPlatformTableUptimeValue', () => {
  it('formats one-unit platform table uptime labels with the canonical empty marker', () => {
    expect(formatPlatformTableUptimeValue(undefined)).toBe('—');
    expect(formatPlatformTableUptimeValue(0)).toBe('—');
    expect(formatPlatformTableUptimeValue(Number.NaN)).toBe('—');
    expect(formatPlatformTableUptimeValue(30)).toBe('0m');
    expect(formatPlatformTableUptimeValue(60)).toBe('1m');
    expect(formatPlatformTableUptimeValue(3_600)).toBe('1h');
    expect(formatPlatformTableUptimeValue(86_400)).toBe('1d');
    expect(formatPlatformTableUptimeValue(0, 'n/a')).toBe('n/a');
  });
});

describe('formatPlatformTableBytesValue', () => {
  it('formats positive byte values and preserves table-cell empty markers', () => {
    expect(formatPlatformTableBytesValue(undefined)).toBe('—');
    expect(formatPlatformTableBytesValue(0)).toBe('—');
    expect(formatPlatformTableBytesValue(Number.NaN)).toBe('—');
    expect(formatPlatformTableBytesValue(Number.POSITIVE_INFINITY)).toBe('—');
    expect(formatPlatformTableBytesValue(undefined, '-')).toBe('-');
    expect(formatPlatformTableBytesValue(1024)).toBe('1.00 KB');
    expect(formatPlatformTableBytesValue(1_536)).toBe('1.50 KB');
    expect(formatPlatformTableBytesValue(5 * 1024 * 1024 * 1024)).toBe('5.00 GB');
  });
});

describe('formatPlatformTableDateTimeValue', () => {
  it('formats compact table timestamps with canonical empty markers', () => {
    const value = '2026-05-20T10:15:00Z';
    const dateTimeFormat = {
      ...PLATFORM_TABLE_COMPACT_DATE_TIME_FORMAT,
      timeZone: 'UTC',
      hour12: false,
    } satisfies Intl.DateTimeFormatOptions;

    expect(formatPlatformTableDateTimeValue(value, { dateTimeFormat })).toBe(
      new Date(value).toLocaleString(undefined, dateTimeFormat),
    );
    expect(formatPlatformTableDateTimeValue('  ', { emptyText: '-' })).toBe('-');
    expect(formatPlatformTableDateTimeValue('not-a-date', { emptyText: '-' })).toBe('-');
    expect(formatPlatformTableDateTimeValue(undefined)).toBe('—');
  });

  it('supports caller-owned minimum year cutoffs for placeholder platform dates', () => {
    expect(
      formatPlatformTableDateTimeValue('1970-01-01T00:00:00Z', {
        emptyText: '-',
        minYear: 2000,
      }),
    ).toBe('-');
  });
});

describe('PlatformTableDateTimeValue', () => {
  it('renders compact timestamps with shared tabular styling', () => {
    const value = '2026-05-20T10:15:00Z';
    const dateTimeFormat = {
      timeZone: 'UTC',
      hour12: false,
    } satisfies Intl.DateTimeFormatOptions;
    const { container } = render(() =>
      PlatformTableDateTimeValue({
        value,
        dateTimeFormat,
      }),
    );
    const marker = container.querySelector('span');

    expect(marker?.classList.contains('tabular-nums')).toBe(true);
    expect(marker?.textContent).toBe(
      new Date(value).toLocaleString(undefined, {
        ...PLATFORM_TABLE_COMPACT_DATE_TIME_FORMAT,
        ...dateTimeFormat,
      }),
    );
  });
});

describe('formatPlatformTableRelativeTimeValue', () => {
  it('formats relative timestamps with canonical table empty markers', () => {
    const fiveMinutesAgo = Date.now() - 5 * 60 * 1000;

    expect(formatPlatformTableRelativeTimeValue(fiveMinutesAgo)).toBe('5m ago');
    expect(formatPlatformTableRelativeTimeValue(fiveMinutesAgo, { compact: false })).toBe(
      '5 mins ago',
    );
    expect(formatPlatformTableRelativeTimeValue(undefined)).toBe('—');
    expect(formatPlatformTableRelativeTimeValue('', { emptyText: '-' })).toBe('-');
    expect(formatPlatformTableRelativeTimeValue('not-a-date', { emptyText: '-' })).toBe('-');
  });
});

describe('PlatformTableRelativeTimeValue', () => {
  it('renders relative timestamps with shared tabular styling', () => {
    const fiveMinutesAgo = Date.now() - 5 * 60 * 1000;
    const { container } = render(() => PlatformTableRelativeTimeValue({ value: fiveMinutesAgo }));
    const marker = container.querySelector('span');

    expect(marker?.classList.contains('tabular-nums')).toBe(true);
    expect(marker?.textContent).toBe('5m ago');
  });
});

describe('formatPlatformTableIntegerValue', () => {
  it('formats rounded integer labels with canonical empty markers', () => {
    const expected = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(1235);

    expect(formatPlatformTableIntegerValue(1234.6)).toBe(expected);
    expect(formatPlatformTableIntegerValue(0)).toBe('0');
    expect(formatPlatformTableIntegerValue(Number.NaN, '0')).toBe('0');
    expect(formatPlatformTableIntegerValue(undefined)).toBe('—');
  });
});

describe('PlatformTableNumberValue', () => {
  it('renders finite numbers with shared tabular styling and empty markers', () => {
    const { container } = render(() => PlatformTableNumberValue({ value: 42 }));
    const marker = container.querySelector('span');

    expect(marker?.textContent).toBe('42');
    expect(marker?.classList.contains('tabular-nums')).toBe(true);

    cleanup();
    render(() => PlatformTableNumberValue({ value: undefined }));
    expect(screen.getByText('—')).toBeInTheDocument();
  });

  it('rejects non-finite values and supports caller-owned formatting', () => {
    render(() => PlatformTableNumberValue({ value: Number.NaN, emptyText: '-' }));
    expect(screen.getByText('-')).toBeInTheDocument();

    cleanup();
    render(() =>
      PlatformTableNumberValue({
        value: 1234,
        format: (value) => value.toLocaleString(),
      }),
    );
    expect(screen.getByText('1,234')).toBeInTheDocument();
  });
});

describe('PlatformTableCountRatioValue', () => {
  it('formats count-ratio labels for string-only table summaries', () => {
    expect(formatPlatformTableCountRatioValue(3, 5, { suffix: 'ready' })).toBe('3/5 ready');
    expect(formatPlatformTableCountRatioValue(undefined, 4, { suffix: 'ready' })).toBe('0/4 ready');
    expect(formatPlatformTableCountRatioValue(undefined, undefined, { emptyText: '' })).toBe('');
  });

  it('renders count ratios through shared tabular number styling', () => {
    const { container } = render(() =>
      PlatformTableCountRatioValue({
        current: 2,
        total: 3,
        currentTone: 'warning',
        suffix: 'ready',
      }),
    );

    expect(container.textContent).toBe('2/3 ready');
    expect(container.querySelectorAll('.tabular-nums')).toHaveLength(2);
    expect(screen.getByText('2').parentElement?.classList.contains('text-amber-700')).toBe(true);
    expect(screen.getByText('/').classList.contains('text-muted')).toBe(true);
    expect(screen.getByText('3').parentElement?.classList.contains('text-muted')).toBe(true);
  });

  it('uses the shared empty marker when neither side is finite', () => {
    render(() =>
      PlatformTableCountRatioValue({
        current: undefined,
        total: Number.NaN,
        emptyText: '-',
      }),
    );

    expect(screen.getByText('-')).toBeInTheDocument();
  });
});

describe('PlatformTablePercentValue', () => {
  it('formats shared percent labels with canonical empty, ratio, and clamp behavior', () => {
    expect(formatPlatformTablePercentValue(42.345)).toBe('42.3%');
    expect(formatPlatformTablePercentValue(undefined)).toBe('—');
    expect(formatPlatformTablePercentValue(Number.NaN, { emptyText: '-' })).toBe('-');
    expect(formatPlatformTablePercentValue(0.812, { normalizeRatio: true })).toBe('81.2%');
    expect(formatPlatformTablePercentValue(120, { clamp: true })).toBe('100.0%');
    expect(formatPlatformTablePercentValue(-3, { clamp: true })).toBe('0.0%');
  });

  it('renders finite percentages with one decimal place and shared tabular styling', () => {
    const { container } = render(() => PlatformTablePercentValue({ value: 42.345 }));
    const marker = container.querySelector('span');

    expect(marker?.textContent).toBe('42.3%');
    expect(marker?.classList.contains('tabular-nums')).toBe(true);

    cleanup();
    render(() => PlatformTablePercentValue({ value: Number.NaN, emptyText: '-' }));
    expect(screen.getByText('-')).toBeInTheDocument();
  });

  it('passes ratio normalization and clamping through the shared component', () => {
    const { container } = render(() =>
      PlatformTablePercentValue({ value: 0.25, normalizeRatio: true, clamp: true }),
    );

    expect(container.querySelector('span')?.textContent).toBe('25.0%');
  });
});

describe('PlatformTableTemperatureValue', () => {
  it('renders positive Celsius values with one decimal place and rejects absent readings', () => {
    const { container } = render(() => PlatformTableTemperatureValue({ value: 38.86 }));
    const marker = container.querySelector('span');

    expect(marker?.textContent).toBe('38.9°C');
    expect(marker?.classList.contains('tabular-nums')).toBe(true);

    cleanup();
    render(() => PlatformTableTemperatureValue({ value: 0, emptyText: 'No sensor' }));
    expect(screen.getByText('No sensor')).toBeInTheDocument();

    cleanup();
    render(() => PlatformTableTemperatureValue({ value: Number.POSITIVE_INFINITY }));
    expect(screen.getByText('—')).toBeInTheDocument();
  });
});

describe('getPlatformTableFiniteMetric', () => {
  it('normalizes platform table metrics to finite numeric values only', () => {
    expect(getPlatformTableFiniteMetric(42)).toBe(42);
    expect(getPlatformTableFiniteMetric(0)).toBe(0);
    expect(getPlatformTableFiniteMetric(undefined)).toBeUndefined();
    expect(getPlatformTableFiniteMetric(Number.NaN)).toBeUndefined();
    expect(getPlatformTableFiniteMetric(Number.POSITIVE_INFINITY)).toBeUndefined();
  });
});

describe('PlatformTableMetricFallback', () => {
  it('renders the default empty metric marker as presentational text', () => {
    const { container } = render(() => PlatformTableMetricFallback());
    const marker = container.querySelector('span');

    expect(marker?.textContent).toBe('—');
    expect(marker?.getAttribute('aria-hidden')).toBe('true');
  });

  it('preserves caller-owned fallback labels and titles', () => {
    render(() =>
      PlatformTableMetricFallback({
        label: 'old agent',
        title: 'Update this agent for full machine telemetry.',
      }),
    );

    const marker = screen.getByLabelText('Update this agent for full machine telemetry.');
    expect(marker.textContent).toBe('old agent');
    expect(marker.getAttribute('title')).toBe('Update this agent for full machine telemetry.');
    expect(marker.getAttribute('aria-hidden')).toBeNull();
  });
});
