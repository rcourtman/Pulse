import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import type { Alert } from '@/types/api';
import { makeSystemAlert } from '@/features/alerts/__fixtures__/systemAlerts';

const { aiAvailabilityMock, canRunPatrolMock, triggerPatrolRunMock, notificationStoreMock } =
  vi.hoisted(() => ({
    aiAvailabilityMock: { enabled: true as boolean | null },
    canRunPatrolMock: vi.fn(() => true),
    triggerPatrolRunMock: vi.fn(),
    notificationStoreMock: {
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn(),
    },
  }));

vi.mock('@/stores/aiAvailability', () => ({
  aiAvailabilityStore: aiAvailabilityMock,
}));

vi.mock('@/stores/sessionSettingsCapabilities', () => ({
  sessionCanRunPatrol: () => canRunPatrolMock(),
}));

vi.mock('@/api/patrol', () => ({
  triggerPatrolRun: (...args: unknown[]) => triggerPatrolRunMock(...(args as [unknown])),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: notificationStoreMock,
}));

import { InvestigateAlertButton } from '../InvestigateAlertButton';

function makeAlert(overrides: Partial<Alert> = {}): Alert {
  return {
    id: 'alert-1',
    type: 'cpu',
    level: 'warning',
    resourceId: 'vm-101',
    resourceName: 'test-vm',
    node: 'pve1',
    nodeDisplayName: 'PVE Node 1',
    instance: '',
    message: 'CPU usage is high',
    value: 82.5,
    threshold: 80,
    startTime: new Date(Date.now() - 5 * 60_000).toISOString(),
    acknowledged: false,
    ...overrides,
  };
}

afterEach(() => {
  cleanup();
  setActiveLocale(DEFAULT_LOCALE);
});

beforeEach(() => {
  setActiveLocale(DEFAULT_LOCALE);
  aiAvailabilityMock.enabled = true;
  canRunPatrolMock.mockReturnValue(true);
  triggerPatrolRunMock.mockReset();
  triggerPatrolRunMock.mockResolvedValue({
    success: true,
    message: 'Triggered targeted Patrol check',
  });
  notificationStoreMock.success.mockReset();
  notificationStoreMock.error.mockReset();
  notificationStoreMock.warning.mockReset();
  notificationStoreMock.info.mockReset();
});

describe('InvestigateAlertButton visibility', () => {
  it.each([false, null])('renders nothing when AI availability is %s', (enabled) => {
    aiAvailabilityMock.enabled = enabled;
    const { container } = render(() => <InvestigateAlertButton alert={makeAlert()} />);
    expect(container.innerHTML).toBe('');
  });

  it('renders nothing when the session cannot start a Patrol run', () => {
    // POST /api/ai/patrol/run needs an admin with ai:execute; a button the
    // route would refuse is only a 403 toast waiting to happen.
    canRunPatrolMock.mockReturnValue(false);
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="text" />);
    expect(screen.queryByRole('button')).toBeNull();
  });

  it('renders nothing for an alert without a resource Patrol can scope to', () => {
    const { container } = render(() => (
      <InvestigateAlertButton alert={makeAlert({ resourceId: '  ' })} variant="text" />
    ));
    expect(container.innerHTML).toBe('');
  });

  it.each([undefined, { systemAlert: true, resourceType: 'vm' }])(
    'renders nothing for a Pulse system alert with metadata %j',
    (metadata) => {
      const alert = makeSystemAlert('backup-evaluation', { resourceId: 'vm-wrong', metadata });
      const { container } = render(() => <InvestigateAlertButton alert={alert} variant="text" />);
      expect(container.innerHTML).toBe('');
    },
  );

  it('keeps Patrol for a monitored VM that happens to be named Pulse', async () => {
    const alert = makeAlert({ resourceName: 'Pulse' });
    render(() => <InvestigateAlertButton alert={alert} variant="text" />);
    fireEvent.click(screen.getByRole('button', { name: 'Have Patrol investigate' }));
    await waitFor(() => expect(triggerPatrolRunMock).toHaveBeenCalledTimes(1));
  });

  it('offers one action and no Assistant menu', () => {
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="text" />);
    expect(screen.getAllByRole('button')).toHaveLength(1);
    expect(screen.queryByRole('button', { name: 'More alert actions' })).toBeNull();
    expect(screen.queryByText(/Assistant/)).toBeNull();
  });
});

