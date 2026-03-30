import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';

import { HistoryTab } from '../tabs/HistoryTab';

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({
    hash: '',
  }),
}));

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    activeAlerts: {},
  }),
}));

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: () => false,
  }),
}));

vi.mock('../useAlertHistoryState', () => ({
  useAlertHistoryState: () => ({
    alertData: () => [],
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
    render(() => (
      <HistoryTab
        hasAIAlertsFeature={() => true}
        licenseLoading={() => false}
        getResource={() => undefined}
        allResources={() => []}
      />
    ));

    expect(screen.getByText('frequency-card')).toBeInTheDocument();
    expect(screen.getByText('filters-card')).toBeInTheDocument();
    expect(screen.getByText('resource-incidents-panel')).toBeInTheDocument();
    expect(screen.getByText('history-table')).toBeInTheDocument();
    expect(screen.getByText('administration-card')).toBeInTheDocument();
  });
});
