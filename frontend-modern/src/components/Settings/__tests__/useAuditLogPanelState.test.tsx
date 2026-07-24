import { renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { apiErrorFromResponse, apiFetch } from '@/utils/apiClient';
import { useAuditLogPanelState } from '../useAuditLogPanelState';

const navigate = vi.fn();

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ pathname: '/settings/security', search: '' }),
  useNavigate: () => navigate,
}));

vi.mock('@/stores/license', () => ({
  getRuntimeCapabilityBlock: () => undefined,
  hasFeature: (feature: string) => feature === 'audit_logging',
  loadRuntimeCapabilities: vi.fn().mockResolvedValue(undefined),
  runtimeCapabilitiesLoaded: () => true,
}));

vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: () => '/upgrade',
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => false,
}));

vi.mock('@/utils/upgradeNavigation', () => ({
  resolveUpgradeDestination: () => '/upgrade',
}));

vi.mock('@/utils/toast', () => ({
  showSuccess: vi.fn(),
  showToast: vi.fn(),
  showWarning: vi.fn(),
}));

vi.mock('@/utils/apiClient', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/utils/apiClient')>();
  return {
    ...actual,
    apiFetch: vi.fn(),
    apiErrorFromResponse: vi.fn(),
  };
});

type Deferred<T> = {
  promise: Promise<T>;
  resolve: (value: T) => void;
};

const deferred = <T,>(): Deferred<T> => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => {
    resolve = next;
  });
  return { promise, resolve };
};

const auditResponse = (ids: string[], total = ids.length): Response =>
  new Response(
    JSON.stringify({
      events: ids.map((id) => ({
        id,
        timestamp: '2026-07-24T09:00:00Z',
        event: 'login',
        user: 'operator',
        ip: '127.0.0.1',
        path: '/api/auth',
        success: true,
        details: id,
      })),
      total,
      persistentLogging: true,
    }),
    { status: 200, headers: { 'Content-Type': 'application/json' } },
  );

describe('useAuditLogPanelState audit request lifecycle', () => {
  beforeEach(() => {
    window.localStorage.clear();
    navigate.mockReset();
    vi.mocked(apiFetch).mockReset();
    vi.mocked(apiErrorFromResponse).mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('keeps the newest response when an older request resolves last', async () => {
    const first = deferred<Response>();
    const second = deferred<Response>();
    vi.mocked(apiFetch)
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);

    const { result, cleanup } = renderHook(() => useAuditLogPanelState());
    await waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(1));

    result.refresh();
    await waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2));
    second.resolve(auditResponse(['newest']));
    await waitFor(() => expect(result.events().map((event) => event.id)).toEqual(['newest']));

    first.resolve(auditResponse(['stale']));
    await Promise.resolve();
    expect(result.events().map((event) => event.id)).toEqual(['newest']);
    expect(result.loading()).toBe(false);
    cleanup();
  });

  it('clears prior rows when a refresh fails', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(auditResponse(['existing']))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: 'audit_store_busy' }), { status: 503 }),
      );
    vi.mocked(apiErrorFromResponse).mockResolvedValue(
      Object.assign(new Error('Audit log storage is temporarily busy.'), {
        code: 'audit_store_busy',
      }),
    );

    const { result, cleanup } = renderHook(() => useAuditLogPanelState());
    await waitFor(() => expect(result.events()).toHaveLength(1));
    result.refresh();
    await waitFor(() => expect(result.error()).toContain('storage is busy'));

    expect(result.events()).toEqual([]);
    expect(result.totalEvents()).toBe(0);
    expect(result.isPersistent()).toBe(false);
    cleanup();
  });

  it('resets and refetches atomically when page size changes', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(
        auditResponse(
          Array.from({ length: 100 }, (_, index) => `old-${index}`),
          250,
        ),
      )
      .mockResolvedValueOnce(
        auditResponse(
          Array.from({ length: 25 }, (_, index) => `new-${index}`),
          250,
        ),
      );

    const { result, cleanup } = renderHook(() => useAuditLogPanelState());
    await waitFor(() => expect(result.events()).toHaveLength(100));
    result.setPageSize(25);
    await waitFor(() => expect(result.events()).toHaveLength(25));

    const lastURL = vi.mocked(apiFetch).mock.calls.at(-1)?.[0];
    expect(String(lastURL)).toContain('limit=25');
    expect(String(lastURL)).toContain('offset=0');
    expect(result.pageSize()).toBe(25);
    expect(result.pageNumber()).toBe(1);
    expect(result.pageRangeText()).toBe('Showing 1-25 of 250');
    cleanup();
  });

  it('rejects null event collections instead of masking the API contract violation', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ events: null, total: 0, persistentLogging: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

    const { result, cleanup } = renderHook(() => useAuditLogPanelState());
    await waitFor(() => expect(result.error()).toBe('Audit log returned an invalid response'));
    expect(result.events()).toEqual([]);
    expect(result.isPersistent()).toBe(false);
    cleanup();
  });
});
