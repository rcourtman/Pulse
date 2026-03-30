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

  it('renders after the async security status resolves to a low score', async () => {
    const pendingStatus = deferred<any>();
    apiFetchJSONMock.mockReturnValue(pendingStatus.promise);

    const { SecurityWarning } = await import('../SecurityWarning');
    render(() => <SecurityWarning />);

    expect(screen.queryByText(/Security score:/i)).not.toBeInTheDocument();

    pendingStatus.resolve({
      apiTokenConfigured: false,
      credentialsEncrypted: true,
      exportProtected: false,
      hasAuditLogging: false,
      hasAuthentication: true,
      hasHTTPS: false,
      publicAccess: false,
    });

    await waitFor(() => {
      expect(screen.getByText(/Security score:/i)).toBeInTheDocument();
    });

    expect(
      screen.getByText(
        'Authentication is enabled, but this Pulse instance is still missing HTTPS, an API token, and protected exports.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/accessible without authentication/i),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Learn More' })).toHaveAttribute(
      'href',
      '/docs/SECURITY.md',
    );
  });
});
