import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { PatrolIntelligenceSurface } from '../PatrolIntelligenceSurface';

vi.mock('../usePatrolIntelligenceState', () => ({
  usePatrolIntelligenceState: () => ({
    shouldShowPatrolSetupOnly: () => false,
    patrolEnabledLocal: () => true,
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

describe('PatrolIntelligenceSurface finding handoff', () => {
  afterEach(() => cleanup());

  it('keeps the selected decision resource in context and lets the operator broaden the list', () => {
    render(() => <PatrolIntelligenceSurface />);

    fireEvent.click(screen.getByRole('button', { name: 'Open scoped finding options' }));

    expect(screen.getByRole('tab', { name: 'Activity' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText(/Showing Patrol findings for Database VM/i)).toBeInTheDocument();
    expect(screen.getByTestId('patrol-workspace')).toHaveAttribute('data-resource-id', 'vm-101');

    fireEvent.click(screen.getByRole('button', { name: 'Show all finding options' }));

    expect(screen.getByTestId('patrol-workspace')).toHaveAttribute('data-resource-id', '');
    expect(screen.queryByText(/Showing Patrol findings for Database VM/i)).not.toBeInTheDocument();
  });
});
