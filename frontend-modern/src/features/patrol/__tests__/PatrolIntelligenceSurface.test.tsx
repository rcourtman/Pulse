import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal, onMount, type JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { PatrolIntelligenceSurface } from '../PatrolIntelligenceSurface';

const patrolState = vi.hoisted(() => ({
  setupOnly: false,
  enabled: true,
  approvalCount: (): number => 0,
  deferInitialLoads: false,
  finishDigest: undefined as (() => void) | undefined,
  finishRecentWork: undefined as (() => void) | undefined,
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    get patrolPendingApprovals() {
      return Array.from({ length: patrolState.approvalCount() });
    },
    patrolFindings: [],
  },
}));

vi.mock('../usePatrolIntelligenceState', () => ({
  usePatrolIntelligenceState: () => ({
    shouldShowPatrolSetupOnly: () => patrolState.setupOnly,
    patrolEnabledLocal: () => patrolState.enabled,
    setActiveTab: vi.fn(),
    setSelectedRun: vi.fn(),
    setFindingsFilterOverride: vi.fn(),
  }),
}));

vi.mock('../PatrolIntelligenceHeader', () => ({
  PatrolIntelligenceHeader: () => <div>Patrol header</div>,
}));

vi.mock('../PatrolIntelligenceBanners', () => ({
  PatrolIntelligenceBanners: () => <div>Patrol banners</div>,
}));

vi.mock('../PatrolAttentionWorkbench', () => ({
  PatrolAttentionWorkbench: (props: {
    onOpenFindings?: (item: Record<string, unknown>) => void;
  }) => (
    <button
      type="button"
      onClick={() =>
        props.onOpenFindings?.({
          subjectResourceId: 'vm-101',
          subjectResourceName: 'Database VM',
        })
      }
    >
      Open scoped finding options
    </button>
  ),
}));

vi.mock('../PatrolIntelligenceWorkspace', () => ({
  PatrolIntelligenceWorkspace: (props: {
    findingResourceId?: string;
    findingResourceName?: string;
  }) => (
    <div
      data-testid="patrol-workspace"
      data-resource-id={props.findingResourceId ?? ''}
      data-resource-name={props.findingResourceName ?? ''}
    />
  ),
}));

vi.mock('../PatrolObjectivesPanel', () => ({
  PatrolObjectivesPanel: () => <div>Objectives</div>,
}));

vi.mock('../PatrolRecentWorkPanel', () => ({
  PatrolRecentWorkPanel: (props: { onInitialLoad?: () => void }) => {
    onMount(() => {
      patrolState.finishRecentWork = props.onInitialLoad;
      if (!patrolState.deferInitialLoads) props.onInitialLoad?.();
    });
    return <div>Recent work</div>;
  },
}));

vi.mock('../PatrolWeeklyDigestCard', () => ({
  PatrolWeeklyDigestCard: (props: { onInitialLoad?: () => void }) => {
    onMount(() => {
      patrolState.finishDigest = props.onInitialLoad;
      if (!patrolState.deferInitialLoads) props.onInitialLoad?.();
    });
    return <div>This week</div>;
  },
}));

vi.mock('@/stores/actionInbox', () => ({
  actionInboxStore: { pendingActionCount: 0 },
}));

vi.mock('@/components/shared/Button', () => ({
  ButtonLink: (props: { href: string; children?: JSX.Element }) => (
    <a href={props.href}>{props.children}</a>
  ),
}));

vi.mock('@/components/shared/MetadataBadge', () => ({
  MetadataBadge: (props: { children?: JSX.Element }) => <span>{props.children}</span>,
}));

const renderSurface = () =>
  render(() => (
    <Router>
      <Route path="*" component={PatrolIntelligenceSurface} />
    </Router>
  ));

