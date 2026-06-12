import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { DataHandlingPanel } from '../DataHandlingPanel';

const unifiedResourcesState = vi.hoisted(() => ({
  error: null as unknown,
  loading: false,
  policyPosture: null as unknown,
  refetch: vi.fn(),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: () => ({
    error: () => unifiedResourcesState.error,
    loading: () => unifiedResourcesState.loading,
    policyPosture: () => unifiedResourcesState.policyPosture,
    refetch: unifiedResourcesState.refetch,
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
    unifiedResourcesState.error = null;
    unifiedResourcesState.loading = false;
    unifiedResourcesState.policyPosture = {
      totalResources: 0,
      sensitivityCounts: {},
      routingCounts: {},
      redactionCounts: {},
    };
    unifiedResourcesState.refetch.mockReset();
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
  });

  it('shows policy posture metrics once resources exist', () => {
    unifiedResourcesState.policyPosture = {
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
