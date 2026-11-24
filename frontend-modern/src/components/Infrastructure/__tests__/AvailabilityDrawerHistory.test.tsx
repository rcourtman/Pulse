import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, fireEvent, screen, waitFor, cleanup } from '@solidjs/testing-library';
import {
  AvailabilityHistoryAPI,
  type AvailabilityHistoryResponse,
} from '@/api/availabilityHistory';
import AvailabilityDrawerHistory from '../AvailabilityDrawerHistory';

const makeHistoryResponse = (
  overrides: Partial<AvailabilityHistoryResponse['targets'][0]> = {},
): AvailabilityHistoryResponse => ({
  start: '2026-11-23T12:00:00Z',
  end: '2026-11-24T12:00:00Z',
  targets: [
    {
      targetId: 'target-1',
      summary: {
        reachableSeconds: 84600,
        unreachableSeconds: 1800,
        indeterminateSeconds: 0,
        unknownSeconds: 0,
        coveragePercent: 100,
        availabilityPercent: 97.92,
        reachableLatencyMillis: {
          average: 24,
          min: 8,
          max: 51,
        },
      },
      buckets: [
        {
          start: '2026-11-23T12:00:00Z',
          end: '2026-11-23T20:00:00Z',
          reachableSeconds: 28800,
          unreachableSeconds: 0,
          indeterminateSeconds: 0,
          unknownSeconds: 0,
          latencyMillis: { average: 18, min: 8, max: 26 },
        },
        {
          start: '2026-11-23T20:00:00Z',
          end: '2026-11-24T04:00:00Z',
          reachableSeconds: 27000,
          unreachableSeconds: 1800,
          indeterminateSeconds: 0,
          unknownSeconds: 0,
          latencyMillis: { average: 32, min: 15, max: 51 },
        },
        {
          start: '2026-11-24T04:00:00Z',
          end: '2026-11-24T12:00:00Z',
          reachableSeconds: 28800,
          unreachableSeconds: 0,
          indeterminateSeconds: 0,
          unknownSeconds: 0,
          latencyMillis: { average: 22, min: 12, max: 30 },
        },
      ],
      revisionBoundaries: [{ revision: 2, at: '2026-11-23T22:00:00Z' }],
      ...overrides,
    },
  ],
});

describe('AvailabilityDrawerHistory', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it('renders status timeline, response-time chart, summary metrics, and range switching', async () => {
    const batchSpy = vi
      .spyOn(AvailabilityHistoryAPI, 'batch')
      .mockResolvedValue(makeHistoryResponse());

    render(() => <AvailabilityDrawerHistory targetId="target-1" enabled />);

    expect(await screen.findByTestId('availability-drawer-timeline')).toBeInTheDocument();
    expect(screen.getByTestId('availability-drawer-latency-chart')).toBeInTheDocument();
    expect(screen.getByText('97.92% uptime')).toBeInTheDocument();
    expect(screen.getByText('30m downtime')).toBeInTheDocument();
    expect(screen.getByText('Avg 24 ms (8-51 ms)')).toBeInTheDocument();
    expect(screen.getByText('Config changed')).toBeInTheDocument();

    // Keyboard arrow inspection updates both readouts cleanly.
    const timelineGroup = screen.getByRole('group', { name: /Availability status timeline/i });
    fireEvent.keyDown(timelineGroup, { key: 'ArrowRight' });
    expect(screen.getByTestId('availability-timeline-readout')).toHaveTextContent('Reachable');
    expect(screen.getByTestId('availability-timeline-readout')).toHaveTextContent('18 ms');

    // Switching range re-queries AvailabilityHistoryAPI with the chosen range.
    fireEvent.click(screen.getByRole('button', { name: '7d' }));
    await waitFor(() => {
      expect(batchSpy).toHaveBeenCalledWith(['target-1'], '7d');
    });
  });

  it('omits the response-time chart when buckets have no latency measurements', async () => {
    vi.spyOn(AvailabilityHistoryAPI, 'batch').mockResolvedValue(
      makeHistoryResponse({
        summary: {
          reachableSeconds: 86400,
          unreachableSeconds: 0,
          indeterminateSeconds: 0,
          unknownSeconds: 0,
          coveragePercent: 100,
          availabilityPercent: 100,
        },
        buckets: [
          {
            start: '2026-11-23T12:00:00Z',
            end: '2026-11-24T12:00:00Z',
            reachableSeconds: 86400,
            unreachableSeconds: 0,
            indeterminateSeconds: 0,
            unknownSeconds: 0,
          },
        ],
        revisionBoundaries: [],
      }),
    );

    render(() => <AvailabilityDrawerHistory targetId="target-1" protocol="push" enabled />);

    expect(await screen.findByTestId('availability-drawer-timeline')).toBeInTheDocument();
    expect(screen.queryByTestId('availability-drawer-latency-chart')).toBeNull();
    expect(screen.getByText('No outages')).toBeInTheDocument();
  });

  it('shows a calm empty state when no history has been observed yet', async () => {
    vi.spyOn(AvailabilityHistoryAPI, 'batch').mockResolvedValue(
      makeHistoryResponse({
        summary: {
          reachableSeconds: 0,
          unreachableSeconds: 0,
          indeterminateSeconds: 0,
          unknownSeconds: 86400,
          coveragePercent: 0,
        },
        buckets: [
          {
            start: '2026-11-23T12:00:00Z',
            end: '2026-11-24T12:00:00Z',
            reachableSeconds: 0,
            unreachableSeconds: 0,
            indeterminateSeconds: 0,
            unknownSeconds: 86400,
          },
        ],
      }),
    );

    render(() => <AvailabilityDrawerHistory targetId="target-1" enabled />);

    expect(await screen.findByTestId('availability-drawer-history-empty')).toBeInTheDocument();
  });

  it('shows an error card with retry when history fetch fails', async () => {
    const batchSpy = vi
      .spyOn(AvailabilityHistoryAPI, 'batch')
      .mockRejectedValueOnce(new Error('History API offline'))
      .mockResolvedValueOnce(makeHistoryResponse());

    render(() => <AvailabilityDrawerHistory targetId="target-1" enabled />);

    expect(await screen.findByTestId('availability-drawer-history-error')).toHaveTextContent(
      'History API offline',
    );
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByTestId('availability-drawer-timeline')).toBeInTheDocument();
    expect(batchSpy).toHaveBeenCalledTimes(2);
  });
});
