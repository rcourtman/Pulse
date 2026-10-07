import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createRoot } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { formatPlatformTableDateTimeValue } from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
import { InlineResourceSummaryTables } from '../ResourceDetailSummary';
import { useResourceDetailDrawerDerivedState } from '../useResourceDetailDrawerDerivedState';
import type { UseResourceDetailDrawerStateResult } from '../useResourceDetailDrawerState';

// The summary only reads identity and coverage accessors from the drawer
// state; the ages under test come from the resource itself.
const drawerState = {
  primaryIdentityRows: () => [],
  identityIpValues: () => [],
  identityAliasValues: () => [],
  aliasPreviewValues: () => [],
  hasAliasOverflow: () => false,
  identityCardHasRichData: () => true,
  sourceSummary: () => null,
  lastSeen: () => '',
  lastSeenAbsolute: () => '',
} as unknown as UseResourceDetailDrawerStateResult;

const makeResource = ({
  id,
  type,
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'host-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  status: 'online',
  type,
  lastSeen: Date.parse('2026-05-24T13:31:00Z'),
  ...overrides,
});

const absoluteTime = (timestamp: string): string =>
  formatPlatformTableDateTimeValue(timestamp, { dateTimeFormat: { year: 'numeric' } });

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

// Time passes with no new data; the next clock tick re-reads the ages.
const hoursLater = (hours: number) => {
  vi.setSystemTime(Date.now() + hours * 60 * 60 * 1000);
  vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);
};

describe('resource drawer summary relative ages', () => {
  it('keeps an open Kubernetes controller summary age moving without new data', () => {
    vi.useFakeTimers({ now: new Date('2026-05-24T13:31:00Z') });

    render(() => (
      <InlineResourceSummaryTables
        resource={makeResource({
          id: 'nightly-import',
          type: 'k8s-job',
          kubernetes: {
            clusterName: 'prod',
            namespace: 'batch',
            resourceKind: 'Job',
            completionTime: '2026-05-24T13:00:00Z',
          },
        })}
        drawer={drawerState}
        showPlatformId={false}
      />
    ));

    const section = () => screen.getByTestId('resource-kubernetes-controller-section');
    expect(
      within(section()).getByText(`${absoluteTime('2026-05-24T13:00:00Z')} (31m ago)`),
    ).toBeInTheDocument();

    hoursLater(2);

    // The absolute time is unchanged; only the age after it moves.
    expect(
      within(section()).getByText(`${absoluteTime('2026-05-24T13:00:00Z')} (2h ago)`),
    ).toBeInTheDocument();
  });

  it('keeps an open container summary Created and Started ages moving without new data', () => {
    vi.useFakeTimers({ now: new Date('2026-05-24T13:31:00Z') });

    render(() => (
      <InlineResourceSummaryTables
        resource={makeResource({
          id: 'web',
          type: 'app-container',
          platformType: 'docker',
          docker: {
            containerId: 'abc123',
            containerState: 'running',
            createdAt: '2026-05-24T11:31:00Z',
            startedAt: '2026-05-24T13:21:00Z',
            finishedAt: '2026-05-24T12:31:00Z',
          },
        })}
        drawer={drawerState}
        showPlatformId={false}
      />
    ));

    const row = (label: string) =>
      within(screen.getByTestId('resource-docker-container-section'))
        .getByText(label)
        .closest('tr');
    expect(row('Created')).toHaveTextContent('2 hours ago');
    expect(row('Started')).toHaveTextContent('10 mins ago');
    expect(row('Finished')).toHaveTextContent('1 hour ago');

    hoursLater(3);

    expect(row('Created')).toHaveTextContent('5 hours ago');
    expect(row('Started')).toHaveTextContent('3 hours ago');
    expect(row('Finished')).toHaveTextContent('4 hours ago');
  });

  it('keeps an open drawer Last seen and platform freshness ages moving without new data', () => {
    vi.useFakeTimers({ now: new Date('2026-05-24T13:31:00Z') });

    const { state, dispose } = createRoot((dispose) => ({
      state: useResourceDetailDrawerDerivedState({
        resource: makeResource({
          id: 'node-1',
          type: 'agent',
          lastSeen: Date.parse('2026-05-24T13:30:00Z'),
          platformData: {
            docker: { updatesLastCheckedAt: '2026-05-24T13:01:00Z' },
            pmg: { lastUpdated: '2026-05-24T13:21:00Z' },
          },
        } as Partial<Resource> & Pick<Resource, 'id' | 'type'>),
        debugEnabled: () => false,
        discoveryFeatureEnabled: () => false,
        resourceIntelligence: () => null,
      }),
      dispose,
    }));

    expect(state.lastSeen()).toBe('1 min ago');
    expect(state.dockerUpdatesCheckedRelative()).toBe('30 mins ago');
    expect(state.pmgUpdatedRelative()).toBe('10 mins ago');

    // The resource stays silent: none of its timestamps change again.
    hoursLater(2);

    expect(state.lastSeen()).toBe('2 hours ago');
    expect(state.dockerUpdatesCheckedRelative()).toBe('2 hours ago');
    expect(state.pmgUpdatedRelative()).toBe('2 hours ago');
    dispose();
  });
});
