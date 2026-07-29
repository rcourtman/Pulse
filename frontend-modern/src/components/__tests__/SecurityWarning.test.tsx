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

    expect(screen.getByText(/public network access detected/i)).toBeInTheDocument();
    const banner = screen.getByRole('status');
    expect(banner).not.toHaveClass('fixed');
    expect(screen.getByRole('link', { name: 'Learn More' })).toHaveAttribute(
      'href',
      '/docs/SECURITY.md',
    );
  });

  it('Issue1650 renders nothing for a status payload truncated below privileged', async () => {
    const pendingStatus = deferred<any>();
    apiFetchJSONMock.mockReturnValue(pendingStatus.promise);

    const { SecurityWarning } = await import('../SecurityWarning');
    render(() => <SecurityWarning />);

    // What a kiosk token (monitoring:read only) actually receives: identity
    // fields, and none of the posture fields the score is computed from.
    pendingStatus.resolve({
      detailLevel: 'authenticated',
      hasAuthentication: true,
      requiresAuth: true,
      tokenScopes: ['monitoring:read'],
    });

    await waitFor(() => {
      expect(apiFetchJSONMock).toHaveBeenCalled();
    });

    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    expect(screen.queryByText(/Security score:/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Enable Security/i })).not.toBeInTheDocument();
  });

  it('Issue1650 still warns a privileged payload that genuinely lacks export protection', async () => {
    const pendingStatus = deferred<any>();
    apiFetchJSONMock.mockReturnValue(pendingStatus.promise);

    const { SecurityWarning } = await import('../SecurityWarning');
    render(() => <SecurityWarning />);

    pendingStatus.resolve({
      detailLevel: 'privileged',
      apiTokenConfigured: false,
      credentialsEncrypted: true,
      exportProtected: false,
      hasAuditLogging: false,
      hasAuthentication: true,
      hasHTTPS: true,
      publicAccess: false,
    });

    await waitFor(() => {
      expect(screen.getByText(/Security score:/i)).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: /Enable Security/i })).toBeInTheDocument();
  });
});
