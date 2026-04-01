import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import type { InfrastructureSummarySeries } from '@/components/Infrastructure/infrastructureSummaryModel';
import { resolveSummaryActiveSeriesId } from '@/components/shared/summaryCardInteraction';
import {
  buildInfrastructureDisplaySeries,
  buildInfrastructureEmptyHistoryLabel,
  buildInfrastructureEmptyMessage,
  buildInfrastructureMetricSeries,
  getFocusedInfrastructureResourceName,
  getSingleDisplayedOnlineInfrastructureResource,
  hasInfrastructureSeriesData,
  isInfrastructureAwaitingFirstSample,
  shouldShowInfrastructureNetworkCard,
} from '@/components/Infrastructure/infrastructureSummaryModel';

const makeSeries = (id: string, overrides: Partial<InfrastructureSummarySeries> = {}): InfrastructureSummarySeries => ({
  key: id,
  id,
  cpu: [],
  memory: [],
  disk: [],
  netin: [],
  netout: [],
  network: [],
  diskio: [],
  color: '#00aaff',
  name: id,
  ...overrides,
});

const makeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'host-1',
  displayName: 'Host 1',
  platformId: 'host-1',
  platformType: 'agent',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: 1_000,
  platformData: { sources: ['agent'] },
  ...overrides,
});

describe('infrastructureSummaryModel', () => {
  it('focuses the display series and focused label through the canonical model owner', () => {
    const allSeries = [makeSeries('host-1', { name: 'Host 1' }), makeSeries('host-2', { name: 'Host 2' })];

    expect(buildInfrastructureDisplaySeries(allSeries, 'host-2')).toEqual([
      makeSeries('host-2', { name: 'Host 2' }),
    ]);
    expect(buildInfrastructureDisplaySeries(allSeries, 'missing')).toEqual(allSeries);
    expect(getFocusedInfrastructureResourceName(allSeries, 'host-1')).toBe('Host 1');
    expect(getFocusedInfrastructureResourceName(allSeries, 'missing')).toBeNull();
  });

  it('allows page-scoped display series while still resolving the focused label canonically', () => {
    const allSeries = [makeSeries('host-1', { name: 'Host 1' }), makeSeries('host-2', { name: 'Host 2' })];

    expect(buildInfrastructureDisplaySeries(allSeries, null)).toEqual(allSeries);
    expect(getFocusedInfrastructureResourceName(allSeries, 'host-2')).toBe('Host 2');
  });

  it('keeps first-sample waiting logic on canonical summary inputs', () => {
    const onlineResource = makeResource({ lastSeen: 5_000 });

    expect(
      getSingleDisplayedOnlineInfrastructureResource([onlineResource], [makeSeries('resource-1')]),
    ).toEqual(onlineResource);
    expect(
      getSingleDisplayedOnlineInfrastructureResource(
        [makeResource({ status: 'offline' })],
        [makeSeries('resource-1')],
      ),
    ).toBeNull();

    expect(
      isInfrastructureAwaitingFirstSample({
        resource: onlineResource,
        isCurrentRangeLoaded: true,
        fetchFailed: false,
        oldestDataTimestamp: null,
      }),
    ).toBe(true);
    expect(
      isInfrastructureAwaitingFirstSample({
        resource: onlineResource,
        isCurrentRangeLoaded: true,
        fetchFailed: false,
        oldestDataTimestamp: 4_000,
      }),
    ).toBe(true);
    expect(
      isInfrastructureAwaitingFirstSample({
        resource: onlineResource,
        isCurrentRangeLoaded: true,
        fetchFailed: true,
        oldestDataTimestamp: 4_000,
      }),
    ).toBe(false);
  });

  it('builds metric series, data checks, and empty-state copy canonically', () => {
    const series = [
      makeSeries('host-1', {
        cpu: [{ timestamp: 1, value: 20 }],
        network: [{ timestamp: 1, value: 300 }],
      }),
      makeSeries('host-2'),
    ];

    expect(buildInfrastructureMetricSeries(series, 'cpu')).toEqual([
      { id: 'host-1', data: [{ timestamp: 1, value: 20 }], color: '#00aaff', name: 'host-1' },
      { id: 'host-2', data: [], color: '#00aaff', name: 'host-2' },
    ]);
    expect(hasInfrastructureSeriesData(series, 'network')).toBe(true);
    expect(hasInfrastructureSeriesData(series, 'diskio')).toBe(false);
    expect(buildInfrastructureEmptyHistoryLabel(true)).toBe('Waiting for first sample');
    expect(buildInfrastructureEmptyHistoryLabel(false)).toBe('No history yet');
    expect(buildInfrastructureEmptyMessage(true, 'No history yet')).toBe('Trend data unavailable');
    expect(buildInfrastructureEmptyMessage(false, 'No history yet')).toBe('No history yet');
  });

  it('keeps network and diskio series on canonical ids when focused summary selection narrows the view', () => {
    const displayedSeries = buildInfrastructureDisplaySeries(
      [
        makeSeries('host-1', {
          name: 'Host 1',
          network: [{ timestamp: 1, value: 120 }],
          diskio: [{ timestamp: 1, value: 45 }],
        }),
        makeSeries('host-2', {
          name: 'Host 2',
          network: [{ timestamp: 2, value: 240 }],
          diskio: [{ timestamp: 2, value: 90 }],
        }),
      ],
      'host-2',
    );

    expect(buildInfrastructureMetricSeries(displayedSeries, 'network')).toEqual([
      { id: 'host-2', data: [{ timestamp: 2, value: 240 }], color: '#00aaff', name: 'Host 2' },
    ]);
    expect(buildInfrastructureMetricSeries(displayedSeries, 'diskio')).toEqual([
      { id: 'host-2', data: [{ timestamp: 2, value: 90 }], color: '#00aaff', name: 'Host 2' },
    ]);
  });

  it('uses one canonical active series id across hover and focused summary selection', () => {
    expect(
      resolveSummaryActiveSeriesId({
        hoveredSeriesId: 'agent-1',
        focusedSeriesId: 'agent-2',
      }),
    ).toBe('agent-1');
    expect(
      resolveSummaryActiveSeriesId({
        hoveredSeriesId: null,
        focusedSeriesId: 'agent-2',
      }),
    ).toBe('agent-2');
  });

  it('shows the network card when either data or network capability exists', () => {
    expect(
      shouldShowInfrastructureNetworkCard(false, [
        makeResource({
          type: 'vm',
          platformType: 'proxmox-vm',
          network: { rxBytes: 10, txBytes: 0 },
        }),
      ]),
    ).toBe(true);
    expect(shouldShowInfrastructureNetworkCard(true, [makeResource()])).toBe(true);
    expect(
      shouldShowInfrastructureNetworkCard(false, [
        makeResource({
          type: 'vm',
          platformType: 'proxmox-vm',
          network: { rxBytes: 0, txBytes: 0 },
          platformData: { sources: ['proxmox'] },
        }),
      ]),
    ).toBe(false);
  });
});