describe('PatrolIntelligenceSurface finding handoff', () => {
  beforeEach(() => {
    window.history.replaceState(null, '', '/patrol');
  });

  afterEach(() => {
    cleanup();
    patrolState.setupOnly = false;
    patrolState.enabled = true;
    patrolState.approvalCount = () => 0;
    patrolState.deferInitialLoads = false;
    patrolState.finishDigest = undefined;
    patrolState.finishRecentWork = undefined;
    vi.restoreAllMocks();
  });

  it('keeps outcome guidance in the opener instead of repeating it above a concrete approval handoff', () => {
    const [count, setCount] = createSignal(1);
    patrolState.approvalCount = count;
    window.history.replaceState(null, '', '/patrol/activity');
    renderSurface();
    fireEvent.click(screen.getByRole('button', { name: /^Finding options and history/ }));
    expect(
      screen.getByText(/Resolve or dismiss findings, remember expected behavior/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Choose Finding options on an active finding/),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId('patrol-workspace')).toBeInTheDocument();
    setCount(0);
    expect(screen.getByText(/Choose Finding options on an active finding/)).toBeInTheDocument();
  });

  it('keeps the independent attention inbox beside the setup task when Patrol is off', () => {
    patrolState.setupOnly = true;
    patrolState.enabled = false;

    renderSurface();

    expect(screen.getByRole('tab', { name: 'Inbox' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('button', { name: 'Open scoped finding options' })).toBeInTheDocument();
    expect(screen.getByTestId('patrol-workspace')).toBeInTheDocument();
    expect(screen.getByTestId('patrol-workspace').parentElement).toHaveClass('pointer-events-none');
  });

  it('keeps the selected decision resource in context and lets the operator broaden the list', async () => {
    renderSurface();

    fireEvent.click(screen.getByRole('button', { name: 'Open scoped finding options' }));

    await vi.waitFor(() =>
      expect(screen.getByRole('tab', { name: 'Activity' })).toHaveAttribute(
        'aria-selected',
        'true',
      ),
    );
    expect(window.location.pathname).toBe('/patrol/activity');
    expect(screen.getByText(/Showing Patrol findings for Database VM/i)).toBeInTheDocument();
    expect(screen.getByTestId('patrol-workspace')).toHaveAttribute('data-resource-id', 'vm-101');
    expect(screen.getByTestId('patrol-workspace')).toHaveAttribute(
      'data-resource-name',
      'Database VM',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Show all finding options' }));

    expect(screen.getByTestId('patrol-workspace')).toHaveAttribute('data-resource-id', '');
    expect(screen.getByTestId('patrol-workspace')).toHaveAttribute('data-resource-name', '');
    expect(screen.queryByText(/Showing Patrol findings for Database VM/i)).not.toBeInTheDocument();
  });

  it('opens the workspace view named by the route and routes each tab change', async () => {
    window.history.replaceState(null, '', '/patrol/activity');
    renderSurface();

    expect(screen.getByRole('tab', { name: 'Activity' })).toHaveAttribute('aria-selected', 'true');

    fireEvent.click(screen.getByRole('tab', { name: 'Protection' }));
    await vi.waitFor(() => expect(window.location.pathname).toBe('/patrol/protection'));
    expect(screen.getByRole('tab', { name: 'Protection' })).toHaveAttribute(
      'aria-selected',
      'true',
    );

    fireEvent.click(screen.getByRole('tab', { name: 'Inbox' }));
    await vi.waitFor(() => expect(window.location.pathname).toBe('/patrol'));
  });

  it('keeps manual-rule reversal reachable in Activity even with the findings panel closed', () => {
    window.history.replaceState(null, '', '/patrol/activity');
    renderSurface();
    const rules = document.getElementById('patrol-suppression-rules');
    expect(rules?.querySelector('summary')).toHaveTextContent('Suppression rules');
    expect(rules).not.toHaveAttribute('open');
    expect(document.getElementById('patrol-operational-records')).not.toHaveAttribute('open');
  });

  it('lands the finding handoff on the findings panel without a scroll to the top', async () => {
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
    const scrollIntoView = vi.fn();
    HTMLElement.prototype.scrollIntoView = scrollIntoView;
    renderSurface();

    fireEvent.click(screen.getByRole('button', { name: 'Open scoped finding options' }));

    await vi.waitFor(() => expect(scrollIntoView).toHaveBeenCalled());
    expect(scrollTo).not.toHaveBeenCalled();
    scrollTo.mockRestore();
  });

  const deferHandoff = async () => {
    patrolState.deferInitialLoads = true;
    const scrollIntoView = vi.fn();
    HTMLElement.prototype.scrollIntoView = scrollIntoView;
    const frames = new Map<number, FrameRequestCallback>();
    let nextFrame = 0;
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
      frames.set(++nextFrame, callback);
      return nextFrame;
    });
    vi.spyOn(window, 'cancelAnimationFrame').mockImplementation((id) => {
      frames.delete(id);
    });
    const flushFrames = () => {
      const callbacks = [...frames.values()];
      frames.clear();
      callbacks.forEach((callback) => callback(0));
    };
    renderSurface();
    fireEvent.click(screen.getByRole('button', { name: 'Open scoped finding options' }));
    await vi.waitFor(() => expect(scrollIntoView).toHaveBeenCalledTimes(1));
    expect(document.activeElement?.id).toBe('patrol-operational-records');
    scrollIntoView.mockClear();
    return { scrollIntoView, flushFrames };
  };

  it.each(['digest', 'recent work'])(
    'keeps the target aligned as each initial read settles, with %s first',
    async (first) => {
      const { scrollIntoView, flushFrames } = await deferHandoff();
      const finishes =
        first === 'digest'
          ? [patrolState.finishDigest, patrolState.finishRecentWork]
          : [patrolState.finishRecentWork, patrolState.finishDigest];
      finishes[0]?.();
      flushFrames();
      expect(scrollIntoView).toHaveBeenCalledTimes(1);
      finishes[1]?.();
      flushFrames();
      expect(scrollIntoView).toHaveBeenCalledTimes(2);
      expect(scrollIntoView.mock.instances[0]).toBe(
        document.getElementById('patrol-operational-records'),
      );
      // Later refresh notifications cannot restart a completed handoff.
      finishes.forEach((finish) => finish?.());
      flushFrames();
      expect(scrollIntoView).toHaveBeenCalledTimes(2);
    },
  );

  it.each(['pointerdown', 'touchstart', 'wheel', 'keydown'])(
    'does not reclaim scroll after %s',
    async (event) => {
      const { scrollIntoView, flushFrames } = await deferHandoff();
      document.dispatchEvent(new Event(event));
      patrolState.finishDigest?.();
      patrolState.finishRecentWork?.();
      flushFrames();
      expect(scrollIntoView).not.toHaveBeenCalled();
    },
  );

  it('cancels a ready alignment when the operator navigates away before its frame', async () => {
    const { scrollIntoView, flushFrames } = await deferHandoff();
    patrolState.finishDigest?.();
    patrolState.finishRecentWork?.();
    fireEvent.click(screen.getByRole('tab', { name: 'Protection' }));
    await vi.waitFor(() => expect(window.location.pathname).toBe('/patrol/protection'));
    flushFrames();
    expect(scrollIntoView).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('tab', { name: 'Activity' }));
    await vi.waitFor(() => expect(window.location.pathname).toBe('/patrol/activity'));
    patrolState.finishDigest?.();
    patrolState.finishRecentWork?.();
    flushFrames();
    expect(scrollIntoView).not.toHaveBeenCalled();
  });

  it('cancels pending alignment on unmount', async () => {
    const { scrollIntoView, flushFrames } = await deferHandoff();
    patrolState.finishDigest?.();
    patrolState.finishRecentWork?.();
    cleanup();
    flushFrames();
    expect(scrollIntoView).not.toHaveBeenCalled();
  });

  it('restores the live Inbox selection on return and never a closed one', async () => {
    window.history.replaceState(null, '', '/patrol?attention=chosen');
    renderSurface();

    fireEvent.click(screen.getByRole('tab', { name: 'Activity' }));
    await vi.waitFor(() => expect(window.location.pathname).toBe('/patrol/activity'));
    expect(window.location.search).toBe('');

    fireEvent.click(screen.getByRole('tab', { name: 'Inbox' }));
    await vi.waitFor(() => expect(window.location.pathname).toBe('/patrol'));
    expect(window.location.search).toBe('?attention=chosen');

    // The workbench closes a selection by writing history directly, which the
    // router does not see. The next round trip must follow the real address.
    window.history.replaceState(null, '', '/patrol');
    fireEvent.click(screen.getByRole('tab', { name: 'Activity' }));
    await vi.waitFor(() => expect(window.location.pathname).toBe('/patrol/activity'));
    fireEvent.click(screen.getByRole('tab', { name: 'Inbox' }));
    await vi.waitFor(() => expect(window.location.pathname).toBe('/patrol'));
    expect(window.location.search).toBe('');
  });
});
