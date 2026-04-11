import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { OperationsPageSurface } from '@/features/operations/OperationsPageSurface';

const navigateSpy = vi.hoisted(() => vi.fn());
const presentationPolicyIsDemoModeMock = vi.hoisted(() => vi.fn(() => false));
const locationState = vi.hoisted(() => ({
  pathname: '/operations',
  hash: '',
  search: '',
  query: {},
}));

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => locationState,
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsDemoMode: () => presentationPolicyIsDemoModeMock(),
}));

vi.mock('@/components/Settings/DiagnosticsPanel', () => ({
  DiagnosticsPanel: () => <div data-testid="diagnostics-panel">Diagnostics</div>,
}));

vi.mock('@/components/Settings/ReportingPanel', () => ({
  ReportingPanel: () => <div data-testid="reporting-panel">Reporting</div>,
}));

vi.mock('@/components/Settings/SystemLogsPanel', () => ({
  SystemLogsPanel: () => <div data-testid="system-logs-panel">Logs</div>,
}));

describe('OperationsPageSurface demo mode', () => {
  beforeEach(() => {
    cleanup();
    navigateSpy.mockReset();
    presentationPolicyIsDemoModeMock.mockReset();
    presentationPolicyIsDemoModeMock.mockReturnValue(false);
    locationState.pathname = '/operations';
    locationState.hash = '';
    locationState.search = '';
  });

  afterEach(() => cleanup());

  it('redirects demo sessions back to the dashboard and hides operations chrome', async () => {
    presentationPolicyIsDemoModeMock.mockReturnValue(true);

    render(() => <OperationsPageSurface />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalledWith('/dashboard', { replace: true });
    });
    expect(screen.queryByText('Diagnostics & Health')).not.toBeInTheDocument();
    expect(screen.queryByTestId('diagnostics-panel')).not.toBeInTheDocument();
  });

  it('keeps operations tabs available outside demo mode', async () => {
    render(() => <OperationsPageSurface />);

    await waitFor(() => {
      expect(screen.getByText('Diagnostics & Health')).toBeInTheDocument();
    });
    expect(screen.getByTestId('diagnostics-panel')).toBeInTheDocument();
    expect(navigateSpy).not.toHaveBeenCalledWith('/dashboard', { replace: true });
  });
});
