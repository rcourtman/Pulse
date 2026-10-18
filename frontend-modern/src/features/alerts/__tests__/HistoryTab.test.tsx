import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';

import { HistoryTab } from '../tabs/HistoryTab';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    hash: '',
  }),
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    activeAlerts: {},
    activeAlertsHydrationStatus: () => 'ready',
    refreshActiveAlerts: () => Promise.resolve(true),
  }),
}));

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: () => false,
  }),
}));

const focus = vi.hoisted(() => ({
  id: '',
  missing: false,
  request: null as null | { rowKey: string; alert: { title: string; resourceName: string } },
  clear: vi.fn(),
}));

vi.mock('../useAlertHistoryState', () => ({
  useAlertHistoryState: () => ({
    alertData: () => [],
    loading: () => false,
    historyLoadError: () => false,
    focusedAlertId: () => focus.id,
    focusedAlertMissing: () => focus.missing,
    focusRequest: () => focus.request,
    focusedAlert: () => focus.request?.alert ?? null,
    clearFocusedAlert: focus.clear,
  }),
}));

vi.mock('../AlertHistoryFrequencyCard', () => ({
  AlertHistoryFrequencyCard: () => <div>frequency-card</div>,
}));

vi.mock('../AlertHistoryFiltersCard', () => ({
  AlertHistoryFiltersCard: () => <div>filters-card</div>,
}));

vi.mock('../AlertResourceIncidentsPanel', () => ({
  AlertResourceIncidentsPanel: () => <div>resource-incidents-panel</div>,
}));

vi.mock('../AlertHistoryTableSection', () => ({
  AlertHistoryTableSection: () => <div>history-table</div>,
}));

vi.mock('../AlertHistoryAdministrationCard', () => ({
  AlertHistoryAdministrationCard: () => <div>administration-card</div>,
}));

describe('HistoryTab', () => {
  it('renders against the current history-state contract without depending on legacy filteredAlerts', () => {
    render(() => <HistoryTab getResource={() => undefined} allResources={() => []} />);

    expect(screen.getByText('frequency-card')).toBeInTheDocument();
    expect(screen.getByText('filters-card')).toBeInTheDocument();
    expect(screen.getByText('history-table')).toBeInTheDocument();
    expect(screen.getByText('administration-card')).toBeInTheDocument();
    // #1687: the resource incidents panel is deliberately NOT a page-level
    // sibling any more. It renders inside the row that opened it, so that a
    // reader scrolled deep into the history sees it appear where they clicked
    // instead of thousands of pixels above the viewport.
    expect(screen.queryByText('resource-incidents-panel')).not.toBeInTheDocument();
  });

  // A notification link opens History narrowed to one alert; the notice
  // names it and offers the way back to every alert.
  it('names the linked alert and offers to show all alerts', () => {
    focus.id = 'disk-1';
    focus.request = { rowKey: 'disk-1::t', alert: { title: 'Disk usage', resourceName: 'nas-01' } };
    try {
      render(() => <HistoryTab getResource={() => undefined} allResources={() => []} />);
      expect(screen.getByRole('status')).toHaveTextContent(
        'Showing Disk usage on nas-01, opened from a link.',
      );
      fireEvent.click(screen.getByRole('button', { name: 'Show all alerts' }));
      expect(focus.clear).toHaveBeenCalledTimes(1);
      expect(screen.queryByRole('button', { name: /retry/i })).toBeNull();
    } finally {
      focus.id = '';
      focus.request = null;
    }
  });
});
