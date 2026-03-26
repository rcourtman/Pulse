import { describe, expect, it } from 'vitest';
import { apiErrorFromResponse } from '@/utils/apiClient';

describe('apiClient structured error extraction', () => {
  it('prefers structured JSON error messages over fallback copy', async () => {
    const error = await apiErrorFromResponse(
      new Response(JSON.stringify({ error: 'Canonical API error' }), { status: 400 }),
      'Fallback message',
    );

    expect(error.message).toBe('Canonical API error');
    expect(error.status).toBe(400);
  });

  it('falls back to plain text when the response is not JSON', async () => {
    const error = await apiErrorFromResponse(
      new Response('temporary failure', { status: 500 }),
      'Fallback message',
    );

    expect(error.message).toBe('temporary failure');
    expect(error.status).toBe(500);
  });
});
