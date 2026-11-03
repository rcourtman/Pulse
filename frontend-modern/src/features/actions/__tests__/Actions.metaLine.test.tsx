import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { getPatrolAutonomySettings } from '@/api/patrol';
import { SecurityAPI } from '@/api/security';
import { aiAvailabilityStore } from '@/stores/aiAvailability';
import type { ActionAuditRecord } from '@/types/actionAudit';
import { getActionResourcePresentation } from '../actionPresentation';
import Actions from '../../../pages/Actions';

vi.mock('@/api/resourceActions', () => ({
  ResourceActionsAPI: { listActions: vi.fn(), getAction: vi.fn() },
}));
vi.mock('@/api/security', () => ({ SecurityAPI: { getStatus: vi.fn() } }));
vi.mock('@/api/patrol', () => ({ getPatrolAutonomySettings: vi.fn().mockResolvedValue({}) }));

const resourceId = 'docker:container:analytics-batch-01';

const audit: ActionAuditRecord = {
  id: 'restart-analytics',
  createdAt: '2026-09-29T18:00:00Z',
  updatedAt: '2026-09-29T18:01:00Z',
  state: 'executing',
  decisionRevision: 1,
  request: {
    requestId: 'request-restart-analytics',
    resourceId,
    capabilityName: 'restart',
    reason: 'Recover checkout after three failed health checks',
    requestedBy: 'operator',
  },
  plan: {
    actionId: 'restart-analytics',
    requestId: 'request-restart-analytics',
    allowed: true,
    requiresApproval: false,
    approvalPolicy: 'none',
    rollbackAvailable: false,
    expiresAt: '2026-09-29T21:00:00Z',
    planHash: 'sha256:plan-restart-analytics',
  },
  verificationOutcome: { status: 'unknown' },
};

beforeEach(() => {
  window.history.replaceState(null, '', '/');
  vi.mocked(ResourceActionsAPI.listActions).mockResolvedValue({
    actions: [audit],
    count: 1,
    view: 'pending',
    readOnly: false,
  });
  vi.mocked(SecurityAPI.getStatus).mockResolvedValue({
    hasAuthentication: true,
    requiresAuth: true,
    settingsCapabilities: { authenticationWrite: false },
  } as Awaited<ReturnType<typeof SecurityAPI.getStatus>>);
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  aiAvailabilityStore.clear();
  window.history.replaceState(null, '', '/');
});

describe('Actions card resource line', () => {
  it('keeps the target name whole and lets its type give way on narrow cards', async () => {
    const { label, detail } = getActionResourcePresentation(resourceId);
    expect(label).toBe('analytics-batch-01');
    expect(detail).not.toBe('');

    render(() => (
      <Router>
        <Route path="*" component={Actions} />
      </Router>
    ));

    // On a phone the name and its type used to shrink together, so the line
    // read "analytics-ba… · Virtual ma…" and hid which resource the action
    // touches. The name never shrinks below its own width; the type and its
    // separator take whatever room is left.
    const name = await screen.findByText(label);
    expect(name).toHaveClass('shrink-0', 'max-w-full', 'truncate');
    const type = screen.getByText(detail).parentElement!;
    expect(type).toHaveClass('min-w-0', 'flex-1', 'truncate');
    expect(type.querySelector('[aria-hidden="true"]')).toHaveTextContent('·');
    expect(type.parentElement).toBe(name.parentElement);
    expect(name.parentElement?.closest(`[title="${resourceId}"]`)).not.toBeNull();
  });
});

describe('Actions empty Open inbox', () => {
  const renderEmptyInbox = () => {
    vi.mocked(ResourceActionsAPI.listActions).mockResolvedValue({
      actions: [],
      count: 0,
      view: 'pending',
      readOnly: false,
    });
    vi.mocked(getPatrolAutonomySettings).mockResolvedValueOnce({
      effective_autonomy_level: 'monitor',
    } as Awaited<ReturnType<typeof getPatrolAutonomySettings>>);
    render(() => (
      <Router>
        <Route path="*" component={Actions} />
      </Router>
    ));
  };

  it('names Watch only Patrol as the reason nothing is queued while AI is on', async () => {
    aiAvailabilityStore.setEnabled(true);
    renderEmptyInbox();

    expect(await screen.findByText(/Patrol runs in Watch only mode/)).toBeInTheDocument();
  });

  it('does not describe Patrol as running while AI is off (#905)', async () => {
    aiAvailabilityStore.setEnabled(false);
    renderEmptyInbox();

    const calmState = await screen.findByTestId('actions-calm-state');
    await waitFor(() => expect(getPatrolAutonomySettings).toHaveBeenCalled());
    // Let the Patrol settings answer land before judging the calm state.
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(calmState).toHaveTextContent(
      'Pulse will place proposed or in-progress changes here for review.',
    );
    expect(calmState).not.toHaveTextContent('Watch only');
  });
});

describe('Actions queue size', () => {
  it('mounts a bounded window of a large action inventory', async () => {
    const actions = Array.from({ length: 150 }, (_, index) => ({
      ...audit,
      id: `restart-${index}`,
      request: {
        ...audit.request,
        requestId: `request-${index}`,
        resourceId: `docker:container:batch-${index}`,
      },
    }));
    vi.mocked(ResourceActionsAPI.listActions).mockResolvedValue({
      actions,
      count: actions.length,
      view: 'pending',
      readOnly: false,
    });

    render(() => (
      <Router>
        <Route path="*" component={Actions} />
      </Router>
    ));

    const list = await screen.findByRole('list', { name: 'Open actions' });
    expect(list.querySelectorAll('[role="listitem"]').length).toBeGreaterThan(0);
    expect(list.querySelectorAll('[role="listitem"]').length).toBeLessThan(actions.length);
    expect(list.querySelectorAll('[data-platform-window-spacer]')).toHaveLength(2);
  });
});
