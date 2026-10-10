import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { PatrolAutonomyLevel } from '@/api/patrol';
import {
  PATROL_CONTROL_ANCHOR,
  PATROL_CONTROL_PATH,
  PATROL_CONTROL_PATH_WITH_STARTER,
  PATROL_OPERATIONS_LOOP_ANCHOR,
} from '@/routing/resourceLinks';
import { PatrolIntelligenceHeader } from '../PatrolIntelligenceHeader';
import type { PatrolIntelligenceState } from '../usePatrolIntelligenceState';

const WATCH_ONLY_DETAIL =
  'Patrol checks infrastructure and reports issues only. It does not start fixes.';

interface HeaderStateOptions {
  autonomyLevel?: PatrolAutonomyLevel;
  autoFixLocked?: boolean | (() => boolean);
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
    autoFixLocked: () => {
      const locked = options.autoFixLocked ?? false;
      return typeof locked === 'function' ? locked() : locked;
    },
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

const renderHeaderAt = (url: string, state: PatrolIntelligenceState) => {
  window.history.replaceState(null, '', url);
  return renderHeader(state);
};

const navigateTo = (url: string) => {
  window.history.pushState(null, '', url);
  window.dispatchEvent(new PopStateEvent('popstate'));
};

const nextFrame = () => new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));

// Settings and licence Patrol mode actions link to #patrol-control, and the
// api-contracts "Patrol control route target" clause requires that anchor to
// land on a visible Patrol mode selector, not on its collapsed disclosure.
describe('PatrolIntelligenceHeader Patrol mode anchors', () => {
  let scrolled: Element[];

  beforeEach(() => {
    scrolled = [];
    HTMLElement.prototype.scrollIntoView = vi.fn(function (this: HTMLElement) {
      scrolled.push(this);
    });
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState(null, '', '/');
  });

  it.each([
    ['the Patrol mode entry points', PATROL_CONTROL_PATH],
    ['the licence starter handoff', PATROL_CONTROL_PATH_WITH_STARTER],
    ['the operations-loop compatibility anchor', `/patrol#${PATROL_OPERATIONS_LOOP_ANCHOR}`],
  ])('opens the Patrol mode selector and scrolls to it for %s', async (_entry, url) => {
    renderHeaderAt(url, createState().state);

    const details = modeDisclosure();
    expect(details.open).toBe(true);
    expect(details.contains(screen.getByRole('group', { name: 'Patrol mode' }))).toBe(true);
    await vi.waitFor(() => expect(scrolled).toEqual([details]));
    expect(HTMLElement.prototype.scrollIntoView).toHaveBeenCalledWith({ block: 'center' });
  });

  it('keeps the disclosure collapsed on a plain Patrol visit', async () => {
    renderHeaderAt('/patrol', createState().state);

    expect(modeDisclosure().open).toBe(false);
    await nextFrame();
    expect(scrolled).toEqual([]);
  });

  it('opens the disclosure when the selector mounts after the licence loads', async () => {
    const [locked, setLocked] = createSignal(true);
    renderHeaderAt(PATROL_CONTROL_PATH, createState({ autoFixLocked: locked }).state);
    expect(screen.queryByText('Mode and automation')).toBeNull();

    setLocked(false);

    await vi.waitFor(() => expect(modeDisclosure().open).toBe(true));
    await vi.waitFor(() => expect(scrolled).toEqual([modeDisclosure()]));
  });

  it('opens the disclosure when navigation moves onto the anchor', async () => {
    renderHeaderAt('/patrol', createState().state);
    expect(modeDisclosure().open).toBe(false);

    navigateTo(PATROL_CONTROL_PATH);

    await vi.waitFor(() => expect(modeDisclosure().open).toBe(true));
  });

  it('leaves a disclosure the user collapsed alone while the anchor stays', async () => {
    renderHeaderAt(PATROL_CONTROL_PATH, createState().state);
    const details = modeDisclosure();
    await vi.waitFor(() => expect(scrolled).toHaveLength(1));

    details.open = false;
    navigateTo(`/patrol?attention=finding-1#${PATROL_CONTROL_ANCHOR}`);
    await nextFrame();

    expect(details.open).toBe(false);
    expect(scrolled).toHaveLength(1);
  });

  it('needs no disclosure for plan-locked installs', async () => {
    renderHeaderAt(PATROL_CONTROL_PATH, createState({ autoFixLocked: true }).state);

    expect(document.querySelector('details')).toBeNull();
    expect(screen.getByText('Patrol mode').closest('#patrol-control')).not.toBeNull();
    await nextFrame();
    expect(scrolled).toEqual([]);
  });
});
