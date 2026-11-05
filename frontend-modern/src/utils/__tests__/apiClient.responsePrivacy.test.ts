import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON, clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';

// Keep the production logger connected: a logger mock would miss a payload
// copied into console.error, including a SyntaxError's engine-provided snippet.
const privateMarker = 'SYNTHETIC_PRIVATE_RESPONSE_NOT_FOR_DIAGNOSTICS';
const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  window.sessionStorage.clear();
  clearAuth();
  setApiToken('synthetic-private-token');
  setOrgID('synthetic-private-org');
  document.cookie = 'pulse_csrf=synthetic-private-csrf; Path=/';
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  vi.spyOn(console, 'error').mockImplementation(() => undefined);
  vi.spyOn(console, 'warn').mockImplementation(() => undefined);
  vi.spyOn(console, 'log').mockImplementation(() => undefined);
});

afterEach(() => {
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  document.cookie = 'pulse_org_id=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const expectNoDiagnostics = () => {
  expect(console.error).not.toHaveBeenCalled();
  expect(console.warn).not.toHaveBeenCalled();
  expect(console.log).not.toHaveBeenCalled();
};

describe('successful JSON response privacy', () => {
  it.each([
    { kind: 'truncated JSON', body: `{"private_data":"${privateMarker}","partial":` },
    { kind: 'malformed JSON', body: `{"${privateMarker}": invalid}` },
    { kind: 'HTML from an upstream', body: `<html><pre>${privateMarker}</pre></html>` },
    { kind: 'plain text', body: privateMarker },
  ])('rejects $kind without copying body or request context into diagnostics', async ({ body }) => {
    fetchMock.mockResolvedValueOnce(
      new Response(body, { status: 200, headers: { 'Content-Type': 'application/json' } }),
    );

    const error = await apiFetchJSON(`/api/private/${privateMarker}?filter=${privateMarker}`, {
      method: 'PUT',
      body: JSON.stringify({ private_input: privateMarker }),
      expectedOrgID: 'synthetic-private-org',
      retry: false,
    }).catch((caught: unknown) => caught);

    expect(error).toBeInstanceOf(Error);
    expect((error as Error).message).toBe('Invalid JSON response from server');
    expect((error as Error).cause).toBeUndefined();
    expect(JSON.stringify(error)).not.toContain(privateMarker);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][1]).toMatchObject({
      method: 'PUT',
      credentials: 'include',
      headers: {
        'X-API-Token': 'synthetic-private-token',
        'X-Pulse-Org-ID': 'synthetic-private-org',
        'X-CSRF-Token': 'synthetic-private-csrf',
      },
    });
    // Exact console arguments reject raw body, URL, headers, parser error,
    // response objects and payload previews, not just this one marker.
    expect(vi.mocked(console.error).mock.calls).toEqual([
      ['%s', '[ERROR] Failed to parse JSON response', ''],
    ]);
    expect(console.warn).not.toHaveBeenCalled();
    expect(console.log).not.toHaveBeenCalled();
  });

  it.each([
    { body: '', expected: null },
    { body: 'null', expected: null },
    { body: 'false', expected: false },
    { body: '0', expected: 0 },
    { body: JSON.stringify(privateMarker), expected: privateMarker },
    { body: JSON.stringify([privateMarker]), expected: [privateMarker] },
    {
      body: JSON.stringify({ private_data: privateMarker }),
      expected: { private_data: privateMarker },
    },
  ])(
    'preserves valid or empty response semantics without logging: $body',
    async ({ body, expected }) => {
      fetchMock.mockResolvedValueOnce(new Response(body));
      expect(await apiFetchJSON('/api/private')).toEqual(expected);
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expectNoDiagnostics();
    },
  );

  it('keeps a definite API rejection and its status separate from a JSON parse failure', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: 'Saved policy rejected', code: 'invalid_policy' }), {
        status: 422,
      }),
    );
    await expect(apiFetchJSON('/api/private', { retry: false })).rejects.toMatchObject({
      message: 'Saved policy rejected',
      status: 422,
      code: 'invalid_policy',
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expectNoDiagnostics();
  });

  it('does not misclassify or log a response-body read failure as malformed JSON', async () => {
    const readError = new Error('Synthetic interrupted response body');
    const response = new Response();
    vi.spyOn(response, 'text').mockRejectedValueOnce(readError);
    fetchMock.mockResolvedValueOnce(response);
    await expect(apiFetchJSON('/api/private')).rejects.toBe(readError);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expectNoDiagnostics();
  });
});