describe('InvestigateAlertButton variants', () => {
  it('labels the text variant with the Patrol action and keeps a touch-sized target', () => {
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="text" />);
    const button = screen.getByRole('button', { name: 'Have Patrol investigate' });
    expect(button).toHaveAttribute('title', 'Have Patrol investigate this alert');
    expect(button).toHaveClass('min-h-11', 'sm:min-h-0');
  });

  it('defaults to the text variant', () => {
    render(() => <InvestigateAlertButton alert={makeAlert()} />);
    expect(screen.getByRole('button', { name: 'Have Patrol investigate' })).toBeInTheDocument();
  });

  it('names the alert and its resource on the icon variant', () => {
    render(() => (
      <InvestigateAlertButton alert={makeAlert()} variant="icon" subjectLabel="CPU usage" />
    ));
    const button = screen.getByRole('button', {
      name: 'Have Patrol investigate CPU usage on test-vm',
    });
    expect(button).toHaveAttribute('title', 'Have Patrol investigate this alert');
    expect(button.textContent).toBe('');
  });

  it('adds the start time to the icon name when the host gives one', () => {
    render(() => (
      <InvestigateAlertButton
        alert={makeAlert()}
        variant="icon"
        subjectLabel="CPU usage"
        subjectTime="12:04"
      />
    ));
    expect(
      screen.getByRole('button', { name: 'Have Patrol investigate CPU usage on test-vm, 12:04' }),
    ).toBeInTheDocument();
  });

  it('falls back to the alert type, then the plain title, for the icon name', () => {
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="icon" />);
    expect(
      screen.getByRole('button', { name: 'Have Patrol investigate cpu on test-vm' }),
    ).toBeInTheDocument();
    cleanup();
    render(() => <InvestigateAlertButton alert={makeAlert({ resourceName: '' })} variant="icon" />);
    expect(
      screen.getByRole('button', { name: 'Have Patrol investigate this alert' }),
    ).toBeInTheDocument();
  });

  it('applies sm icon sizing by default and md when asked', () => {
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="icon" />);
    expect(screen.getByRole('button')).toHaveClass('w-6', 'h-6');
    cleanup();
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="icon" size="md" />);
    expect(screen.getByRole('button')).toHaveClass('w-8', 'h-8');
  });

  it('appends a custom class', () => {
    render(() => (
      <InvestigateAlertButton alert={makeAlert()} variant="icon" class="min-h-11 min-w-11" />
    ));
    expect(screen.getByRole('button')).toHaveClass('min-h-11', 'min-w-11');
  });
});

describe('InvestigateAlertButton Patrol run', () => {
  it('starts a manual Patrol check scoped to the alert resource', async () => {
    const alert = makeAlert({ id: 'alert-9', type: 'cpu', resourceId: 'vm-101' });
    render(() => <InvestigateAlertButton alert={alert} variant="text" />);

    await fireEvent.click(screen.getByRole('button', { name: 'Have Patrol investigate' }));

    // Exactly the fields the backend's manualScopedPatrolRequest accepts; the
    // run endpoint rejects unknown fields with a 400.
    await waitFor(() =>
      expect(triggerPatrolRunMock).toHaveBeenCalledWith({
        resource_ids: ['vm-101'],
        alert_identifier: 'alert-9',
        alert_type: 'cpu',
      }),
    );
    await waitFor(() =>
      expect(notificationStoreMock.success).toHaveBeenCalledWith(
        'Patrol is investigating test-vm. Findings appear on the Patrol page.',
      ),
    );
  });

  it('starts the same check from the icon variant', async () => {
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="icon" />);
    await fireEvent.click(screen.getByRole('button'));
    await waitFor(() => expect(triggerPatrolRunMock).toHaveBeenCalledTimes(1));
  });

  it('starts one run per click while the first request is in flight', async () => {
    let finish: (value: { success: boolean }) => void = () => {};
    triggerPatrolRunMock.mockReturnValueOnce(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="text" />);
    const button = screen.getByRole('button', { name: 'Have Patrol investigate' });

    await fireEvent.click(button);
    await fireEvent.click(button);
    expect(triggerPatrolRunMock).toHaveBeenCalledTimes(1);
    expect(button).toBeDisabled();

    finish({ success: true });
    await waitFor(() => expect(button).not.toBeDisabled());
    await fireEvent.click(button);
    await waitFor(() => expect(triggerPatrolRunMock).toHaveBeenCalledTimes(2));
  });

  it('names the resource ID when the alert has no resource name', async () => {
    render(() => (
      <InvestigateAlertButton alert={makeAlert({ resourceName: undefined })} variant="text" />
    ));
    await fireEvent.click(screen.getByRole('button', { name: 'Have Patrol investigate' }));
    await waitFor(() =>
      expect(notificationStoreMock.success).toHaveBeenCalledWith(
        'Patrol is investigating vm-101. Findings appear on the Patrol page.',
      ),
    );
  });

  it('warns with the server message when Patrol declines the run', async () => {
    triggerPatrolRunMock.mockResolvedValue({ success: false, message: 'Patrol is paused' });
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="text" />);
    await fireEvent.click(screen.getByRole('button', { name: 'Have Patrol investigate' }));
    await waitFor(() =>
      expect(notificationStoreMock.warning).toHaveBeenCalledWith('Patrol is paused'),
    );
  });

  it('falls back to a generic warning when Patrol declines without a message', async () => {
    triggerPatrolRunMock.mockResolvedValue({ success: false, message: '' });
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="text" />);
    await fireEvent.click(screen.getByRole('button', { name: 'Have Patrol investigate' }));
    await waitFor(() =>
      expect(notificationStoreMock.warning).toHaveBeenCalledWith("Couldn't start the Patrol check"),
    );
  });

  it('reports a failed request as an error', async () => {
    triggerPatrolRunMock.mockRejectedValue(new Error('Admin access required'));
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="text" />);
    await fireEvent.click(screen.getByRole('button', { name: 'Have Patrol investigate' }));
    await waitFor(() =>
      expect(notificationStoreMock.error).toHaveBeenCalledWith('Admin access required'),
    );
  });

  it('keeps the click inside the button so table rows do not also toggle', () => {
    const rowClick = vi.fn();
    render(() => (
      <div onClick={rowClick}>
        <InvestigateAlertButton alert={makeAlert()} variant="icon" />
      </div>
    ));
    const event = new MouseEvent('click', { bubbles: true, cancelable: true });
    screen.getByRole('button').dispatchEvent(event);
    expect(event.defaultPrevented).toBe(true);
    expect(rowClick).not.toHaveBeenCalled();
  });
});
