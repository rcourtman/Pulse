import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, fireEvent, waitFor } from '@solidjs/testing-library';
import type { Alert } from '@/types/api';

const mockGetIncidentTimeline = vi.fn();

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ hash: '', pathname: '/alerts', search: '', query: {} }),
}));

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    getIncidentTimeline: (...args: unknown[]) => mockGetIncidentTimeline(...args),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn() },
}));

vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));

import { OverviewTab } from '../OverviewTab';

function defaultProps(overrides: Record<string, unknown> = {}) {
  return {
    overrides: [] as never[],
    activeAlerts: {
      'alert-1': {
        id: 'alert-1',
        type: 'cpu',
        level: 'warning' as const,
        resourceId: 'vm-100',
        resourceName: 'test-vm',
        node: 'node1',
        instance: 'cpu0',
        message: 'CPU high',
        value: 95,
        threshold: 90,
        startTime: '2026-01-01T00:00:00Z',
        acknowledged: false,
      },
    } as Record<string, Alert>,
    updateAlert: vi.fn(),
    showQuickTip: () => false,
    dismissQuickTip: vi.fn(),
    showAcknowledged: () => false,
    setShowAcknowledged: vi.fn(),
    alertsDisabled: () => false,
    hasAIAlertsFeature: () => true,
    licenseLoading: () => false,
    ...overrides,
  };
}

describe('OverviewTab incident timeline error state', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('shows error message with Retry button when timeline fetch fails', async () => {
    mockGetIncidentTimeline.mockRejectedValueOnce(new Error('Network error'));

    render(() => <OverviewTab {...defaultProps()} />);

    // Click the Timeline button to expand
    const timelineBtn = screen.getByText('Timeline');
    fireEvent.click(timelineBtn);

    // Wait for the error state to appear
    await waitFor(() => {
      expect(screen.getByText('Failed to load timeline.')).toBeInTheDocument();
    });
    expect(screen.getByText('Retry')).toBeInTheDocument();
    expect(screen.queryByText('No incident timeline available.')).not.toBeInTheDocument();
  });

  it('retries and shows timeline on successful retry after error', async () => {
    mockGetIncidentTimeline.mockRejectedValueOnce(new Error('Network error'));

    render(() => <OverviewTab {...defaultProps()} />);

    // Click Timeline to expand — first call fails
    const timelineBtn = screen.getByText('Timeline');
    fireEvent.click(timelineBtn);

    await waitFor(() => {
      expect(screen.getByText('Failed to load timeline.')).toBeInTheDocument();
    });

    // Now set up a successful response for retry
    mockGetIncidentTimeline.mockResolvedValueOnce({
      id: 'inc-1',
      status: 'open',
      acknowledged: false,
      openedAt: '2026-01-01T00:00:00Z',
      closedAt: null,
      events: [],
    });

    // Click Retry
    const retryBtn = screen.getByText('Retry');
    fireEvent.click(retryBtn);

    // Wait for successful load — error message should disappear
    await waitFor(() => {
      expect(screen.queryByText('Failed to load timeline.')).not.toBeInTheDocument();
    });
    expect(screen.getByText('Incident')).toBeInTheDocument();
    expect(screen.getByText('open')).toBeInTheDocument();
  });

  it('shows "No incident timeline available." when fetch succeeds with null', async () => {
    mockGetIncidentTimeline.mockResolvedValueOnce(null);

    render(() => <OverviewTab {...defaultProps()} />);

    const timelineBtn = screen.getByText('Timeline');
    fireEvent.click(timelineBtn);

    await waitFor(() => {
      expect(screen.getByText('No incident timeline available.')).toBeInTheDocument();
    });
    expect(screen.queryByText('Failed to load timeline.')).not.toBeInTheDocument();
    expect(screen.queryByText('Retry')).not.toBeInTheDocument();
  });
});
