import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { PatrolIntelligenceSurface } from '../PatrolIntelligenceSurface';

const patrolState = vi.hoisted(() => ({ setupOnly: false, enabled: true }));

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
  PatrolIntelligenceWorkspace: (props: { findingResourceId?: string }) => (
    <div data-testid="patrol-workspace" data-resource-id={props.findingResourceId ?? ''} />
  ),
}));

vi.mock('../PatrolObjectivesPanel', () => ({
  PatrolObjectivesPanel: () => <div>Objectives</div>,
}));

vi.mock('../PatrolRecentWorkPanel', () => ({
  PatrolRecentWorkPanel: () => <div>Recent work</div>,
}));

vi.mock('../PatrolWeeklyDigestCard', () => ({
  PatrolWeeklyDigestCard: () => <div>This week</div>,
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

    fireEvent.click(screen.getByRole('button', { name: 'Show all finding options' }));

    expect(screen.getByTestId('patrol-workspace')).toHaveAttribute('data-resource-id', '');
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
