import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { DataHandlingPanel } from '../DataHandlingPanel';

const resourceStatsState = vi.hoisted(() => ({
  error: null as unknown,
  loading: false,
  policyPosture: null as unknown,
  refetch: vi.fn(),
}));

vi.mock('@/hooks/useResourceStats', () => ({
  useResourceStats: () => ({
    error: () => resourceStatsState.error,
    loading: () => resourceStatsState.loading,
    policyPosture: () => resourceStatsState.policyPosture,
    refetch: resourceStatsState.refetch,
  }),
}));

const renderPanel = () =>
  render(() => (
    <Router>
      <Route path="/" component={() => <DataHandlingPanel />} />
    </Router>
  ));

describe('DataHandlingPanel', () => {
  beforeEach(() => {
    resourceStatsState.error = null;
    resourceStatsState.loading = false;
    resourceStatsState.policyPosture = {
      totalResources: 0,
      sensitivityCounts: {},
      routingCounts: {},
      redactionCounts: {},
    };
    resourceStatsState.refetch.mockReset();
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState({}, '', '/');
  });

  it('explains the empty resource posture instead of leading with zero-value counters', () => {
    renderPanel();

    expect(screen.getByText('Resource Data Policy')).toBeInTheDocument();
    expect(screen.getByText('Read-only resource privacy posture')).toBeInTheDocument();
    expect(screen.getByText('No monitored resources to classify')).toBeInTheDocument();
    expect(screen.getByText(/fresh instance, before discovery finishes/i)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open Infrastructure' })).toHaveAttribute(
      'href',
      '/settings/infrastructure',
    );
    expect(screen.queryByText('Governed Resources')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Refresh' })).toHaveClass('min-h-11', 'sm:min-h-0');
  });

  it('shows policy posture metrics once resources exist', () => {
    resourceStatsState.policyPosture = {
      totalResources: 4,
      sensitivityCounts: {
        internal: 1,
        sensitive: 2,
        restricted: 1,
      },
      routingCounts: {
        'cloud-summary': 1,
        'local-first': 2,
        'local-only': 1,
      },
      redactionCounts: {
        hostname: 2,
        'ip-address': 1,
      },
    };

    renderPanel();

    expect(screen.getByText('Governed Resources')).toBeInTheDocument();
    expect(screen.getByText('Local-Only')).toBeInTheDocument();
    expect(screen.getByText('Redaction Hints')).toBeInTheDocument();
    expect(screen.getByText('Sensitivity')).toBeInTheDocument();
    expect(screen.getByText('Handling Boundary')).toBeInTheDocument();
    expect(screen.queryByText('No monitored resources to classify')).not.toBeInTheDocument();
  });
});
