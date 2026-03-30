import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const apiFetchJSONMock = vi.hoisted(() => vi.fn());

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: apiFetchJSONMock,
}));

vi.mock('@/utils/logger', () => ({
  logger: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

vi.mock('@/utils/url', () => ({
  isPulseHttps: vi.fn(() => false),
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

describe('SecurityWarning', () => {
  beforeEach(() => {
    apiFetchJSONMock.mockReset();
    localStorage.clear();
  });

  afterEach(cleanup);

  it('does not render for private authenticated setup debt', async () => {
    const pendingStatus = deferred<any>();
    apiFetchJSONMock.mockReturnValue(pendingStatus.promise);

    const { SecurityWarning } = await import('../SecurityWarning');
    render(() => <SecurityWarning />);

    expect(screen.queryByText(/Security score:/i)).not.toBeInTheDocument();

    pendingStatus.resolve({
      apiTokenConfigured: false,
      credentialsEncrypted: true,
      exportProtected: true,
      hasAuditLogging: false,
      hasAuthentication: true,
      hasHTTPS: false,
      publicAccess: false,
    });

    await waitFor(() => {
      expect(apiFetchJSONMock).toHaveBeenCalled();
    });

    expect(screen.queryByText(/Security score:/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('renders for active exposure states', async () => {
    const pendingStatus = deferred<any>();
    apiFetchJSONMock.mockReturnValue(pendingStatus.promise);

    const { SecurityWarning } = await import('../SecurityWarning');
    render(() => <SecurityWarning />);

    pendingStatus.resolve({
      apiTokenConfigured: false,
      credentialsEncrypted: true,
      exportProtected: true,
      hasAuditLogging: false,
      hasAuthentication: false,
      hasHTTPS: false,
      publicAccess: true,
    });

    await waitFor(() => {
      expect(screen.getByText(/Security score:/i)).toBeInTheDocument();
    });

    expect(
      screen.getByText(/public network access detected/i),
    ).toBeInTheDocument();
    const banner = screen.getByRole('status');
    expect(banner).not.toHaveClass('fixed');
    expect(screen.getByRole('link', { name: 'Learn More' })).toHaveAttribute(
      'href',
      '/docs/SECURITY.md',
    );
  });
});
