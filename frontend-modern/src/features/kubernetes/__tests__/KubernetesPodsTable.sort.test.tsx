import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesPodsTable } from '../KubernetesPodsTable';

const makePod = ({ id, ...overrides }: Partial<Resource> & Pick<Resource, 'id'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'cluster-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'online',
  type: 'pod',
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

// Mixed statuses so both the attention-first default order and the user
// sort are observable: zulu is the only danger row and floats by default.
const FIXTURE: Resource[] = [
  makePod({
    id: 'alpha',
    kubernetes: {
      podPhase: 'Running',
      podContainers: [{ ready: true, state: 'running', restartCount: 9 }],
    },
  }),
  makePod({
    id: 'zulu',
    kubernetes: {
      podPhase: 'Running',
      podContainers: [
        { ready: false, state: 'waiting', reason: 'CrashLoopBackOff', restartCount: 3 },
      ],
    },
  }),
  makePod({
    id: 'mike',
    kubernetes: {
      podPhase: 'Running',
    },
  }),
];

const renderTable = () =>
  render(() => (
    <KubernetesPodsTable
      resources={FIXTURE}
      emptyIcon={<span />}
      emptyTitle="No pods"
      emptyDescription="No pods"
      showToolbar={false}
    />
  ));

const visibleRowOrder = (): string[] =>
  Array.from(document.querySelectorAll('tr[data-kubernetes-pod-row]')).map(
    (row) => row.getAttribute('data-kubernetes-pod-row') ?? '',
  );

const headerFor = (label: string): HTMLElement => {
  const header = screen
    .getAllByRole('columnheader')
    .find((th) => th.textContent?.trim().startsWith(label));
  if (!header) throw new Error(`No column header labelled ${label}`);
  return header;
};

afterEach(() => {
  window.localStorage.clear();
  cleanup();
});

describe('KubernetesPodsTable user sorting', () => {
  it('cycles a name sort and falls back to the attention-first order', () => {
    renderTable();

    // Built-in order: attention first (zulu is CrashLoopBackOff), then names.
    expect(visibleRowOrder()).toEqual(['zulu', 'alpha', 'mike']);

    fireEvent.click(headerFor('Pod'));
    expect(headerFor('Pod')).toHaveAttribute('aria-sort', 'ascending');
    expect(visibleRowOrder()).toEqual(['alpha', 'mike', 'zulu']);

    fireEvent.click(headerFor('Pod'));
    expect(headerFor('Pod')).toHaveAttribute('aria-sort', 'descending');
    expect(visibleRowOrder()).toEqual(['zulu', 'mike', 'alpha']);

    // Third click clears back to the built-in attention-first order.
    fireEvent.click(headerFor('Pod'));
    expect(headerFor('Pod')).not.toHaveAttribute('aria-sort');
    expect(visibleRowOrder()).toEqual(['zulu', 'alpha', 'mike']);
  });

  it('sorts restarts descending first with missing values last', () => {
    renderTable();

    fireEvent.click(headerFor('Restarts'));
    expect(headerFor('Restarts')).toHaveAttribute('aria-sort', 'descending');
    // mike reports no containers and no restart count, so it sinks.
    expect(visibleRowOrder()).toEqual(['alpha', 'zulu', 'mike']);
  });

  it('persists the chosen sort across a remount', () => {
    renderTable();
    fireEvent.click(headerFor('Pod'));
    expect(visibleRowOrder()).toEqual(['alpha', 'mike', 'zulu']);
    expect(window.localStorage.getItem('kubernetesPodsSortKey')).toBe('pod');
    expect(window.localStorage.getItem('kubernetesPodsSortDirection')).toBe('asc');

    cleanup();

    renderTable();
    expect(headerFor('Pod')).toHaveAttribute('aria-sort', 'ascending');
    expect(visibleRowOrder()).toEqual(['alpha', 'mike', 'zulu']);
  });
});
