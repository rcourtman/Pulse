import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { PatrolAutonomyLevel } from '@/api/patrol';
import { PatrolIntelligenceHeader } from '../PatrolIntelligenceHeader';
import type { PatrolIntelligenceState } from '../usePatrolIntelligenceState';

const WATCH_ONLY_DETAIL =
  'Patrol checks infrastructure and reports issues only. It does not start fixes.';

interface HeaderStateOptions {
  autonomyLevel?: PatrolAutonomyLevel;
  autoFixLocked?: boolean;
  setupOnly?: boolean;
  autopilotDialogOpen?: boolean;
}

const createState = (options: HeaderStateOptions = {}) => {
  const autonomyLevel = options.autonomyLevel ?? 'approval';
  const handleAutonomyChange = vi.fn();
  const state = {
    autopilotStatus: () => null,
    autonomyLevel: () => autonomyLevel,
    requestedAutonomyLevel: () => autonomyLevel,
    autoFixLocked: () => options.autoFixLocked ?? false,
    autoFixCapabilityBlock: () => undefined,
    licenseRuntimeIdentity: () => undefined,
    patrolRunHistory: { value: () => [] },
    patrolStatus: () => null,
    patrolReadiness: () => null,
    blockedCause: () => null,
    runtimeState: () => 'active',
    canTriggerPatrol: () => true,
    triggerPatrolDisabledReason: () => '',
    isTriggeringPatrol: () => false,
    manualRunRequested: () => false,
    patrolStream: { isStreaming: () => false },
    patrolEnabledLocal: () => true,
    isTogglingPatrol: () => false,
    isUpdatingAutonomy: () => false,
    shouldShowPatrolSetupOnly: () => options.setupOnly ?? false,
    autopilotDialogOpen: () => options.autopilotDialogOpen ?? false,
    setAutopilotDialogOpen: vi.fn(),
    acknowledgeAndActivateAutopilot: vi.fn(),
    revokeAutopilot: vi.fn(),
    handleRunPatrol: vi.fn(),
    handleTogglePatrol: vi.fn(),
    handleAutonomyChange,
  } as unknown as PatrolIntelligenceState;
  return { state, handleAutonomyChange };
};

const renderHeader = (state: PatrolIntelligenceState) =>
  render(() => (
    <Router>
      <Route path="*" component={() => <PatrolIntelligenceHeader state={state} />} />
    </Router>
  ));

const modeDisclosure = () => {
  const summary = screen.getByText('Mode and automation');
  const details = summary.closest('details');
  expect(details).not.toBeNull();
  expect(summary.tagName).toBe('SUMMARY');
  return details!;
};

// These pin the Patrol header facts that the api-contracts, frontend-primitives,
// and patrol-intelligence contracts state about the Patrol mode selector.
describe('PatrolIntelligenceHeader Patrol mode placement', () => {
  afterEach(cleanup);

  it('keeps the Patrol mode selector inside the collapsed Mode and automation disclosure', () => {
    const { state, handleAutonomyChange } = createState();
    renderHeader(state);

    const details = modeDisclosure();
    expect(details.open).toBe(false);
    expect(details.hasAttribute('open')).toBe(false);

    const selector = screen.getByRole('group', { name: 'Patrol mode' });
    expect(details.contains(selector)).toBe(true);
    expect(selector.closest('#patrol-control')).not.toBeNull();
    expect(
      within(selector)
        .getAllByRole('button')
        .map((button) => button.getAttribute('aria-label')),
    ).toEqual(['Watch only', 'Ask first', 'Safe auto-fix', 'Autopilot']);

    fireEvent.click(within(selector).getByRole('button', { name: 'Safe auto-fix' }));
    expect(handleAutonomyChange).toHaveBeenCalledWith('assisted');
  });

  it('shows plan-locked installs one inline Patrol mode line instead of a selector', () => {
    const { state } = createState({ autonomyLevel: 'approval', autoFixLocked: true });
    renderHeader(state);

    expect(screen.queryByText('Mode and automation')).toBeNull();
    expect(screen.queryByRole('group', { name: 'Patrol mode' })).toBeNull();
    expect(document.querySelector('details')).toBeNull();

    const label = screen.getByText('Patrol mode');
    const line = label.parentElement!;
    expect(line.textContent).toContain(WATCH_ONLY_DETAIL);
    expect(line.closest('#patrol-control')).not.toBeNull();
  });

  it('keeps the Mode and automation disclosure while setup hides the run and settings controls', () => {
    const { state } = createState({ setupOnly: true });
    renderHeader(state);

    expect(screen.queryByText('Check now')).toBeNull();
    expect(screen.queryByRole('link', { name: 'Open Patrol settings' })).toBeNull();
    const details = modeDisclosure();
    expect(details.contains(screen.getByRole('group', { name: 'Patrol mode' }))).toBe(true);
  });

  it('opens no Patrol control dialog; the Autopilot acknowledgement is its only dialog', () => {
    const closed = createState();
    renderHeader(closed.state);
    expect(screen.queryAllByRole('dialog')).toHaveLength(0);
    cleanup();

    const opened = createState({ autopilotDialogOpen: true });
    renderHeader(opened.state);
    const dialogs = screen.getAllByRole('dialog');
    expect(dialogs).toHaveLength(1);
    expect(dialogs[0]).toHaveAccessibleName('Activate Autopilot');
  });
});
